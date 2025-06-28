package connector

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs" // For Mkdir permissions
	"os"
	"os/exec"
	"path/filepath"
	"runtime" // To get local OS info
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Helper to escape paths for shell commands to prevent injection.
// A simple version for common cases.
func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// LocalConnector implements the Connector interface for local command execution.
type LocalConnector struct {
	connCfg  ConnectionCfg // Store for potential future use (e.g. if local connection needs config)
	cachedOS *OS
	// No actual "connection" is needed for local, so isConnected is effectively always true
	// after a successful (no-op) Connect call.
}

// Connect for LocalConnector is a no-op, as operations are performed locally.
// It can store the config if needed for any local-specific parameters in the future.
func (l *LocalConnector) Connect(ctx context.Context, cfg ConnectionCfg) error {
	l.connCfg = cfg
	// For local connector, connection is implicitly always available.
	return nil
}

// IsConnected for LocalConnector always returns true as it operates locally.
func (l *LocalConnector) IsConnected() bool {
	return true
}

// Close for LocalConnector is a no-op.
func (l *LocalConnector) Close() error {
	return nil
}

// Exec executes a command locally.
// Enhanced with robust retry logic and clearer structure.
func (l *LocalConnector) Exec(ctx context.Context, cmd string, options *ExecOptions) (stdout, stderr []byte, err error) {
	effectiveOptions := ExecOptions{} // Use a copy to avoid side effects
	if options != nil {
		effectiveOptions = *options
	}

	fullCmdString := cmd
	if effectiveOptions.Sudo {
		if l.connCfg.Password != "" {
			// Use -S to read password from stdin. -p '' prevents sudo from printing a default prompt.
			fullCmdString = "sudo -S -p '' -E -- " + cmd
		} else {
			fullCmdString = "sudo -E -- " + cmd
		}
	}

	// runOnce is a helper to execute the command a single time.
	runOnce := func(runCtx context.Context) ([]byte, []byte, error) {
		shell := []string{"/bin/sh", "-c"}
		if runtime.GOOS == "windows" {
			shell = []string{"cmd", "/C"}
		}

		actualCmd := exec.CommandContext(runCtx, shell[0], append(shell[1:], fullCmdString)...)

		if len(effectiveOptions.Env) > 0 {
			actualCmd.Env = append(os.Environ(), effectiveOptions.Env...)
		}

		// IMPORTANT: This Stdin logic is for `sudo -S` reading password.
		// It should not interfere with commands that genuinely need their own stdin,
		// like `tee` in the WriteFile/Copy sudo path (which now use direct exec.Command).
		if effectiveOptions.Sudo && l.connCfg.Password != "" && strings.HasPrefix(fullCmdString, "sudo -S") {
			actualCmd.Stdin = strings.NewReader(l.connCfg.Password + "\n")
		}

		var stdoutBuf, stderrBuf bytes.Buffer
		if effectiveOptions.Stream != nil {
			actualCmd.Stdout = io.MultiWriter(&stdoutBuf, effectiveOptions.Stream)
			actualCmd.Stderr = io.MultiWriter(&stderrBuf, effectiveOptions.Stream)
		} else {
			actualCmd.Stdout = &stdoutBuf
			actualCmd.Stderr = &stderrBuf
		}

		err := actualCmd.Run()
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), err
	}

	var finalErr error
	for i := 0; i <= effectiveOptions.Retries; i++ {
		// Create a new context for each attempt to handle timeouts correctly.
		attemptCtx := ctx
		var attemptCancel context.CancelFunc // Keep cancel func to call it explicitly

		if effectiveOptions.Timeout > 0 {
			// Use a new timeout for each attempt.
			attemptCtx, attemptCancel = context.WithTimeout(context.Background(), effectiveOptions.Timeout) // Use Background for timeout independence
		}

		stdout, stderr, err = runOnce(attemptCtx)

		if attemptCancel != nil { // Explicitly cancel the context for this attempt if it was created
			attemptCancel()
		}

		if err == nil {
			return stdout, stderr, nil // Success
		}

		finalErr = err // Store the last error

		// Don't retry if the context was cancelled (e.g., main context or attempt-specific timeout).
		// Check attemptCtx.Err() for attempt-specific timeout, and ctx.Err() for overall context cancellation.
		if attemptCtx.Err() != nil || (ctx.Err() != nil && ctx.Err() != context.Canceled && ctx.Err() != context.DeadlineExceeded) { // Check if the specific attempt timed out or main context done
			// If main context (ctx) is done, and it's not due to cancellation that might be part of a graceful shutdown, break.
			// This logic ensures that if the *overall* context is done (e.g. application shutting down), we don't keep retrying.
			// If only the *attemptCtx* is done (timeout for this specific try), that's also a reason to break if not successful.
			break
		}


		if i < effectiveOptions.Retries { // Only sleep if there are more retries planned
			if effectiveOptions.RetryDelay > 0 {
				time.Sleep(effectiveOptions.RetryDelay)
			}
		} else { // This was the last attempt (either initial try if Retries=0, or final retry)
			break
		}
	}

	// If we are here, all attempts failed. Wrap the last error.
	exitCode := -1
	if exitErr, ok := finalErr.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			exitCode = status.ExitStatus()
		}
	}
	// Ensure stdout and stderr from the last attempt are returned with the error.
	// This requires runOnce to return them even on error, which it does.
	return stdout, stderr, &CommandError{Cmd: cmd, ExitCode: exitCode, Stdout: string(stdout), Stderr: string(stderr), Underlying: finalErr}
}

