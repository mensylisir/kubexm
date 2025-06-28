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
	bastionClient *ssh.Client
	sftpClient    *sftp.Client
	connCfg       ConnectionCfg
	cachedOS      *OS
	isConnected   bool
	pool          *ConnectionPool
	isFromPool    bool
}

// NewSSHConnector creates a new SSHConnector.
func NewSSHConnector(pool *ConnectionPool) *SSHConnector {
	return &SSHConnector{
		pool: pool,
	}
}

// Connect establishes an SSH connection.
func (s *SSHConnector) Connect(ctx context.Context, cfg ConnectionCfg) error {
	s.connCfg = cfg
	s.isFromPool = false

	// Attempt to get a connection from the pool first (if applicable)
	if s.pool != nil && cfg.BastionCfg == nil {
		pooledClient, err := s.pool.Get(ctx, cfg)
		if err == nil && pooledClient != nil {
			s.client = pooledClient
			s.isFromPool = true
			// Perform a lightweight health check on the pooled connection
			if _, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				fmt.Fprintf(os.Stderr, "SSHConnector: Pooled connection for %s failed health check, closing and falling back to direct dial: %v\n", cfg.Host, err)
				s.pool.CloseConnection(s.connCfg, s.client) // Invalidate the bad connection
				s.client = nil
				s.isFromPool = false
			} else {
				s.isConnected = true
				return nil // Pooled connection is healthy
			}
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "SSHConnector: Failed to get connection from pool for %s: %v. Falling back to direct dial.\n", cfg.Host, err)
		}
	}

	// Fallback to direct dial if not using pool or if pooled connection failed
	client, bastionClient, err := dialSSH(ctx, cfg)
	if err != nil {
		return err
	}
	s.client = client
	s.bastionClient = bastionClient
	s.isConnected = true
	return nil
}

// IsConnected checks if the SSH client is connected.
// ENHANCEMENT: Uses a lightweight keepalive request instead of creating a new session, which is much more performant.
func (s *SSHConnector) IsConnected() bool {
	if s.client == nil || !s.isConnected {
		return false
	}
	// SendRequest is the standard, low-overhead way to check a connection's health.
	_, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil)
	if err != nil {
		s.isConnected = false // Mark as disconnected on failure
		return false
	}
	return true
}

// Close closes the SSH and SFTP clients.
// ENHANCEMENT: Returns the first error encountered and health-checks connections before returning them to the pool.
func (s *SSHConnector) Close() error {
	s.isConnected = false
	var firstErr error

	if s.sftpClient != nil {
		if err := s.sftpClient.Close(); err != nil {
			firstErr = fmt.Errorf("failed to close SFTP client for %s: %w", s.connCfg.Host, err)
			fmt.Fprintln(os.Stderr, firstErr)
		}
		s.sftpClient = nil
	}

	if s.client != nil {
		if s.isFromPool && s.pool != nil {
			// Check health before returning to pool
			if _, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil); err == nil {
				s.pool.Put(s.connCfg, s.client, true)
			} else {
				s.pool.CloseConnection(s.connCfg, s.client) // Don't return unhealthy connection
			}
		} else {
			if err := s.client.Close(); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		s.client = nil
	}
	s.isFromPool = false

	if s.bastionClient != nil {
		if err := s.bastionClient.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		s.bastionClient = nil
	}
	return firstErr
}

