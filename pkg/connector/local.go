package connector

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mensylisir/kubexm/pkg/logger"
)

func shellEscape(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

type LocalConnector struct {
	connCfg  ConnectionCfg
	cachedOS *OS
}

func NewLocalConnector() (*LocalConnector, error) {
	return &LocalConnector{}, nil
}

func (l *LocalConnector) Connect(ctx context.Context, cfg ConnectionCfg) error {
	l.connCfg = cfg
	return nil
}

func (l *LocalConnector) IsConnected() bool {
	return true
}

func (l *LocalConnector) Close() error {
	return nil
}

func (l *LocalConnector) Exec(ctx context.Context, cmd string, options *ExecOptions) (stdout, stderr []byte, err error) {
	effectiveOptions := ExecOptions{}
	if options != nil {
		effectiveOptions = *options
	}

	fullCmdString := cmd
	if effectiveOptions.Sudo {
		if l.connCfg.Password != "" {
			fullCmdString = "sudo -S -p '' -E -- " + cmd
		} else {
			fullCmdString = "sudo -E -- " + cmd
		}
	}

	runOnce := func(runCtx context.Context) ([]byte, []byte, error) {
		shell := []string{"/bin/sh", "-c"}
		if runtime.GOOS == "windows" {
			shell = []string{"cmd", "/C"}
		}

		actualCmd := exec.CommandContext(runCtx, shell[0], append(shell[1:], fullCmdString)...)

		if len(effectiveOptions.Env) > 0 {
			actualCmd.Env = append(os.Environ(), effectiveOptions.Env...)
		}

		if effectiveOptions.Sudo && l.connCfg.Password != "" {
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

		if effectiveOptions.Dir != "" {
			actualCmd.Dir = effectiveOptions.Dir
		}

		err := actualCmd.Run()
		return stdoutBuf.Bytes(), stderrBuf.Bytes(), err
	}

	var finalErr error
	for i := 0; i <= effectiveOptions.Retries; i++ {
		attemptCtx := ctx
		var attemptCancel context.CancelFunc

		if effectiveOptions.Timeout > 0 {
			attemptCtx, attemptCancel = context.WithTimeout(context.Background(), effectiveOptions.Timeout)
		}

		stdout, stderr, err = runOnce(attemptCtx)

		if attemptCancel != nil {
			attemptCancel()
		}

		if err == nil {
			return stdout, stderr, nil // Success
		}

		finalErr = err
		if attemptCtx.Err() != nil || ctx.Err() != nil {
			break
		}

		if i < effectiveOptions.Retries {
			if effectiveOptions.RetryDelay > 0 {
				time.Sleep(effectiveOptions.RetryDelay)
			}
		} else {
			break
		}
	}

	if ctx.Err() != nil {
		return stdout, stderr, &CommandError{Cmd: cmd, ExitCode: -1, Stdout: string(stdout), Stderr: string(stderr), Underlying: ctx.Err()}
	}

	exitCode := -1
	if exitErr, ok := finalErr.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			exitCode = status.ExitStatus()
		}
	}
	return stdout, stderr, &CommandError{Cmd: cmd, ExitCode: exitCode, Stdout: string(stdout), Stderr: string(stderr), Underlying: finalErr}
}

func (l *LocalConnector) Upload(ctx context.Context, localPath, remotePath string, options *FileTransferOptions) error {
	return l.Copy(ctx, localPath, remotePath, options)
}

func (l *LocalConnector) Download(ctx context.Context, remotePath, localPath string, options *FileTransferOptions) error {
	return l.Copy(ctx, remotePath, localPath, options)
}

