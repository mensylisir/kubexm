package connector

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"net"
	"strings" // Added import
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh" // SSH import
)

// Shared mocking infrastructure for connector tests

var (
	actualDialCount      int
	actualDialCountMutex sync.Mutex
)

// SetTestDialerOverride allows overriding the package-level currentDialer function for testing.
// currentDialer should be a package variable in pool.go (or another central place) like:
// var currentDialer dialSSHFunc = dialSSH (where dialSSH is the real implementation)
// This helper needs access to that currentDialer.
func SetTestDialerOverride(overrideFn dialSSHFunc) func() {
	original := currentDialer // Assumes currentDialer is accessible in this package
	currentDialer = overrideFn
	return func() {
		currentDialer = original
	}
}

func resetActualDialCount() {
	actualDialCountMutex.Lock()
	defer actualDialCountMutex.Unlock()
	actualDialCount = 0
}

func incrementActualDialCount() {
	actualDialCountMutex.Lock()
	defer actualDialCountMutex.Unlock()
	actualDialCount++
}

func getActualDialCount() int {
	actualDialCountMutex.Lock()
	defer actualDialCountMutex.Unlock()
	return actualDialCount
}

// createMockSSHServer sets up an in-memory SSH server for testing.
// It returns the server's address and a cleanup function to close the listener.
func createMockSSHServer(t *testing.T) (string, func()) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to listen on a port: %v", err)
	}

	privateKey, err := generateTestKey()
	if err != nil {
		t.Fatalf("Failed to generate server key: %v", err)
	}

	config := &ssh.ServerConfig{
		NoClientAuth: true, // For simplicity in tests, don't require client auth
	}
	config.AddHostKey(privateKey)

	go func() {
		for { // Loop to accept multiple connections
			conn, err := listener.Accept()
			if err != nil {
				if ne, ok := err.(net.Error); ok && ne.Temporary() {
					time.Sleep(5 * time.Millisecond)
					continue
				}
				return // Listener closed or other fatal error
			}

			go func(c net.Conn) { // Handle each connection in its own goroutine
				defer c.Close()
				sconn, chans, reqs, handshakeErr := ssh.NewServerConn(c, config)
				if handshakeErr != nil {
					return // Handshake failed for this client
				}

				go ssh.DiscardRequests(reqs)

				var wg sync.WaitGroup
				wg.Add(1)
				go func() {
					defer wg.Done()
					for newChannel := range chans {
						if newChannel.ChannelType() == "session" {
							channel, channelRequests, acceptErr := newChannel.Accept()
							if acceptErr != nil {
								// t.Logf("mock server: could not accept channel type %s: %v", newChannel.ChannelType(), acceptErr)
								continue
							}
							// Handle exec requests on the session channel
							go func(in <-chan *ssh.Request, ch ssh.Channel) {
								for req := range in {
									ok := false
									switch req.Type {
									case "exec":
										var payload struct{ Command string }
										ssh.Unmarshal(req.Payload, &payload)
										// t.Logf("mock server: exec request for command: %s", payload.Command)

										if strings.HasPrefix(payload.Command, "echo ") {
											echoContent := strings.TrimPrefix(payload.Command, "echo ")
											ch.Write([]byte(echoContent + "\n"))
										}
										// Simulate successful execution for any exec command
										ok = true
										// Send exit-status 0 for success
										statusPayload := struct{ Status uint32 }{0}
										// Log error if sending exit-status fails, but don't stop the test for it
										if _, errSendStatus := ch.SendRequest("exit-status", false, ssh.Marshal(&statusPayload)); errSendStatus != nil {
											// t.Logf("mock server: failed to send exit-status for exec %s: %v", payload.Command, errSendStatus)
										}
										// Do NOT close the channel here. The client (session) should close it.
										// The client's session.Close() will send an EOF, then a "close" channel message.
										// The server's mux will then see the channel as closed.
									case "shell", "pty-req", "subsystem":
										// Deny these for simplicity
										if req.WantReply {
											req.Reply(false, nil)
										}
									default:
										// t.Logf("mock server: discarding/ignoring session request type %s", req.Type)
										if req.WantReply {
											req.Reply(true, nil) // Default positive reply if unsure
										}
									}
									// If request required a reply and we haven't sent one (e.g. for non-exec)
									if req.WantReply && !ok { // ok would be true if handled above
										req.Reply(ok, nil)
									}
								}
							}(channelRequests, channel)
						} else {
							// t.Logf("mock server: rejecting unknown channel type: %s", newChannel.ChannelType())
							if rejectErr := newChannel.Reject(ssh.UnknownChannelType, "unknown channel type"); rejectErr != nil {
								// t.Logf("mock server: failed to reject channel type %s: %v", newChannel.ChannelType(), rejectErr)
							}
						}
					}
					// t.Logf("mock server: channel handling loop exited for client %s", c.RemoteAddr())
				}()
				sconn.Wait()
				wg.Wait()
				// t.Logf("mock server: %s all server goroutines finished for client %s", listener.Addr().String(), c.RemoteAddr())
			}(conn)
		}
	}()

	cleanup := func() {
		listener.Close()
	}
	return listener.Addr().String(), cleanup
}

// generateTestKey is a helper to create an RSA private key for the mock server.
func generateTestKey() (ssh.Signer, error) {
	privateRSAKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate RSA private key: %w", err)
	}
	signer, err := ssh.NewSignerFromKey(privateRSAKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create signer from RSA private key: %w", err)
	}
	return signer, nil
}

// newMockDialer creates a dialer that connects to our in-memory SSH server.
// It uses the incrementActualDialCount helper.
func newMockDialer(serverAddr string) dialSSHFunc {
	return func(ctx context.Context, cfg ConnectionCfg, connectTimeout time.Duration) (*ssh.Client, *ssh.Client, error) {
		incrementActualDialCount()
		// Use the connectTimeout from parameters for the net.Dialer
		dialer := net.Dialer{Timeout: connectTimeout}
		conn, err := dialer.DialContext(ctx, "tcp", serverAddr)
		if err != nil {
			return nil, nil, err
		}

		// Use cfg.User for ClientConfig, and cfg.HostKeyCallback if provided, otherwise InsecureIgnoreHostKey
		hostKeyCallback := cfg.HostKeyCallback
		if hostKeyCallback == nil {
			hostKeyCallback = ssh.InsecureIgnoreHostKey()
		}

		sshConfig := &ssh.ClientConfig{
			User:            cfg.User,
			Auth:            []ssh.AuthMethod{}, // Mock dialer doesn't need real auth for mock server
			HostKeyCallback: hostKeyCallback,
			Timeout:         connectTimeout, // This timeout is for the SSH handshake itself
		}
		if cfg.Password != "" { // Allow password for completeness, though mock server doesn't check
			sshConfig.Auth = append(sshConfig.Auth, ssh.Password(cfg.Password))
		}


		c, chans, reqs, err := ssh.NewClientConn(conn, serverAddr, sshConfig)
		if err != nil {
			return nil, nil, err
		}

		client := ssh.NewClient(c, chans, reqs)
		return client, nil, nil // No bastion client in this simplified mock setup
	}
}
