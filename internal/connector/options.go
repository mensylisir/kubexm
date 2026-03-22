package connector

import (
	"io"
	"time"
)

// RunOptions for command execution
type RunOptions struct {
	Sudo       bool
	Timeout    time.Duration
	Env        []string
	Retries    int
	RetryDelay time.Duration
	Fatal      bool
	Hidden     bool
	Stream     io.Writer
	Stdin      []byte
	Dir        string
}

// RunResult contains command execution result
type RunResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// CopyOptions for file transfer operations
type CopyOptions struct {
	Permissions string
	Owner       string
	Group       string
	Timeout     time.Duration
	Sudo        bool
}

// ReadOptions for file read operations
type ReadOptions struct {
	Timeout time.Duration
	Sudo    bool
}

// WriteOptions for file write operations
type WriteOptions struct {
	Permissions string
	Owner       string
	Group       string
	Timeout     time.Duration
	Sudo        bool
}

// MkdirOptions for directory creation
type MkdirOptions struct {
	Permissions string
	Sudo        bool
	Timeout     time.Duration
}

// RemoveOptions for file/directory removal
type RemoveOptions struct {
	Recursive      bool
	IgnoreNotExist bool
	Sudo           bool
	Timeout        time.Duration
}

// StatOptions for file stat operations
type StatOptions struct {
	Sudo    bool
	Timeout time.Duration
}

// LookPathOptions for executable lookup
type LookPathOptions struct {
	Sudo    bool
	Timeout time.Duration
}

// Backward compatibility aliases
type ExecOptions = RunOptions
type FileTransferOptions = CopyOptions
