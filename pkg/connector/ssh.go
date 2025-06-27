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

	effectiveSudo := false
	var effectivePerms string
	if options != nil {
		effectiveSudo = options.Sudo
		effectivePerms = options.Permissions
	}

	if effectiveSudo {
		tmpPath := filepath.Join("/tmp", fmt.Sprintf("kubexm-copycontent-%d-%s", time.Now().UnixNano(), filepath.Base(dstPath)))

		// 1. Upload to temporary path (non-sudo)
		// Use default restrictive perms for temp file, final perms applied by sudo chmod.
		err := s.writeFileViaSFTP(context.Background(), content, tmpPath, "0600") // Use background context for temp operations
		if err != nil {
			return fmt.Errorf("failed to upload to temporary path %s for sudo CopyContent: %w", tmpPath, err)
		}
		defer func() {
			rmCmd := fmt.Sprintf("rm -f %s", tmpPath)
			// Use a new context for cleanup, don't let original ctx timeout affect cleanup.
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_, _, rmErr := s.Exec(cleanupCtx, rmCmd, &ExecOptions{Sudo: false}) // Try non-sudo first
			if rmErr != nil {
				_, _, rmSudoErr := s.Exec(cleanupCtx, rmCmd, &ExecOptions{Sudo: true})
				if rmSudoErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to remove temporary file %s on host %s (tried non-sudo and sudo): %v / %v\n", tmpPath, s.connCfg.Host, rmErr, rmSudoErr)
				}
			}
		}()

		// 2. Ensure destination directory exists using sudo
		destDir := filepath.Dir(dstPath)
		if destDir != "." && destDir != "/" {
			mkdirCmd := fmt.Sprintf("mkdir -p %s", destDir)
			execCtx, cancel := context.WithTimeout(ctx, 15*time.Second) // Use main ctx for timeout
			_, _, mkdirErr := s.Exec(execCtx, mkdirCmd, &ExecOptions{Sudo: true})
			cancel()
			if mkdirErr != nil {
				return fmt.Errorf("failed to sudo mkdir -p %s on host %s: %w", destDir, s.connCfg.Host, mkdirErr)
			}
		}

		// 3. Move file to destination using sudo
		mvCmd := fmt.Sprintf("mv %s %s", tmpPath, dstPath)
		execCtxMv, cancelMv := context.WithTimeout(ctx, 30*time.Second) // Potentially longer for mv
		_, _, mvErr := s.Exec(execCtxMv, mvCmd, &ExecOptions{Sudo: true})
		cancelMv()
		if mvErr != nil {
			return fmt.Errorf("failed to sudo mv %s to %s on host %s: %w", tmpPath, dstPath, s.connCfg.Host, mvErr)
		}

		// 4. Apply permissions using sudo
		if effectivePerms != "" {
			if _, err := strconv.ParseUint(effectivePerms, 8, 32); err != nil { // Validate format
				return fmt.Errorf("invalid permissions format '%s' for sudo chmod on host %s: %w", effectivePerms, s.connCfg.Host, err)
			}
			chmodCmd := fmt.Sprintf("chmod %s %s", effectivePerms, dstPath)
			execCtxChmod, cancelChmod := context.WithTimeout(ctx, 15*time.Second)
			_, _, chmodErr := s.Exec(execCtxChmod, chmodCmd, &ExecOptions{Sudo: true})
			cancelChmod()
			if chmodErr != nil {
				return fmt.Errorf("failed to sudo chmod %s to %s on host %s: %w", dstPath, effectivePerms, s.connCfg.Host, chmodErr)
			}
		}
		// TODO: Handle Owner/Group from FileTransferOptions if they are set, via sudo chown.
		return nil
	} else {
		// Non-sudo: direct SFTP write
		return s.writeFileViaSFTP(ctx, content, dstPath, effectivePerms)
	}
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

	osInfo := &OS{} // Initialize osInfo at the beginning
	var content, stderr []byte // Declare content and stderr
	var err error             // Declare err

	content, stderr, err = s.Exec(ctx, "cat /etc/os-release", nil)

	if err != nil {
		// Attempt to get at least Arch and Kernel if /etc/os-release fails
		var archStdout, kernelStdout []byte // Declare variables for this block
		var archErr, kernelErr error    // Declare error variables for this block

		archStdout, _, archErr = s.Exec(ctx, "uname -m", nil)
		if archErr == nil {
			osInfo.Arch = strings.TrimSpace(string(archStdout))
		}
		kernelStdout, _, kernelErr = s.Exec(ctx, "uname -r", nil) // Corrected from := to =
		if kernelErr == nil {
			osInfo.Kernel = strings.TrimSpace(string(kernelStdout))
		}
		return osInfo, fmt.Errorf("failed to cat /etc/os-release on host %s: %w (stderr: %s)", s.connCfg.Host, err, string(stderr))
	}

	lines := strings.Split(string(content), "\n")
	vars := make(map[string]string)
	for _, line := range lines {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.Trim(strings.TrimSpace(parts[1]), "\"")
			vars[key] = val
		}
	}
	osInfo.ID = vars["ID"]
	osInfo.VersionID = vars["VERSION_ID"]
	osInfo.PrettyName = vars["PRETTY_NAME"]
	osInfo.Codename = vars["VERSION_CODENAME"] // Populate Codename

	archStdout, _, archErr := s.Exec(ctx, "uname -m", nil)
	if archErr == nil {
		osInfo.Arch = strings.TrimSpace(string(archStdout))
	} else {
		fmt.Fprintf(os.Stderr, "Warning: failed to get arch for host %s: %v\n", s.connCfg.Host, archErr)
	}

	kernelStdout, _, kernelErr := s.Exec(ctx, "uname -r", nil)
	if kernelErr == nil {
		osInfo.Kernel = strings.TrimSpace(string(kernelStdout))
	} else {
		fmt.Fprintf(os.Stderr, "Warning: failed to get kernel for host %s: %v\n", s.connCfg.Host, kernelErr)
	}

	s.cachedOS = osInfo
	return s.cachedOS, nil
}