func (l *LocalConnector) Copy(ctx context.Context, srcPath, dstPath string, options *FileTransferOptions) error {
	opts := FileTransferOptions{}
	if options != nil {
		opts = *options
	}

	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	srcStat, err := os.Stat(srcPath)
	if err != nil {
		return fmt.Errorf("source path %s does not exist or is not accessible: %w", srcPath, err)
	}

	if dstStat, err := os.Stat(dstPath); err == nil {
		if os.SameFile(srcStat, dstStat) {
			return fmt.Errorf("source and destination are the same file: %s", srcPath)
		}
	}

	if !opts.Sudo {
		if srcStat.IsDir() {
			return l.copyDir(srcPath, dstPath, opts)
		}
		return l.copyFile(srcPath, dstPath, opts)
	}

	tmpDir, err := os.MkdirTemp("", "localconnector-copy-")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	stagedPath := filepath.Join(tmpDir, filepath.Base(srcPath))



	if srcStat.IsDir() {
		// Use sudo cp -r for directories
		cpCmd := fmt.Sprintf("cp -r %s %s", shellEscape(srcPath), shellEscape(stagedPath))
		_, stderr, err := l.Exec(ctx, cpCmd, &ExecOptions{Sudo: true})
		if err != nil {
			return fmt.Errorf("failed to stage directory %s to %s with sudo: %s (underlying error %w)", srcPath, stagedPath, string(stderr), err)
		}
	} else {
		// Use sudo cp for files
		cpCmd := fmt.Sprintf("cp %s %s", shellEscape(srcPath), shellEscape(stagedPath))
		_, stderr, err := l.Exec(ctx, cpCmd, &ExecOptions{Sudo: true})
		if err != nil {
			return fmt.Errorf("failed to stage file %s to %s with sudo: %s (underlying error %w)", srcPath, stagedPath, string(stderr), err)
		}
	}

	destParentDir := filepath.Dir(dstPath)
	if destParentDir != "." && destParentDir != "/" && destParentDir != "" {
		mkdirCmd := fmt.Sprintf("mkdir -p %s", shellEscape(destParentDir))
		_, stderr, mkdirErr := l.Exec(ctx, mkdirCmd, &ExecOptions{Sudo: true})
		if mkdirErr != nil {
			return fmt.Errorf("failed to create destination parent directory %s with sudo: %s (underlying error %w)", destParentDir, string(stderr), mkdirErr)
		}
	}

	mvCmd := fmt.Sprintf("mv %s %s", shellEscape(stagedPath), shellEscape(dstPath))
	_, stderr, mvErr := l.Exec(ctx, mvCmd, &ExecOptions{Sudo: true})
	if mvErr != nil {
		return fmt.Errorf("failed to move staged content from %s to %s with sudo: %s (underlying error %w)", stagedPath, dstPath, string(stderr), mvErr)
	}

	return l.applySudoPermissions(ctx, dstPath, opts)
}

func (l *LocalConnector) copyFile(src, dst string, opts FileTransferOptions) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file %s for copyFile: %w", src, err)
	}
	defer srcFile.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
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
	return nil
}

func (l *LocalConnector) copyDir(src, dst string, opts FileTransferOptions) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source directory %s for copyDir: %w", src, err)
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("failed to create destination directory %s for copyDir: %w", dst, err)
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory %s for copyDir: %w", src, err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := l.copyDir(srcPath, dstPath, opts); err != nil {
				return err // Error already wrapped by recursive call
			}
		} else {
			if err := l.copyFile(srcPath, dstPath, opts); err != nil {
				return err // Error already wrapped by copyFile
			}
		}
	}
	return nil
}

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
	
	// Set ownership - default to root if not specified
	ownerAndGroup := opts.Owner
	if opts.Group != "" {
		ownerAndGroup = fmt.Sprintf("%s:%s", opts.Owner, opts.Group)
	}
	// If no owner specified, default to root for sudo operations
	if ownerAndGroup == "" {
		ownerAndGroup = "root"
	}
	
	targetStat, statErr := os.Stat(path)
	chownFlags := ""
	if statErr == nil && targetStat.IsDir() {
		chownFlags = "-R"
	}

	chownCmd := fmt.Sprintf("chown %s %s %s", chownFlags, shellEscape(ownerAndGroup), shellEscape(path))
	chownCmd = strings.TrimSpace(strings.ReplaceAll(chownCmd, "  ", " "))

	_, stderr, err := l.Exec(ctx, chownCmd, &ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to set ownership on %s with sudo chown: %s (underlying error %w)", path, string(stderr), err)
	}
	
	return nil
}

