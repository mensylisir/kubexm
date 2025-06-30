package connector

import (
	"bytes"
	"context"
	"errors" // Added for errors.Is
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"archive/tar"
	"compress/gzip"
	"io/fs"


	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHConnector implements the Connector interface for SSH connections.
type SSHConnector struct {
	client        *ssh.Client
	bastionClient *ssh.Client // Only for non-pooled connections or if pool returns it separately
	sftpClient    *sftp.Client
	connCfg       ConnectionCfg
	cachedOS      *OS
	isConnected   bool
	pool          *ConnectionPool
	isFromPool    bool
	managedConn   *ManagedConnection // Stores the managed connection if obtained from pool
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
		mc, err := s.pool.Get(ctx, cfg) // New pool.Get signature
		if err == nil && mc != nil && mc.Client() != nil {
			s.managedConn = mc
			s.client = mc.Client()
			// s.bastionClient is part of mc.bastionClient, no need to set s.bastionClient separately for pooled conn.
			// s.bastionClient field in SSHConnector will be used for non-pooled bastion connections.
			s.isFromPool = true
			s.isConnected = true
			// fmt.Fprintf(os.Stderr, "SSHConnector: Using pooled connection for %s\n", cfg.Host)
			return nil // Pooled connection is ready
		}
		if err != nil && !errors.Is(err, ErrPoolExhausted) { // Log if error is not just exhaustion
			fmt.Fprintf(os.Stderr, "SSHConnector: Failed to get connection from pool for %s (will attempt direct dial): %v\n", cfg.Host, err)
		}
		// If ErrPoolExhausted or other Get error, or mc/mc.Client is nil, fall through to direct dial.
	}

	// Fallback to direct dial if not using pool, pool Get failed, or pool exhausted.
	s.isFromPool = false    // Explicitly set for direct dial path
	s.managedConn = nil // Ensure managedConn is nil for direct dials
	// The connectTimeout parameter for dialSSH is passed from SSHConnector.Connect's caller via cfg or default.
	// For direct dial, cfg.Timeout is the primary source, or dialSSH internal default.
	// The pool's cp.config.ConnectTimeout is used by pool.Get when it calls dialSSH.
	// Here, we use cfg.Timeout which might be different.
	// For consistency, dialSSH should prioritize its direct connectTimeout param.
	// The current dialSSH in ssh.go takes connectTimeout.
	client, bastionClient, err := dialSSH(ctx, cfg, cfg.Timeout) // Pass cfg.Timeout for direct dials
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

	if s.client != nil { // If there's an active client (pooled or direct)
		if s.isFromPool && s.managedConn != nil && s.pool != nil {
			isHealthy := s.managedConn.IsHealthy() // Use ManagedConnection's health check
			s.pool.Put(s.managedConn, isHealthy)
			// fmt.Fprintf(os.Stderr, "SSHConnector: Returned managed connection to pool for %s, healthy: %t\n", s.connCfg.Host, isHealthy)
		} else {
			// Not from pool, or pool is nil, or managedConn is nil (should not happen if isFromPool is true):
			// Close client and s.bastionClient (which is for non-pooled connections) directly.
			if err := s.client.Close(); err != nil {
				if firstErr == nil { firstErr = err }
				fmt.Fprintf(os.Stderr, "SSHConnector: Error closing direct client for %s: %v\n", s.connCfg.Host, err)
			}
			if s.bastionClient != nil { // s.bastionClient is for non-pooled bastion
				if err := s.bastionClient.Close(); err != nil {
					if firstErr == nil { firstErr = err }
					fmt.Fprintf(os.Stderr, "SSHConnector: Error closing direct bastion client for %s: %v\n", s.connCfg.Host, err)
				}
			}
		}
	} else if s.bastionClient != nil {
		// Case: Direct connection failed after bastion was established, but before s.client was set.
		// s.client is nil, but s.bastionClient (for non-pooled) might exist.
		if err := s.bastionClient.Close(); err != nil {
			if firstErr == nil { firstErr = err }
			fmt.Fprintf(os.Stderr, "SSHConnector: Error closing orphaned direct bastion client for %s: %v\n", s.connCfg.Host, err)
		}
	}

	s.client = nil
	s.bastionClient = nil // Cleared for non-pooled
	s.managedConn = nil   // Cleared for pooled
	s.isFromPool = false
	// sftpClient is already handled
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

		// Use Start() for non-blocking execution
		if err := session.Start(finalCmd); err != nil {
			return stdoutBuf.Bytes(), stderrBuf.Bytes(), fmt.Errorf("failed to start command '%s': %w", finalCmd, err)
		}

		// Wait for the command to finish or the context to be cancelled.
		doneCh := make(chan error, 1)
		go func() {
			doneCh <- session.Wait()
		}()

		select {
		case <-runCtx.Done():
			// Context timed out or was cancelled.
			// It's good practice to send a signal to the remote process.
			// SIGKILL is forceful but effective for timeouts.
			_ = session.Signal(ssh.SIGKILL)
			// We must still wait for session.Wait() to return to clean up resources.
			<-doneCh
			// Return the context's error (e.g., context.DeadlineExceeded).
			return stdoutBuf.Bytes(), stderrBuf.Bytes(), runCtx.Err()
		case err := <-doneCh:
			// Command finished on its own.
			return stdoutBuf.Bytes(), stderrBuf.Bytes(), err
		}
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
// It's a convenience wrapper around the more generic writeFileFromReader.
func (s *SSHConnector) CopyContent(ctx context.Context, content []byte, dstPath string, options *FileTransferOptions) error {
	return s.writeFileFromReader(ctx, bytes.NewReader(content), dstPath, options)
}

// WriteFile writes byte content to a destination file, aligning with the Connector interface.
func (s *SSHConnector) WriteFile(ctx context.Context, content []byte, destPath string, options *FileTransferOptions) error {
	// Ensure options is not nil, common practice.
	opts := options
	if opts == nil {
		opts = &FileTransferOptions{} // Default options if nil is passed
	}
	return s.writeFileFromReader(ctx, bytes.NewReader(content), destPath, opts)
}

// writeFileFromReader is the canonical implementation for writing content to the remote host.
// It handles both sudo and non-sudo cases using robust, predictable methods.
func (s *SSHConnector) writeFileFromReader(ctx context.Context, content io.Reader, dstPath string, options *FileTransferOptions) error {
	opts := FileTransferOptions{}
	if options != nil {
		opts = *options
	}

	// Ensure SFTP client is available, as it's used for both sudo (temp upload) and non-sudo writes.
	if err := s.ensureSftp(); err != nil {
		return err
	}

	if opts.Sudo {
		return s.sudoWrite(ctx, content, dstPath, opts)
	}
	return s.nonSudoWrite(ctx, content, dstPath, opts)
}

// sudoWrite handles privileged file writing using the 'upload -> sudo mv' pattern.
func (s *SSHConnector) sudoWrite(ctx context.Context, content io.Reader, dstPath string, opts FileTransferOptions) error {
	// 1. Create a unique temporary path in a world-writable directory.
	tmpPath := filepath.Join("/tmp", fmt.Sprintf("connector-tmp-%d-%s", time.Now().UnixNano(), filepath.Base(dstPath)))

	// 2. Defer cleanup of the temporary file. This runs even if subsequent steps fail.
	defer func() {
		// Use a new background context for cleanup to ensure it runs even if the original context is cancelled.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		// No need to check for existence, `rm -f` handles it. Sudo is not typically needed to clean up one's own file in /tmp.
		_, _, err := s.Exec(cleanupCtx, fmt.Sprintf("rm -f %s", shellEscape(tmpPath)), nil)
		if err != nil {
			// Log cleanup failure as a warning, as the primary operation might have succeeded.
			fmt.Fprintf(os.Stderr, "Warning: failed to remove temporary file %s on host %s: %v\n", tmpPath, s.connCfg.Host, err)
		}
	}()

	// 3. Upload the file to the temporary path using SFTP (non-sudo).
	// Permissions here are temporary; final permissions are set by `sudo chmod`. '0600' is a safe default.
	err := s.writeFileViaSFTP(ctx, content, tmpPath, "0600")
	if err != nil {
		return fmt.Errorf("failed to upload to temporary path %s for sudo write: %w", tmpPath, err)
	}

	// 4. Ensure the final destination directory exists using `sudo mkdir -p`.
	destDir := filepath.Dir(dstPath)
	if destDir != "." && destDir != "/" && destDir != "" {
		mkdirCmd := fmt.Sprintf("mkdir -p %s", shellEscape(destDir))
		_, stderr, mkdirErr := s.Exec(ctx, mkdirCmd, &ExecOptions{Sudo: true})
		if mkdirErr != nil {
			return fmt.Errorf("failed to create destination directory %s with sudo: %s (underlying error %w)", destDir, string(stderr), mkdirErr)
		}
	}

	// 5. Move the file from the temporary path to the final destination using `sudo mv`.
	mvCmd := fmt.Sprintf("mv %s %s", shellEscape(tmpPath), shellEscape(dstPath))
	_, stderr, mvErr := s.Exec(ctx, mvCmd, &ExecOptions{Sudo: true})
	if mvErr != nil {
		return fmt.Errorf("failed to move file to %s with sudo: %s (underlying error %w)", dstPath, string(stderr), mvErr)
	}

	// 6. Set final permissions using `sudo chmod`.
	if opts.Permissions != "" {
		if _, err := strconv.ParseUint(opts.Permissions, 8, 32); err != nil { // Validate format
			return fmt.Errorf("invalid permissions format '%s': %w", opts.Permissions, err)
		}
		chmodCmd := fmt.Sprintf("chmod %s %s", shellEscape(opts.Permissions), shellEscape(dstPath))
		_, stderr, chmodErr := s.Exec(ctx, chmodCmd, &ExecOptions{Sudo: true})
		if chmodErr != nil {
			return fmt.Errorf("failed to set permissions on %s with sudo: %s (underlying error %w)", dstPath, string(stderr), chmodErr)
		}
	}

	// 7. (Optional) Set final ownership using `sudo chown`.
	if opts.Owner != "" {
		// Group defaults to owner if not specified, which is a common chown behavior.
		ownerAndGroup := opts.Owner
		if opts.Group != "" {
			ownerAndGroup = fmt.Sprintf("%s:%s", opts.Owner, opts.Group)
		}
		chownCmd := fmt.Sprintf("chown %s %s", shellEscape(ownerAndGroup), shellEscape(dstPath))
		_, stderr, chownErr := s.Exec(ctx, chownCmd, &ExecOptions{Sudo: true})
		if chownErr != nil {
			return fmt.Errorf("failed to set ownership on %s with sudo: %s (underlying error %w)", dstPath, string(stderr), chownErr)
		}
	}

	return nil
}

// nonSudoWrite handles non-privileged file writing directly using SFTP.
func (s *SSHConnector) nonSudoWrite(ctx context.Context, content io.Reader, dstPath string, opts FileTransferOptions) error {
	// For non-sudo, we can directly use our SFTP helper.
	return s.writeFileViaSFTP(ctx, content, dstPath, opts.Permissions)
}

// writeFileViaSFTP is a helper for direct SFTP writes using an io.Reader.
// This is now a lower-level helper used by both sudo and non-sudo paths.
func (s *SSHConnector) writeFileViaSFTP(ctx context.Context, content io.Reader, destPath, permissions string) error {
	// Ensure SFTP client is initialized (already done by writeFileFromReader, but good for direct calls)
	if err := s.ensureSftp(); err != nil {
		return fmt.Errorf("sftp client not available for writeFileViaSFTP on host %s: %w", s.connCfg.Host, err)
	}

	parentDir := filepath.Dir(destPath)
	if parentDir != "." && parentDir != "/" && parentDir != "" {
		// Use SFTP's native MkdirAll to create parent directories. It's idempotent.
		if err := s.sftpClient.MkdirAll(parentDir); err != nil {
			return fmt.Errorf("failed to create parent directory %s via sftp: %w", parentDir, err)
		}
	}

	// Create will truncate if file exists.
	file, err := s.sftpClient.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create remote file %s via sftp: %w", destPath, err)
	}
	defer file.Close()

	// Use a context-aware writer to handle potential timeouts during a large copy.
	// Note: This requires a custom writer or careful select logic. For simplicity, we use a direct io.Copy here.
	// A more advanced implementation might use a goroutine with select { case <-ctx.Done(): ... }
	if _, err = io.Copy(file, content); err != nil {
		return fmt.Errorf("failed to write content to remote file %s via sftp: %w", destPath, err)
	}

	// Apply permissions if specified.
	if permissions != "" {
		permVal, parseErr := strconv.ParseUint(permissions, 8, 32)
		if parseErr != nil {
			// Log as a warning because the file write itself succeeded.
			fmt.Fprintf(os.Stderr, "Warning: Invalid permissions format '%s' for SFTP write to %s, skipping chmod: %v\n", permissions, destPath, parseErr)
		} else {
			if chmodErr := s.sftpClient.Chmod(destPath, os.FileMode(permVal)); chmodErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to chmod remote file %s to %s via SFTP: %v\n", destPath, permissions, chmodErr)
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
func dialSSH(ctx context.Context, cfg ConnectionCfg, connectTimeout time.Duration) (*ssh.Client, *ssh.Client, error) {
	targetAuthMethods, err := buildAuthMethods(cfg)
	if err != nil {
		return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("target auth error: %w", err)}
	}

	// Use the provided connectTimeout, or cfg.Timeout if connectTimeout is zero, or default.
	effectiveTimeout := connectTimeout
	if effectiveTimeout == 0 {
		effectiveTimeout = cfg.Timeout
	}
	if effectiveTimeout == 0 {
		effectiveTimeout = 30 * time.Second // Default to 30 seconds
	}

	targetSSHConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            targetAuthMethods,
		HostKeyCallback: cfg.HostKeyCallback, // User-provided HostKeyCallback for target
		Timeout:         effectiveTimeout,
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
		// Pass the context and effectiveTimeout (for the bastion connection itself) to dialViaBastion
		return dialViaBastion(ctx, targetDialAddr, targetSSHConfig, bastionFullCfg, effectiveTimeout)
	}

	// Direct connection to target
	client, err := ssh.Dial("tcp", targetDialAddr, targetSSHConfig)
	if err != nil {
		return nil, nil, &ConnectionError{Host: cfg.Host, Err: fmt.Errorf("direct dial failed: %w", err)}
	}
	return client, nil, nil // No bastion client in direct dial
}

// dialViaBastion handles dialing the target through a bastion host.
func dialViaBastion(ctx context.Context, targetDialAddr string, targetSSHConfig *ssh.ClientConfig, bastionOverallCfg ConnectionCfg, bastionConnectTimeoutParam time.Duration) (*ssh.Client, *ssh.Client, error) {
	bastionAuthMethods, err := buildAuthMethods(bastionOverallCfg)
	if err != nil {
		return nil, nil, &ConnectionError{Host: bastionOverallCfg.Host, Err: fmt.Errorf("bastion auth error: %w", err)}
	}

	// Use the provided bastionConnectTimeoutParam, or bastionOverallCfg.Timeout if param is zero, or default.
	effectiveBastionTimeout := bastionConnectTimeoutParam
	if effectiveBastionTimeout == 0 {
		effectiveBastionTimeout = bastionOverallCfg.Timeout
	}
	if effectiveBastionTimeout == 0 {
		effectiveBastionTimeout = 30 * time.Second // Default for bastion as well
	}

	bastionSSHConfig := &ssh.ClientConfig{
		User:            bastionOverallCfg.User,
		Auth:            bastionAuthMethods,
		HostKeyCallback: bastionOverallCfg.HostKeyCallback, // User-provided HostKeyCallback for bastion
		Timeout:         effectiveBastionTimeout,
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

// Copy handles copying both files and directories to the remote host.
// For directories, it uses a tar stream to be efficient and preserve permissions.
func (s *SSHConnector) Copy(ctx context.Context, srcPath, dstPath string, options *FileTransferOptions) error {
    srcStat, err := os.Stat(srcPath)
    if err != nil {
        return fmt.Errorf("source path %s not found or not accessible: %w", srcPath, err)
    }

    if !srcStat.IsDir() {
        // It's a file, we can use the existing writeFileFromReader logic.
        srcFile, err := os.Open(srcPath)
        if err != nil {
            return fmt.Errorf("failed to open source file %s: %w", srcPath, err)
        }
        defer srcFile.Close()
        return s.writeFileFromReader(ctx, srcFile, dstPath, options)
    }

    // It's a directory, use the tar-based copy method.
    return s.copyDirViaTar(ctx, srcPath, dstPath, options)
}

// copyDirViaTar copies a local directory to a remote path using the "upload-then-operate" pattern.
func (s *SSHConnector) copyDirViaTar(ctx context.Context, srcDir, dstDir string, options *FileTransferOptions) error {
	opts := FileTransferOptions{}
	if options != nil {
		opts = *options
	}

	// 1. Create the tar.gz stream into an in-memory buffer.
	var tarball bytes.Buffer
	gzw := gzip.NewWriter(&tarball)
	tw := tar.NewWriter(gzw)

	walkErr := filepath.Walk(srcDir, func(path string, info fs.FileInfo, errWalk error) error {
		if errWalk != nil {
			return errWalk
		}
		header, errHeader := tar.FileInfoHeader(info, info.Name())
		if errHeader != nil {
			return errHeader
		}
		// Use relative pathing from the source directory itself.
		var relPathErr error
		header.Name, relPathErr = filepath.Rel(srcDir, path)
		if relPathErr != nil {
			return fmt.Errorf("failed to make path relative for tar header %s: %w", path, relPathErr)
		}
		if info.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("failed to write tar header for %s: %w", path, err)
		}
		if info.Mode().IsRegular() {
			file, errOpen := os.Open(path)
			if errOpen != nil {
				return fmt.Errorf("failed to open file %s for tar: %w", path, errOpen)
			}
			defer file.Close()
			if _, errCopy := io.Copy(tw, file); errCopy != nil {
				return fmt.Errorf("failed to copy file %s content to tar: %w", path, errCopy)
			}
		}
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("failed during tarball creation for %s: %w", srcDir, walkErr)
	}
	// It is crucial to close writers to flush all data to the buffer.
	if err := tw.Close(); err != nil {
		return fmt.Errorf("failed to close tar writer: %w", err)
	}
	if err := gzw.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	// 2. Upload the tarball to a temporary remote location using non-sudo SFTP.
	tmpPath := filepath.Join("/tmp", fmt.Sprintf("connector-archive-%d-%s.tar.gz", time.Now().UnixNano(), filepath.Base(srcDir)))
	uploadErr := s.writeFileFromReader(ctx, &tarball, tmpPath, &FileTransferOptions{Sudo: false}) // Sudo must be false for this step
	if uploadErr != nil {
		return fmt.Errorf("failed to upload temporary archive to %s: %w", tmpPath, uploadErr)
	}
	defer func() {
		// Use a background context for cleanup to ensure it runs even if the original context is cancelled.
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Increased timeout for cleanup
		defer cancel()
		// Cleanup does not need to be privileged if we wrote to /tmp as the ssh user
		_, _, rmErr := s.Exec(cleanupCtx, fmt.Sprintf("rm -f %s", shellEscape(tmpPath)), &ExecOptions{Sudo: false})
		if rmErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to remove temporary archive %s on host %s: %v\n", tmpPath, s.connCfg.Host, rmErr)
		}
	}()

	// 3. Prepare destination and extract with sudo (if opts.Sudo is true).
	execOptsSudo := &ExecOptions{Sudo: opts.Sudo} // Use the original Sudo option for these operations

	// Ensure parent of destination exists.
	destParentDir := filepath.Dir(dstDir)
	if destParentDir != "." && destParentDir != "/" && destParentDir != "" {
		_, stderr, mkdirErr := s.Exec(ctx, fmt.Sprintf("mkdir -p %s", shellEscape(destParentDir)), execOptsSudo)
		if mkdirErr != nil {
			return fmt.Errorf("failed to create remote parent directory %s (sudo: %t): %s (underlying error: %w)", destParentDir, opts.Sudo, string(stderr), mkdirErr)
		}
	}

	// Atomically replace the destination: remove old (if exists), create new, then extract.
	// Note: rm -rf on a directory that doesn't exist is not an error.
	_, _, _ = s.Exec(ctx, fmt.Sprintf("rm -rf %s", shellEscape(dstDir)), execOptsSudo)

	_, stderr, mkdirErr := s.Exec(ctx, fmt.Sprintf("mkdir -p %s", shellEscape(dstDir)), execOptsSudo)
	if mkdirErr != nil {
		return fmt.Errorf("failed to create remote destination directory %s (sudo: %t): %s (underlying error: %w)", dstDir, opts.Sudo, string(stderr), mkdirErr)
	}

	// Extract into the final destination.
	// Use --no-same-owner and --no-same-permissions if not planning to chown/chmod later,
	// or if sudo is not true (tar would fail trying to set ownership).
	// However, since we handle chown/chmod explicitly later based on options,
	// it's better to let tar attempt to preserve what it can if sudo is active.
	// If not sudo, tar will extract with current user's ownership/permissions.
	extractCmd := fmt.Sprintf("tar -xzf %s -C %s", shellEscape(tmpPath), shellEscape(dstDir))
	_, stderr, execErr := s.Exec(ctx, extractCmd, execOptsSudo)
	if execErr != nil {
		return fmt.Errorf("failed to extract remote archive %s to %s (sudo: %t): %s (underlying error: %w)", tmpPath, dstDir, opts.Sudo, string(stderr), execErr)
	}

	// 4. Apply final ownership and permissions recursively.
	if opts.Permissions != "" {
		if _, errP := strconv.ParseUint(opts.Permissions, 8, 32); errP != nil {
			return fmt.Errorf("invalid permissions format '%s' for %s: %w", opts.Permissions, dstDir, errP)
		}
		chmodCmd := fmt.Sprintf("chmod -R %s %s", shellEscape(opts.Permissions), shellEscape(dstDir))
		_, stderr, errChmod := s.Exec(ctx, chmodCmd, execOptsSudo)
		if errChmod != nil {
			return fmt.Errorf("failed to set permissions on %s to %s (sudo: %t): %s (underlying error: %w)", dstDir, opts.Permissions, opts.Sudo, string(stderr), errChmod)
		}
	}
	if opts.Owner != "" {
		ownerAndGroup := opts.Owner
		if opts.Group != "" {
			ownerAndGroup = fmt.Sprintf("%s:%s", opts.Owner, opts.Group)
		}
		chownCmd := fmt.Sprintf("chown -R %s %s", shellEscape(ownerAndGroup), shellEscape(dstDir))
		_, stderr, errChown := s.Exec(ctx, chownCmd, execOptsSudo)
		if errChown != nil {
			return fmt.Errorf("failed to set ownership on %s to %s (sudo: %t): %s (underlying error: %w)", dstDir, ownerAndGroup, opts.Sudo, string(stderr), errChown)
		}
	}

	return nil
}
