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

// Copy copies a local file or directory to another local path, with sudo support.
func (l *LocalConnector) Copy(ctx context.Context, srcPath, dstPath string, options *FileTransferOptions) error {
    opts := FileTransferOptions{}
    if options != nil {
        opts = *options
    }

    // Apply timeout to the entire Copy operation if specified
    var cancel context.CancelFunc
    if opts.Timeout > 0 {
        ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
        defer cancel() // Ensure cancel is called to free resources
    }

    srcStat, err := os.Stat(srcPath)
    if err != nil {
        return fmt.Errorf("source path %s does not exist or is not accessible: %w", srcPath, err)
    }

    if !opts.Sudo {
        // Non-sudo: Use a simple recursive copy.
        if srcStat.IsDir() {
            return l.copyDir(srcPath, dstPath, opts)
        }
        return l.copyFile(srcPath, dstPath, opts)
    }

    // Sudo mode: copy to temp -> sudo mv
    // Create a temporary directory to stage the copy.
    tmpDir, err := os.MkdirTemp("", "localconnector-copy-")
    if err != nil {
        return fmt.Errorf("failed to create temporary directory: %w", err)
    }
    defer os.RemoveAll(tmpDir)

    // Stage the copy into the temporary directory. The destination name inside tmpDir is the basename of the original source.
    stagedPath := filepath.Join(tmpDir, filepath.Base(srcPath))

    // Use non-sudo copy to stage the file/dir into the temp location.
    // For staging, use default permissions; final permissions are applied by sudo.
    stagingOpts := FileTransferOptions{} // No specific perms/owner/group for staging copy itself

    if srcStat.IsDir() {
        if err := l.copyDir(srcPath, stagedPath, stagingOpts); err != nil {
            return fmt.Errorf("failed to stage directory %s to %s: %w", srcPath, stagedPath, err)
        }
    } else {
        if err := l.copyFile(srcPath, stagedPath, stagingOpts); err != nil {
            return fmt.Errorf("failed to stage file %s to %s: %w", srcPath, stagedPath, err)
        }
    }

    // Ensure final destination parent directory exists using sudo.
    destParentDir := filepath.Dir(dstPath)
    if destParentDir != "." && destParentDir != "/" && destParentDir != "" { // Check if it's not current or root
        mkdirCmd := fmt.Sprintf("mkdir -p %s", shellEscape(destParentDir))
        _, stderr, mkdirErr := l.Exec(ctx, mkdirCmd, &ExecOptions{Sudo: true})
        if mkdirErr != nil {
            return fmt.Errorf("failed to create destination parent directory %s with sudo: %s (underlying error %w)", destParentDir, string(stderr), mkdirErr)
        }
    }

    // Move the staged content to the final destination using sudo.
    mvCmd := fmt.Sprintf("mv %s %s", shellEscape(stagedPath), shellEscape(dstPath))
    _, stderr, mvErr := l.Exec(ctx, mvCmd, &ExecOptions{Sudo: true})
    if mvErr != nil {
        return fmt.Errorf("failed to move staged content from %s to %s with sudo: %s (underlying error %w)", stagedPath, dstPath, string(stderr), mvErr)
    }

    // Apply final permissions and ownership using sudo.
    return l.applySudoPermissions(ctx, dstPath, opts)
}

// copyFile is a non-sudo helper for copying a single file.
func (l *LocalConnector) copyFile(src, dst string, opts FileTransferOptions) error {
    srcFile, err := os.Open(src)
    if err != nil {
        return fmt.Errorf("failed to open source file %s for copyFile: %w", src, err)
    }
    defer srcFile.Close()

    if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil { // Sensible default for MkdirAll
        return fmt.Errorf("failed to create destination directory %s for copyFile: %w", filepath.Dir(dst), err)
    }

    dstFile, err := os.Create(dst)
    if err != nil {
        return fmt.Errorf("failed to create destination file %s for copyFile: %w", dst, err)
    }
    defer dstFile.Close()

    if _, err := io.Copy(dstFile, srcFile); err != nil {
        return fmt.Errorf("failed to copy content from %s to %s: %w", src, dst, err)
    }

    if opts.Permissions != "" {
        perm, parseErr := strconv.ParseUint(opts.Permissions, 8, 32)
        if parseErr != nil {
            return fmt.Errorf("invalid permissions format '%s' for %s: %w", opts.Permissions, dst, parseErr)
        }
        if err := os.Chmod(dst, os.FileMode(perm)); err != nil {
            return fmt.Errorf("failed to set permissions on %s: %w", dst, err)
        }
    }
    // Note: Non-sudo copy does not handle Owner/Group.
    return nil
}

// copyDir is a non-sudo helper for recursively copying a directory.
func (l *LocalConnector) copyDir(src, dst string, opts FileTransferOptions) error {
    srcInfo, err := os.Stat(src)
    if err != nil {
        return fmt.Errorf("failed to stat source directory %s for copyDir: %w", src, err)
    }
    // Create destination directory with source directory's permissions.
    if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
        return fmt.Errorf("failed to create destination directory %s for copyDir: %w", dst, err)
    }

    entries, err := os.ReadDir(src)
    if err != nil {
        return fmt.Errorf("failed to read source directory %s for copyDir: %w", src, err)
    }

    for _, entry := range entries {
        srcPath := filepath.Join(src, entry.Name())
        // Corrected filepath.join to filepath.Join
        dstPath := filepath.Join(dst, entry.Name())

        if entry.IsDir() {
            // For subdirectories, pass along the original options (e.g., for permissions on files within)
            // though copyDir itself mainly uses srcInfo.Mode() for directory creation.
            // The file copy part will use opts.Permissions if set.
            if err := l.copyDir(srcPath, dstPath, opts); err != nil {
                return err // Error already wrapped by recursive call
            }
        } else {
            // This could also handle symlinks, etc. For now, just files.
            // Pass original opts to copyFile so file permissions are applied if specified.
            if err := l.copyFile(srcPath, dstPath, opts); err != nil {
                return err // Error already wrapped by copyFile
            }
        }
    }
    return nil
}

