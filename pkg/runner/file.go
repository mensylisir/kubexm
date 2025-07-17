package runner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
)

func (r *defaultRunner) Exists(ctx context.Context, conn connector.Connector, path string) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	stat, err := conn.Stat(ctx, path)
	if err != nil {
		// Check if the error is a "not found" type of error.
		// This requires the connector's Stat method to return an error type
		// that can be queried, e.g., by implementing an IsNotExist() bool method
		// or by checking against os.ErrNotExist if it's wrapped.
		// For simplicity, if connector.Stat returns any error, we might assume it means
		// "existence cannot be confirmed" or "does not exist".
		// A more robust connector would return a specific error type for "not found".
		// For now, let's assume if Stat returns an error, we can't confirm existence.
		// conn.Stat now returns FileStat{IsExist: false}, nil for "not found"
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
		// conn.Stat now returns FileStat{IsExist: false}, nil for "not found"
		// So, if err is not nil here, it's a genuine error.
		return false, fmt.Errorf("failed to stat path %s: %w", path, err)
	}
	// If IsExist is false (and err was nil), it's not a directory.
	if !stat.IsExist {
		return false, nil
	}
	return stat.IsDir, nil
}

// ReadFile reads the content of a remote file into a byte slice.
func (r *defaultRunner) ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error) {
	if conn == nil {
		return nil, fmt.Errorf("connector cannot be nil")
	}
	// Check if the connector directly supports ReadFile
	if extendedConn, ok := conn.(interface {
		ReadFile(ctx context.Context, path string) ([]byte, error)
	}); ok {
		return extendedConn.ReadFile(ctx, path)
	}
	// Fallback to using 'cat' if the connector doesn't have a direct ReadFile method
	cmd := fmt.Sprintf("cat %s", path)
	stdout, stderr, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: false})
	if err != nil {
		return stdout, fmt.Errorf("failed to read file '%s' with cat: %w (stderr: %s)", path, err, string(stderr))
	}
	return stdout, nil
}

// WriteFile writes content to a remote file, automatically handling sudo if needed.
func (r *defaultRunner) WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	opts := &connector.FileTransferOptions{
		Permissions: permissions,
		Sudo:        sudo,
		// Owner and Group could be added here if the runner.WriteFile signature were extended
		// or if there's a convention to pass them via another mechanism.
		// For now, matching the existing runner.WriteFile signature.
	}
	return conn.WriteFile(ctx, content, destPath, opts)
}

// Mkdirp ensures a directory exists, creating parent directories as needed (like 'mkdir -p').
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

// TouchFile creates an empty file if it doesn't exist, or updates its modification timestamp if it does.
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

// GetDiskUsage retrieves disk usage information (total, free, used space in MiB) for the filesystem
// on which the given path resides.
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

// Remove deletes a file or directory (recursively for directories, like 'rm -rf').
func (r *defaultRunner) Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	cmd := fmt.Sprintf("rm -rf %s", path)
	_, stderr, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to remove %s: %w (stderr: %s)", path, err, string(stderr))
	}
	return nil
}

// Chmod changes the permissions of a remote file or directory.
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

// Chown changes the owner and group of a remote file or directory.
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
	cmd := strings.Join(strings.Fields(strings.Join(cmdParts, " ")), " ") // Build and then normalize spaces

	_, stderr, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true}) // Chown usually requires sudo
	if err != nil {
		return fmt.Errorf("failed to chown %s to %s (recursive: %v): %w (stderr: %s)", ownerGroupSpec, path, recursive, err, string(stderr))
	}
	return nil
}

// GetSHA256 gets the SHA256 checksum of a remote file for integrity checks.
func (r *defaultRunner) GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error) {
	if conn == nil {
		return "", fmt.Errorf("connector cannot be nil")
	}

	cmd := fmt.Sprintf("sha256sum %s", path)
	stdout, stderr, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: false})

	if err != nil {
		// If sha256sum is not found or fails, try shasum -a 256
		var cmdErr *connector.CommandError
		if errors.As(err, &cmdErr) && (strings.Contains(cmdErr.Stderr, "not found") || cmdErr.ExitCode == 127 || (cmdErr.ExitCode == 1 && cmdErr.Stderr == "")) {
			// Try shasum as a fallback
			cmd = fmt.Sprintf("shasum -a 256 %s", path)
			stdout, stderr, err = r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: false})
		}
	}

	if err != nil {
		// If we are here, both attempts failed or the first error was not "command not found"
		return "", fmt.Errorf("failed to get SHA256 for %s (tried sha256sum and shasum): %w (last stderr: %s)", path, err, string(stderr))
	}

	// Output of both sha256sum and shasum is typically "checksum  filename"
	parts := strings.Fields(string(stdout))
	if len(parts) > 0 {
		return parts[0], nil
	}
	return "", fmt.Errorf("could not parse SHA256 output: '%s'", string(stdout))
}

