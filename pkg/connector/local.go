package connector

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"crypto/sha256" // Added
	"encoding/hex"  // Added
	// "fmt" // Removed duplicate
	// "io" // Removed duplicate
	"os"
	"os/exec"
	"path/filepath"
	"runtime" // To get local OS info
	"strconv"
	"strings"
	"syscall"
	"io/fs" // For Mkdir permissions
	"time"
)

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
	// We can do a basic check like ensuring the user from cfg exists if needed,
	// but for now, it's a no-op.
	return nil
}

// IsConnected for LocalConnector always returns true as it operates locally.
func (l *LocalConnector) IsConnected() bool {
	return true // Local execution doesn't have a "connection" in the remote sense.
}

// Close for LocalConnector is a no-op.
func (l *LocalConnector) Close() error {
	// No resources to release for local execution.
	return nil
}

// Exec executes a command locally.
func (l *LocalConnector) Exec(ctx context.Context, cmd string, options *ExecOptions) (stdout, stderr []byte, err error) {
	var actualCmd *exec.Cmd
	shell := []string{"/bin/sh", "-c"} // Default shell
	if runtime.GOOS == "windows" {
		shell = []string{"cmd", "/C"}
	}

	fullCmdString := cmd
	if options != nil && options.Sudo {
		// Note: Local sudo might require password input if not configured for NOPASSWD.
		// This implementation doesn't handle interactive password prompts.
		// -E is used to preserve environment variables with sudo.
		fullCmdString = "sudo -E -- " + cmd
	}

	// Use context for timeout if provided
	if options != nil && options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	actualCmd = exec.CommandContext(ctx, shell[0], append(shell[1:], fullCmdString)...)

	if options != nil && len(options.Env) > 0 {
		actualCmd.Env = append(os.Environ(), options.Env...)
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	if options != nil && options.Stream != nil {
		actualCmd.Stdout = io.MultiWriter(&stdoutBuf, options.Stream)
		actualCmd.Stderr = io.MultiWriter(&stderrBuf, options.Stream)
	} else {
		actualCmd.Stdout = &stdoutBuf
		actualCmd.Stderr = &stderrBuf
	}

	err = actualCmd.Run()

	stdout = stdoutBuf.Bytes()
	stderr = stderrBuf.Bytes()

	if err != nil {
		exitCode := -1
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
				exitCode = status.ExitStatus()
			}
		}

		// Retry logic
		if options != nil && options.Retries > 0 {
			for i := 0; i < options.Retries; i++ {
				if options.RetryDelay > 0 {
					time.Sleep(options.RetryDelay)
				}

				// Create new command for retry
				retryCmd := exec.CommandContext(ctx, shell[0], append(shell[1:], fullCmdString)...)
				if options.Env != nil {
					retryCmd.Env = append(os.Environ(), options.Env...)
				}

				stdoutBuf.Reset() // Clear buffers for retry
				stderrBuf.Reset()
				if options.Stream != nil {
					retryCmd.Stdout = io.MultiWriter(&stdoutBuf, options.Stream)
					retryCmd.Stderr = io.MultiWriter(&stderrBuf, options.Stream)
				} else {
					retryCmd.Stdout = &stdoutBuf
					retryCmd.Stderr = &stderrBuf
				}

				err = retryCmd.Run()
				stdout = stdoutBuf.Bytes()
				stderr = stderrBuf.Bytes()
				if err == nil {
					break // Success on retry
				}
				if exitErr, ok := err.(*exec.ExitError); ok {
					if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
						exitCode = status.ExitStatus()
					}
				}
			}
		}

		if err != nil { // If error persists after retries
			return stdout, stderr, &CommandError{Cmd: cmd, ExitCode: exitCode, Stdout: string(stdout), Stderr: string(stderr), Underlying: err}
		}
	}
	return stdout, stderr, nil
}

// Copy copies a local file or directory to another local path.
func (l *LocalConnector) Copy(ctx context.Context, srcPath, dstPath string, options *FileTransferOptions) error {
	// Timeout handling (for the whole operation, though os.Copy is blocking)
	if options != nil && options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	// TODO: Implement sudo logic if options.Sudo is true.
	// This would involve copying to a temp location then using `sudo mv` and `sudo chmod/chown`.
	// For now, direct copy.

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", srcPath, err)
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dstPath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination directory for %s: %w", dstPath, err)
	}

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dstPath, err)
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy content from %s to %s: %w", srcPath, dstPath, err)
	}

	// Apply permissions and ownership
	if options != nil {
		if options.Permissions != "" {
			perm, err := strconv.ParseUint(options.Permissions, 8, 32)
			if err == nil {
				if errChmod := os.Chmod(dstPath, os.FileMode(perm)); errChmod != nil {
					// Log or return error, potentially wrapped
				}
			}
		}
		// For local, Chown requires process to have privileges.
		// if options.Owner != "" || options.Group != "" {
		// This part is complex due to UID/GID lookup and permissions.
		// os.Chown might fail if not run as root or with appropriate capabilities.
		// If options.Sudo is true, this should be done via `sudo chown` command.
		// }
	}
	return nil
}