func (l *LocalConnector) CopyContent(ctx context.Context, content []byte, dstPath string, options *FileTransferOptions) error {
	opts := FileTransferOptions{}
	if options != nil {
		opts = *options
	}

	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	if !opts.Sudo {
		permMode := fs.FileMode(0644)
		if opts.Permissions != "" {
			if perm, err := strconv.ParseUint(opts.Permissions, 8, 32); err == nil {
				permMode = fs.FileMode(perm)
			} else {
				return fmt.Errorf("invalid permissions format '%s' for CopyContent to %s: %w", opts.Permissions, dstPath, err)
			}
		}
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return fmt.Errorf("failed to create destination directory %s for CopyContent: %w", filepath.Dir(dstPath), err)
		}
		return os.WriteFile(dstPath, content, permMode)
	}

	tmpFile, err := os.CreateTemp("", "localconnector-content-")
	if err != nil {
		return fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(content); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write content to temporary file %s: %w", tmpFile.Name(), err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temporary file %s: %w", tmpFile.Name(), err)
	}

	destParentDir := filepath.Dir(dstPath)
	if destParentDir != "." && destParentDir != "/" && destParentDir != "" {
		mkdirCmd := fmt.Sprintf("mkdir -p %s", shellEscape(destParentDir))
		_, stderr, mkdirErr := l.Exec(ctx, mkdirCmd, &ExecOptions{Sudo: true})
		if mkdirErr != nil {
			return fmt.Errorf("failed to create destination parent directory %s with sudo: %s (underlying error %w)", destParentDir, string(stderr), mkdirErr)
		}
	}

	mvCmd := fmt.Sprintf("mv %s %s", shellEscape(tmpFile.Name()), shellEscape(dstPath))
	_, stderr, mvErr := l.Exec(ctx, mvCmd, &ExecOptions{Sudo: true})
	if mvErr != nil {
		return fmt.Errorf("failed to move temporary file from %s to %s with sudo: %s (underlying error %w)", tmpFile.Name(), dstPath, string(stderr), mvErr)
	}

	return l.applySudoPermissions(ctx, dstPath, opts)
}