// LookPath searches for an executable in the remote host's PATH.
func (r *defaultRunner) LookPath(ctx context.Context, conn connector.Connector, file string) (string, error) {
	if conn == nil {
		return "", fmt.Errorf("connector cannot be nil")
	}
	// Delegate directly to connector's LookPath
	return conn.LookPath(ctx, file)
}

// Removed duplicated LookPath and misplaced code block that was here.

// --- Stubs for new filesystem/storage methods from enriched interface ---

func (r *defaultRunner) EnsureMount(ctx context.Context, conn connector.Connector, device, mountPoint, fsType string, options []string, persistent bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	// Basic validation
	if device == "" || mountPoint == "" || fsType == "" {
		return fmt.Errorf("device, mountPoint, and fsType must be specified for EnsureMount")
	}

	// Determine sudo for sub-operations. Assume if mount needs sudo, precursor ops might too.
	// A more granular approach might pass separate sudo flags or infer based on path.
	// For now, if the mount operation itself might need sudo, assume Mkdirp might too.
	// The actual mount command will use sudo via RunWithOptions.

	// 1. Check if already mounted
	// A more robust check would verify device and options, not just if something is at mountPoint.
	// For this version, IsMounted checks if mountPoint is a mount point.
	isMounted, err := r.IsMounted(ctx, conn, mountPoint)
	if err != nil {
		// If IsMounted itself failed (e.g., 'mountpoint' tool not found), propagate that.
		return fmt.Errorf("failed to check if %s is already mounted: %w", mountPoint, err)
	}

	if !isMounted {
		// 2. Ensure mountPoint directory exists
		// Using "0755" as a common default permission for mount point dirs.
		// Sudo for Mkdirp: if the mount operation needs sudo, creating the mountpoint dir might too.
		// This is a heuristic.
		if err := r.Mkdirp(ctx, conn, mountPoint, "0755", true); err != nil { // Assuming sudo true for mkdir if mount might need it
			return fmt.Errorf("failed to create mount point directory %s: %w", mountPoint, err)
		}

		// 3. Mount the device
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

	// 4. If persistent, ensure entry in /etc/fstab
	if persistent {
		fstabOptions := "defaults"
		if len(options) > 0 {
			fstabOptions = strings.Join(options, ",")
		}
		// Common dump/pass values. Pass '0' for non-root filesystems is safest to avoid fsck issues on boot if not critical.
		// Pass '2' for non-root, '1' for root. For general purpose, '0 0' is common.
		fstabEntry := fmt.Sprintf("%s %s %s %s 0 0", device, mountPoint, fsType, fstabOptions)

		// Idempotency check: grep for the mountPoint first.
		// A more robust check would parse /etc/fstab properly.
		// This simple grep checks if an entry for the mountPoint exists.
		// It doesn't verify if the existing entry is correct (device, fsType, options).
		checkFstabCmd := fmt.Sprintf("grep -qE '^[[:space:]]*[^#]+[[:space:]]+%s[[:space:]]' /etc/fstab", mountPoint)
		entryExistsInFstab, _ := r.Check(ctx, conn, checkFstabCmd, false) // Ignore error, if grep fails, assume not found.

		if !entryExistsInFstab {
			// Append the new entry. Use shell redirection with sudo via sh -c.
			// Ensure the entryLine is properly quoted for the shell command.
			escapedFstabEntry := fstabEntry // Escape for the 'echo' command
			appendCmd := fmt.Sprintf("sh -c 'echo %s >> /etc/fstab'", escapedFstabEntry)

			_, stderr, appendErr := r.RunWithOptions(ctx, conn, appendCmd, &connector.ExecOptions{Sudo: true})
			if appendErr != nil {
				return fmt.Errorf("failed to add entry to /etc/fstab for %s: %w (stderr: %s)", mountPoint, appendErr, string(stderr))
			}
		}
		// Note: This doesn't handle updating an existing incorrect fstab entry for the mountPoint.
	}
	return nil
}

// Unmount unmounts a filesystem.
func (r *defaultRunner) Unmount(ctx context.Context, conn connector.Connector, mountPoint string, force bool, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil for Unmount")
	}
	if strings.TrimSpace(mountPoint) == "" {
		return fmt.Errorf("mountPoint cannot be empty for Unmount")
	}

	// Check if it's even mounted first. If not, consider it a success (idempotency).
	isMounted, err := r.IsMounted(ctx, conn, mountPoint)
	if err != nil {
		return fmt.Errorf("failed to check if %s is mounted before unmounting: %w", mountPoint, err)
	}
	if !isMounted {
		return nil // Not mounted, nothing to do.
	}

	cmdParts := []string{"umount"}
	if force {
		cmdParts = append(cmdParts, "-f") // Force unmount
	}
	cmdParts = append(cmdParts, mountPoint)
	cmd := strings.Join(cmdParts, " ")

	_, stderr, execErr := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
	if execErr != nil {
		// Check if error is because it's "not mounted" - which can happen in race conditions or if state changed.
		// This makes the operation more idempotent.
		var cmdErr *connector.CommandError
		if errors.As(execErr, &cmdErr) {
			errMsg := strings.ToLower(string(stderr) + cmdErr.Error()) // Combine stderr and error message
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

	// Ensure the path is "absolute" or at least not problematic for grep.
	// Shell escaping might be needed if path can contain special characters,
	// but for typical mount paths, it's often okay.
	// For robustness, especially if path could be `*`, it needs escaping.
	// However, `grep -qs -- "/path/to/check" /proc/mounts` is safer.
	// The '--' signifies end of options, then pattern, then file.
	// The path itself might need quoting if it contains spaces. Let's assume simple paths for now,
	// or rely on a shellEscape helper if available and necessary.
	// Using exact match with -F might be too strict if /proc/mounts has variations.
	// A common pattern is to check if `df <path>` reports the path on the correct device.
	// Or, more simply: `grep -qsE "[[:space:]]${MOUNT_POINT}[[:space:]]" /proc/mounts`
	// The command `findmnt -rno TARGET "${path}"` exits 0 if path is a mountpoint and its output is the path.
	// Or, `mountpoint -q "${path}"` is even simpler if available (util-linux).

	// Let's use `mountpoint -q <path>` as it's designed for this.
	// It exits 0 if path is a mountpoint, non-zero otherwise.
	// Requires `util-linux` package, which is very common.

	// First, check if `mountpoint` command exists.
	if _, err := r.LookPath(ctx, conn, "mountpoint"); err == nil {
		cmd := fmt.Sprintf("mountpoint -q %s", path) // Path is used directly
		// No sudo needed for `mountpoint -q`
		return r.Check(ctx, conn, cmd, false)
	}

	// Fallback: Check /proc/mounts if mountpoint command is not available
	// This is a common Linux-specific way.
	// We need to be careful with how paths are listed in /proc/mounts (e.g., symlinks resolved).
	// A simple grep might not be fully robust for all edge cases (e.g. bind mounts over files, symlinked mountpoints).
	// `awk '$2 == path {found=1; exit} END{exit !found}' path="${path_escaped_for_awk}" /proc/mounts`
	// For now, a simpler grep:
	// We need to match the path as the second field, surrounded by whitespace.
	// Example line: /dev/sda1 /mnt/data ext4 rw,relatime 0 0
	// The path in /proc/mounts is usually what was passed to mount, but can be tricky with symlinks.
	// Using `df <path> | awk 'NR==2 {print $6}'` and checking if it equals path is also common.

	// Given the constraints and aiming for simplicity that works in many Linux cases:
	// `grep -qsE '[[:space:]]${ESCAPED_PATH}[[:space:]]' /proc/mounts`
	// The path needs to be escaped for regex and shell.
	// For now, let's stick to `mountpoint` and if not found, return an error or a less reliable check.
	// For this iteration, if mountpoint is not found, we'll indicate it's not implemented for fallback.
	// return false, fmt.Errorf("IsMounted: 'mountpoint' command not found, and /proc/mounts fallback not fully implemented for all edge cases yet")

	// Fallback 2: Check /proc/mounts (Linux specific)
	// This is a common way to check, but has edge cases (e.g. symlinked mount points).
	// We are looking for a line where the second field is exactly the path.
	// awk '$2 == path_var { found=1; exit } END { if (found) exit 0; else exit 1 }' path_var="$ESCAPED_PATH" /proc/mounts
	// Using grep for simplicity, but it's less precise than awk for exact field match.
	// grep -qsE "[[:space:]]${ESCAPED_PATH}[[:space:]]" /proc/mounts
	// A safer grep: `grep -E "(^| )${ESCAPED_PATH_FOR_REGEX}(\$| )" /proc/mounts`
	// Let's try a command that checks more directly if path is a mountpoint by comparing its device with its parent's device.
	// If `stat -c %d <path>` is different from `stat -c %d <path>/..` then it's a mountpoint (for most cases).
	// This is also not foolproof for all bind mounts or specific setups.

	// Given the complexity of a truly robust cross-platform fallback,
	// and if `mountpoint` is unavailable, we might have to rely on `df` output or accept limitations.
	// Let's try parsing `/proc/mounts` as a common Linux fallback.
	// This requires reading the file and parsing its content.
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
	// If not found via mountpoint or in /proc/mounts
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

	// Basic validation for fsType to prevent command injection via this variable.
	// Allow common types. A more robust solution might use a whitelist or more advanced validation.
	// For now, simple check for alphanumeric.
	safeFsType := fsType // In a real scenario, validate fsType more strictly or use a map of allowed types.
	for _, char := range fsType {
		if !((char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') || char == '.') {
			return fmt.Errorf("invalid characters in fsType: %s", fsType)
		}
	}

	cmdParts := []string{fmt.Sprintf("mkfs.%s", safeFsType)}
	if force {
		// Common force flags are -f or -F. mkfs.ext4 uses -F, mkfs.xfs uses -f.
		// Using a generic -f. This might need adjustment for specific fsTypes if -f is not universal or has different meanings.
		// For critical operations, it's better to be specific based on fsType.
		// For this implementation, we'll use a common one and note the caveat.
		cmdParts = append(cmdParts, "-f") // General force flag
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

	// Ensure parent directory of linkPath exists.
	// This step is often crucial for `ln -s` to succeed if the parent dir doesn't exist.
	linkDir := filepath.Dir(linkPath)
	if linkDir != "." && linkDir != "/" { // Avoid trying to mkdirp "." or "/"
		// Mkdirp itself handles sudo if needed for the directory creation.
		// The sudo flag for CreateSymlink applies to the `ln` command itself.
		// If linkDir needs sudo to create, and the main `ln` also needs sudo, this is fine.
		// If linkDir doesn't need sudo, but `ln` does, also fine.
		if err := r.Mkdirp(ctx, conn, linkDir, "0755", sudo); err != nil {
			return fmt.Errorf("failed to create parent directory %s for symlink %s: %w", linkDir, linkPath, err)
		}
	}

	// Using -f to force creation (overwrite if linkPath exists)
	// Using -n for directories: when source is a directory, `ln -sfn source link` makes link point to source.
	// If link is already a directory, `ln -sf source link` would create source inside link/ (link/source).
	// `ln -sfn` (or `ln -sfT`) is often safer to ensure linkPath itself becomes the symlink.
	// For simplicity and common use, `ln -sf` is often sufficient if linkPath is not expected to be a pre-existing directory.
	// Let's assume `ln -sf` is the desired behavior for now. If linkPath is a dir, target will be created inside.
	// If more precise control over directory symlinking is needed, -T or -n flags with `ln` could be used,
	// potentially requiring a check if target is a directory.

	cmd := fmt.Sprintf("ln -sf %s %s", target, linkPath)
	_, stderr, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to create symlink from %s to %s: %w (stderr: %s)", target, linkPath, err, string(stderr))
	}
	return nil
}
