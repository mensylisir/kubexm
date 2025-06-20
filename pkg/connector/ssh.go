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
	client        *ssh.Client
	bastionClient *ssh.Client // Client for the bastion host, if used
	sftpClient    *sftp.Client
	connCfg       ConnectionCfg // Stores the config used for the current connection
	cachedOS      *OS
	isConnected   bool
	pool          *ConnectionPool // Connection pool instance
	isFromPool    bool          // True if the current client is from the pool
}

// NewSSHConnector creates a new SSHConnector, optionally with a connection pool.
func NewSSHConnector(pool *ConnectionPool) *SSHConnector {
	return &SSHConnector{
		pool: pool,
	}
}

// Connect establishes an SSH connection to the host specified in cfg.
// It may use a connection pool if configured and applicable.
func (s *SSHConnector) Connect(ctx context.Context, cfg ConnectionCfg) error {
	s.connCfg = cfg // Store for potential use in Close (Put to pool)
	s.isFromPool = false // Reset for this connection attempt

	// Try pool first if available and connection is poolable (e.g., no bastion for now)
	if s.pool != nil && cfg.Bastion == nil {
		// Attempt to get a connection from the pool
		pooledClient, err := s.pool.Get(ctx, cfg)
		if err == nil && pooledClient != nil {
			s.client = pooledClient
			s.isFromPool = true
			s.isConnected = true

			// Perform a basic test to ensure the client from the pool is usable
			session, testErr := s.client.NewSession()
			if testErr != nil {
				// Pooled connection is bad, discard it properly
				s.pool.CloseConnection(s.connCfg, s.client) // CloseConnection handles numActive
				s.client = nil
				s.isFromPool = false
				s.isConnected = false
				// Log this event and fall through to direct dial
				// Consider using a logger if available from context or connector options
				fmt.Fprintf(os.Stderr, "SSHConnector: Pooled connection for %s failed health check, falling back to direct dial: %v\n", cfg.Host, testErr)
			} else {
				session.Close()
				// fmt.Fprintf(os.Stdout, "SSHConnector: Reused connection from pool for %s\n", cfg.Host)
				return nil // Successfully connected using a pooled connection
			}
		} else if err != nil {
			// Log error from pool.Get but still fall through to direct dial
			fmt.Fprintf(os.Stderr, "SSHConnector: Failed to get connection from pool for %s: %v. Falling back to direct dial.\n", cfg.Host, err)
		}
	}

	// Direct Dial Logic (Fallback or Default if pool not used/failed)
	if !s.isFromPool {
		// fmt.Fprintf(os.Stdout, "SSHConnector: Attempting direct dial for %s (pool not used or fallback)\n", cfg.Host)
		client, bastionClient, err := dialSSH(ctx, cfg, cfg.Timeout) // Use cfg.Timeout for direct dials
		if err != nil {
			return err // dialSSH already wraps errors in ConnectionError where appropriate
		}
		s.client = client
		s.bastionClient = bastionClient // Store bastion client if one was used
		// Test connection by opening a session (dialSSH does not do this final test)
		session, testErr := s.client.NewSession()
		if testErr != nil {
			if s.client != nil {
				s.client.Close()
			}
			if s.bastionClient != nil {
				s.bastionClient.Close()
			}
			return &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to create test session after direct dial: %w", testErr)}
		}
		session.Close()
		s.isConnected = true
		// s.isFromPool remains false
	}
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
	var sshClientErr error

	if s.sftpClient != nil {
		if err := s.sftpClient.Close(); err != nil {
			// Log SFTP close error but don't let it override SSH client error
			fmt.Fprintf(os.Stderr, "SSHConnector: Failed to close SFTP client for %s: %v\n", s.connCfg.Host, err)
		}
		s.sftpClient = nil
	}

	if s.client != nil {
		if s.isFromPool && s.pool != nil {
			// Return to pool, assume healthy for now. Pool's Get will re-verify.
			// connCfg should be the one used to Get this client.
			s.pool.Put(s.connCfg, s.client, true)
			// fmt.Fprintf(os.Stdout, "SSHConnector: Returned connection for %s to pool\n", s.connCfg.Host)
		} else {
			// Not from pool, or pool is nil, so directly close it.
			// fmt.Fprintf(os.Stdout, "SSHConnector: Closing direct-dialed connection for %s\n", s.connCfg.Host)
			sshClientErr = s.client.Close()
		}
		s.client = nil
	}
	s.isFromPool = false // Reset pool status

	// TODO: Handle bastion client closure if it was stored and managed by this connector instance
	// For now, bastion client is closed during Connect if subsequent steps fail, or not stored.
	// If it was a direct dial with bastion, we need to close it.
	if !s.isFromPool && s.bastionClient != nil {
		// Log error from bastion close but prioritize main client close error
		if berr := s.bastionClient.Close(); berr != nil {
			fmt.Fprintf(os.Stderr, "SSHConnector: Failed to close direct-dialed bastion client for %s: %v\n", s.connCfg.Host, berr)
		}
		s.bastionClient = nil
	}

	return sshClientErr // Only return error from direct client.Close()
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

// dialSSHFunc defines the signature for the SSH dialing function, allowing it to be mocked.
type dialSSHFunc func(ctx context.Context, cfg ConnectionCfg, effectiveConnectTimeout time.Duration) (*ssh.Client, *ssh.Client, error)

// actualDialSSH holds the real implementation for dialing SSH.
var actualDialSSH dialSSHFunc = func(ctx context.Context, cfg ConnectionCfg, effectiveConnectTimeout time.Duration) (*ssh.Client, *ssh.Client, error) {
	var authMethods []ssh.AuthMethod

	if cfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Password))
	}

	if len(cfg.PrivateKey) > 0 {
		signer, err := ssh.ParsePrivateKey(cfg.PrivateKey)
		if err != nil {
			return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to parse private key: %w", err)}
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else if cfg.PrivateKeyPath != "" {
		key, err := os.ReadFile(cfg.PrivateKeyPath)
		if err != nil {
			return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to read private key file %s: %w", cfg.PrivateKeyPath, err)}
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to parse private key from file %s: %w", cfg.PrivateKeyPath, err)}
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if len(authMethods) == 0 {
		return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("no authentication method provided (password or private key)")}
	}

	sshConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Make this configurable
		Timeout:         effectiveConnectTimeout,     // Use the passed-in timeout
	}

	var client *ssh.Client
	var bastionSshClient *ssh.Client // Explicitly separate bastion client variable
	var err error

	dialAddr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	if cfg.Bastion != nil {
		bastionAuthMethods := []ssh.AuthMethod{}
		if cfg.Bastion.Password != "" {
			bastionAuthMethods = append(bastionAuthMethods, ssh.Password(cfg.Bastion.Password))
		}
		if len(cfg.Bastion.PrivateKey) > 0 {
			signer, err := ssh.ParsePrivateKey(cfg.Bastion.PrivateKey)
			if err != nil {
				return nil, nil, &ConnectionError{Host: cfg.Bastion.Host, Err: fmt.Errorf("failed to parse bastion private key: %w", err)}
			}
			bastionAuthMethods = append(bastionAuthMethods, ssh.PublicKeys(signer))
		} else if cfg.Bastion.PrivateKeyPath != "" {
			key, err := os.ReadFile(cfg.Bastion.PrivateKeyPath)
			if err != nil {
				return nil, nil, &ConnectionError{Host: cfg.Bastion.Host, Err: fmt.Errorf("failed to read bastion private key file %s: %w", cfg.Bastion.PrivateKeyPath, err)}
			}
			signer, err := ssh.ParsePrivateKey(key)
			if err != nil {
				return nil, nil, &ConnectionError{Host: cfg.Bastion.Host, Err: fmt.Errorf("failed to parse bastion private key from file %s: %w", cfg.Bastion.PrivateKeyPath, err)}
			}
			bastionAuthMethods = append(bastionAuthMethods, ssh.PublicKeys(signer))
		}

		if len(bastionAuthMethods) == 0 {
			return nil, nil, &ConnectionError{Host: cfg.Bastion.Host, Err: fmt.Errorf("no authentication method provided for bastion (password or private key)")}
		}

		bastionConfig := &ssh.ClientConfig{
			User:            cfg.Bastion.User,
			Auth:            bastionAuthMethods,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Make this configurable
			Timeout:         cfg.Bastion.Timeout,         // Bastion has its own timeout in its Cfg
		}

		bastionDialAddr := net.JoinHostPort(cfg.Bastion.Host, strconv.Itoa(cfg.Bastion.Port))
		bastionSshClient, err = ssh.Dial("tcp", bastionDialAddr, bastionConfig)
		if err != nil {
			return nil, nil, &ConnectionError{Host: cfg.Bastion.Host, Err: fmt.Errorf("failed to dial bastion: %w", err)}
		}

		conn, err := bastionSshClient.Dial("tcp", dialAddr)
		if err != nil {
			bastionSshClient.Close() // Close bastion if dialing target fails
			return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to dial target host via bastion: %w", err)}
		}

		ncc, chans, reqs, err := ssh.NewClientConn(conn, dialAddr, sshConfig)
		if err != nil {
			bastionSshClient.Close() // Close bastion if NewClientConn to target fails
			return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to create new client connection via bastion: %w", err)}
		}
		client = ssh.NewClient(ncc, chans, reqs)
	} else {
		client, err = ssh.Dial("tcp", dialAddr, sshConfig)
		if err != nil {
			return nil, nil, &ConnectionError{Host: cfg.Host, Err: err}
		}
	}
	// The final test session is done by the caller (Connect or Pool's Get)
	return client, bastionSshClient, nil
}

// G_dialSSHOverride allows overriding the dialSSH behavior for testing.
var G_dialSSHOverride dialSSHFunc

// SetDialSSHOverrideForTesting replaces the actual SSH dialer with a mock and returns a cleanup function.
func SetDialSSHOverrideForTesting(fn dialSSHFunc) (cleanupFunc func()) {
	G_dialSSHOverride = fn
	return func() {
		G_dialSSHOverride = nil
	}
}

// dialSSH is a wrapper that either calls the override or the actual implementation.
// This is the function that SSHConnector.Connect and ConnectionPool.Get should call.
func dialSSH(ctx context.Context, cfg ConnectionCfg, effectiveConnectTimeout time.Duration) (*ssh.Client, *ssh.Client, error) {
	if G_dialSSHOverride != nil {
		return G_dialSSHOverride(ctx, cfg, effectiveConnectTimeout)
	}
	return actualDialSSH(ctx, cfg, effectiveConnectTimeout)
}