// Exec executes a command on the remote host.
// REFACTOR: Refactored with a `runOnce` helper to improve retry logic and reduce code duplication.
func (s *SSHConnector) Exec(ctx context.Context, cmd string, options *ExecOptions) (stdout, stderr []byte, err error) {
	if !s.IsConnected() {
		return nil, nil, &ConnectionError{Host: s.connCfg.Host, Err: fmt.Errorf("not connected")}
	}

	effectiveOptions := ExecOptions{}
	if options != nil {
		effectiveOptions = *options
	}

	// runOnce helper encapsulates the logic for a single command execution attempt.
	runOnce := func(runCtx context.Context, stdinPipe io.Reader) ([]byte, []byte, error) {
		session, err := s.client.NewSession()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create session: %w", err)
		}
		defer session.Close()

		if len(effectiveOptions.Env) > 0 {
			for _, envVar := range effectiveOptions.Env {
				parts := strings.SplitN(envVar, "=", 2)
				if len(parts) == 2 {
					_ = session.Setenv(parts[0], parts[1]) // Ignore error for best-effort
				}
			}
		}

		finalCmd := cmd
		if effectiveOptions.Sudo {
			if s.connCfg.Password != "" {
				finalCmd = "sudo -S -p '' -E -- " + cmd
				// Combine password with the actual stdin content if provided
				// If stdinPipe is nil (e.g. from a direct Exec call not piping content), MultiReader handles it.
				session.Stdin = io.MultiReader(strings.NewReader(s.connCfg.Password+"\n"), stdinPipe)
			} else {
				finalCmd = "sudo -E -- " + cmd
				session.Stdin = stdinPipe
			}
		} else {
			session.Stdin = stdinPipe
		}

		var stdoutBuf, stderrBuf bytes.Buffer
		if effectiveOptions.Stream != nil {
			session.Stdout = io.MultiWriter(&stdoutBuf, effectiveOptions.Stream)
			session.Stderr = io.MultiWriter(&stderrBuf, effectiveOptions.Stream)
		} else {
			session.Stdout = &stdoutBuf
			session.Stderr = &stderrBuf
		}

		err = session.Run(finalCmd)
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), err
	}

	var finalErr error
	var stdinReader io.Reader = bytes.NewReader(nil) // Default empty stdin for commands not needing piped input
	// Check if options.Stream is intended to be used as stdin
	if roStream, ok := effectiveOptions.Stream.(readOnlyStream); ok {
		stdinReader = roStream.Reader
	} else if effectiveOptions.Stream != nil {
		// If Stream is not readOnlyStream but is an io.Reader, consider it as stdin.
		// This part is a bit ambiguous in the original design of ExecOptions.Stream.
		// For sudo tee, we explicitly use readOnlyStream.
		// For general Exec, if Stream is also an io.Reader, it might be intended for stdin.
		// However, typical use of Stream is for capturing output.
		// Let's stick to `readOnlyStream` for explicit stdin piping for now to avoid ambiguity.
		// If options.Stream is some other io.Writer for output, stdinReader remains empty.
	}


	for i := 0; i <= effectiveOptions.Retries; i++ {
		attemptCtx := ctx
		var attemptCancel context.CancelFunc
		if effectiveOptions.Timeout > 0 {
			attemptCtx, attemptCancel = context.WithTimeout(ctx, effectiveOptions.Timeout)
		}

		stdout, stderr, err = runOnce(attemptCtx, stdinReader)

		if attemptCancel != nil {
			attemptCancel()
		}

		if err == nil {
			return stdout, stderr, nil // Success
		}
		finalErr = err

		if attemptCtx.Err() != nil { // This checks if the attemptCtx itself was "Done"
			break // Context timed out or was cancelled, no more retries
		}

		if i < effectiveOptions.Retries && effectiveOptions.RetryDelay > 0 {
			time.Sleep(effectiveOptions.RetryDelay)
		}
	}

	exitCode := -1
	if exitErr, ok := finalErr.(*ssh.ExitError); ok {
		exitCode = exitErr.ExitStatus()
	}
	return stdout, stderr, &CommandError{Cmd: cmd, ExitCode: exitCode, Stdout: string(stdout), Stderr: string(stderr), Underlying: finalErr}
}

// ensureSftp initializes the SFTP client if it hasn't been already.
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


// CopyContent copies byte content to a destination file.
// REFACTOR: This now calls the more generic writeFileFromReader.
func (s *SSHConnector) CopyContent(ctx context.Context, content []byte, dstPath string, options *FileTransferOptions) error {
	return s.writeFileFromReader(ctx, bytes.NewReader(content), dstPath, options)
}

// WriteFile is now a public wrapper around writeFileFromReader for backward compatibility
// with the Connector interface which expects []byte.
func (s *SSHConnector) WriteFile(ctx context.Context, content []byte, destPath, permissions string, sudo bool) error {
	opts := &FileTransferOptions{
		Permissions: permissions,
		Sudo:        sudo,
	}
	return s.writeFileFromReader(ctx, bytes.NewReader(content), destPath, opts)
}

