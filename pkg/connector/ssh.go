package connector

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHConnector implements the Connector interface for SSH connections.
type SSHConnector struct {
	client      *ssh.Client
	sftpClient  *sftp.Client
	connCfg     ConnectionCfg
	cachedOS    *OS
	isConnected bool
}

// Connect establishes an SSH connection to the host specified in cfg.
func (s *SSHConnector) Connect(ctx context.Context, cfg ConnectionCfg) error {
	s.connCfg = cfg
	var authMethods []ssh.AuthMethod

	if cfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Password))
	}

	if len(cfg.PrivateKey) > 0 {
		signer, err := ssh.ParsePrivateKey(cfg.PrivateKey)
		if err != nil {
			return &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to parse private key: %w", err)}
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else if cfg.PrivateKeyPath != "" {
		key, err := os.ReadFile(cfg.PrivateKeyPath)
		if err != nil {
			return &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to read private key file %s: %w", cfg.PrivateKeyPath, err)}
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to parse private key from file %s: %w", cfg.PrivateKeyPath, err)}
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if len(authMethods) == 0 {
		return &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("no authentication method provided (password or private key)")}
	}

	sshConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Make this configurable
		Timeout:         cfg.Timeout,
	}

	var client *ssh.Client
	var err error

	if cfg.Bastion != nil {
		bastionAuthMethods := []ssh.AuthMethod{}
		if cfg.Bastion.Password != "" {
			bastionAuthMethods = append(bastionAuthMethods, ssh.Password(cfg.Bastion.Password))
		}
		if len(cfg.Bastion.PrivateKey) > 0 {
			signer, err := ssh.ParsePrivateKey(cfg.Bastion.PrivateKey)
			if err != nil {
				return &ConnectionError{Host: cfg.Bastion.Host, Err: fmt.Errorf("failed to parse bastion private key: %w", err)}
			}
			bastionAuthMethods = append(bastionAuthMethods, ssh.PublicKeys(signer))
		} else if cfg.Bastion.PrivateKeyPath != "" {
			key, err := os.ReadFile(cfg.Bastion.PrivateKeyPath)
			if err != nil {
				return &ConnectionError{Host: cfg.Bastion.Host, Err: fmt.Errorf("failed to read bastion private key file %s: %w", cfg.Bastion.PrivateKeyPath, err)}
			}
			signer, err := ssh.ParsePrivateKey(key)
			if err != nil {
				return &ConnectionError{Host: cfg.Bastion.Host, Err: fmt.Errorf("failed to parse bastion private key from file %s: %w", cfg.Bastion.PrivateKeyPath, err)}
			}
			bastionAuthMethods = append(bastionAuthMethods, ssh.PublicKeys(signer))
		}

		if len(bastionAuthMethods) == 0 {
			return &ConnectionError{Host: cfg.Bastion.Host, Err: fmt.Errorf("no authentication method provided for bastion (password or private key)")}
		}


		bastionConfig := &ssh.ClientConfig{
			User:            cfg.Bastion.User,
			Auth:            bastionAuthMethods,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Make this configurable
			Timeout:         cfg.Bastion.Timeout,
		}

		bastionClient, err := ssh.Dial("tcp", net.JoinHostPort(cfg.Bastion.Host, strconv.Itoa(cfg.Bastion.Port)), bastionConfig)
		if err != nil {
			return &ConnectionError{Host: cfg.Bastion.Host, Err: fmt.Errorf("failed to dial bastion: %w", err)}
		}

		conn, err := bastionClient.Dial("tcp", net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)))
		if err != nil {
			bastionClient.Close()
			return &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to dial target host via bastion: %w", err)}
		}

		ncc, chans, reqs, err := ssh.NewClientConn(conn, net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)), sshConfig)
		if err != nil {
			bastionClient.Close() // Close bastion client if NewClientConn fails
			return &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to create new client connection via bastion: %w", err)}
		}
		client = ssh.NewClient(ncc, chans, reqs)
		// Keep bastionClient to close it later in s.Close() or handle it if main client closes
		// For now, we are not storing bastionClient in SSHConnector, it will be closed if connection fails
		// or when the SSHConnector is closed (though not explicitly handled yet).
		// A more robust solution might involve storing bastionClient and closing it in SSHConnector.Close().
	} else {
		client, err = ssh.Dial("tcp", net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)), sshConfig)
		if err != nil {
			return &ConnectionError{Host: cfg.Host, Err: err}
		}
	}

	s.client = client

	// Test connection by opening a session
	session, err := s.client.NewSession()
	if err != nil {
		s.client.Close()
		return &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to create test session: %w", err)}
	}
	session.Close() // Close the test session immediately

	s.isConnected = true
	return nil
}

