package connector

import (
	"context"
	"fmt"      // Added for ExitError.Error
	"io/fs"    // For fs.FileMode
	// "os"       // For os.FileMode, can use fs.FileMode consistently. Let's remove direct os import if fs.FileMode is used.
	"time"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
)

// OS represents operating system details.
type OS struct {
	ID         string
	VersionID  string
	PrettyName string
	Arch       string
	Kernel     string
}

// BastionCfg defines configuration for a bastion/jump host.
type BastionCfg struct {
	Host           string        `json:"host,omitempty" yaml:"host,omitempty"`
	Port           int           `json:"port,omitempty" yaml:"port,omitempty"`
	User           string        `json:"user,omitempty" yaml:"user,omitempty"`
	Password       string        `json:"password,omitempty" yaml:"password,omitempty"`
	PrivateKey     []byte        `json:"-" yaml:"-"`
	PrivateKeyPath string        `json:"privateKeyPath,omitempty" yaml:"privateKeyPath,omitempty"`
	Timeout        time.Duration `json:"timeout,omitempty" yaml:"timeout,omitempty"`
}

// ProxyCfg defines proxy configuration.
type ProxyCfg struct {
	URL string
}

// ConnectionCfg holds all parameters needed to establish a connection.
type ConnectionCfg struct {
	Host              string
	Port              int
	User              string
	Password          string
	PrivateKey        []byte
	PrivateKeyPath    string
	Timeout           time.Duration
	BastionCfg        *BastionCfg
	ProxyCfg          *ProxyCfg
}

// ExecOptions is defined in options.go
// FileTransferOptions is defined in options.go

// FileStat describes basic file metadata.
type FileStat struct {
	Name    string
	Size    int64
	Mode    fs.FileMode // Using fs.FileMode consistently
	ModTime time.Time
	IsDir   bool
	IsExist bool
}

// ExitError is an error type that includes the exit code of a command.
type ExitError struct {
	Err      error
	ExitCode int
	Stdout   string
	Stderr   string
	Cmd      string
}

func (e *ExitError) Error() string {
	if e.Cmd != "" {
		return fmt.Sprintf("command '%s' failed with exit code %d: %s (stderr: %s)", e.Cmd, e.ExitCode, e.Err, e.Stderr)
	}
	return fmt.Sprintf("command failed with exit code %d: %s (stderr: %s)", e.ExitCode, e.Err, e.Stderr)
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

// Connector defines the interface for connecting to and interacting with a host.
type Connector interface {
	Connect(ctx context.Context, cfg ConnectionCfg) error
	Exec(ctx context.Context, cmd string, opts *ExecOptions) (stdout, stderr []byte, err error)
	CopyContent(ctx context.Context, content []byte, destPath string, options *FileTransferOptions) error
	Stat(ctx context.Context, path string) (*FileStat, error)
	LookPath(ctx context.Context, file string) (string, error)

	Close() error
	IsConnected() bool
	GetOS(ctx context.Context) (*OS, error)
	ReadFile(ctx context.Context, path string) ([]byte, error)
	WriteFile(ctx context.Context, content []byte, destPath, permissions string, sudo bool) error
	Mkdir(ctx context.Context, path string, perm string) error
	Remove(ctx context.Context, path string, opts RemoveOptions) error
	GetFileChecksum(ctx context.Context, path string, checksumType string) (string, error)
}

// RemoveOptions defines options for file/directory removal.
type RemoveOptions struct {
	Recursive      bool
	IgnoreNotExist bool
}

// Host represents a configured host in the cluster.
type Host interface {
	GetName() string
	GetAddress() string
	GetPort() int
	GetUser() string
	GetRoles() []string
	GetHostSpec() v1alpha1.HostSpec
	GetArch() string
}