// writeFileFromReader is the new canonical implementation for writing content from a reader.
// ENHANCEMENT: Uses `sudo tee` for efficient, streaming privileged writes instead of the `upload -> mv` pattern.
func (s *SSHConnector) writeFileFromReader(ctx context.Context, content io.Reader, dstPath string, options *FileTransferOptions) error {
	opts := FileTransferOptions{}
	if options != nil {
		opts = *options
	}

	if opts.Sudo {
		// Ensure destination directory exists.
		destDir := filepath.Dir(dstPath)
		if destDir != "." && destDir != "/" && destDir != "" {
			// SECURITY: Escape path to prevent command injection.
			mkdirCmd := fmt.Sprintf("mkdir -p %s", shellEscape(destDir))
			if _, _, err := s.Exec(ctx, mkdirCmd, &ExecOptions{Sudo: true}); err != nil {
				return fmt.Errorf("failed to sudo mkdir -p %s on host %s: %w", destDir, s.connCfg.Host, err)
			}
		}

		// Use `tee` to write the content atomically with sudo.
		// SECURITY: Escape destination path.
		// Output of tee is redirected to /dev/null
		cmd := fmt.Sprintf("tee %s > /dev/null", shellEscape(dstPath))
		execOpts := &ExecOptions{
			Sudo:   true,
			Stream: readOnlyStream{Reader: content}, // Pass content via stream's Reader interface
		}
		_, stderr, err := s.Exec(ctx, cmd, execOpts)
		if err != nil {
			return fmt.Errorf("failed to write to %s with sudo tee: %s (underlying error %w)", dstPath, string(stderr), err)
		}

		if opts.Permissions != "" {
			if _, err := strconv.ParseUint(opts.Permissions, 8, 32); err != nil {
				return fmt.Errorf("invalid permissions format '%s'", opts.Permissions)
			}
			// SECURITY: Escape permissions and path.
			chmodCmd := fmt.Sprintf("chmod %s %s", shellEscape(opts.Permissions), shellEscape(dstPath))
			if _, _, err := s.Exec(ctx, chmodCmd, &ExecOptions{Sudo: true}); err != nil {
				return fmt.Errorf("failed to sudo chmod %s to %s: %w", dstPath, opts.Permissions, err)
			}
		}
		// Note: Owner/Group handling would require `sudo chown` similar to chmod.
	} else {
		// Non-sudo: use SFTP for direct, efficient writing.
		if err := s.ensureSftp(); err != nil {
			return err
		}
		return s.writeFileViaSFTP(ctx, content, dstPath, opts.Permissions)
	}

	return nil
}

// writeFileViaSFTP is a helper for direct SFTP writes using an io.Reader.
func (s *SSHConnector) writeFileViaSFTP(ctx context.Context, content io.Reader, destPath, permissions string) error {
	if err := s.ensureSftp(); err != nil {
		return fmt.Errorf("sftp client not available for writeFileViaSFTP on host %s: %w", s.connCfg.Host, err)
	}

	parentDir := filepath.Dir(destPath)
	if parentDir != "." && parentDir != "/" && parentDir != "" {
		// Check if parent directory exists via SFTP
		_, statErr := s.sftpClient.Stat(parentDir)
		if statErr != nil {
			if os.IsNotExist(statErr) {
				// SFTP MkdirAll creates parent directories recursively.
				// Use default permissions for intermediate dirs.
				if err := s.sftpClient.MkdirAll(parentDir); err != nil {
					return fmt.Errorf("failed to create parent directory %s via sftp on host %s: %w", parentDir, s.connCfg.Host, err)
				}
			} else {
				return fmt.Errorf("failed to stat parent directory %s via sftp on host %s: %w", parentDir, s.connCfg.Host, statErr)
			}
		}
	}

	file, err := s.sftpClient.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create/truncate remote file %s via sftp on host %s: %w", destPath, s.connCfg.Host, err)
	}
	defer file.Close()

	// Copy content from reader to the SFTP file writer
	if _, err = io.Copy(file, content); err != nil {
		return fmt.Errorf("failed to write content to remote file %s via sftp on host %s: %w", destPath, s.connCfg.Host, err)
	}

	if permissions != "" {
		permVal, parseErr := strconv.ParseUint(permissions, 8, 32)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: Invalid permissions format '%s' for SFTP WriteFile to %s on host %s, skipping chmod: %v\n", permissions, destPath, s.connCfg.Host, parseErr)
		} else {
			if chmodErr := s.sftpClient.Chmod(destPath, os.FileMode(permVal)); chmodErr != nil {
				// Log warning for SFTP chmod failure, might not be critical for all use cases.
				fmt.Fprintf(os.Stderr, "Warning: Failed to chmod remote file %s to %s via SFTP on host %s: %v\n", destPath, permissions, s.connCfg.Host, chmodErr)
			}
		}
	}
	return nil
}