// applySudoPermissions is a helper to apply final permissions/ownership via `sudo`.
func (l *LocalConnector) applySudoPermissions(ctx context.Context, path string, opts FileTransferOptions) error {
    if opts.Permissions != "" {
        if _, parseErr := strconv.ParseUint(opts.Permissions, 8, 32); parseErr != nil {
             return fmt.Errorf("invalid permissions format '%s' for applySudoPermissions on %s: %w", opts.Permissions, path, parseErr)
        }
        chmodCmd := fmt.Sprintf("chmod %s %s", shellEscape(opts.Permissions), shellEscape(path))
        _, stderr, err := l.Exec(ctx, chmodCmd, &ExecOptions{Sudo: true})
        if err != nil {
            return fmt.Errorf("failed to set permissions on %s with sudo chmod: %s (underlying error %w)", path, string(stderr), err)
        }
    }
    if opts.Owner != "" {
        ownerAndGroup := opts.Owner
		if opts.Group != "" {
			ownerAndGroup = fmt.Sprintf("%s:%s", opts.Owner, opts.Group)
		}
        // Use -R for chown if it's a directory, check this.
        // For simplicity, if we copied a dir, it's likely we want recursive chown.
        // Stat the path to see if it's a directory.
        targetStat, statErr := os.Stat(path) // Local stat after mv
        chownFlags := ""
        if statErr == nil && targetStat.IsDir() {
            chownFlags = "-R"
        }

        chownCmd := fmt.Sprintf("chown %s %s %s", chownFlags, shellEscape(ownerAndGroup), shellEscape(path))
        chownCmd = strings.TrimSpace(strings.ReplaceAll(chownCmd, "  ", " ")) // Clean up potential double space if chownFlags is empty

        _, stderr, err := l.Exec(ctx, chownCmd, &ExecOptions{Sudo: true})
        if err != nil {
            return fmt.Errorf("failed to set ownership on %s with sudo chown: %s (underlying error %w)", path, string(stderr), err)
        }
    }
    return nil
}


// CopyContent for LocalConnector, now uses the `upload -> mv` pattern for sudo.
func (l *LocalConnector) CopyContent(ctx context.Context, content []byte, dstPath string, options *FileTransferOptions) error {
    opts := FileTransferOptions{}
    if options != nil {
        opts = *options
    }

    // Apply timeout to the entire CopyContent operation if specified
    var cancel context.CancelFunc
    if opts.Timeout > 0 {
        ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
        defer cancel() // Ensure cancel is called to free resources
    }

    if !opts.Sudo {
        permMode := fs.FileMode(0644) // Default perm
        if opts.Permissions != "" {
            if perm, err := strconv.ParseUint(opts.Permissions, 8, 32); err == nil {
                permMode = fs.FileMode(perm)
            } else {
                 return fmt.Errorf("invalid permissions format '%s' for CopyContent to %s: %w", opts.Permissions, dstPath, err)
            }
        }
        if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil { // Sensible default for MkdirAll
            return fmt.Errorf("failed to create destination directory %s for CopyContent: %w", filepath.Dir(dstPath), err)
        }
        return os.WriteFile(dstPath, content, permMode)
    }

    // Sudo mode: write to temp file -> sudo mv
    tmpFile, err := os.CreateTemp("", "localconnector-content-")
    if err != nil {
        return fmt.Errorf("failed to create temporary file: %w", err)
    }
    defer os.Remove(tmpFile.Name()) // Cleanup

    if _, err := tmpFile.Write(content); err != nil {
        tmpFile.Close()
        return fmt.Errorf("failed to write content to temporary file %s: %w", tmpFile.Name(), err)
    }
    if err := tmpFile.Close(); err != nil { // Close before mv
        return fmt.Errorf("failed to close temporary file %s: %w", tmpFile.Name(), err)
    }


    // Ensure final destination parent directory exists using sudo.
    destParentDir := filepath.Dir(dstPath)
    if destParentDir != "." && destParentDir != "/" && destParentDir != "" {
        mkdirCmd := fmt.Sprintf("mkdir -p %s", shellEscape(destParentDir))
        _, stderr, mkdirErr := l.Exec(ctx, mkdirCmd, &ExecOptions{Sudo: true})
        if mkdirErr != nil {
            return fmt.Errorf("failed to create destination parent directory %s with sudo: %s (underlying error %w)", destParentDir, string(stderr), mkdirErr)
        }
    }

    // Move the temp file to the final destination using sudo.
    mvCmd := fmt.Sprintf("mv %s %s", shellEscape(tmpFile.Name()), shellEscape(dstPath))
    _, stderr, mvErr := l.Exec(ctx, mvCmd, &ExecOptions{Sudo: true})
    if mvErr != nil {
        return fmt.Errorf("failed to move temporary file from %s to %s with sudo: %s (underlying error %w)", tmpFile.Name(), dstPath, string(stderr), mvErr)
    }

    return l.applySudoPermissions(ctx, dstPath, opts)
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