func (l *LocalConnector) Fetch(ctx context.Context, remotePath, localPath string, options *FileTransferOptions) error {
	return l.Copy(ctx, remotePath, localPath, options)
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

func (l *LocalConnector) StatWithOptions(ctx context.Context, path string, opts *StatOptions) (*FileStat, error) {
	useSudo := opts != nil && opts.Sudo

	fi, err := os.Lstat(path)
	if err == nil {
		return &FileStat{
			Name:    fi.Name(),
			Size:    fi.Size(),
			Mode:    fi.Mode(),
			ModTime: fi.ModTime(),
			IsDir:   fi.IsDir(),
			IsExist: true,
		}, nil
	}

	if os.IsNotExist(err) {
		return &FileStat{Name: filepath.Base(path), IsExist: false}, nil
	}

	if !useSudo {
		return nil, fmt.Errorf("failed to stat local path %s: %w", path, err)
	}

	cmdExists := fmt.Sprintf("test -e %s", path)
	_, _, errExists := l.Exec(ctx, cmdExists, &ExecOptions{Sudo: true})
	if errExists != nil {
		return &FileStat{Name: filepath.Base(path), IsExist: false}, nil
	}

	cmdIsDir := fmt.Sprintf("test -d %s", path)
	_, _, errIsDir := l.Exec(ctx, cmdIsDir, &ExecOptions{Sudo: true})
	isDir := (errIsDir == nil)
	return &FileStat{
		Name:    filepath.Base(path),
		IsDir:   isDir,
		IsExist: true,
	}, nil
}

func (l *LocalConnector) LookPath(ctx context.Context, file string) (string, error) {
	return exec.LookPath(file)
}

func (l *LocalConnector) LookPathWithOptions(ctx context.Context, file string, opts *LookPathOptions) (string, error) {
	if strings.ContainsAny(file, " \t\n\r`;&|$<>()!{}[]*?^~") {
		return "", fmt.Errorf("invalid characters in executable name for LookPath: %q", file)
	}

	cmd := fmt.Sprintf("which %s", file)

	useSudo := opts != nil && opts.Sudo

	execOpts := &ExecOptions{
		Sudo: useSudo,
	}

	stdout, stderr, err := l.Exec(ctx, cmd, execOpts)
	if err != nil {
		return "", fmt.Errorf("failed to find executable '%s' locally (sudo: %v): %s (underlying error: %w)", file, useSudo, string(stderr), err)
	}

	path := strings.TrimSpace(string(stdout))
	if path == "" {
		return "", fmt.Errorf("executable '%s' not found in local PATH (sudo: %v, stderr: %s)", file, useSudo, string(stderr))
	}

	return path, nil
}

func (l *LocalConnector) GetOS(ctx context.Context) (*OS, error) {
	log := logger.Get()
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
			log.Warn(os.Stderr, "warning: failed to get kernel version for local connector: %v\n", errKernel)
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
			if id, ok := vars["ID"]; ok {
				osInfo.ID = id
			}
			if verID, ok := vars["VERSION_ID"]; ok {
				osInfo.VersionID = verID
			}
			if name, ok := vars["PRETTY_NAME"]; ok {
				osInfo.PrettyName = name
			}
			if cname, ok := vars["VERSION_CODENAME"]; ok {
				osInfo.Codename = cname
			}
		} else {
			if osInfo.ID == "" {
				osInfo.ID = "linux"
			}
			if osInfo.PrettyName == "" {
				osInfo.PrettyName = "Linux"
			}
			log.Warn(os.Stderr, "warning: failed to read /etc/os-release for local connector: %v\n", err)
		}
	case "darwin":
		osInfo.ID = "darwin"
		swVersCmdName := exec.CommandContext(ctx, "sw_vers", "-productName")
		prodName, errProdName := swVersCmdName.Output()
		if errProdName == nil {
			osInfo.PrettyName = strings.TrimSpace(string(prodName))
		}

		swVersCmdVersion := exec.CommandContext(ctx, "sw_vers", "-productVersion")
		prodVer, errProdVer := swVersCmdVersion.Output()
		if errProdVer == nil {
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
		osInfo.PrettyName = "Windows"
	default:
		if osInfo.ID == "" {
			osInfo.ID = runtime.GOOS
		}
		if osInfo.PrettyName == "" {
			osInfo.PrettyName = runtime.GOOS
		}
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

func (l *LocalConnector) ReadFileWithOptions(ctx context.Context, path string, opts *FileTransferOptions) ([]byte, error) {
	useSudo := false
	if opts != nil && opts.Sudo {
		useSudo = true
	}
	if !useSudo {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read local file %s: %w", path, err)
		}
		return data, nil
	}

	cmd := fmt.Sprintf("cat %s", path)
	execOpts := &ExecOptions{
		Sudo: true,
	}
	if opts != nil && opts.Timeout > 0 {
		execOpts.Timeout = opts.Timeout
	}
	stdout, _, err := l.Exec(ctx, cmd, execOpts)
	if err != nil {
		return stdout, fmt.Errorf("failed to read file '%s' with local sudo cat: %w", path, err)
	}

	return stdout, nil
}

func (l *LocalConnector) WriteFile(ctx context.Context, content []byte, destPath string, options *FileTransferOptions) error {
	opts := FileTransferOptions{}
	if options != nil {
		opts = *options
	}

	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	if opts.Sudo {
		if runtime.GOOS == "windows" {
			return fmt.Errorf("sudo write not supported on Windows for path %s", destPath)
		}

		destDir := filepath.Dir(destPath)
		if destDir != "." && destDir != "/" && destDir != "" {
			mkdirCmd := fmt.Sprintf("mkdir -p %s", shellEscape(destDir))
			_, stderr, err := l.Exec(ctx, mkdirCmd, &ExecOptions{Sudo: true})
			if err != nil {
				return fmt.Errorf("failed to create parent directory %s with sudo: %s (underlying error: %w)", destDir, string(stderr), err)
			}
		}

		// Write to a temporary file first (as current user)
		tmpFile, err := os.CreateTemp("", "localconnector-sudo-write-")
		if err != nil {
			return fmt.Errorf("failed to create temporary file for sudo write: %w", err)
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)

		if _, err := tmpFile.Write(content); err != nil {
			tmpFile.Close()
			return fmt.Errorf("failed to write content to temporary file %s: %w", tmpPath, err)
		}
		if err := tmpFile.Close(); err != nil {
			return fmt.Errorf("failed to close temporary file %s: %w", tmpPath, err)
		}

		// Move the temporary file to the destination with sudo
		mvCmd := fmt.Sprintf("mv %s %s", shellEscape(tmpPath), shellEscape(destPath))
		_, stderr, mvErr := l.Exec(ctx, mvCmd, &ExecOptions{Sudo: true})
		if mvErr != nil {
			return fmt.Errorf("failed to move temporary file from %s to %s with sudo: %s (underlying error: %w)", tmpPath, destPath, string(stderr), mvErr)
		}
		return l.applySudoPermissions(ctx, destPath, opts)

	} else {
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %w", destPath, err)
		}
		permMode := fs.FileMode(0644)
		if opts.Permissions != "" {
			permVal, parseErr := strconv.ParseUint(opts.Permissions, 8, 32)
			if parseErr != nil {
				return fmt.Errorf("invalid permissions format '%s' for local WriteFile to %s: %w", opts.Permissions, destPath, parseErr)
			}
			permMode = fs.FileMode(permVal)
		}
		if err := os.WriteFile(destPath, content, permMode); err != nil {
			return fmt.Errorf("failed to write file %s: %w", destPath, err)
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

	if _, err := io.Copy(hasher, file); err != nil {
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
	if opts.Sudo {
		if runtime.GOOS == "windows" {
			return fmt.Errorf("sudo remove not supported on Windows for path %s", path)
		}
		cmdParts := []string{"rm"}
		if opts.Recursive {
			cmdParts = append(cmdParts, "-r")
		}
		cmdParts = append(cmdParts, "-f")
		cmdParts = append(cmdParts, shellEscape(path))
		rmCmd := strings.Join(cmdParts, " ")

		_, stderr, err := l.Exec(ctx, rmCmd, &ExecOptions{Sudo: true})
		if err != nil {
			return fmt.Errorf("failed to remove %s with sudo: %s (underlying error: %w)", path, string(stderr), err)
		}
	} else {
		if opts.Recursive {
			removeErr = os.RemoveAll(path)
		} else {
			removeErr = os.Remove(path)
		}
		if removeErr != nil {
			// For non-sudo, if IgnoreNotExist is true, this error might be filtered by the caller
			// if the error is os.ErrNotExist. The check at the beginning handles this.
			return fmt.Errorf("failed to remove %s: %w", path, removeErr)
		}
	}
	return nil
}

type hash interface {
	io.Writer
	Sum(b []byte) []byte
}

func getHasher(checksumType string) (hash, bool) {
	switch strings.ToLower(checksumType) {
	case "sha256":
		return sha256.New(), true
	case "md5":
		return md5.New(), true
	default:
		return nil, false
	}
}

func (l *LocalConnector) GetConnectionConfig() ConnectionCfg {
	return l.connCfg
}

// Run executes a command and returns structured result
func (l *LocalConnector) Run(ctx context.Context, cmd string, opts *RunOptions) (RunResult, error) {
	stdout, stderr, err := l.Exec(ctx, cmd, opts)
	exitCode := 0
	if err != nil {
		exitCode = -1
		if cmdErr, ok := err.(*CommandError); ok {
			exitCode = cmdErr.ExitCode
		}
	}
	return RunResult{Stdout: stdout, Stderr: stderr, ExitCode: exitCode}, err
}

// Read reads file content from the specified path
func (l *LocalConnector) Read(ctx context.Context, path string, opts *ReadOptions) ([]byte, error) {
	if opts == nil {
		return l.ReadFile(ctx, path)
	}
	return l.ReadFileWithOptions(ctx, path, &CopyOptions{Timeout: opts.Timeout, Sudo: opts.Sudo})
}

// Write writes content to the specified path
func (l *LocalConnector) Write(ctx context.Context, content []byte, path string, opts *WriteOptions) error {
	if opts == nil {
		return l.WriteFile(ctx, content, path, nil)
	}
	return l.WriteFile(ctx, content, path, &CopyOptions{
		Permissions: opts.Permissions,
		Owner:       opts.Owner,
		Group:       opts.Group,
		Timeout:     opts.Timeout,
		Sudo:        opts.Sudo,
	})
}

func (l *LocalConnector) IsFile(ctx context.Context, path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !fi.IsDir(), nil
}

func (l *LocalConnector) IsDir(ctx context.Context, path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return fi.IsDir(), nil
}

func (l *LocalConnector) GetFileMode(ctx context.Context, path string) (fs.FileMode, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return fi.Mode(), nil
}

func (l *LocalConnector) GetFileOwner(ctx context.Context, path string) (string, string, error) {
	if runtime.GOOS == "windows" {
		return "unknown", "unknown", nil
	}
	// Use stat command to get user and group names
	// GNU stat: stat -c "%U %G"
	// BSD stat: stat -f "%Su %Sg"
	var cmd string
	if runtime.GOOS == "darwin" { // BSD stat
		cmd = fmt.Sprintf("stat -f '%%Su %%Sg' %s", shellEscape(path))
	} else { // Assume GNU stat (Linux)
		cmd = fmt.Sprintf("stat -c '%%U %%G' %s", shellEscape(path))
	}

	stdout, _, err := l.Exec(ctx, cmd, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to get file owner for %s: %w", path, err)
	}
	parts := strings.Fields(string(stdout))
	if len(parts) < 2 {
		return "", "", fmt.Errorf("unexpected output from stat command: %s", string(stdout))
	}
	return parts[0], parts[1], nil
}

func (l *LocalConnector) GetOSRelease(ctx context.Context) (map[string]string, error) {
var content []byte
var err error

for _, path := range osReleasePaths {
content, err = l.ReadFile(ctx, path)
if err == nil {
return parseKeyValues(string(content), "=", "\""), nil
}
}

return nil, fmt.Errorf("failed to read os-release file from any known location: %w", err)
}

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

var _ Connector = &LocalConnector{}

var osReleasePaths = []string{"/etc/os-release", "/usr/lib/os-release"}