// Copy copies a local file to another local path, with sudo support.
func (l *LocalConnector) Copy(ctx context.Context, srcPath, dstPath string, options *FileTransferOptions) error {
	opts := FileTransferOptions{} // Default empty options
	if options != nil {
		opts = *options // Copy to avoid modifying the original
	}

	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	// Open source file first to fail early if it doesn't exist.
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", srcPath, err)
	}
	defer srcFile.Close()

	if opts.Sudo {
		if runtime.GOOS == "windows" {
			return fmt.Errorf("sudo copy not supported on Windows for path %s", dstPath)
		}
		// Ensure destination directory exists using sudo mkdir -p
		destDir := filepath.Dir(dstPath)
		if destDir != "." && destDir != "/" {
			mkdirCmd := fmt.Sprintf("mkdir -p %s", shellEscape(destDir))
			_, _, err := l.Exec(ctx, mkdirCmd, &ExecOptions{Sudo: true})
			if err != nil {
				return fmt.Errorf("failed to create parent directory %s with sudo: %w", destDir, err)
			}
		}

		// Use `cat <src> | sudo tee <dst> > /dev/null` pattern.
		// `l.Exec` handles sudo password logic.
		// `tee` writes to the file and also to stdout. We redirect tee's stdout to /dev/null.
		// The content of srcFile needs to be piped to the command's stdin.
		// This is a bit complex with the current Exec method as it uses connCfg.Password for stdin with sudo -S.
		// The `cat` command will produce the content on its stdout, which needs to become stdin for `sudo tee`.

		// The example from the problem description uses `sudoStream{stdin: srcFile}` with ExecOptions.Stream.
		// This implies ExecOptions.Stream can sometimes act as Stdin.
		// Let's look at how `Exec` handles `Stream`. It's used for `io.MultiWriter` for Stdout/Stderr.
		// This `sudoStream` trick is clever if `Exec` can be adapted or if it already has a hidden stdin pipe feature.

		// The provided solution's `Copy` method (sudo path) uses:
		// cmd := fmt.Sprintf("tee %s > /dev/null", shellEscape(dstPath))
		// execOpts := &ExecOptions{ Sudo: true, Stream: &sudoStream{stdin: srcFile} }
		// _, stderr, err := l.Exec(ctx, cmd, execOpts)
		// This implies `Exec` needs to be aware of `sudoStream` or have a general way to use `Stream` as `Stdin`.
		// The current `Exec` in `local.go` doesn't show this behavior.
		// The `sudoStream` in the prompt solution is a placeholder for `Stdin` piping.
		// The `Exec` in the prompt's `LocalConnector` solution has this `actualCmd.Stdin = strings.NewReader(l.connCfg.Password + "\n")`
		// This is specific to password.
		// For `cat | sudo tee`, the `cat` part runs as current user, `sudo tee` as root.

		// Let's use a direct os/exec.Command approach here for clarity, similar to the WriteFile sudo path.
		// Command: sh -c "cat <srcPath> | sudo tee <dstPath> > /dev/null"
		// If sudo needs password: sh -c "cat <srcPath> | sudo -S -p '' tee <dstPath> > /dev/null" (and provide password to sudo's stdin)

		shell := []string{"/bin/sh", "-c"}
		var cmdStr string
		var actualCmd *exec.Cmd

		// We need to escape srcPath for cat and dstPath for tee.
		escapedSrcPath := shellEscape(srcPath)
		escapedDstPath := shellEscape(dstPath)

		if l.connCfg.Password != "" {
			cmdStr = fmt.Sprintf("cat %s | sudo -S -p '' -E -- tee %s > /dev/null", escapedSrcPath, escapedDstPath)
			actualCmd = exec.CommandContext(ctx, shell[0], append(shell[1:], cmdStr)...)
			actualCmd.Stdin = strings.NewReader(l.connCfg.Password + "\n") // sudo -S reads password from stdin
		} else {
			cmdStr = fmt.Sprintf("cat %s | sudo -E -- tee %s > /dev/null", escapedSrcPath, escapedDstPath)
			actualCmd = exec.CommandContext(ctx, shell[0], append(shell[1:], cmdStr)...)
			// No specific stdin needed beyond what `cat` produces for `tee` if no password for sudo.
		}

		var stderrBuf bytes.Buffer
		actualCmd.Stderr = &stderrBuf

		// `cat` part reads from srcFile, its stdout is piped to `sudo tee` by the shell.
		// So, `srcFile` itself is not `actualCmd.Stdin`.
		// The `cat` command handles reading `srcFile`.

		if errRun := actualCmd.Run(); errRun != nil {
			return fmt.Errorf("failed to copy with sudo (cat | tee) from %s to %s: %s (underlying error: %w)", srcPath, dstPath, stderrBuf.String(), errRun)
		}

	} else {
		// Standard, non-sudo copy
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil { // 0755 for directory permissions
			return fmt.Errorf("failed to create destination directory for %s: %w", dstPath, err)
		}

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return fmt.Errorf("failed to create destination file %s: %w", dstPath, err)
		}
		defer dstFile.Close()

		if _, err = io.Copy(dstFile, srcFile); err != nil {
			return fmt.Errorf("failed to copy content from %s to %s: %w", srcPath, dstPath, err)
		}
	}

	// Apply permissions after copy.
	if opts.Permissions != "" {
		perm, parseErr := strconv.ParseUint(opts.Permissions, 8, 32)
		if parseErr != nil {
			return fmt.Errorf("invalid permissions format '%s' for %s: %w", opts.Permissions, dstPath, parseErr)
		}

		if opts.Sudo {
			// Use l.Exec to run `sudo chmod`
			chmodCmdStr := fmt.Sprintf("chmod %s %s", opts.Permissions, shellEscape(dstPath))
			_, stderr, err := l.Exec(ctx, chmodCmdStr, &ExecOptions{Sudo: true})
			if err != nil {
				// For sudo chmod, a failure could be a warning if `tee` already set good permissions,
				// but it's safer to return an error as the requested permissions were not applied.
				return fmt.Errorf("failed to set permissions with sudo chmod on %s: %s (underlying error: %w)", dstPath, string(stderr), err)
			}
		} else {
			if errChmod := os.Chmod(dstPath, os.FileMode(perm)); errChmod != nil {
				return fmt.Errorf("failed to set permissions on %s: %w", dstPath, errChmod)
			}
		}
	}
	return nil
}

