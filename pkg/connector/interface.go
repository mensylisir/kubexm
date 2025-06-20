package connector

import (
	"context"
	"time" // Added for ConnectionCfg.Timeout
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // For HostSpec if NewHostFromSpec is here
)

// OS represents operating system details.
type OS struct {
	ID         string // e.g., "ubuntu", "centos", "windows"
	VersionID  string // e.g., "20.04", "7", "10.0.19042"
	PrettyName string // e.g., "Ubuntu 20.04.3 LTS"
	Arch       string // Added Arch as it's commonly needed
	Kernel     string // Added Kernel as it's commonly needed
}

// ConnectionCfg holds all parameters needed to establish a connection.
type ConnectionCfg struct {
	Host              string
	Port              int
	User              string
	Password          string
	PrivateKey        []byte // Actual private key content
	PrivateKeyPath    string // Path to private key file
	Timeout           time.Duration
	BastionCfg        *BastionCfg // Optional bastion/jump host config
	ProxyCfg          *ProxyCfg   // Optional proxy config
	// SSH specific arguments can go here if needed
}

type BastionCfg struct {
	// ... fields for bastion host connection ...
	// Example:
	// Host           string
	// Port           int
	// User           string
	// Password       string
	// PrivateKey     []byte
	// PrivateKeyPath string
	// Timeout        time.Duration
}

type ProxyCfg struct {
	// ... fields for proxy settings ...
	// Example:
	// URL string // e.g., "socks5://user:pass@host:port"
	// Or separate fields for type, host, port, user, pass
}

// ExecOptions defines options for command execution.
// This was missing from the provided snippet but is generally part of a connector interface.
type ExecOptions struct {
	Sudo    bool
	Timeout time.Duration
	Env     []string
	Check   bool // If true, non-zero exit codes might not be errors but indicate failure of a check
	// Add other options like Stream, Hidden, etc. as needed
}

// FileTransferOptions defines options for file transfer.
// This was also missing and is commonly needed.
type FileTransferOptions struct {
	Permissions string
	Owner       string
	Group       string
	Sudo        bool
	Timeout     time.Duration
}

// FileStat describes basic file metadata.
// This was missing and is commonly needed for Exists/IsDir type checks.
type FileStat struct {
	Name    string
	Size    int64
	Mode    uint32 // Using uint32 for os.FileMode compatibility, or os.FileMode directly
	ModTime time.Time
	IsDir   bool
	IsExist bool // Explicitly track if the file exists
}

// ExitError is an error type that includes the exit code of a command.
// This is a common pattern for command execution errors.
type ExitError struct {
	Err      error
	ExitCode int
}

func (e *ExitError) Error() string {
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

func (e *ExitError) IsNotExist() bool { // Example method for Exists/IsDir checks
    // This depends on how your Exec returns errors for "file not found" via commands.
    // For 'test -e', a non-zero exit code implies not found.
    // This is a simplified example.
    return e.ExitCode != 0
}


// Connector defines the interface for connecting to and interacting with a host.
type Connector interface {
	Connect(ctx context.Context, cfg ConnectionCfg) error
	Exec(ctx context.Context, cmd string, opts *ExecOptions) (stdout, stderr []byte, err error)
	// ReadFile and WriteFile are now part of the Runner, which uses the Connector.
	// If a Connector implementation can do this more efficiently (e.g. SFTP direct read/write),
	// the Runner's default implementation can try to type-assert to a more specific interface.
	// For now, let's assume the basic Connector only needs Exec and CopyContent for file ops by runner.
	CopyContent(ctx context.Context, content []byte, destPath string, options *FileTransferOptions) error
	Stat(ctx context.Context, path string) (*FileStat, error) // For Exists/IsDir checks by runner
	LookPath(ctx context.Context, file string) (string, error) // For finding commands

	Close() error
	IsConnected() bool
	GetOS(ctx context.Context) (*OS, error)
	ReadFile(ctx context.Context, path string) ([]byte, error) // Added as per runner's expectation
	WriteFile(ctx context.Context, content []byte, destPath, permissions string, sudo bool) error // Added
}

// Host represents a configured host in the cluster, along with its specific configuration.
// It's an abstraction over the v1alpha1.HostSpec to decouple runner/engine from API types.
type Host interface {
	GetName() string                 // Returns the unique name of the host.
	GetAddress() string              // Returns the primary address (IP or FQDN) for connection.
	GetPort() int                    // Returns the connection port.
	GetUser() string                 // Returns the user for connection.
	GetRoles() []string              // Returns the list of roles assigned to this host.
	GetHostSpec() v1alpha1.HostSpec // Provides access to the original spec if needed for deeper details by some components
}

// Ensure connector.Connector has ReadFile and WriteFile methods
// These are assumed by the defaultRunner implementations of ReadFile/WriteFile.
// If not, those methods need to be implemented via Exec.
// This is a type alias for documentation, not functional.
type ExtendedConnector interface {
	Connector
	ReadFile(ctx context.Context, path string) ([]byte, error)
	WriteFile(ctx context.Context, content []byte, destPath, permissions string, sudo bool) error
}