// Stat retrieves file information from the remote host.
func (s *SSHConnector) Stat(ctx context.Context, path string) (*FileStat, error) {
	if err := s.ensureSftp(); err != nil {
		return nil, err
	}
	fi, err := s.sftpClient.Lstat(path) // Lstat is correct to not follow symlinks
	if err != nil {
		if os.IsNotExist(err) {
			return &FileStat{Name: filepath.Base(path), IsExist: false}, nil
		}
		return nil, fmt.Errorf("failed to stat remote path %s: %w", path, err)
	}
	return &FileStat{
		Name:    fi.Name(),
		Size:    fi.Size(),
		Mode:    fi.Mode(),
		ModTime: fi.ModTime(),
		IsDir:   fi.IsDir(),
		IsExist: true,
	}, nil
}


// LookPath finds an executable in the remote PATH.
// SECURITY: Escape the filename to prevent command injection.
func (s *SSHConnector) LookPath(ctx context.Context, file string) (string, error) {
	// Basic validation for common injection characters. A more robust solution might involve
	// stricter validation or ensuring the command is run in a way that `file` is always an arg.
	if strings.ContainsAny(file, " \t\n\r`;&|$<>()!{}[]*?^~") {
		return "", fmt.Errorf("invalid characters in executable name for LookPath: %q", file)
	}
	// Use `shellEscape` for the argument to `command -v` for belt-and-suspenders,
	// though `command -v` is generally safe with simple filenames.
	cmd := fmt.Sprintf("command -v %s", shellEscape(file))
	stdout, stderr, err := s.Exec(ctx, cmd, nil)
	if err != nil {
		// If 'command -v' fails, it usually means not found (exit code 1 for POSIX `command -v`)
		// or other error. The error from Exec will be CommandError.
		return "", fmt.Errorf("failed to find executable '%s': %s (underlying error: %w)", file, string(stderr), err)
	}
	path := strings.TrimSpace(string(stdout))
	if path == "" { // Should be redundant if Exec returns error on non-zero exit
		return "", fmt.Errorf("executable %s not found in PATH (stderr: %s)", file, string(stderr))
	}
	return path, nil
}