// CopyContent copies byte content to a destination file.
// It now calls the more generic WriteFile method, which includes sudo support.
func (l *LocalConnector) CopyContent(ctx context.Context, content []byte, dstPath string, options *FileTransferOptions) error {
	var permissions string
	var sudo bool
	effectiveTimeout := time.Duration(0)

	if options != nil {
		permissions = options.Permissions
		sudo = options.Sudo
		if options.Timeout > 0 {
			effectiveTimeout = options.Timeout
		}
	}

	if effectiveTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, effectiveTimeout)
		defer cancel()
	}

	return l.WriteFile(ctx, content, dstPath, permissions, sudo)
}

func (l *LocalConnector) Fetch(ctx context.Context, remotePath, localPath string) error {
	return l.Copy(ctx, remotePath, localPath, nil)
}

func (l *LocalConnector) Stat(ctx context.Context, path string) (*FileStat, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileStat{Name: filepath.Base(path), IsExist: false}, nil
		}
		return nil, fmt.Errorf("failed to stat local path %s: %w", path, err)
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

func (l *LocalConnector) LookPath(ctx context.Context, file string) (string, error) {
	return exec.LookPath(file)
}

func (l *LocalConnector) GetOS(ctx context.Context) (*OS, error) {
	if l.cachedOS != nil {
		return l.cachedOS, nil
	}
	osInfo := &OS{
		ID:   strings.ToLower(runtime.GOOS),
		Arch: runtime.GOARCH,
	}
	switch runtime.GOOS {
	case "linux":
		kernelCmd := exec.CommandContext(ctx, "uname", "-r")
		kernelOut, errKernel := kernelCmd.Output()
		if errKernel == nil {
			osInfo.Kernel = strings.TrimSpace(string(kernelOut))
		} else {
			fmt.Fprintf(os.Stderr, "warning: failed to get kernel version for local connector: %v\n", errKernel)
		}
		content, err := os.ReadFile("/etc/os-release")
		if err == nil {
			vars := make(map[string]string)
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					key := strings.TrimSpace(parts[0])
					val := strings.Trim(strings.TrimSpace(parts[1]), "\"")
					vars[key] = val
				}
			}
			if id, ok := vars["ID"]; ok { osInfo.ID = id }
			if verID, ok := vars["VERSION_ID"]; ok { osInfo.VersionID = verID }
			if name, ok := vars["PRETTY_NAME"]; ok { osInfo.PrettyName = name }
			if cname, ok := vars["VERSION_CODENAME"]; ok { osInfo.Codename = cname }
		} else {
			if osInfo.ID == "" { osInfo.ID = "linux" }
			if osInfo.PrettyName == "" { osInfo.PrettyName = "Linux" }
			fmt.Fprintf(os.Stderr, "warning: failed to read /etc/os-release for local connector: %v\n", err)
		}
	case "darwin":
		osInfo.ID = "darwin"
		swVersCmdName := exec.CommandContext(ctx, "sw_vers", "-productName")
		prodName, errProdName := swVersCmdName.Output()
		if errProdName == nil { osInfo.PrettyName = strings.TrimSpace(string(prodName)) }

		swVersCmdVersion := exec.CommandContext(ctx, "sw_vers", "-productVersion")
		prodVer, errProdVer := swVersCmdVersion.Output()
		if errProdVer == nil { osInfo.VersionID = strings.TrimSpace(string(prodVer)) }

		kernelCmd := exec.CommandContext(ctx, "uname", "-r")
		kernelOut, errKernel := kernelCmd.Output()
		if errKernel == nil { osInfo.Kernel = strings.TrimSpace(string(kernelOut)) }

		if osInfo.PrettyName == "" { osInfo.PrettyName = "macOS" }
	case "windows":
		osInfo.ID = "windows"
		osInfo.PrettyName = "Windows"
	default:
		if osInfo.ID == "" { osInfo.ID = runtime.GOOS }
		if osInfo.PrettyName == "" { osInfo.PrettyName = runtime.GOOS }
	}
	l.cachedOS = osInfo
	return l.cachedOS, nil
}

