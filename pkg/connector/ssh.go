package connector

import (
	"bytes"
	"context"
	// "crypto/sha256" // Not used directly here, checksumming is via remote command
	// "encoding/hex"  // Not used directly here
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
	bastionClient *ssh.Client
	sftpClient    *sftp.Client
	connCfg       ConnectionCfg
	cachedOS      *OS
	isConnected   bool
	pool          *ConnectionPool
	isFromPool    bool
}

// NewSSHConnector creates a new SSHConnector, optionally with a connection pool.
func NewSSHConnector(pool *ConnectionPool) *SSHConnector {
	return &SSHConnector{
		pool: pool,
	}
}

// Connect establishes an SSH connection to the host specified in cfg.
func (s *SSHConnector) Connect(ctx context.Context, cfg ConnectionCfg) error {
	s.connCfg = cfg
	s.isFromPool = false

	if s.pool != nil && cfg.BastionCfg == nil {
		pooledClient, err := s.pool.Get(ctx, cfg)
		if err == nil && pooledClient != nil {
			s.client = pooledClient
			s.isFromPool = true
			s.isConnected = true
			session, testErr := s.client.NewSession()
			if testErr != nil {
				s.pool.CloseConnection(s.connCfg, s.client)
				s.client = nil
				s.isFromPool = false
				s.isConnected = false
				fmt.Fprintf(os.Stderr, "SSHConnector: Pooled connection for %s failed health check, falling back to direct dial: %v\n", cfg.Host, testErr)
			} else {
				session.Close()
				return nil
			}
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "SSHConnector: Failed to get connection from pool for %s: %v. Falling back to direct dial.\n", cfg.Host, err)
		}
	}

	if !s.isFromPool {
		client, bastionClient, err := dialSSH(ctx, cfg, cfg.Timeout)
		if err != nil {
			return err
		}
		s.client = client
		s.bastionClient = bastionClient
		session, testErr := s.client.NewSession()
		if testErr != nil {
			if s.client != nil { s.client.Close() }
			if s.bastionClient != nil { s.bastionClient.Close() }
			return &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to create test session after direct dial: %w", testErr)}
		}
		session.Close()
		s.isConnected = true
	}
	return nil
}