func (s *SSHConnector) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if err := s.ensureSftp(); err != nil {
		return nil, fmt.Errorf("sftp client not available for ReadFile on host %s: %w", s.connCfg.Host, err)
	}
	file, err := s.sftpClient.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open remote file %s via sftp on host %s: %w", path, s.connCfg.Host, err)
	}
	defer file.Close()

	contentBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote file %s via sftp on host %s: %w", path, s.connCfg.Host, err)
	}
	return contentBytes, nil
}

func (s *SSHConnector) WriteFile(ctx context.Context, content []byte, destPath, permissions string, sudo bool) error {
	if err := s.ensureSftp(); err != nil {
		return fmt.Errorf("sftp client not available for WriteFile on host %s: %w", s.connCfg.Host, err)
	}
	if err := s.ensureSftp(); err != nil { // Ensure SFTP client is available for non-sudo or initial upload part of sudo
		return fmt.Errorf("sftp client not available for WriteFile on host %s: %w", s.connCfg.Host, err)
	}

	if sudo {
		// Sudo strategy: upload to temp, then sudo mv, sudo chmod, sudo chown
		tmpPath := filepath.Join("/tmp", fmt.Sprintf("kubexm-write-%d-%s", time.Now().UnixNano(), filepath.Base(destPath)))

		// 1. Upload to temporary path without sudo
		err := s.writeFileViaSFTP(ctx, content, tmpPath, "0644") // Write with temp permissions
		if err != nil {
			return fmt.Errorf("failed to upload to temporary path %s on host %s for sudo write: %w", tmpPath, s.connCfg.Host, err)
		}
		// Defer removal of temporary file
		defer func() {
			// Best effort removal, log if it fails but don't fail the WriteFile operation itself for this.
			// Sudo might be needed if the user running kubexm can't delete from /tmp (unlikely but possible)
			// For simplicity, using non-sudo rm.
			rmCmd := fmt.Sprintf("rm -f %s", tmpPath)
			_, _, rmErr := s.Exec(ctx, rmCmd, &ExecOptions{Sudo: false}) // Try non-sudo first
			if rmErr != nil {
				// Attempt with sudo if non-sudo failed, as a fallback cleanup
				_, _, rmSudoErr := s.Exec(ctx, rmCmd, &ExecOptions{Sudo: true})
				if rmSudoErr != nil {
					fmt.Fprintf(os.Stderr, "Warning: failed to remove temporary file %s on host %s (tried non-sudo and sudo): %v / %v\n", tmpPath, s.connCfg.Host, rmErr, rmSudoErr)
				}
			}
		}()

		// 2. Ensure destination directory exists using sudo
		destDir := filepath.Dir(destPath)
		if destDir != "." && destDir != "/" {
			mkdirCmd := fmt.Sprintf("mkdir -p %s", destDir)
			_, _, mkdirErr := s.Exec(ctx, mkdirCmd, &ExecOptions{Sudo: true})
			if mkdirErr != nil {
				return fmt.Errorf("failed to sudo mkdir -p %s on host %s: %w", destDir, s.connCfg.Host, mkdirErr)
			}
		}

		// 3. Move file to destination using sudo
		mvCmd := fmt.Sprintf("mv %s %s", tmpPath, destPath)
		_, _, mvErr := s.Exec(ctx, mvCmd, &ExecOptions{Sudo: true})
		if mvErr != nil {
			return fmt.Errorf("failed to sudo mv %s to %s on host %s: %w", tmpPath, destPath, s.connCfg.Host, mvErr)
		}

		// 4. Apply permissions using sudo
		if permissions != "" {
			// Validate permissions format briefly before sending to chmod
			if _, err := strconv.ParseUint(permissions, 8, 32); err != nil {
				return fmt.Errorf("invalid permissions format '%s' for sudo chmod on host %s: %w", permissions, s.connCfg.Host, err)

			}
			chmodCmd := fmt.Sprintf("chmod %s %s", permissions, destPath)
			_, _, chmodErr := s.Exec(ctx, chmodCmd, &ExecOptions{Sudo: true})
			if chmodErr != nil {
				return fmt.Errorf("failed to sudo chmod %s to %s on host %s: %w", destPath, permissions, s.connCfg.Host, chmodErr)
			}
		}
		// Note: Chown is not part of WriteFile's signature, but could be added to FileTransferOptions
		// and handled here if options were passed to WriteFile.
		// For now, permissions are handled.
	} else {
		// Non-sudo: direct SFTP write
		return s.writeFileViaSFTP(ctx, content, destPath, permissions)
	}
	return nil
}