// CopyContent writes content to a local file.
func (l *LocalConnector) CopyContent(ctx context.Context, content []byte, dstPath string, options *FileTransferOptions) error {
	if options != nil && options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	// TODO: Sudo handling as in Copy.
	if err := os.MkdirAll(filepath.Dir(dstPath), os.ModePerm); err != nil {
		return fmt.Errorf("failed to create destination directory for %s: %w", dstPath, err)
	}

	err := os.WriteFile(dstPath, content, 0666) // Default permissions, then chmod
	if err != nil {
		return fmt.Errorf("failed to write content to %s: %w", dstPath, err)
	}

	if options != nil {
		if options.Permissions != "" {
			perm, err := strconv.ParseUint(options.Permissions, 8, 32)
			if err == nil {
				if errChmod := os.Chmod(dstPath, os.FileMode(perm)); errChmod != nil {
					// Log or return error
				}
			}
		}
		// Sudo/Chown handling as in Copy.
	}
	return nil
}

// Fetch copies a local file to another local path (effectively same as Copy for local).
func (l *LocalConnector) Fetch(ctx context.Context, remotePath, localPath string) error {
	// For local connector, remotePath is just another local path.
	return l.Copy(ctx, remotePath, localPath, nil) // No options for simplicity, or pass them through.
}

// Stat gets information about a local file or directory.
func (l *LocalConnector) Stat(ctx context.Context, path string) (*FileStat, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileStat{Name: filepath.Base(path), IsExist: false}, nil
		}
		return nil, fmt.Errorf("failed to stat local path %s: %w", path, err)
	}

	// TODO: Get Owner/Group (requires user/group ID lookup from syscall.Stat_t)
	// TODO: Get SHA256 sum if needed by reading the file.
	return &FileStat{
		Name:    fi.Name(),
		Size:    fi.Size(),
		Mode:    fi.Mode(), // Use directly as fs.FileMode
		ModTime: fi.ModTime(),
		IsDir:   fi.IsDir(),
		IsExist: true,
	}, nil
}

// LookPath searches for an executable in the local PATH.
func (l *LocalConnector) LookPath(ctx context.Context, file string) (string, error) {
	return exec.LookPath(file)
}

// GetOS retrieves local operating system information.
func (l *LocalConnector) GetOS(ctx context.Context) (*OS, error) {
	if l.cachedOS != nil {
		return l.cachedOS, nil
	}

	osInfo := &OS{
		ID:   strings.ToLower(runtime.GOOS), // e.g., "linux", "darwin", "windows"
		Arch: runtime.GOARCH,                // e.g., "amd64", "arm64"
	}

	// Getting VersionID, Codename, Kernel for local can be more involved
	// and platform-specific.
	osInfo.Arch = runtime.GOARCH // Already correct

	switch runtime.GOOS {
	case "linux":
		// Kernel version from `uname -r`
		kernelCmd := exec.CommandContext(ctx, "uname", "-r")
		kernelOut, errKernel := kernelCmd.Output()
		if errKernel == nil {
			osInfo.Kernel = strings.TrimSpace(string(kernelOut))
		} else {
			// Log warning or error, but proceed
			fmt.Fprintf(os.Stderr, "warning: failed to get kernel version for local connector: %v\n", errKernel)
		}

		// Read /etc/os-release
		content, err := os.ReadFile("/etc/os-release")
		if err == nil {
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
			osInfo.Codename = vars["VERSION_CODENAME"]
		} else {
			// Fallback if /etc/os-release is not readable
			osInfo.ID = "linux"
			if osInfo.PrettyName == "" {
				osInfo.PrettyName = "Linux"
			}
			fmt.Fprintf(os.Stderr, "warning: failed to read /etc/os-release for local connector: %v\n", err)
		}

	case "darwin":
		osInfo.ID = "darwin"
		// Populate PrettyName, VersionID, Kernel using `sw_vers` and `uname -r`
		swVersCmd := exec.CommandContext(ctx, "sw_vers", "-productName")
		prodName, errProd := swVersCmd.Output()
		if errProd == nil {
			osInfo.PrettyName = strings.TrimSpace(string(prodName))
		}
		swVersCmd = exec.CommandContext(ctx, "sw_vers", "-productVersion")
		prodVer, errVer := swVersCmd.Output()
		if errVer == nil {
			osInfo.VersionID = strings.TrimSpace(string(prodVer))
		}
		kernelCmd := exec.CommandContext(ctx, "uname", "-r")
		kernelOut, errKernel := kernelCmd.Output()
		if errKernel == nil {
			osInfo.Kernel = strings.TrimSpace(string(kernelOut))
		}
		if osInfo.PrettyName == "" {
			osInfo.PrettyName = "macOS"
		}
	case "windows":
		osInfo.ID = "windows"
		osInfo.PrettyName = "Windows" // Basic, can be enhanced with `ver` or registry checks
		// Kernel and VersionID might require more complex parsing of `ver` or systeminfo
	default:
		osInfo.ID = runtime.GOOS
		osInfo.PrettyName = runtime.GOOS
	}

	l.cachedOS = osInfo
	return l.cachedOS, nil
}