// IsConnected checks if the SSH client is connected.
func (s *SSHConnector) IsConnected() bool {
	if s.client == nil || !s.isConnected { return false }
	session, err := s.client.NewSession()
	if err != nil {
		s.isConnected = false
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
			fmt.Fprintf(os.Stderr, "SSHConnector: Failed to close SFTP client for %s: %v\n", s.connCfg.Host, err)
		}
		s.sftpClient = nil
	}
	if s.client != nil {
		if s.isFromPool && s.pool != nil {
			s.pool.Put(s.connCfg, s.client, true)
		} else {
			sshClientErr = s.client.Close()
		}
		s.client = nil
	}
	s.isFromPool = false
	if !s.isFromPool && s.bastionClient != nil {
		if berr := s.bastionClient.Close(); berr != nil {
			fmt.Fprintf(os.Stderr, "SSHConnector: Failed to close direct-dialed bastion client for %s: %v\n", s.connCfg.Host, berr)
		}
		s.bastionClient = nil
	}
	return sshClientErr
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
		ctx = newCtx
	}
	session, err = s.client.NewSession()
	if err != nil { return nil, nil, fmt.Errorf("failed to create session: %w", err) }
	defer session.Close()
	if options != nil && len(options.Env) > 0 {
		for _, envVar := range options.Env {
			parts := strings.SplitN(envVar, "=", 2)
			if len(parts) == 2 {
				if err := session.Setenv(parts[0], parts[1]); err != nil { /* Log error */ }
			}
		}
	}
	finalCmd := cmd
	if options != nil && options.Sudo { finalCmd = "sudo -E -- " + cmd }
	var stdoutBuf, stderrBuf bytes.Buffer
	if options != nil && options.Stream != nil {
		session.Stdout = io.MultiWriter(&stdoutBuf, options.Stream)
		session.Stderr = io.MultiWriter(&stderrBuf, options.Stream)
	} else {
		session.Stdout = &stdoutBuf
		session.Stderr = &stderrBuf
	}
	errChan := make(chan error, 1)
	go func() { errChan <- session.Run(finalCmd) }()
	select {
	case err = <-errChan:
	case <-ctx.Done():
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), fmt.Errorf("command execution context done: %w", ctx.Err())
	}
	stdout = stdoutBuf.Bytes()
	stderr = stderrBuf.Bytes()
	if err != nil {
		exitCode := -1
		if exitErr, ok := err.(*ssh.ExitError); ok { exitCode = exitErr.ExitStatus() }
		if options != nil && options.Retries > 0 {
			for i := 0; i < options.Retries; i++ {
				if options.RetryDelay > 0 { time.Sleep(options.RetryDelay) }
				retrySession, retryErr := s.client.NewSession()
				if retryErr != nil {
					return stdout, stderr, &CommandError{Cmd: cmd, ExitCode: exitCode, Stdout: string(stdout), Stderr: string(stderr), Underlying: fmt.Errorf("failed to create retry session: %w", retryErr)}
				}
				if options.Stream != nil {
					retrySession.Stdout = io.MultiWriter(&stdoutBuf, options.Stream)
					retrySession.Stderr = io.MultiWriter(&stderrBuf, options.Stream)
				} else {
					stdoutBuf.Reset(); stderrBuf.Reset()
					retrySession.Stdout = &stdoutBuf
					retrySession.Stderr = &stderrBuf
				}
				if len(options.Env) > 0 {
					for _, envVar := range options.Env {
						parts := strings.SplitN(envVar, "=", 2)
						if len(parts) == 2 { retrySession.Setenv(parts[0], parts[1]) }
					}
				}
				err = retrySession.Run(finalCmd)
				retrySession.Close()
				stdout = stdoutBuf.Bytes(); stderr = stderrBuf.Bytes()
				if err == nil { break }
				if exitErr, ok := err.(*ssh.ExitError); ok { exitCode = exitErr.ExitStatus() }
			}
		}
		if err != nil {
			return stdout, stderr, &CommandError{Cmd: cmd, ExitCode: exitCode, Stdout: string(stdout), Stderr: string(stderr), Underlying: err}
		}
	}
	return stdout, stderr, nil
}

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

func (s *SSHConnector) CopyContent(ctx context.Context, content []byte, dstPath string, options *FileTransferOptions) error {
	if err := s.ensureSftp(); err != nil { return err }
	if options != nil && options.Sudo {
		fmt.Fprintf(os.Stderr, "Warning: CopyContent with sudo=true for SSH connector attempts direct SFTP write to %s; this may fail for restricted paths.\n", dstPath)
	}
	dstFile, err := s.sftpClient.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create remote file %s for content: %w", dstPath, err)
	}
	defer dstFile.Close()
	_, err = dstFile.Write(content)
	if err != nil { return fmt.Errorf("failed to write content to remote file %s: %w", dstPath, err) }
	if options != nil && options.Permissions != "" {
		perm, parseErr := strconv.ParseUint(options.Permissions, 8, 32)
		if parseErr == nil {
			if errChmod := s.sftpClient.Chmod(dstPath, os.FileMode(perm)); errChmod != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to chmod remote file %s to %s via SFTP: %v\n", dstPath, options.Permissions, errChmod)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Invalid permissions format '%s' for SFTP CopyContent to %s, skipping chmod: %v\n", options.Permissions, dstPath, parseErr)
		}
	}
	return nil
}

func (s *SSHConnector) Stat(ctx context.Context, path string) (*FileStat, error) {
	if err := s.ensureSftp(); err != nil { return nil, err }
	fi, err := s.sftpClient.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) { return &FileStat{Name: filepath.Base(path), IsExist: false}, nil }
		return nil, fmt.Errorf("failed to stat remote path %s: %w", path, err)
	}
	return &FileStat{
		Name: fi.Name(), Size: fi.Size(), Mode: fi.Mode(),
		ModTime: fi.ModTime(), IsDir: fi.IsDir(), IsExist: true,
	}, nil
}