// IsConnected checks if the SSH client is connected.
func (s *SSHConnector) IsConnected() bool {
	if s.client == nil || !s.isConnected {
		return false
	}
	// Sending a keepalive or checking session status might be more robust,
	// but for now, we rely on the initial connection success and the Close method.
	// A simple check is to see if a new session can be opened.
	session, err := s.client.NewSession()
	if err != nil {
		s.isConnected = false // Mark as disconnected if new session fails
		return false
	}
	session.Close()
	return true
}

// Close closes the SSH and SFTP clients.
func (s *SSHConnector) Close() error {
	s.isConnected = false
	if s.sftpClient != nil {
		if err := s.sftpClient.Close(); err != nil {
			// Log SFTP close error but try to close SSH client anyway
			// fmt.Fprintf(os.Stderr, "Failed to close SFTP client: %v
", err)
		}
		s.sftpClient = nil
	}
	if s.client != nil {
		err := s.client.Close()
		s.client = nil
		return err
	}
	// TODO: Handle bastion client closure if it was stored
	return nil
}

// Exec executes a command on the remote host.
func (s *SSHConnector) Exec(ctx context.Context, cmd string, options *ExecOptions) (stdout, stderr []byte, err error) {
	if !s.IsConnected() {
		return nil, nil, &ConnectionError{Host: s.connCfg.Host, Err: fmt.Errorf("not connected")}
	}

	var session *ssh.Session
	var cancelFunc context.CancelFunc

	if options != nil && options.Timeout > 0 {
		var newCtx context.Context
		newCtx, cancelFunc = context.WithTimeout(ctx, options.Timeout)
		defer cancelFunc()
		ctx = newCtx // Use the new context with timeout
	}

	session, err = s.client.NewSession()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	if options != nil && len(options.Env) > 0 {
		for _, envVar := range options.Env {
			parts := strings.SplitN(envVar, "=", 2)
			if len(parts) == 2 {
				if err := session.Setenv(parts[0], parts[1]); err != nil {
					// Log or handle individual Setenv errors if necessary
				}
			}
		}
	}

	var finalCmd = cmd
	if options != nil && options.Sudo {
		finalCmd = "sudo -E -- " + cmd // Ensure environment variables are preserved with -E
	}

	var stdoutBuf, stderrBuf bytes.Buffer

	if options != nil && options.Stream != nil {
		session.Stdout = io.MultiWriter(&stdoutBuf, options.Stream)
		session.Stderr = io.MultiWriter(&stderrBuf, options.Stream)
	} else {
		session.Stdout = &stdoutBuf
		session.Stderr = &stderrBuf
	}


	errChan := make(chan error, 1)
	go func() {
		errChan <- session.Run(finalCmd)
	}()

	select {
	case err = <-errChan:
		// continue
	case <-ctx.Done():
		// This case handles timeouts or cancellations from the parent context.
		// We need to ensure the session is killed if the context is done.
		// Sending a signal or closing the session might be necessary.
		// For now, we rely on the defer session.Close() and the potential error from session.Run().
		// A more robust way would be to kill the process on the remote side.
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), fmt.Errorf("command execution context done: %w", ctx.Err())
	}


	stdout = stdoutBuf.Bytes()
	stderr = stderrBuf.Bytes()

	if err != nil {
		exitCode := -1
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		}

		// Retry logic
		if options != nil && options.Retries > 0 {
			for i := 0; i < options.Retries; i++ {
				if options.RetryDelay > 0 {
					time.Sleep(options.RetryDelay)
				}
				// Create a new session for retry
				retrySession, retryErr := s.client.NewSession()
				if retryErr != nil {
					return stdout, stderr, &CommandError{Cmd: cmd, ExitCode: exitCode, Stdout: string(stdout), Stderr: string(stderr), Underlying: fmt.Errorf("failed to create retry session: %w", retryErr)}
				}

				if options.Stream != nil {
					retrySession.Stdout = io.MultiWriter(&stdoutBuf, options.Stream) // Reset buffer or use new ones
					retrySession.Stderr = io.MultiWriter(&stderrBuf, options.Stream)
				} else {
					stdoutBuf.Reset()
					stderrBuf.Reset()
					retrySession.Stdout = &stdoutBuf
					retrySession.Stderr = &stderrBuf
				}

				if len(options.Env) > 0 { // Apply Env to retry session
					for _, envVar := range options.Env {
						parts := strings.SplitN(envVar, "=", 2)
						if len(parts) == 2 {
							retrySession.Setenv(parts[0], parts[1])
						}
					}
				}

				err = retrySession.Run(finalCmd) // Use finalCmd
				retrySession.Close()
				stdout = stdoutBuf.Bytes()
				stderr = stderrBuf.Bytes()
				if err == nil {
					break // Success on retry
				}
				if exitErr, ok := err.(*ssh.ExitError); ok {
					exitCode = exitErr.ExitStatus()
				}
			}
		}

		// If error still persists after retries
		if err != nil {
			return stdout, stderr, &CommandError{Cmd: cmd, ExitCode: exitCode, Stdout: string(stdout), Stderr: string(stderr), Underlying: err}
		}
	}
	return stdout, stderr, nil
}