// writeFileViaSFTP is a helper for direct SFTP writes.
func (s *SSHConnector) writeFileViaSFTP(ctx context.Context, content []byte, destPath, permissions string) error {
	parentDir := filepath.Dir(destPath)
	if parentDir != "." && parentDir != "/" {
		_, statErr := s.sftpClient.Stat(parentDir)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				// Use Exec for robust recursive directory creation (non-sudo context for this helper)
				mkdirCmd := fmt.Sprintf("mkdir -p %s", parentDir)
				execCtx, execCancel := context.WithTimeout(ctx, 15*time.Second) // Use parent ctx for timeout
				_, _, mkdirErr := s.Exec(execCtx, mkdirCmd, &ExecOptions{Sudo: false})
				execCancel()
				if mkdirErr != nil {
					return fmt.Errorf("failed to create parent directory %s via exec on host %s: %w", parentDir, s.connCfg.Host, mkdirErr)
				}
			} else {
				// Other stat errors (e.g., permission denied to stat parent) are problematic.
				return fmt.Errorf("failed to stat parent directory %s on host %s: %w", parentDir, s.connCfg.Host, statErr)
			}
		}
		// If statErr is nil, directory exists.
	}

	file, err := s.sftpClient.Create(destPath) // Create will truncate if file exists
	if err != nil {
		return fmt.Errorf("failed to create/truncate remote file %s via sftp on host %s: %w", destPath, s.connCfg.Host, err)
	}
	defer file.Close()

	_, err = file.Write(content)
	if err != nil {
		return fmt.Errorf("failed to write content to remote file %s via sftp on host %s: %w", destPath, s.connCfg.Host, err)
	}

	if permissions != "" {
		permVal, parseErr := strconv.ParseUint(permissions, 8, 32)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: Invalid permissions format '%s' for SFTP WriteFile to %s on host %s, skipping chmod: %v\n", permissions, destPath, s.connCfg.Host, parseErr)
		} else {
			if chmodErr := s.sftpClient.Chmod(destPath, os.FileMode(permVal)); chmodErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to chmod remote file %s to %s via SFTP on host %s: %v\n", destPath, permissions, s.connCfg.Host, chmodErr)
			}
		}
	}
	return nil
}

func (s *SSHConnector) Mkdir(ctx context.Context, path string, perm string) error {
	// Construct the command with sudo if needed, though typically mkdir -p handles permissions well.
	// For simplicity, assuming Runner handles sudo if path requires it.
	// Connector's Mkdir itself won't use sudo directly unless ExecOptions are passed.
	cmd := fmt.Sprintf("mkdir -p %s", path) // -p makes it idempotent
	_, stderrBytes, err := s.Exec(ctx, cmd, &ExecOptions{Sudo: false}) // Assuming no sudo for basic mkdir by connector
	if err != nil {
		// Check if it's CommandError to provide more details
		if cmdErr, ok := err.(*CommandError); ok {
			return fmt.Errorf("failed to create directory %s on %s (exit code %d): %w (stderr: %s)", path, s.connCfg.Host, cmdErr.ExitCode, err, cmdErr.Stderr)
		}
		return fmt.Errorf("failed to create directory %s on %s: %w (stderr: %s)", path, s.connCfg.Host, err, string(stderrBytes))
	}

	if perm != "" {
		chmodCmd := fmt.Sprintf("chmod %s %s", perm, path)
		// Pass Sudo false, as chmod on a directory usually depends on ownership or prior sudo for mkdir.
		_, chmodStderrBytes, chmodErr := s.Exec(ctx, chmodCmd, &ExecOptions{Sudo: false})
		if chmodErr != nil {
			if cmdErr, ok := chmodErr.(*CommandError); ok {
				return fmt.Errorf("failed to chmod directory %s on %s to %s (exit code %d): %w (stderr: %s)", path, s.connCfg.Host, perm, cmdErr.ExitCode, chmodErr, cmdErr.Stderr)
			}
			return fmt.Errorf("failed to chmod directory %s on %s to %s: %w (stderr: %s)", path, s.connCfg.Host, perm, chmodErr, string(chmodStderrBytes))
		}
	}
	return nil
}