func (s *SSHConnector) LookPath(ctx context.Context, file string) (string, error) {
	cmd := fmt.Sprintf("command -v %s", file)
	stdout, stderr, err := s.Exec(ctx, cmd, nil)
	if err != nil {
		if cmdErr, ok := err.(*CommandError); ok {
			if cmdErr.ExitCode == 1 {
				return "", fmt.Errorf("executable %s not found in PATH (stderr: %s)", file, string(stderr))
			}
		}
		return "", fmt.Errorf("failed to execute 'command -v %s': %w (stdout: %s, stderr: %s)", file, err, string(stdout), string(stderr))
	}
	path := strings.TrimSpace(string(stdout))
	if path == "" { return "", fmt.Errorf("executable %s not found in PATH (stderr: %s)", file, string(stderr)) }
	return path, nil
}

func (s *SSHConnector) GetOS(ctx context.Context) (*OS, error) {
	if s.cachedOS != nil { return s.cachedOS, nil }
	stdout, _, err := s.Exec(ctx, "cat /etc/os-release", nil)
	if err != nil { return nil, fmt.Errorf("failed to cat /etc/os-release: %w", err) }
	osInfo := &OS{}
	lines := strings.Split(string(stdout), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key, val := strings.TrimSpace(parts[0]), strings.Trim(strings.TrimSpace(parts[1]), "\"")
			switch key {
			case "ID": osInfo.ID = val
			case "VERSION_ID": osInfo.VersionID = val
			case "PRETTY_NAME": osInfo.PrettyName = val
			}
		}
	}
	archStdout, _, err := s.Exec(ctx, "uname -m", nil)
	if err == nil { osInfo.Arch = strings.TrimSpace(string(archStdout)) }
	kernelStdout, _, err := s.Exec(ctx, "uname -r", nil)
	if err == nil { osInfo.Kernel = strings.TrimSpace(string(kernelStdout)) }
	s.cachedOS = osInfo
	return s.cachedOS, nil
}

func (s *SSHConnector) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if err := s.ensureSftp(); err != nil {
		return nil, fmt.Errorf("sftp client not available for ReadFile: %w", err)
	}
	file, err := s.sftpClient.Open(path)
	if err != nil { return nil, fmt.Errorf("failed to open remote file %s via sftp: %w", path, err) }
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil { return nil, fmt.Errorf("failed to read remote file %s via sftp: %w", path, err) }
	return content, nil
}

func (s *SSHConnector) WriteFile(ctx context.Context, content []byte, destPath, permissions string, sudo bool) error {
	if err := s.ensureSftp(); err != nil {
		return fmt.Errorf("sftp client not available for WriteFile: %w", err)
	}
	if sudo {
		fmt.Fprintf(os.Stderr, "Warning: WriteFile with sudo=true for SSH connector attempts direct SFTP write to %s; this may fail for restricted paths.\n", destPath)
	}

	file, err := s.sftpClient.Create(destPath)
	if err != nil {
		parentDir := filepath.Dir(destPath)
		if parentDir != "." && parentDir != "/" {
			_ = s.sftpClient.Mkdir(parentDir) // Best effort mkdir
		}
		file, err = s.sftpClient.Create(destPath) // Retry create
		if err != nil {
			return fmt.Errorf("failed to create/truncate remote file %s via sftp: %w", destPath, err)
		}
	}
	defer file.Close()
	_, err = file.Write(content)
	if err != nil { return fmt.Errorf("failed to write content to remote file %s via sftp: %w", destPath, err) }
	if permissions != "" {
		permVal, parseErr := strconv.ParseUint(permissions, 8, 32)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: Invalid permissions format '%s' for SFTP WriteFile to %s, skipping chmod: %v\n", permissions, destPath, parseErr)
		} else {
			if chmodErr := s.sftpClient.Chmod(destPath, os.FileMode(permVal)); chmodErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to chmod remote file %s to %s via SFTP: %v\n", destPath, permissions, chmodErr)
			}
		}
	}
	return nil
}

