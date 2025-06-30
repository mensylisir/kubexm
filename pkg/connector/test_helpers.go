package connector

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt" // Needed for fmt.Errorf
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	actualDialCount      int
	actualDialCountMutex sync.Mutex
)

type ExecHandlerFunc func(command string, ch ssh.Channel, t *testing.T)

type MockServer struct {
	listener    net.Listener
	config      *ssh.ServerConfig
	t           *testing.T
	ExecHandler ExecHandlerFunc
	serverDone chan struct{} // Closed when the acceptLoop exits
	wg         sync.WaitGroup
}

func SetTestDialerOverride(overrideFn dialSSHFunc) func() {
	original := currentDialer
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

func NewMockServer(t *testing.T, execHandler ExecHandlerFunc) (server *MockServer, addr string, cleanup func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("NewMockServer: Failed to listen: %v", err)
	}

	privateKey, err := generateTestKey()
	if err != nil {
		listener.Close()
		t.Fatalf("NewMockServer: Failed to generate key: %v", err)
	}

	serverConfig := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	serverConfig.AddHostKey(privateKey)

	ms := &MockServer{
		listener:    listener,
		config:      serverConfig,
		t:           t,
		ExecHandler: execHandler,
		serverDone:  make(chan struct{}),
	}

	ms.wg.Add(1) // For the acceptLoop itself
	go ms.acceptLoop()

	cleanupFunc := func() {
		ms.listener.Close()
		<-ms.serverDone      // Wait for acceptLoop to finish
		ms.wg.Wait()         // Wait for all handleConnection goroutines
	}
	return ms, listener.Addr().String(), cleanupFunc
}

func (ms *MockServer) acceptLoop() {
	defer ms.wg.Done()
	defer close(ms.serverDone) // Signal that acceptLoop has exited
	for {
		conn, err := ms.listener.Accept()
		if err != nil {
			// Check if the error is due to the listener being closed.
			if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
				return // Normal shutdown
			}
			// ms.t.Logf("MockServer: Accept error: %v. Exiting acceptLoop.", err) // t.Logf can't be used in non-test goroutine
			return
		}
		ms.wg.Add(1) // Add for the handleConnection goroutine
		go ms.handleConnection(conn)
	}
}

func (ms *MockServer) handleConnection(c net.Conn) {
	defer ms.wg.Done() // Decrement counter when this connection handler exits
	defer c.Close()
	sconn, _, globalReqs, err := ssh.NewServerConn(c, ms.config) // chans removed
	if err != nil {
		// ms.t.Logf("MockServer: Handshake error with %s: %v", c.RemoteAddr(), err)
		return
	}
	defer sconn.Close() // Ensure SSH transport is closed

	go ssh.DiscardRequests(globalReqs) // Handle keepalives etc.

	// This simplified handler only deals with the connection lifecycle.
	// It does not process session channels or exec requests.
	// This is for testing basic connect/disconnect and pool keepalives.
	// For exec testing, the more complex handler (commented out below) is needed.

	// Wait for the SSH connection to terminate, or server shutdown.
	// Temporarily removing sconn.Context().Done() due to build issues.
	select {
	// case <-sconn.Context().Done(): // Connection closed by client or error
	// ms.t.Logf("MockServer: sconn context done for %s: %v", sconn.RemoteAddr(), sconn.Context().Err())
	case <-ms.serverDone: // Server is shutting down
		// ms.t.Logf("MockServer: serverDone signaled for %s, closing sconn.", sconn.RemoteAddr())
		// sconn.Close() already deferred
	}
	// sconn.Wait() // sconn.Context().Done() is usually preferred for select
}


/* // Original more complex handleConnection - keep for reference or future exec/sftp tests
func (ms *MockServer) handleConnection_Full(c net.Conn) {
	defer ms.wg.Done()
	defer c.Close()
	sconn, chans, globalReqs, err := ssh.NewServerConn(c, ms.config)
	if err != nil {
		return
	}
	defer sconn.Close()
	go ssh.DiscardRequests(globalReqs)

	var connChanWg sync.WaitGroup
	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			continue
		}
		connChanWg.Add(1)
		go func(ch ssh.Channel, reqsChan <-chan *ssh.Request) {
			defer connChanWg.Done()
			defer ch.Close()
			for req := range reqsChan {
				switch req.Type {
				case "exec":
					var payload struct{ Command string }
					ssh.Unmarshal(req.Payload, &payload)
					if req.WantReply {
						req.Reply(true, nil)
					}
					if ms.ExecHandler != nil {
						ms.ExecHandler(payload.Command, ch, ms.t)
					} else {
						if strings.HasPrefix(payload.Command, "echo ") {
							ch.Write([]byte(strings.TrimPrefix(payload.Command, "echo ") + "\n"))
						}
						ch.CloseWrite()
						statusPayload := struct{ Status uint32 }{0}
						ch.SendRequest("exit-status", false, ssh.Marshal(&statusPayload))
						ch.Close()
					}
					return
				case "subsystem":
					var payload struct{ Name string }
					ssh.Unmarshal(req.Payload, &payload)
					if payload.Name == "sftp" {
						if req.WantReply { req.Reply(true, nil) }
					} else {
						if req.WantReply { req.Reply(false, nil) }
					}
				case "shell", "pty-req":
					if req.WantReply { req.Reply(false, nil) }
				default:
					if req.WantReply { req.Reply(false, nil) }
				}
			}
		}(channel, requests)
	}
	connChanWg.Wait()
	sconn.Wait()
}
*/

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

func newMockDialer(serverAddr string) dialSSHFunc {
	return func(ctx context.Context, cfg ConnectionCfg, connectTimeout time.Duration) (*ssh.Client, *ssh.Client, error) {
		incrementActualDialCount()
		dialer := net.Dialer{Timeout: connectTimeout}
		conn, err := dialer.DialContext(ctx, "tcp", serverAddr)
		if err != nil {
			return nil, nil, err
		}

		hostKeyCallback := cfg.HostKeyCallback
		if hostKeyCallback == nil {
			hostKeyCallback = ssh.InsecureIgnoreHostKey()
		}

		sshConfig := &ssh.ClientConfig{
			User:            cfg.User,
			Auth:            []ssh.AuthMethod{},
			HostKeyCallback: hostKeyCallback,
			Timeout:         connectTimeout,
		}
		if cfg.Password != "" {
			sshConfig.Auth = append(sshConfig.Auth, ssh.Password(cfg.Password))
		}

		clientConn, chans, reqs, err := ssh.NewClientConn(conn, serverAddr, sshConfig)
		if err != nil {
			return nil, nil, err
		}

		client := ssh.NewClient(clientConn, chans, reqs)
		return client, nil, nil
	}
}
