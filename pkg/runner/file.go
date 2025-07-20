package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

func (r *defaultRunner) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	stat, err := conn.Stat(ctx, path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to stat path %s: %w", path, err)
	}
	return stat.IsExist, nil
}

func (r *defaultRunner) IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	stat, err := conn.Stat(ctx, path)
	if err != nil {
		return false, fmt.Errorf("failed to stat path %s: %w", path, err)
	}
	if !stat.IsExist {
		return false, nil
	}
	return stat.IsDir, nil
}

func (r *defaultRunner) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) {
	if conn == nil {
		return nil, fmt.Errorf("connector cannot be nil")
	}
	if extendedConn, ok := conn.(interface {
		ReadFile(ctx context.Context, path string) ([]byte, error)
	}); ok {
		return extendedConn.ReadFile(ctx, path)
	}
	cmd := fmt.Sprintf("cat %s", path)
	stdout, stderr, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: false})
	if err != nil {
		return stdout, fmt.Errorf("failed to read file '%s' with cat: %w (stderr: %s)", path, err, string(stderr))
	}
	return stdout, nil
}

func (r *defaultRunner) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	opts := &connector.FileTransferOptions{
		Permissions: permissions,
		Sudo:        sudo,
	}
	return conn.WriteFile(ctx, content, destPath, opts)
}

func (r *defaultRunner) Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	cmd := fmt.Sprintf("mkdir -p %s", path)
	_, _, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to mkdir -p %s: %w", path, err)
	}

	if permissions != "" {
		if errChmod := r.Chmod(ctx, conn, path, permissions, sudo); errChmod != nil {
			return fmt.Errorf("failed to chmod %s on directory %s after mkdirp: %w", permissions, path, errChmod)
		}
	}
	return nil
}

func (r *defaultRunner) TouchFile(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil for TouchFile")
	}
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("path cannot be empty for TouchFile")
	}

	parentDir := filepath.Dir(path)
	if parentDir != "." && parentDir != "/" && parentDir != "" {
		if err := r.Mkdirp(ctx, conn, parentDir, "0755", sudo); err != nil {
			return fmt.Errorf("failed to create parent directory %s for touch: %w", parentDir, err)
		}
	}

	cmd := fmt.Sprintf("touch %s", path)
	_, stderr, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to touch file %s: %w (stderr: %s)", path, err, string(stderr))
	}
	return nil
}

func (r *defaultRunner) GetDiskUsage(ctx context.Context, conn connector.Connector, path string) (totalMiB uint64, freeMiB uint64, usedMiB uint64, err error) {
	if conn == nil {
		err = fmt.Errorf("connector cannot be nil for GetDiskUsage")
		return
	}
	if strings.TrimSpace(path) == "" {
		err = fmt.Errorf("path cannot be empty for GetDiskUsage")
		return
	}

	cmd := fmt.Sprintf("df -BM -P %s", path)
	stdoutBytes, stderrBytes, execErr := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: false})

	if execErr != nil {
		err = fmt.Errorf("failed to execute df command for path %s: %w (stderr: %s)", path, execErr, string(stderrBytes))
		return
	}

	output := string(stdoutBytes)
	lines := strings.Split(strings.TrimSpace(output), "\n")

	if len(lines) < 2 {
		err = fmt.Errorf("unexpected output format from df for path %s: not enough lines (output: %s)", path, output)
		return
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 4 {
		err = fmt.Errorf("unexpected output format from df for path %s: not enough fields in data line (line: '%s', output: %s)", path, lines[1], output)
		return
	}

	parseMiB := func(valueWithSuffix string) (uint64, error) {
		if !strings.HasSuffix(valueWithSuffix, "M") {
			return 0, fmt.Errorf("expected value to end with 'M', got %s", valueWithSuffix)
		}
		valueStr := strings.TrimSuffix(valueWithSuffix, "M")
		val, parseErr := strconv.ParseUint(valueStr, 10, 64)
		if parseErr != nil {
			return 0, fmt.Errorf("failed to parse numeric value from %s: %w", valueStr, parseErr)
		}
		return val, nil
	}

	totalMiB, err = parseMiB(fields[1])
	if err != nil {
		err = fmt.Errorf("failed to parse total disk space from df output ('%s'): %w", fields[1], err)
		return
	}
	usedMiB, err = parseMiB(fields[2])
	if err != nil {
		err = fmt.Errorf("failed to parse used disk space from df output ('%s'): %w", fields[2], err)
		return
	}
	freeMiB, err = parseMiB(fields[3])
	if err != nil {
		err = fmt.Errorf("failed to parse free disk space from df output ('%s'): %w", fields[3], err)
		return
	}
	return
}

