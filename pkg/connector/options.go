package connector

import (
	"io"
	"time"
)

type ExecOptions struct {
	Sudo       bool
	Timeout    time.Duration
	Env        []string
	Retries    int
	RetryDelay time.Duration
	Fatal      bool
	Hidden     bool
	Stream     io.Writer
	Stdin      []byte
}

type FileTransferOptions struct {
	Permissions string
	Owner       string
	Group       string
	Timeout     time.Duration
	Sudo        bool
}

type RemoveOptions struct {
	Recursive      bool
	IgnoreNotExist bool
	Sudo           bool
}

type StatOptions struct {
	Sudo bool
}

type LookPathOptions struct {
	Sudo bool
}