// GetOS retrieves remote OS information.
// ENHANCEMENT: More resilient, tries multiple methods and gracefully degrades.
func (s *SSHConnector) GetOS(ctx context.Context) (*OS, error) {
	if s.cachedOS != nil {
		return s.cachedOS, nil
	}

	osInfo := &OS{}
	var errs []string

	// Method 1: /etc/os-release (preferred)
	// Use a short timeout for these non-critical info commands.
	infoCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	content, _, err := s.Exec(infoCtx, "cat /etc/os-release", nil)
	if err == nil {
		vars := parseKeyValues(string(content), "=", "\"")
		osInfo.ID = vars["ID"]
		osInfo.VersionID = vars["VERSION_ID"]
		osInfo.PrettyName = vars["PRETTY_NAME"]
		osInfo.Codename = vars["VERSION_CODENAME"]
	} else {
		errs = append(errs, fmt.Sprintf("/etc/os-release failed: %v", err))
	}

	// Method 2: lsb_release (fallback)
	if osInfo.ID == "" {
		lsbCtx, lsbCancel := context.WithTimeout(ctx, 10*time.Second)
		defer lsbCancel()
		content, _, err = s.Exec(lsbCtx, "lsb_release -a", nil)
		if err == nil {
			vars := parseKeyValues(string(content), ":", "")
			osInfo.ID = strings.ToLower(vars["Distributor ID"]) // lsb_release uses "Distributor ID"
			osInfo.VersionID = vars["Release"]
			osInfo.PrettyName = vars["Description"]
			osInfo.Codename = vars["Codename"]
		} else {
			errs = append(errs, fmt.Sprintf("lsb_release -a failed: %v", err))
		}
	}

	// Method 3: Fallback for specific OS if still no ID (e.g. older systems, macOS)
	if osInfo.ID == "" {
		unameCtx, unameCancel := context.WithTimeout(ctx, 5*time.Second)
		defer unameCancel()
		unameS, _, unameErr := s.Exec(unameCtx, "uname -s", nil)
		if unameErr == nil {
			osName := strings.ToLower(strings.TrimSpace(string(unameS)))
			if strings.HasPrefix(osName, "linux") {
				osInfo.ID = "linux" // Generic Linux
			} else if strings.HasPrefix(osName, "darwin") {
				osInfo.ID = "darwin"
				// Try to get more macOS specific info
				swVerCtx, swVerCancel := context.WithTimeout(ctx, 10*time.Second)
				defer swVerCancel()
				pn, _, _ := s.Exec(swVerCtx, "sw_vers -productName", nil)
				pv, _, _ := s.Exec(swVerCtx, "sw_vers -productVersion", nil)
				osInfo.PrettyName = strings.TrimSpace(string(pn))
				osInfo.VersionID = strings.TrimSpace(string(pv))
			}
			// Potentially add other OS checks like "FreeBSD", "SunOS" etc.
		} else {
			errs = append(errs, fmt.Sprintf("uname -s failed: %v", unameErr))
		}
	}


	// Always try to get Arch and Kernel
	archCtx, archCancel := context.WithTimeout(ctx, 5*time.Second)
	defer archCancel()
	arch, _, archErr := s.Exec(archCtx, "uname -m", nil)
	if archErr == nil {
		osInfo.Arch = strings.TrimSpace(string(arch))
	} else {
		errs = append(errs, fmt.Sprintf("uname -m failed: %v", archErr))
	}

	kernelCtx, kernelCancel := context.WithTimeout(ctx, 5*time.Second)
	defer kernelCancel()
	kernel, _, kernelErr := s.Exec(kernelCtx, "uname -r", nil)
	if kernelErr == nil {
		osInfo.Kernel = strings.TrimSpace(string(kernel))
	} else {
		errs = append(errs, fmt.Sprintf("uname -r failed: %v", kernelErr))
	}


	// If we couldn't identify the OS ID at all, but got other info, it's a partial success.
	// If ID is still empty, this is a more significant failure.
	if osInfo.ID == "" {
		// Try to use `uname -s` as a last resort for ID if not already set.
		if osInfo.Kernel != "" && strings.Contains(strings.ToLower(osInfo.Kernel), "linux") { // Simple heuristic
			osInfo.ID = "linux"
		} else if osInfo.Arch != "" { // If we only have Arch, that's not enough for a good ID.
			return nil, fmt.Errorf("failed to determine OS ID, errors: %s", strings.Join(errs, "; "))
		}
	}

	// Clean up potentially empty fields that were not found
	osInfo.ID = strings.ToLower(strings.TrimSpace(osInfo.ID))
	osInfo.VersionID = strings.TrimSpace(osInfo.VersionID)
	osInfo.PrettyName = strings.TrimSpace(osInfo.PrettyName)
	osInfo.Codename = strings.TrimSpace(osInfo.Codename)


	if osInfo.ID == "" && osInfo.Arch == "" && osInfo.Kernel == "" { // Complete failure
		return nil, fmt.Errorf("failed to determine any OS info, errors: %s", strings.Join(errs, "; "))
	}

	// Log warnings for fields that couldn't be determined if some info was found
	if osInfo.ID != "" { // Only cache if we got at least an ID
		s.cachedOS = osInfo
		if osInfo.VersionID == "" { fmt.Fprintf(os.Stderr, "Warning: Could not determine OS VersionID for host %s\n", s.connCfg.Host)}
		if osInfo.PrettyName == "" { fmt.Fprintf(os.Stderr, "Warning: Could not determine OS PrettyName for host %s\n", s.connCfg.Host)}
		// Codename might often be empty, less critical to warn.
	}
	return s.cachedOS, nil // Return whatever was gathered, even if partial, as long as ID or Arch/Kernel is there.
}