func (r *defaultRunner) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool, recursive bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	cmd := "rm -f"
	if recursive {
		cmd += "r"
	}
	cmd = fmt.Sprintf("%s %s", cmd, path)
	_, stderr, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to remove %s: %w (stderr: %s)", path, err, string(stderr))
	}
	return nil
}

func (r *defaultRunner) Chmod(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if permissions == "" {
		return fmt.Errorf("permissions cannot be empty for Chmod")
	}
	cmd := fmt.Sprintf("chmod %s %s", permissions, path)
	_, stderr, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to chmod %s on %s: %w (stderr: %s)", permissions, path, err, string(stderr))
	}
	return nil
}

func (r *defaultRunner) Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if owner == "" && group == "" {
		return fmt.Errorf("owner and group cannot both be empty for Chown")
	}

	ownerGroupSpec := owner
	if group != "" {
		if owner != "" {
			ownerGroupSpec += ":"
		}
		ownerGroupSpec += group
	}

	recursiveFlag := ""
	if recursive {
		recursiveFlag = "-R"
	}
	cmdParts := []string{"chown", recursiveFlag, ownerGroupSpec, path}
	cmd := strings.Join(strings.Fields(strings.Join(cmdParts, " ")), " ")

	_, stderr, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to chown %s to %s (recursive: %v): %w (stderr: %s)", ownerGroupSpec, path, recursive, err, string(stderr))
	}
	return nil
}

func (r *defaultRunner) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) {
	if conn == nil {
		return "", fmt.Errorf("connector cannot be nil")
	}

	cmd := fmt.Sprintf("sha256sum %s", path)
	stdout, stderr, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: false})

	if err != nil {
		var cmdErr *connector.CommandError
		if errors.As(err, &cmdErr) && (strings.Contains(cmdErr.Stderr, "not found") || cmdErr.ExitCode == 127 || (cmdErr.ExitCode == 1 && cmdErr.Stderr == "")) {
			cmd = fmt.Sprintf("shasum -a 256 %s", path)
			stdout, stderr, err = r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: false})
		}
	}

	if err != nil {
		return "", fmt.Errorf("failed to get SHA256 for %s (tried sha256sum and shasum): %w (last stderr: %s)", path, err, string(stderr))
	}

	parts := strings.Fields(string(stdout))
	if len(parts) > 0 {
		return parts[0], nil
	}
	return "", fmt.Errorf("could not parse SHA256 output: '%s'", string(stdout))
}

func (r *defaultRunner) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) {
	if conn == nil {
		return "", fmt.Errorf("connector cannot be nil")
	}
	return conn.LookPath(ctx, file)
}

func (r *defaultRunner) EnsureMount(ctx context.Context, conn connector.Connector, device, mountPoint, fsType string, options []string, persistent bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if device == "" || mountPoint == "" || fsType == "" {
		return fmt.Errorf("device, mountPoint, and fsType must be specified for EnsureMount")
	}
	isMounted, err := r.IsMounted(ctx, conn, mountPoint)
	if err != nil {
		// If IsMounted itself failed (e.g., 'mountpoint' tool not found), propagate that.
		return fmt.Errorf("failed to check if %s is already mounted: %w", mountPoint, err)
	}

	if !isMounted {
		if err := r.Mkdirp(ctx, conn, mountPoint, "0755", true); err != nil { // Assuming sudo true for mkdir if mount might need it
			return fmt.Errorf("failed to create mount point directory %s: %w", mountPoint, err)
		}

		mountCmdParts := []string{"mount"}
		if len(options) > 0 {
			mountCmdParts = append(mountCmdParts, "-o", strings.Join(options, ","))
		}
		mountCmdParts = append(mountCmdParts, "-t", fsType)
		mountCmdParts = append(mountCmdParts, device)
		mountCmdParts = append(mountCmdParts, mountPoint)
		mountCmd := strings.Join(mountCmdParts, " ")

		_, stderr, mountErr := r.RunWithOptions(ctx, conn, mountCmd, &connector.ExecOptions{Sudo: true})
		if mountErr != nil {
			return fmt.Errorf("failed to mount %s to %s: %w (stderr: %s)", device, mountPoint, mountErr, string(stderr))
		}
	}

	if persistent {
		fstabOptions := "defaults"
		if len(options) > 0 {
			fstabOptions = strings.Join(options, ",")
		}
		fstabEntry := fmt.Sprintf("%s %s %s %s 0 0", device, mountPoint, fsType, fstabOptions)

		checkFstabCmd := fmt.Sprintf("grep -qE '^[[:space:]]*[^#]+[[:space:]]+%s[[:space:]]' /etc/fstab", mountPoint)
		entryExistsInFstab, _ := r.Check(ctx, conn, checkFstabCmd, false)

		if !entryExistsInFstab {
			escapedFstabEntry := fstabEntry
			appendCmd := fmt.Sprintf("sh -c 'echo %s >> /etc/fstab'", escapedFstabEntry)

			_, stderr, appendErr := r.RunWithOptions(ctx, conn, appendCmd, &connector.ExecOptions{Sudo: true})
			if appendErr != nil {
				return fmt.Errorf("failed to add entry to /etc/fstab for %s: %w (stderr: %s)", mountPoint, appendErr, string(stderr))
			}
		}
	}
	return nil
}

