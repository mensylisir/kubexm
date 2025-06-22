package runner

import (
	"context"
	"errors" // Added for errors.As
	"fmt"
	// "path/filepath" // Removed as unused
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector" // Corrected import path
)

// Exists checks if a file or directory exists at the given path.
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

// IsDir checks if the given path is a directory.
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
	// Check if the connector directly supports WriteFile or CopyContent
	if extendedConn, ok := conn.(interface {
		WriteFile(ctx context.Context, content []byte, destPath, permissions string, sudo bool) error
	}); ok {
		return extendedConn.WriteFile(ctx, content, destPath, permissions, sudo)
	}
	if extendedConnCopy, ok := conn.(interface {
		CopyContent(ctx context.Context, content []byte, destPath string, options *connector.FileTransferOptions) error
	}); ok {
		opts := &connector.FileTransferOptions{
			Permissions: permissions,
			Sudo:        sudo,
		}
		return extendedConnCopy.CopyContent(ctx, content, destPath, opts)
	}
	// Fallback to a command-based approach if direct methods aren't available (more complex)
	return fmt.Errorf("WriteFile not directly supported by connector and command-based fallback not implemented in this refactor step")
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
	cmd := fmt.Sprintf("chown %s %s %s", recursiveFlag, ownerGroupSpec, path)

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