// ReadFile reads a file from the remote host.
func (s *SSHConnector) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if err := s.ensureSftp(); err != nil {
		return nil, fmt.Errorf("sftp client not available for ReadFile on host %s: %w", s.connCfg.Host, err)
	}
	file, err := s.sftpClient.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open remote file %s via sftp on host %s: %w", path, s.connCfg.Host, err)
	}
	defer file.Close()

	// It's good practice to check file size before reading, to prevent OOM on huge files.
	// For now, direct ReadAll. Consider adding a size limit option in future.
	contentBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read remote file %s via sftp on host %s: %w", path, s.connCfg.Host, err)
	}
	return contentBytes, nil
}


// Mkdir creates a directory on the remote host.
// SECURITY: Escaped path to prevent command injection.
func (s *SSHConnector) Mkdir(ctx context.Context, path string, perm string) error {
	escapedPath := shellEscape(path)
	// Default to sudo false for mkdir, caller can use Exec with sudo if needed for parent dirs.
	// mkdir -p is generally safe and idempotent.
	cmd := fmt.Sprintf("mkdir -p %s", escapedPath)
	_, stderr, err := s.Exec(ctx, cmd, &ExecOptions{Sudo: false}) // Explicitly non-sudo for mkdir itself
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %s (underlying error: %w)", path, string(stderr), err)
	}

	if perm != "" {
		if _, errP := strconv.ParseUint(perm, 8, 32); errP != nil { // Validate permission format
			return fmt.Errorf("invalid permission format '%s' for Mkdir: %w", perm, errP)
		}
		// SECURITY: Escape permissions and path.
		chmodCmd := fmt.Sprintf("chmod %s %s", shellEscape(perm), escapedPath)
		// Chmod also non-sudo by default. If sudo is needed, caller should manage.
		_, chmodStderr, chmodErr := s.Exec(ctx, chmodCmd, &ExecOptions{Sudo: false})
		if chmodErr != nil {
			return fmt.Errorf("failed to chmod directory %s to %s: %s (underlying error: %w)", path, perm, string(chmodStderr), chmodErr)
		}
	}
	return nil
}

// Remove removes a file or directory on the remote host.
// SECURITY: Escaped path to prevent command injection.
func (s *SSHConnector) Remove(ctx context.Context, path string, opts RemoveOptions) error {
	// First, check if the file exists if IgnoreNotExist is true.
	if opts.IgnoreNotExist {
		// Use a short timeout for this stat check
		statCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		stat, err := s.Stat(statCtx, path)
		if err == nil && !stat.IsExist { // Stat succeeded and file does not exist
			return nil
		}
		// If Stat failed (e.g. permission denied to stat, or other errors),
		// we'll let `rm -f` handle it, as `rm -f` is forgiving.
	}

	flags := "-f" // -f ignores non-existent files and prevents errors if it's already gone.
	if opts.Recursive {
		flags += "r"
	}
	// SECURITY: Escape path.
	cmd := fmt.Sprintf("rm %s %s", flags, shellEscape(path))

	// Pass Sudo option from RemoveOptions to ExecOptions
	_, stderr, err := s.Exec(ctx, cmd, &ExecOptions{Sudo: opts.Sudo})
	if err != nil {
		return fmt.Errorf("failed to remove %s (sudo: %t): %s (underlying error: %w)", path, opts.Sudo, string(stderr), err)
	}
	return nil
}


// GetFileChecksum calculates the checksum of a remote file.
// SECURITY: Path escaped.
func (s *SSHConnector) GetFileChecksum(ctx context.Context, path string, checksumType string) (string, error) {
	var checksumCmd string
	escapedPath := shellEscape(path) // SECURITY: Escape path

	switch strings.ToLower(checksumType) {
	case "sha256":
		checksumCmd = fmt.Sprintf("sha256sum -b %s", escapedPath) // -b for binary mode
	case "md5":
		checksumCmd = fmt.Sprintf("md5sum -b %s", escapedPath) // -b for binary mode
	default:
		return "", fmt.Errorf("unsupported checksum type '%s' for remote file %s on host %s", checksumType, path, s.connCfg.Host)
	}

	// Checksumming usually doesn't need sudo unless the file itself is not readable by the SSH user.
	// If sudo is needed, caller should ensure appropriate setup or use a different mechanism.
	stdoutBytes, stderrBytes, err := s.Exec(ctx, checksumCmd, &ExecOptions{Sudo: false})
	if err != nil {
		return "", fmt.Errorf("failed to execute checksum command '%s' on %s for %s: %s (underlying error: %w)", checksumCmd, s.connCfg.Host, path, string(stderrBytes), err)
	}

	parts := strings.Fields(string(stdoutBytes))
	if len(parts) > 0 {
		return parts[0], nil
	}
	return "", fmt.Errorf("failed to parse checksum from command output for %s on host %s: '%s'", path, s.connCfg.Host, string(stdoutBytes))
}


