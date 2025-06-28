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
func (l *LocalConnector) Exec(ctx context.Context, cmd string, options *ExecOptions) (stdout, stderr []byte, err error) {
	var actualCmd *exec.Cmd
	shell := []string{"/bin/sh", "-c"}
	if runtime.GOOS == "windows" {
		shell = []string{"cmd", "/C"}
	}

	fullCmdString := cmd
	effectiveOptions := &ExecOptions{} // Create a new instance to avoid modifying the input options
	if options != nil {
		*effectiveOptions = *options // Copy options
	}

	if effectiveOptions.Sudo {
		fullCmdString = "sudo -E -- " + cmd
		if l.connCfg.Password != "" { // Check if password is provided in connection config
			fullCmdString = "sudo -S -p '' -E -- " + cmd // Use -S to read password from stdin
		}
	}

	if effectiveOptions.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, effectiveOptions.Timeout)
		defer cancel()
	}

	actualCmd = exec.CommandContext(ctx, shell[0], append(shell[1:], fullCmdString)...)

	if len(effectiveOptions.Env) > 0 {
		actualCmd.Env = append(os.Environ(), effectiveOptions.Env...)
	}

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

		if effectiveOptions.Retries > 0 {
			for i := 0; i < effectiveOptions.Retries; i++ {
				if effectiveOptions.RetryDelay > 0 {
					time.Sleep(effectiveOptions.RetryDelay)
				}
				retryCtx := ctx // Use the potentially timed-out context for retries as well
				if options != nil && options.Timeout > 0 { // Re-arm timeout if needed for each retry attempt
					var retryCancel context.CancelFunc
					retryCtx, retryCancel = context.WithTimeout(context.Background(), options.Timeout) // Use fresh background context for timeout
					defer retryCancel()
				}


				retryCmd := exec.CommandContext(retryCtx, shell[0], append(shell[1:], fullCmdString)...)
				if len(effectiveOptions.Env) > 0 {
					retryCmd.Env = append(os.Environ(), effectiveOptions.Env...)
				}
				if effectiveOptions.Sudo && l.connCfg.Password != "" && strings.HasPrefix(fullCmdString, "sudo -S") {
					retryCmd.Stdin = strings.NewReader(l.connCfg.Password + "\n")
				}

				stdoutBuf.Reset()
				stderrBuf.Reset()
				if effectiveOptions.Stream != nil {
					retryCmd.Stdout = io.MultiWriter(&stdoutBuf, effectiveOptions.Stream)
					retryCmd.Stderr = io.MultiWriter(&stderrBuf, effectiveOptions.Stream)
				} else {
					retryCmd.Stdout = &stdoutBuf
					retryCmd.Stderr = &stderrBuf
				}

				err = retryCmd.Run()
				stdout = stdoutBuf.Bytes()
				stderr = stderrBuf.Bytes()
				if err == nil {
					break
				}
				if exitErr, ok := err.(*exec.ExitError); ok {
					if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
						exitCode = status.ExitStatus()
					}
				}
			}
		}

		if err != nil {
			return stdout, stderr, &CommandError{Cmd: cmd, ExitCode: exitCode, Stdout: string(stdout), Stderr: string(stderr), Underlying: err}
		}
	}
	return stdout, stderr, nil
}

func (l *LocalConnector) Copy(ctx context.Context, srcPath, dstPath string, options *FileTransferOptions) error {
	if options != nil && options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	if options != nil && options.Sudo {
		return fmt.Errorf("sudo not implemented for LocalConnector.Copy, target path %s may require privileges", dstPath)
	}

	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", srcPath, err)
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
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

	if options != nil && options.Permissions != "" {
		perm, parseErr := strconv.ParseUint(options.Permissions, 8, 32)
		if parseErr == nil {
			if errChmod := os.Chmod(dstPath, os.FileMode(perm)); errChmod != nil {
				return fmt.Errorf("failed to chmod %s to %s: %w", dstPath, options.Permissions, errChmod)
			}
		} else {
			return fmt.Errorf("invalid permissions format '%s' for %s: %w", options.Permissions, dstPath, parseErr)
		}
	}
	return nil
}

func (l *LocalConnector) CopyContent(ctx context.Context, content []byte, dstPath string, options *FileTransferOptions) error {
	if options != nil && options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	if options != nil && options.Sudo {
		return fmt.Errorf("sudo not implemented for LocalConnector.CopyContent, target path %s may require privileges", dstPath)
	}

	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory for %s: %w", dstPath, err)
	}

	permMode := fs.FileMode(0644)
	if options != nil && options.Permissions != "" {
		permVal, parseErr := strconv.ParseUint(options.Permissions, 8, 32)
		if parseErr == nil {
			permMode = fs.FileMode(permVal)
		} else {
			// Log warning but proceed with default, or return error.
			// For now, returning error for invalid permission string.
			return fmt.Errorf("invalid permissions format '%s' for local CopyContent to %s: %w", options.Permissions, dstPath, parseErr)
		}
	}

	err := os.WriteFile(dstPath, content, permMode)
	if err != nil {
		return fmt.Errorf("failed to write content to %s: %w", dstPath, err)
	}
	return nil
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

func (l *LocalConnector) WriteFile(ctx context.Context, content []byte, destPath, permissions string, sudo bool) error {
	if sudo {
		return fmt.Errorf("sudo not implemented for LocalConnector.WriteFile, target path %s may require privileges", destPath)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory for %s: %w", destPath, err)
	}
	permMode := fs.FileMode(0644)
	if permissions != "" {
		permVal, parseErr := strconv.ParseUint(permissions, 8, 32)
		if parseErr == nil {
			permMode = fs.FileMode(permVal)
		} else {
			// Return error for invalid permission string, consistent with CopyContent change
			return fmt.Errorf("invalid permissions format '%s' for local WriteFile to %s: %w", permissions, destPath, parseErr)
		}
	}
	err := os.WriteFile(destPath, content, permMode)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", destPath, err)
	}
	return nil
}

func (l *LocalConnector) GetFileChecksum(ctx context.Context, path string, checksumType string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open local file %s for checksum: %w", path, err)
	}
	defer file.Close()

	var hasher io.Writer
	switch strings.ToLower(checksumType) {
	case "sha256":
		hasher = sha256.New()
	// case "md5": // md5 is currently not supported as per previous logic
	// 	hasher = md5.New()
	default:
		return "", fmt.Errorf("unsupported checksum type '%s' for local file %s", checksumType, path)
	}

	if _, err := io.Copy(hasher.(io.Writer), file); err != nil { // Assert hasher is an io.Writer
		return "", fmt.Errorf("failed to read local file %s for checksum calculation: %w", path, err)
	}

	// Type assert back to hash.Hash to call Sum
	// The specific type *sha256.digest is not exported.
	// sha256.New() returns hash.Hash, which has the Sum method.
	if ch, ok := hasher.(interface{ Sum(b []byte) []byte }); ok {
		return hex.EncodeToString(ch.Sum(nil)), nil
	}
	// This fallback should ideally not be reached if hasher was correctly initialized
	// from a known crypto package like sha256 or md5 (if implemented).
	return "", fmt.Errorf("checksum hasher for %s does not support Sum method", checksumType)
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

var _ Connector = &LocalConnector{}