func (l *LocalConnector) ReadFile(ctx context.Context, path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read local file %s: %w", path, err)
	}
	return data, nil
}

// WriteFile writes content to a destination file, with sudo support.
func (l *LocalConnector) WriteFile(ctx context.Context, content []byte, destPath, permissions string, sudo bool) error {
	if sudo {
		if runtime.GOOS == "windows" {
			return fmt.Errorf("sudo write not supported on Windows for path %s", destPath)
		}
		// Ensure destination directory exists using sudo mkdir -p
		// This is important if destPath is in a directory that requires sudo to create.
		destDir := filepath.Dir(destPath)
		if destDir != "." && destDir != "/" {
			// We use l.Exec for this to handle sudo password if needed.
			// The command `mkdir -p` is generally safe and idempotent.
			// We don't capture stdout/stderr here as it's usually not problematic for mkdir.
			// A dedicated timeout for this operation might be too granular; it uses the parent context.
			mkdirCmd := fmt.Sprintf("mkdir -p %s", shellEscape(destDir))
			_, _, err := l.Exec(ctx, mkdirCmd, &ExecOptions{Sudo: true})
			if err != nil {
				// If mkdir fails (e.g. permission denied even with sudo, which is unlikely for mkdir -p but possible),
				// wrap the error.
				return fmt.Errorf("failed to create parent directory %s with sudo: %w", destDir, err)
			}
		}

		// Use `tee` to write the file content. `tee` writes to the file and also to stdout.
		// We redirect tee's stdout to /dev/null as we only care about the file write.
		// Removed unused 'cmd' variable below. finalCmdStr is used for the actual command.

		// We need to pass the content as stdin to the command.
		// The Exec method already handles sudo password input if `l.connCfg.Password` is set
		// and the command string starts with `sudo -S`.
		// We'll construct the ExecOptions to provide the content via a custom Stream
		// or by modifying Exec to accept an io.Reader for Stdin if that becomes cleaner.
		// For now, Exec options for sudo password handling should cover this if cmd is `sudo -S tee ...`
		// Let's adjust the cmd string directly for `sudo -S` if password is set.
		// Removed unused fullCmdString and execOpts from here, as direct exec.CommandContext is used below.

		// If a password is configured, Exec will prepend "sudo -S -p '' -E -- "
		// and provide the password via stdin.
		// The `tee` command itself doesn't need -S, it's `sudo` that does.
		// The `l.Exec` method should correctly form the `sudo -S ... tee ...` command.

		// We need to ensure the content is passed to `tee`'s stdin.
		// The current `Exec` function passes `l.connCfg.Password` to `sudo -S`.
		// It does not have a generic way to pass arbitrary data to the command's stdin *after* the password.
		// This is a limitation.
		// For `sudo tee`, `sudo` reads password, then `tee` reads from its stdin (which is now the original stdin).

		// Simplest way with current Exec:
		// 1. If password, `sudo -S -p '' tee ...` -> password from `l.connCfg.Password`
		// 2. `tee` needs `content` from its stdin.
		// This requires `Exec` to handle `Stdin` more flexibly.
		// The example solution directly calls `exec.CommandContext` in `WriteFile` for sudo.
		// Let's follow that pattern for directness here, as it avoids modifying Exec's Stdin general logic for now.

		shell := []string{"/bin/sh", "-c"}
		// `sudo -S -p '' -E -- tee /path > /dev/null`
		// The password will be `l.connCfg.Password + "\n"`
		// The content will be after that. This is WRONG. `sudo -S` consumes the password, then `tee` gets the rest.
		// So, `actualCmd.Stdin` should be `password\ncontent`.

		// The provided solution's `WriteFile` with sudo has a direct `exec.CommandContext` call.
		// Let's adapt that logic.
		// Removed unused sudoCmdStr. The finalCmdStr is constructed directly.
		var actualCmd *exec.Cmd
		// Note: The `> /dev/null` might be tricky with `exec.Command` as it's a shell feature.
		// Better to use `sh -c "sudo ... tee ... > /dev/null"`
		// Let's refine the command string for `sh -c`.

		var finalCmdStr string
		var stdinPipe io.Reader
		if l.connCfg.Password != "" {
			finalCmdStr = fmt.Sprintf("sudo -S -p '' -E -- tee %s > /dev/null", shellEscape(destPath))
			stdinPipe = strings.NewReader(l.connCfg.Password + "\n" + string(content))
		} else {
			finalCmdStr = fmt.Sprintf("sudo -E -- tee %s > /dev/null", shellEscape(destPath))
			stdinPipe = bytes.NewReader(content)
		}

		actualCmd = exec.CommandContext(ctx, shell[0], append(shell[1:], finalCmdStr)...)
		actualCmd.Stdin = stdinPipe

		var stderrBuf bytes.Buffer
		actualCmd.Stderr = &stderrBuf

		if err := actualCmd.Run(); err != nil {
			return fmt.Errorf("failed to write to %s with sudo tee: %s (underlying error: %w)", destPath, stderrBuf.String(), err)
		}

	} else {
		// Non-sudo: standard os.WriteFile after creating parent dirs.
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %w", destPath, err)
		}
		permMode := fs.FileMode(0644) // Default permission
		if permissions != "" {
			permVal, parseErr := strconv.ParseUint(permissions, 8, 32)
			if parseErr != nil {
				return fmt.Errorf("invalid permissions format '%s' for local WriteFile to %s: %w", permissions, destPath, parseErr)
			}
			permMode = fs.FileMode(permVal)
		}
		if err := os.WriteFile(destPath, content, permMode); err != nil {
			return fmt.Errorf("failed to write file %s: %w", destPath, err)
		}
	}

	// Apply permissions after copy, works for both sudo and non-sudo cases.
	// For sudo, this assumes the user running the command (even if it's root via sudo)
	// can chmod. `tee` itself might create the file with root ownership and default (umask-affected) permissions.
	// A separate `sudo chmod` command is more reliable for setting permissions with sudo.
	if permissions != "" {
		permVal, parseErr := strconv.ParseUint(permissions, 8, 32)
		if parseErr != nil {
			return fmt.Errorf("invalid permissions format '%s' for %s: %w", permissions, destPath, parseErr)
		}

		if sudo {
			// Use l.Exec to run `sudo chmod`
			chmodCmdStr := fmt.Sprintf("chmod %s %s", permissions, shellEscape(destPath))
			// Use a short timeout for chmod, or rely on parent context.
			// For simplicity, using parent context. Sudo is handled by l.Exec.
			_, stderr, err := l.Exec(ctx, chmodCmdStr, &ExecOptions{Sudo: true})
			if err != nil {
				return fmt.Errorf("failed to set permissions with sudo chmod on %s: %s (underlying error: %w)", destPath, string(stderr), err)
			}
		} else {
			// Non-sudo chmod
			if errChmod := os.Chmod(destPath, os.FileMode(permVal)); errChmod != nil {
				return fmt.Errorf("failed to set permissions on %s: %w", destPath, errChmod)
			}
		}
	}
	return nil
}