func (r *defaultRunner) Unmount(ctx context.Context, conn connector.Connector, mountPoint string, force bool, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil for Unmount")
	}
	if strings.TrimSpace(mountPoint) == "" {
		return fmt.Errorf("mountPoint cannot be empty for Unmount")
	}

	isMounted, err := r.IsMounted(ctx, conn, mountPoint)
	if err != nil {
		return fmt.Errorf("failed to check if %s is mounted before unmounting: %w", mountPoint, err)
	}
	if !isMounted {
		return nil // Not mounted, nothing to do.
	}

	cmdParts := []string{"umount"}
	if force {
		cmdParts = append(cmdParts, "-f")
	}
	cmdParts = append(cmdParts, mountPoint)
	cmd := strings.Join(cmdParts, " ")

	_, stderr, execErr := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
	if execErr != nil {
		var cmdErr *connector.CommandError
		if errors.As(execErr, &cmdErr) {
			errMsg := strings.ToLower(string(stderr) + cmdErr.Error())
			if strings.Contains(errMsg, "not mounted") || strings.Contains(errMsg, "not currently mounted") {
				return nil // Already unmounted or was never mounted, consider success.
			}
		}
		return fmt.Errorf("failed to unmount %s: %w (stderr: %s)", mountPoint, execErr, string(stderr))
	}
	return nil
}

func (r *defaultRunner) IsMounted(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(path) == "" {
		return false, fmt.Errorf("path cannot be empty for IsMounted")
	}

	if _, err := r.LookPath(ctx, conn, "mountpoint"); err == nil {
		cmd := fmt.Sprintf("mountpoint -q %s", path)
		return r.Check(ctx, conn, cmd, false)
	}

	procMounts, err := r.ReadFile(ctx, conn, "/proc/mounts")
	if err != nil {
		return false, fmt.Errorf("IsMounted: 'mountpoint' command not found and failed to read /proc/mounts: %w", err)
	}
	lines := strings.Split(string(procMounts), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == path {
			return true, nil // Path found as a mount point in /proc/mounts
		}
	}
	return false, nil
}

func (r *defaultRunner) MakeFilesystem(ctx context.Context, conn connector.Connector, device, fsType string, force bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(device) == "" {
		return fmt.Errorf("device cannot be empty for MakeFilesystem")
	}
	if strings.TrimSpace(fsType) == "" {
		return fmt.Errorf("fsType cannot be empty for MakeFilesystem")
	}

	safeFsType := fsType
	for _, char := range fsType {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '.') {
			return fmt.Errorf("invalid characters in fsType: %s", fsType)
		}
	}

	cmdParts := []string{fmt.Sprintf("mkfs.%s", safeFsType)}
	if force {
		cmdParts = append(cmdParts, "-f")
	}
	cmdParts = append(cmdParts, device)
	cmd := strings.Join(cmdParts, " ")

	_, stderr, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to make filesystem type %s on device %s: %w (stderr: %s)", fsType, device, err, string(stderr))
	}
	return nil
}

func (r *defaultRunner) CreateSymlink(ctx context.Context, conn connector.Connector, target, linkPath string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(target) == "" {
		return fmt.Errorf("target cannot be empty for CreateSymlink")
	}
	if strings.TrimSpace(linkPath) == "" {
		return fmt.Errorf("linkPath cannot be empty for CreateSymlink")
	}

	linkDir := filepath.Dir(linkPath)
	if linkDir != "." && linkDir != "/" {
		if err := r.Mkdirp(ctx, conn, linkDir, "0755", sudo); err != nil {
			return fmt.Errorf("failed to create parent directory %s for symlink %s: %w", linkDir, linkPath, err)
		}
	}

	cmd := fmt.Sprintf("ln -sf %s %s", target, linkPath)
	_, stderr, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to create symlink from %s to %s: %w (stderr: %s)", target, linkPath, err, string(stderr))
	}
	return nil
}

