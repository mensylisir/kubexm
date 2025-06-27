package connector

import (
	"context"
	// "fmt" // Removed as unused
	"io/fs" // For fs.FileMode
	"time"

	"golang.org/x/crypto/ssh" // Added to resolve undefined: ssh

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
)

type Factory interface {
	NewSSHConnector(pool *ConnectionPool) Connector
	NewLocalConnector() Connector
}

// OS represents operating system details.
type OS struct {
	ID         string // e.g., "ubuntu", "centos", "windows"
	VersionID  string // e.g., "20.04", "7", "10.0.19042"
	PrettyName string // e.g., "Ubuntu 20.04.3 LTS"
	Codename   string // e.g., "focal", "bionic"
	Arch       string // e.g., "amd64", "arm64"
	Kernel     string // e.g., "5.4.0-80-generic"
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
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
}

// ConnectionCfg holds all parameters needed to establish a connection.
type ConnectionCfg struct {
	Host           string
	Port           int
	User           string
	Password       string
	PrivateKey     []byte
	PrivateKeyPath string
	Timeout        time.Duration
	BastionCfg     *BastionCfg // Renamed from Bastion in design doc to match existing code and be clearer
	ProxyCfg       *ProxyCfg
	HostKeyCallback ssh.HostKeyCallback `json:"-" yaml:"-"` // Callback for verifying server keys
}

// ExecOptions is defined in options.go
// FileTransferOptions is defined in options.go

// FileStat describes basic file metadata.
type FileStat struct {
	Name    string
	Size    int64
	Mode    fs.FileMode // Using fs.FileMode consistently as per existing code
	ModTime time.Time
	IsDir   bool
	IsExist bool
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