func (s *SSHConnector) Mkdir(ctx context.Context, path string, perm string) error {
	cmd := fmt.Sprintf("mkdir -p %s", path)
	_, stderr, err := s.Exec(ctx, cmd, &ExecOptions{Sudo: false})
	if err != nil {
		return fmt.Errorf("failed to create directory %s on %s: %w (stderr: %s)", path, s.connCfg.Host, err, string(stderr))
	}
	if perm != "" {
		chmodCmd := fmt.Sprintf("chmod %s %s", perm, path)
		_, stderrChmod, errChmod := s.Exec(ctx, chmodCmd, &ExecOptions{Sudo: false})
		if errChmod != nil {
			return fmt.Errorf("failed to chmod directory %s on %s to %s: %w (stderr: %s)", path, s.connCfg.Host, perm, errChmod, string(stderrChmod))
		}
	}
	return nil
}

func (s *SSHConnector) Remove(ctx context.Context, path string, opts RemoveOptions) error {
	var cmd string
	if opts.Recursive { cmd = fmt.Sprintf("rm -rf %s", path)
	} else { cmd = fmt.Sprintf("rm -f %s", path) }
	_, stderr, err := s.Exec(ctx, cmd, &ExecOptions{Sudo: false})
	if err != nil {
		if opts.IgnoreNotExist && (strings.Contains(string(stderr), "No such file or directory") || strings.Contains(string(stderr), "cannot remove") && strings.Contains(string(stderr), "No such file or directory")) {
			return nil
		}
		return fmt.Errorf("failed to remove %s on %s: %w (stderr: %s)", path, s.connCfg.Host, err, string(stderr))
	}
	return nil
}

func (s *SSHConnector) GetFileChecksum(ctx context.Context, path string, checksumType string) (string, error) {
	var cmd string
	switch strings.ToLower(checksumType) {
	case "sha256": cmd = fmt.Sprintf("sha256sum %s", path)
	case "md5": cmd = fmt.Sprintf("md5sum %s", path)
	default: return "", fmt.Errorf("unsupported checksum type '%s' for remote file %s", checksumType, path)
	}
	stdout, stderr, err := s.Exec(ctx, cmd, &ExecOptions{Sudo: false})
	if err != nil {
		return "", fmt.Errorf("failed to execute checksum command '%s' on %s for %s: %w (stderr: %s)", cmd, s.connCfg.Host, path, err, string(stderr))
	}
	parts := strings.Fields(string(stdout))
	if len(parts) > 0 { return parts[0], nil }
	return "", fmt.Errorf("failed to parse checksum from command output: '%s'", string(stdout))
}

var _ Connector = &SSHConnector{}

type dialSSHFunc func(ctx context.Context, cfg ConnectionCfg, effectiveConnectTimeout time.Duration) (*ssh.Client, *ssh.Client, error)