func (r *defaultRunner) VerifyChecksum(ctx context.Context, conn connector.Connector, filePath, expectedChecksum, checksumType string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if filePath == "" || expectedChecksum == "" || checksumType == "" {
		return fmt.Errorf("filePath, expectedChecksum, and checksumType must not be empty")
	}

	var cmd string
	var checksumTool string

	switch strings.ToLower(checksumType) {
	case "sha256":
		if _, err := r.LookPath(ctx, conn, "sha256sum"); err == nil {
			checksumTool = "sha256sum"
			cmd = fmt.Sprintf("sha256sum %s | awk '{print $1}'", filePath)
		} else if _, err := r.LookPath(ctx, conn, "shasum"); err == nil {
			checksumTool = "shasum -a 256"
			cmd = fmt.Sprintf("shasum -a 256 %s | awk '{print $1}'", filePath)
		} else {
			return fmt.Errorf("no sha256sum or shasum command found on the remote host")
		}

	case "sha512":
		if _, err := r.LookPath(ctx, conn, "sha512sum"); err == nil {
			checksumTool = "sha512sum"
			cmd = fmt.Sprintf("sha512sum %s | awk '{print $1}'", filePath)
		} else if _, err := r.LookPath(ctx, conn, "shasum"); err == nil {
			checksumTool = "shasum -a 512"
			cmd = fmt.Sprintf("shasum -a 512 %s | awk '{print $1}'", filePath)
		} else {
			return fmt.Errorf("no sha512sum or shasum command found on the remote host")
		}

	case "md5":
		if _, err := r.LookPath(ctx, conn, "md5sum"); err == nil {
			checksumTool = "md5sum"
			cmd = fmt.Sprintf("md5sum %s | awk '{print $1}'", filePath)
		} else if _, err := r.LookPath(ctx, conn, "md5"); err == nil {
			checksumTool = "md5"
			cmd = fmt.Sprintf("md5 -r %s | awk '{print $1}'", filePath)
		} else {
			return fmt.Errorf("no md5sum or md5 command found on the remote host")
		}
	default:
		return fmt.Errorf("unsupported checksum type: %s", checksumType)
	}
	r.logger.Debug("Executing remote checksum command", "tool", checksumTool, "path", filePath)
	stdoutBytes, stderrBytes, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to calculate remote checksum for %s: %w. Stderr: %s", filePath, err, string(stderrBytes))
	}
	calculatedChecksum := strings.TrimSpace(string(stdoutBytes))
	expectedChecksum = strings.ToLower(strings.TrimSpace(expectedChecksum))
	calculatedChecksum = strings.ToLower(calculatedChecksum)

	if calculatedChecksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", filePath, expectedChecksum, calculatedChecksum)
	}
	r.logger.Debug("Remote checksum verified successfully", "path", filePath)
	return nil
}

type remoteFileInfo struct {
	name  string
	mode  os.FileMode
	isDir bool
}

func (rfi *remoteFileInfo) Name() string       { return rfi.name }
func (rfi *remoteFileInfo) Size() int64        { return 0 }
func (rfi *remoteFileInfo) Mode() os.FileMode  { return rfi.mode }
func (rfi *remoteFileInfo) ModTime() time.Time { return time.Time{} }
func (rfi *remoteFileInfo) IsDir() bool        { return rfi.isDir }
func (rfi *remoteFileInfo) Sys() interface{}   { return nil }

func (r *defaultRunner) Stat(ctx context.Context, conn connector.Connector, path string) (os.FileInfo, error) {
	cmd := fmt.Sprintf("stat -c \"%%a %%F\" %s", path)
	output, err := r.Run(ctx, conn, cmd, false)

	if err != nil {
		if strings.Contains(strings.ToLower(output), "no such file or directory") {
			return nil, fmt.Errorf("stat failed for %s: %w", path, os.ErrNotExist)
		}
		return nil, fmt.Errorf("failed to run remote stat on %s: %w. Output: %s", path, err, output)
	}

	parts := strings.Split(strings.TrimSpace(output), " ")
	if len(parts) < 2 {
		return nil, fmt.Errorf("unexpected output from remote stat: %q", output)
	}

	perm, err := strconv.ParseUint(parts[0], 8, 32)
	if err != nil {
		return nil, fmt.Errorf("failed to parse permissions from stat output %q: %w", output, err)
	}

	fileType := parts[1]
	isDir := fileType == "directory"

	info := &remoteFileInfo{
		name:  path,
		mode:  os.FileMode(perm),
		isDir: isDir,
	}

	return info, nil
}
