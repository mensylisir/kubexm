package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kubexms/kubexms/pkg/connector" // Assuming this is the correct path
)

// Exists checks if a file or directory exists at the given path.
func (r *Runner) Exists(ctx context.Context, path string) (bool, error) {
	if r.Conn == nil {
		return false, fmt.Errorf("runner has no valid connector")
	}
	stat, err := r.Conn.Stat(ctx, path)
	if err != nil {
		// If Stat itself returns an error (e.g. connection issue), propagate it.
		// Stat is expected to return IsExist=false and nil error if file simply doesn't exist.
		return false, err
	}
	return stat.IsExist, nil
}

// IsDir checks if the given path is a directory.
func (r *Runner) IsDir(ctx context.Context, path string) (bool, error) {
	if r.Conn == nil {
		return false, fmt.Errorf("runner has no valid connector")
	}
	stat, err := r.Conn.Stat(ctx, path)
	if err != nil {
		return false, err
	}
	if !stat.IsExist { // If it doesn't exist, it's not a directory
		return false, nil
	}
	return stat.IsDir, nil
}

// ReadFile reads the content of a remote file into a byte slice.
// This implementation uses 'cat' for simplicity. A more robust solution might
// use Conn.Fetch to a temporary local file or extend Connector for direct content retrieval.
func (r *Runner) ReadFile(ctx context.Context, path string) ([]byte, error) {
	if r.Conn == nil {
		return nil, fmt.Errorf("runner has no valid connector")
	}
	// Using Exec to 'cat' the file. Sudo is false by default.
	// Consider if sudo should be an option here, or if a separate ReadFileSudo is needed.
	// For now, assuming read doesn't typically require sudo.
	stdout, stderr, err := r.Conn.Exec(ctx, fmt.Sprintf("cat %s", path), &connector.ExecOptions{Sudo: false})
	if err != nil {
		// If cat fails (e.g. file not found, permission denied), CommandError will be returned.
		// We wrap it to provide more context.
		return stdout, fmt.Errorf("failed to read file '%s' with cat: %w (stderr: %s)", path, err, string(stderr))
	}
	return stdout, nil
}

// WriteFile writes content to a remote file, automatically handling sudo if needed.
func (r *Runner) WriteFile(ctx context.Context, content []byte, destPath, permissions string, sudo bool) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}
	opts := &connector.FileTransferOptions{
		Permissions: permissions,
		Sudo:        sudo,
		// Owner/Group can be added if needed, though Chown is separate.
	}
	return r.Conn.CopyContent(ctx, content, destPath, opts)
}

// Mkdirp ensures a directory exists, creating parent directories as needed (like 'mkdir -p').
// This is an idempotent operation.
func (r *Runner) Mkdirp(ctx context.Context, path, permissions string, sudo bool) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}
	// The actual 'mkdir -p' command handles idempotency.
	cmd := fmt.Sprintf("mkdir -p %s", path)
	_, _, err := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to mkdir -p %s: %w", path, err)
	}

	if permissions != "" {
		// After creating the directory, set permissions if specified.
		// This Chmod call needs to handle the path of the created directory.
		if err := r.Chmod(ctx, path, permissions, sudo); err != nil {
			return fmt.Errorf("failed to chmod %s on directory %s after mkdirp: %w", permissions, path, err)
		}
	}
	return nil
}

// Remove deletes a file or directory (recursively for directories, like 'rm -rf').
func (r *Runner) Remove(ctx context.Context, path string, sudo bool) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}
	cmd := fmt.Sprintf("rm -rf %s", path)
	// Using RunWithOptions to ensure stderr is captured in case of error.
	_, stderr, err := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to remove %s: %w (stderr: %s)", path, err, string(stderr))
	}
	return nil
}

// Chmod changes the permissions of a remote file or directory.
func (r *Runner) Chmod(ctx context.Context, path, permissions string, sudo bool) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}
	if permissions == "" {
		return fmt.Errorf("permissions cannot be empty for Chmod")
	}
	cmd := fmt.Sprintf("chmod %s %s", permissions, path)
	_, stderr, err := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: sudo})
	if err != nil {
		return fmt.Errorf("failed to chmod %s on %s: %w (stderr: %s)", permissions, path, err, string(stderr))
	}
	return nil
}

// Chown changes the owner and group of a remote file or directory.
// Chown almost always needs sudo.
func (r *Runner) Chown(ctx context.Context, path, owner, group string, recursive bool) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}
	if owner == "" && group == "" {
		return fmt.Errorf("owner and group cannot both be empty for Chown")
	}

	ownerGroup := ""
	if owner != "" {
		ownerGroup += owner
	}
	if group != "" {
		if owner != "" {
			ownerGroup += ":"
		}
		ownerGroup += group
	}

	cmdBase := "chown"
	if recursive {
		cmdBase += " -R"
	}
	cmd := fmt.Sprintf("%s %s %s", cmdBase, ownerGroup, path)

	// Chown usually requires sudo. The `sudo` parameter is implicitly true here.
	// If finer control is needed, a `sudo bool` param can be added to Chown.
	// Forcing sudo: true for Chown.
	_, stderr, err := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to chown %s to %s (recursive: %v): %w (stderr: %s)", ownerGroup, path, recursive, err, string(stderr))
	}
	return nil
}

// GetSHA256 gets the SHA256 checksum of a remote file for integrity checks.
func (r *Runner) GetSHA256(ctx context.Context, path string) (string, error) {
	if r.Conn == nil {
		return "", fmt.Errorf("runner has no valid connector")
	}
	// Check for available commands: sha256sum, shasum -a 256
	// For simplicity, trying sha256sum first.
	// A more robust solution might use LookPath or try multiple commands.
	cmd := fmt.Sprintf("sha256sum %s | awk '{print $1}'", path)
	stdout, stderr, err := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: false})

	if err != nil {
		// If sha256sum is not found or fails, try shasum -a 256
		// This error check is basic. A proper check would be on exit code for "command not found".
		if cmdErr, ok := err.(*connector.CommandError); ok && (strings.Contains(string(stderr), "not found") || cmdErr.ExitCode == 127) {
			cmd = fmt.Sprintf("shasum -a 256 %s | awk '{print $1}'", path)
			stdout, stderr, err = r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: false})
		}
	}

	if err != nil {
		return "", fmt.Errorf("failed to get SHA256 for %s: %w (stderr: %s)", path, err, string(stderr))
	}
	return strings.TrimSpace(string(stdout)), nil
}