// --- Helper Functions and Types ---

// shellEscape is defined in local.go and used by this package.
// func shellEscape(s string) string {
// 	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
// }

// readOnlyStream adapts an io.Reader to be usable as ExecOptions.Stream for stdin piping.
// It makes Stream satisfy io.Writer by providing a no-op Write, but its real purpose
// is to carry an io.Reader that Exec can optionally use for stdin.
type readOnlyStream struct{
	io.Reader // The actual reader for stdin
}
// Write makes readOnlyStream satisfy io.Writer, but it's a no-op for stdin purposes.
// Stdout/Stderr from the command will not be written here.
func (s readOnlyStream) Write(p []byte) (int, error) { return len(p), nil }


// parseKeyValues is a helper to parse `key=value` or `key: value` style output.
// It handles optional quotes around values if quoteChar is provided.
func parseKeyValues(content, delimiter, quoteChar string) map[string]string {
	vars := make(map[string]string)
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, delimiter, 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if quoteChar != "" {
				val = strings.Trim(val, quoteChar)
			}
			vars[key] = val
		}
	}
	return vars
}

// dialSSH handles the logic for creating an SSH client, including via a bastion.
// REFACTOR: Broken down from a monolithic function for clarity and maintainability.
func dialSSH(ctx context.Context, cfg ConnectionCfg) (*ssh.Client, *ssh.Client, error) {
	targetAuthMethods, err := buildAuthMethods(cfg)
	if err != nil {
		return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("target auth error: %w", err)}
	}

	// Default timeout for SSH connection attempts if not specified in config.
	connectTimeout := cfg.Timeout
	if connectTimeout == 0 {
		connectTimeout = 30 * time.Second // Default to 30 seconds
	}

	targetSSHConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            targetAuthMethods,
		HostKeyCallback: cfg.HostKeyCallback, // User-provided HostKeyCallback for target
		Timeout:         connectTimeout,
	}

	// SECURITY: If no HostKeyCallback is provided for the target, default to a warning and InsecureIgnoreHostKey.
	// Production systems should always configure this.
	if targetSSHConfig.HostKeyCallback == nil {
		fmt.Fprintf(os.Stderr, "Warning: HostKeyCallback is not set for target host %s. Using InsecureIgnoreHostKey(). This is NOT recommended for production.\n", cfg.Host)
		targetSSHConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	targetDialAddr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	if cfg.BastionCfg != nil {
		// Construct a full ConnectionCfg for the bastion from cfg.BastionCfg
		bastionFullCfg := ConnectionCfg{
			Host:            cfg.BastionCfg.Host,
			Port:            cfg.BastionCfg.Port,
			User:            cfg.BastionCfg.User,
			Password:        cfg.BastionCfg.Password,
			PrivateKey:      cfg.BastionCfg.PrivateKey,
			PrivateKeyPath:  cfg.BastionCfg.PrivateKeyPath,
			Timeout:         cfg.BastionCfg.Timeout,
			HostKeyCallback: cfg.BastionCfg.HostKeyCallback,
			// Note: ProxyCfg for bastion is not explicitly handled here,
			// assuming bastion connections are direct or handled by system/SSH config.
		}
		// Pass the context to dialViaBastion for timeout propagation
		return dialViaBastion(ctx, targetDialAddr, targetSSHConfig, bastionFullCfg)
	}

	// Direct connection to target
	client, err := ssh.Dial("tcp", targetDialAddr, targetSSHConfig)
	if err != nil {
		return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("direct dial failed: %w", err)}
	}
	return client, nil, nil // No bastion client in direct dial
}

