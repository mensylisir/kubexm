package connector

import (
	"io"
	"time"
)

// ExecOptions defines the command execution options.
type ExecOptions struct {
	// Sudo specifies whether to use 'sudo -E' to execute the command.
	// '-E' ensures that environment variables are preserved.
	Sudo bool
	// Timeout is the timeout duration for command execution.
	// Default is 0 (no timeout).
	Timeout time.Duration
	// Env sets additional environment variables for the command,
	// in the format "VAR=VALUE".
	Env []string
	// Retries is the number of retries if the command fails (non-zero exit code).
	Retries int
	// RetryDelay is the waiting time between each retry.
	RetryDelay time.Duration
	// Fatal indicates whether the failure of this command is a fatal error.
	// The upper-level Runner can capture this flag and abort the process.
	Fatal bool
	// Hidden specifies whether to hide the command itself in the logs
	// (used for sensitive information like passwords).
	Hidden bool
	// Stream, if not nil, will have the command's stdout and stderr
	// written to it in real-time.
	Stream io.Writer
}

// FileTransferOptions defines the options for file transfer.
type FileTransferOptions struct {
	// Permissions is the permission mode for the destination file, e.g., "0644".
	// If empty, default permissions are used.
	Permissions string
	// Owner is the owner of the destination file, e.g., "root". Requires sudo permission.
	Owner string
	// Group is the group of the destination file, e.g., "root". Requires sudo permission.
	Group string
	// Timeout is the timeout duration for file transfer.
	Timeout time.Duration
	// Sudo specifies whether to use sudo to write the destination file
	// (by writing to a temporary file and then using sudo mv).
	Sudo bool
}

// RemoveOptions defines options for file/directory removal.
// Moved from interface.go for better organization.
type RemoveOptions struct {
	Recursive      bool
	IgnoreNotExist bool
	Sudo           bool // Added to support sudo for remove operations
}