// Helper to initialize SFTP client if not already done.
func (s *SSHConnector) ensureSftp() error {
	if s.sftpClient == nil {
		if !s.IsConnected() {
			return &ConnectionError{Host: s.connCfg.Host, Err: fmt.Errorf("not connected, cannot initialize SFTP")}
		}
		var err error
		s.sftpClient, err = sftp.NewClient(s.client)
		if err != nil {
			return fmt.Errorf("failed to create SFTP client: %w", err)
		}
	}
	return nil
}

// Copy copies a local file or directory to the remote host.
// For simplicity, this initial implementation only supports single file copy.
// Directory copy would require recursive logic.
func (s *SSHConnector) Copy(ctx context.Context, srcPath, dstPath string, options *FileTransferOptions) error {
	if err := s.ensureSftp(); err != nil {
		return err
	}

	// Timeout handling
	if options != nil && options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	// Read source file
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", srcPath, err)
	}
	defer srcFile.Close()

	// Create destination file
	// TODO: Handle sudo for dstPath creation if options.Sudo is true.
	// This might involve copying to a temp location and then using sudo mv.
	// For now, direct write.
	dstFile, err := s.sftpClient.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create remote file %s: %w", dstPath, err)
	}
	defer dstFile.Close()

	// Copy content
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy content to remote file %s: %w", dstPath, err)
	}

	// Apply permissions and ownership if specified
	if options != nil {
		if options.Permissions != "" {
			perm, err := strconv.ParseUint(options.Permissions, 8, 32)
			if err == nil {
				if err := s.sftpClient.Chmod(dstPath, os.FileMode(perm)); err != nil {
					// Log or return error for chmod failure
				}
			}
		}
		// Ownership (Owner, Group) via SFTP is tricky and often not directly supported
		// or requires root privileges on the SFTP server side.
		// For full sudo support, commands via Exec might be needed after upload.
		// Example: _, _, err := s.Exec(ctx, fmt.Sprintf("chown %s:%s %s", options.Owner, options.Group, dstPath), &ExecOptions{Sudo: true})
	}

	return nil
}

// CopyContent writes content to a remote file.
func (s *SSHConnector) CopyContent(ctx context.Context, content []byte, dstPath string, options *FileTransferOptions) error {
	if err := s.ensureSftp(); err != nil {
		return err
	}

	// Timeout handling
	if options != nil && options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	// TODO: Handle sudo for dstPath creation if options.Sudo is true.
	// This might involve copying to a temp location and then using sudo mv.
	// For now, direct write.
	dstFile, err := s.sftpClient.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create remote file %s for content: %w", dstPath, err)
	}
	defer dstFile.Close()

	_, err = dstFile.Write(content)
	if err != nil {
		return fmt.Errorf("failed to write content to remote file %s: %w", dstPath, err)
	}

	// Apply permissions and ownership
	if options != nil {
		if options.Permissions != "" {
			perm, err := strconv.ParseUint(options.Permissions, 8, 32)
			if err == nil {
				if err := s.sftpClient.Chmod(dstPath, os.FileMode(perm)); err != nil {
					// Log or return error
				}
			}
		}
		// Ownership similar to Copy method
	}
	return nil
}