var actualDialSSH dialSSHFunc = func(ctx context.Context, cfg ConnectionCfg, effectiveConnectTimeout time.Duration) (*ssh.Client, *ssh.Client, error) {
	var authMethods []ssh.AuthMethod
	if cfg.Password != "" { authMethods = append(authMethods, ssh.Password(cfg.Password)) }
	if len(cfg.PrivateKey) > 0 {
		signer, err := ssh.ParsePrivateKey(cfg.PrivateKey)
		if err != nil { return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to parse private key: %w", err)} }
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	} else if cfg.PrivateKeyPath != "" {
		key, err := os.ReadFile(cfg.PrivateKeyPath)
		if err != nil { return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to read private key file %s: %w", cfg.PrivateKeyPath, err)} }
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil { return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to parse private key from file %s: %w", cfg.PrivateKeyPath, err)} }
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	if len(authMethods) == 0 { return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("no authentication method provided (password or private key)")} }

	sshConfig := &ssh.ClientConfig{
		User: cfg.User, Auth: authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: effectiveConnectTimeout,
	}
	var client *ssh.Client
	var bastionSshClient *ssh.Client
	var err error
	dialAddr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	if cfg.BastionCfg != nil {
		var bastionAuthMethods []ssh.AuthMethod
		if cfg.BastionCfg.Password != "" { bastionAuthMethods = append(bastionAuthMethods, ssh.Password(cfg.BastionCfg.Password)) }
		if len(cfg.BastionCfg.PrivateKey) > 0 {
			signer, err := ssh.ParsePrivateKey(cfg.BastionCfg.PrivateKey)
			if err != nil { return nil, nil, &ConnectionError{Host: cfg.BastionCfg.Host, Err: fmt.Errorf("failed to parse bastion private key: %w", err)} }
			bastionAuthMethods = append(bastionAuthMethods, ssh.PublicKeys(signer))
		} else if cfg.BastionCfg.PrivateKeyPath != "" {
			key, err := os.ReadFile(cfg.BastionCfg.PrivateKeyPath)
			if err != nil { return nil, nil, &ConnectionError{Host: cfg.BastionCfg.Host, Err: fmt.Errorf("failed to read bastion private key file %s: %w", cfg.BastionCfg.PrivateKeyPath, err)} }
			signer, err := ssh.ParsePrivateKey(key)
			if err != nil { return nil, nil, &ConnectionError{Host: cfg.BastionCfg.Host, Err: fmt.Errorf("failed to parse bastion private key from file %s: %w", cfg.BastionCfg.PrivateKeyPath, err)} }
			bastionAuthMethods = append(bastionAuthMethods, ssh.PublicKeys(signer))
		}
		if len(bastionAuthMethods) == 0 { return nil, nil, &ConnectionError{Host: cfg.BastionCfg.Host, Err: fmt.Errorf("no authentication method provided for bastion (password or private key)")} }

		bastionConfig := &ssh.ClientConfig{
			User: cfg.BastionCfg.User, Auth: bastionAuthMethods,
			HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: cfg.BastionCfg.Timeout,
		}
		bastionDialAddr := net.JoinHostPort(cfg.BastionCfg.Host, strconv.Itoa(cfg.BastionCfg.Port))
		bastionSshClient, err = ssh.Dial("tcp", bastionDialAddr, bastionConfig)
		if err != nil { return nil, nil, &ConnectionError{Host: cfg.BastionCfg.Host, Err: fmt.Errorf("failed to dial bastion: %w", err)} }

		conn, err := bastionSshClient.Dial("tcp", dialAddr)
		if err != nil {
			bastionSshClient.Close()
			return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to dial target host via bastion: %w", err)}
		}
		ncc, chans, reqs, err := ssh.NewClientConn(conn, dialAddr, sshConfig)
		if err != nil {
			bastionSshClient.Close()
			return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("failed to create new client connection via bastion: %w", err)}
		}
		client = ssh.NewClient(ncc, chans, reqs)
	} else {
		client, err = ssh.Dial("tcp", dialAddr, sshConfig)
		if err != nil { return nil, nil, &ConnectionError{Host: cfg.Host, Err: err} }
	}
	return client, bastionSshClient, nil
}

var G_dialSSHOverride dialSSHFunc

func SetDialSSHOverrideForTesting(fn dialSSHFunc) (cleanupFunc func()) {
	G_dialSSHOverride = fn
	return func() { G_dialSSHOverride = nil }
}

func dialSSH(ctx context.Context, cfg ConnectionCfg, effectiveConnectTimeout time.Duration) (*ssh.Client, *ssh.Client, error) {
	if G_dialSSHOverride != nil {
		return G_dialSSHOverride(ctx, cfg, effectiveConnectTimeout)
	}
	return actualDialSSH(ctx, cfg, effectiveConnectTimeout)
}