func (s *SSHConnector) Remove(ctx context.Context, path string, opts RemoveOptions) error {
	var cmd string
	flags := "-f" // Default to force, helps with idempotency if file doesn't exist with -f
	if opts.Recursive {
		flags += "r"
	}
	cmd = fmt.Sprintf("rm %s %s", flags, path)

	// Assuming no sudo for basic remove by connector. Runner would apply sudo if needed.
	// (Sudo should be part of ExecOptions if needed by caller)
	execOpts := &ExecOptions{Sudo: false} // Default, caller can override via opts in a more generic Exec

	if opts.IgnoreNotExist {
		statCtx, statCancel := context.WithTimeout(ctx, 10*time.Second)
		fileStat, statErr := s.Stat(statCtx, path)
		statCancel()

		if statErr == nil && fileStat != nil && !fileStat.IsExist {
			return nil // File does not exist, success as per IgnoreNotExist
		}
		// If Stat itself had an error other than "not found", we might still want to proceed with rm,
		// as rm -f is quite forgiving. Or, we could return statErr here if it's not an os.IsNotExist type.
		// For now, if Stat says it exists, or Stat had an unrelated error, we proceed to rm.
	}

	_, stderrBytes, err := s.Exec(ctx, cmd, execOpts)
	if err != nil {
		// If IgnoreNotExist was true AND Stat indicated it existed (or Stat errored non-fatally),
		// then an error from `rm` is a real error.
		if cmdErr, ok := err.(*CommandError); ok {
			// If IgnoreNotExist was true, but rm still failed (e.g. permission denied on existing file)
			// that's a valid error to report. The Stat check above only handles "already not there".
			return fmt.Errorf("failed to remove %s on %s (exit code %d): %w (stderr: %s)", path, s.connCfg.Host, cmdErr.ExitCode, err, cmdErr.Stderr)
		}
		return fmt.Errorf("failed to remove %s on %s: %w (stderr: %s)", path, s.connCfg.Host, err, string(stderrBytes))
	}
	return nil
}

func (s *SSHConnector) GetFileChecksum(ctx context.Context, path string, checksumType string) (string, error) {
	var checksumCmd string
	switch strings.ToLower(checksumType) {
	case "sha256":
		checksumCmd = fmt.Sprintf("sha256sum %s", path)
	case "md5":
		checksumCmd = fmt.Sprintf("md5sum %s", path)
	default:
		return "", fmt.Errorf("unsupported checksum type '%s' for remote file %s on host %s", checksumType, path, s.connCfg.Host)
	}

	stdoutBytes, stderrBytes, err := s.Exec(ctx, checksumCmd, &ExecOptions{Sudo: false}) // Assuming no sudo for checksum
	if err != nil {
		if cmdErr, ok := err.(*CommandError); ok {
			return "", fmt.Errorf("failed to execute checksum command '%s' on %s for %s (exit code %d): %w (stderr: %s)", checksumCmd, s.connCfg.Host, path, cmdErr.ExitCode, err, cmdErr.Stderr)
		}
		return "", fmt.Errorf("failed to execute checksum command '%s' on %s for %s: %w (stderr: %s)", checksumCmd, s.connCfg.Host, path, err, string(stderrBytes))
	}

	parts := strings.Fields(string(stdoutBytes))
	if len(parts) > 0 {
		return parts[0], nil
	}
	return "", fmt.Errorf("failed to parse checksum from command output for %s on host %s: '%s'", path, s.connCfg.Host, string(stdoutBytes))
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

	hostKeyCallback := cfg.HostKeyCallback
	if hostKeyCallback == nil {
		fmt.Fprintf(os.Stderr, "Warning: HostKeyCallback is not set for host %s. Using InsecureIgnoreHostKey(). This is not recommended for production.\n", cfg.Host)
		hostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	sshConfig := &ssh.ClientConfig{
		User: cfg.User, Auth: authMethods,
		HostKeyCallback: hostKeyCallback, Timeout: effectiveConnectTimeout,
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