// Fetch retrieves a file from the remote host to local.
// For simplicity, this initial implementation only supports single file fetch.
func (s *SSHConnector) Fetch(ctx context.Context, remotePath, localPath string) error {
	if err := s.ensureSftp(); err != nil {
		return err
	}

	// Open remote file
	remoteFile, err := s.sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file %s: %w", remotePath, err)
	}
	defer remoteFile.Close()

	// Create local file
	// Ensure local directory exists
	if err := os.MkdirAll(filepath.Dir(localPath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create local directory for %s: %w", localPath, err)
	}
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file %s: %w", localPath, err)
	}
	defer localFile.Close()

	// Copy content
	_, err = io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("failed to copy content from remote file %s to local %s: %w", remotePath, localPath, err)
	}
	return nil
}


// Stat gets information about a remote file or directory.
func (s *SSHConnector) Stat(ctx context.Context, path string) (*FileStat, error) {
	if err := s.ensureSftp(); err != nil {
		return nil, err
	}

	fi, err := s.sftpClient.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileStat{Name: filepath.Base(path), IsExist: false}, nil
		}
		return nil, fmt.Errorf("failed to stat remote path %s: %w", path, err)
	}

	// TODO: Get Owner/Group and SHA256 sum if needed.
	// Owner/Group might require exec "stat ..." command.
	// SHA256 sum would require reading the file or exec "sha256sum ..."
	return &FileStat{
		Name:    fi.Name(),
		Size:    fi.Size(),
		Mode:    fi.Mode(),
		ModTime: fi.ModTime(),
		IsDir:   fi.IsDir(),
		IsExist: true,
	}, nil
}

// LookPath searches for an executable in the remote host's PATH.
func (s *SSHConnector) LookPath(ctx context.Context, file string) (string, error) {
	// A common way to implement LookPath over SSH is to use `command -v` or `which`.
	// `command -v` is POSIX standard and generally preferred.
	cmd := fmt.Sprintf("command -v %s", file)
	stdout, stderr, err := s.Exec(ctx, cmd, nil) // No options for this internal command

	if err != nil {
		// If command execution itself failed (not just non-zero exit)
		if cmdErr, ok := err.(*CommandError); ok {
			// command -v returns 1 if not found, which is an expected "not found" scenario
			if cmdErr.ExitCode == 1 {
				return "", fmt.Errorf("executable %s not found in PATH (stderr: %s)", file, string(stderr))
			}
		}
		return "", fmt.Errorf("failed to execute 'command -v %s': %w (stdout: %s, stderr: %s)", file, err, string(stdout), string(stderr))
	}

	path := strings.TrimSpace(string(stdout))
	if path == "" {
		return "", fmt.Errorf("executable %s not found in PATH (stderr: %s)", file, string(stderr))
	}
	return path, nil
}

// GetOS retrieves operating system information.
func (s *SSHConnector) GetOS(ctx context.Context) (*OS, error) {
	if s.cachedOS != nil {
		return s.cachedOS, nil
	}

	// Try /etc/os-release first
	stdout, _, err := s.Exec(ctx, "cat /etc/os-release", nil)
	if err != nil {
		// Fallback or handle error if /etc/os-release is not available
		// For now, we'll return a partial error if this primary method fails.
		// A more robust solution might try other commands like lsb_release -a
		return nil, fmt.Errorf("failed to cat /etc/os-release: %w", err)
	}

	osInfo := &OS{}
	lines := strings.Split(string(stdout), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), "\"")
			switch key {
			case "ID":
				osInfo.ID = val
			case "VERSION_ID":
				osInfo.VersionID = val
			case "PRETTY_NAME": // Often includes codename
				if osInfo.Codename == "" && strings.Contains(val, "(") && strings.Contains(val, ")") {
					start := strings.Index(val, "(")
					end := strings.Index(val, ")")
					if start != -1 && end != -1 && start < end {
						osInfo.Codename = strings.ToLower(strings.TrimSpace(val[start+1 : end]))
					}
				}
			case "VERSION_CODENAME":
				osInfo.Codename = val
			}
		}
	}

	// Get Arch
	archStdout, _, err := s.Exec(ctx, "uname -m", nil)
	if err == nil {
		osInfo.Arch = strings.TrimSpace(string(archStdout))
	}

	// Get Kernel
	kernelStdout, _, err := s.Exec(ctx, "uname -r", nil)
	if err == nil {
		osInfo.Kernel = strings.TrimSpace(string(kernelStdout))
	}

	// TODO: Variant detection if necessary

	s.cachedOS = osInfo
	return s.cachedOS, nil
}

// Ensure SSHConnector implements Connector interface
var _ Connector = &SSHConnector{}