// ReadFile reads a file from the local filesystem.
func (l *LocalConnector) ReadFile(ctx context.Context, path string) ([]byte, error) {
	// TODO: Consider context cancellation/timeout for large file reads if necessary.
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read local file %s: %w", path, err)
	}
	return data, nil
}

// WriteFile writes content to a local file.
// Permissions are applied after writing. Sudo is complex for local writes
// and usually means the process itself needs privileges or uses a helper.
// For simplicity, this WriteFile does not implement sudo move logic like remote might.
func (l *LocalConnector) WriteFile(ctx context.Context, content []byte, destPath, permissions string, sudo bool) error {
	if sudo {
		// Local sudo for WriteFile is tricky. Typically means writing to a restricted path.
		// This would require writing to temp and then `sudo mv` and `sudo chmod`.
		// For now, returning an error if sudo is requested for local write,
		// as it implies a privileged operation not directly handled here.
		return fmt.Errorf("sudo not implemented for LocalConnector.WriteFile, target path %s may require privileges", destPath)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil { // Default permissive mkdir for parent
		return fmt.Errorf("failed to create parent directory for %s: %w", destPath, err)
	}

	// Default file permission if not specified or invalid
	permMode := fs.FileMode(0644) // Default to rw-r--r--
	if permissions != "" {
		permVal, parseErr := strconv.ParseUint(permissions, 8, 32)
		if parseErr == nil {
			permMode = fs.FileMode(permVal)
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Invalid permissions format '%s' for local WriteFile to %s, using default 0644: %v\n", permissions, destPath, parseErr)
		}
	}

	err := os.WriteFile(destPath, content, permMode)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", destPath, err)
	}
	// Chmod is effectively done by WriteFile's perm argument if permissions string was valid,
	// but explicit chmod can be done if WriteFile used a temp perm.
	// For simplicity, WriteFile directly uses the target perm.
	return nil
}


// GetFileChecksum calculates the checksum of a local file.
// Supports "sha256", "md5".
func (l *LocalConnector) GetFileChecksum(ctx context.Context, path string, checksumType string) (string, error) {
	// TODO: Context cancellation for large files
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open local file %s for checksum: %w", path, err)
	}
	defer file.Close()

	hasher := sha256.New() // Default hasher
	switch strings.ToLower(checksumType) {
	case "sha256":
		// Hasher already sha256.New()
	case "md5":
		// hasher = md5.New() // To use md5, import "crypto/md5"
		return "", fmt.Errorf("md5 checksum not implemented for local connector, requested for %s", path)
	default:
		return "", fmt.Errorf("unsupported checksum type '%s' for local file %s", checksumType, path)
	}

	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to read local file %s for checksum calculation: %w", path, err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// Mkdir creates a directory on the local filesystem.
// perm is an octal string like "0755".
func (l *LocalConnector) Mkdir(ctx context.Context, path string, perm string) error {
	// TODO: Handle context cancellation if os.MkdirAll can be long-running (unlikely for mkdir)
	var mode fs.FileMode = 0755 // Default permission
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

// Remove removes a file or directory on the local filesystem.
// For LocalConnector, RemoveOptions are used to control behavior.
func (l *LocalConnector) Remove(ctx context.Context, path string, opts RemoveOptions) error {
	// TODO: Handle context cancellation
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


// Ensure LocalConnector implements Connector interface
var _ Connector = &LocalConnector{}