func (l *LocalConnector) GetFileChecksum(ctx context.Context, path string, checksumType string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open local file %s for checksum: %w", path, err)
	}
	defer file.Close()

	hasher, ok := getHasher(checksumType)
	if !ok {
		return "", fmt.Errorf("unsupported checksum type '%s' for local file %s", checksumType, path)
	}

	if _, err := io.Copy(hasher, file); err != nil { // hasher is already an io.Writer via the local hash interface
		return "", fmt.Errorf("failed to read local file %s for checksum calculation: %w", path, err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func (l *LocalConnector) Mkdir(ctx context.Context, path string, perm string) error {
	var mode fs.FileMode = 0755
	if perm != "" {
		parsedMode, err := strconv.ParseUint(perm, 8, 32)
		if err != nil {
			return fmt.Errorf("invalid permission format '%s' for Mkdir: %w", perm, err)
		}
		mode = fs.FileMode(parsedMode)
	}
	err := os.MkdirAll(path, mode)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}
	return nil
}

func (l *LocalConnector) Remove(ctx context.Context, path string, opts RemoveOptions) error {
	_, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			if opts.IgnoreNotExist {
				return nil
			}
			return fmt.Errorf("path %s does not exist: %w", path, err)
		}
		return fmt.Errorf("failed to stat path %s before removal: %w", path, err)
	}

	var removeErr error
	if opts.Recursive {
		removeErr = os.RemoveAll(path)
	} else {
		removeErr = os.Remove(path)
	}

	if removeErr != nil {
		return fmt.Errorf("failed to remove %s: %w", path, removeErr)
	}
	return nil
}

// hash is a local interface subset of crypto.Hash and io.Writer for getHasher.
type hash interface {
	io.Writer
	Sum(b []byte) []byte
}

// getHasher returns a new hash.Hash interface for the given checksum type.
func getHasher(checksumType string) (hash, bool) {
	switch strings.ToLower(checksumType) {
	case "sha256":
		return sha256.New(), true
	// case "md5": // Example if md5 were to be supported
	//	 return md5.New(), true
	default:
		return nil, false
	}
}

var _ Connector = &LocalConnector{}