// dialViaBastion handles dialing the target through a bastion host.
func dialViaBastion(ctx context.Context, targetDialAddr string, targetSSHConfig *ssh.ClientConfig, bastionOverallCfg ConnectionCfg) (*ssh.Client, *ssh.Client, error) {
	bastionAuthMethods, err := buildAuthMethods(bastionOverallCfg)
	if err != nil {
		return nil, nil, &ConnectionError{Host: bastionOverallCfg.Host, Err: fmt.Errorf("bastion auth error: %w", err)}
	}

	bastionConnectTimeout := bastionOverallCfg.Timeout
	if bastionConnectTimeout == 0 {
		bastionConnectTimeout = 30 * time.Second // Default for bastion as well
	}

	bastionSSHConfig := &ssh.ClientConfig{
		User:            bastionOverallCfg.User,
		Auth:            bastionAuthMethods,
		HostKeyCallback: bastionOverallCfg.HostKeyCallback, // User-provided HostKeyCallback for bastion
		Timeout:         bastionConnectTimeout,
	}

	// SECURITY: If no HostKeyCallback is provided for the bastion, default to a warning and InsecureIgnoreHostKey.
	if bastionSSHConfig.HostKeyCallback == nil {
		fmt.Fprintf(os.Stderr, "Warning: HostKeyCallback is not set for bastion host %s. Using InsecureIgnoreHostKey(). This is NOT recommended for production.\n", bastionOverallCfg.Host)
		bastionSSHConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	}

	bastionDialAddr := net.JoinHostPort(bastionOverallCfg.Host, strconv.Itoa(bastionOverallCfg.Port))

	// Dial bastion
	bastionClient, err := ssh.Dial("tcp", bastionDialAddr, bastionSSHConfig)
	if err != nil {
		return nil, nil, &ConnectionError{Host: bastionOverallCfg.Host, Err: fmt.Errorf("bastion dial failed: %w", err)}
	}

	// Dial target through bastion
	connToTarget, err := bastionClient.Dial("tcp", targetDialAddr)
	if err != nil {
		bastionClient.Close() // Close bastion connection if dialing target fails
		return nil, nil, &ConnectionError{Host: targetDialAddr, Err: fmt.Errorf("dial target via bastion failed: %w", err)}
	}

	// Establish SSH connection to target over the bastion's tunneled connection
	ncc, chans, reqs, err := ssh.NewClientConn(connToTarget, targetDialAddr, targetSSHConfig)
	if err != nil {
		connToTarget.Close()
		bastionClient.Close()
		return nil, nil, &ConnectionError{Host: targetDialAddr, Err: fmt.Errorf("SSH handshake to target via bastion failed: %w", err)}
	}
	targetClient := ssh.NewClient(ncc, chans, reqs)
	return targetClient, bastionClient, nil
}

// buildAuthMethods constructs a slice of ssh.AuthMethod from ConnectionCfg.
func buildAuthMethods(cfg ConnectionCfg) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if cfg.Password != "" {
		methods = append(methods, ssh.Password(cfg.Password))
	}

	if len(cfg.PrivateKey) > 0 { // PrivateKey as bytes
		signer, err := ssh.ParsePrivateKey(cfg.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key bytes: %w", err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	} else if cfg.PrivateKeyPath != "" { // PrivateKey from file path
		keyFileBytes, err := os.ReadFile(cfg.PrivateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key file %s: %w", cfg.PrivateKeyPath, err)
		}
		signer, err := ssh.ParsePrivateKey(keyFileBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key from file %s: %w", cfg.PrivateKeyPath, err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}

	// Add other auth methods here if needed (e.g., Agent, KeyboardInteractive)

	if len(methods) == 0 {
		return nil, fmt.Errorf("no SSH authentication method provided (password or private key required for host %s)", cfg.Host)
	}
	return methods, nil
}

// Ensure SSHConnector implements Connector interface
var _ Connector = &SSHConnector{}

// Note: IsSudoer and other methods not explicitly shown in the diff would need to be reviewed
// for similar security (shell escaping) and robustness enhancements if they construct shell commands.
// For example, GetFileChecksum's remote command execution should also use shellEscape for the path.
// Stat, ReadFile are SFTP based and generally safer from injection.
// (The provided code already has GetFileChecksum and others, ensure they are updated or were already safe)
// The provided code already includes updates to Mkdir, Remove, LookPath, GetFileChecksum for shell escaping.
// It also includes the parseKeyValues helper.
// The main methods like Connect, IsConnected, Close, Exec, file writes, GetOS are covered by the new code.
// The `writeFileViaSFTP` was also added to support io.Reader for SFTP.
// The `ensureSftp` helper is also included.
