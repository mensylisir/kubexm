package connector

import (
	"context"
	"os"
	"time"
)

// FileStat describes the metadata of a remote file or directory.
type FileStat struct {
	Name    string      // Name of the file or directory
	Size    int64       // Size of the file in bytes
	Mode    os.FileMode // File permissions and mode
	ModTime time.Time   // Last modification time
	IsDir   bool        // True if it's a directory
	IsExist bool        // True if the file or directory exists
	Sum     string      // SHA256 checksum of the file (if calculated)
	Owner   string      // File owner
	Group   string      // File group
}

// OS stores detailed information about the remote operating system.
type OS struct {
	ID        string // e.g., "ubuntu", "centos", "debian", "rhel"
	VersionID string // e.g., "22.04", "7", "9", "11"
	Codename  string // e.g., "jammy", "bullseye"
	Arch      string // e.g., "amd64", "arm64"
	Variant   string // e.g., "server"
	Kernel    string // Kernel version, e.g., "5.15.0-41-generic"
}

// ConnectionCfg defines all configurations needed to establish a connection.
type ConnectionCfg struct {
	Host           string
	Port           int
	User           string
	Password       string
	PrivateKey     []byte // Content of the private key
	PrivateKeyPath string // Path to the private key file
	Timeout        time.Duration
	Bastion        *ConnectionCfg // Bastion/jump host configuration
}

// Connector is the feature-rich interface for interacting with a host.
type Connector interface {
	// Connect establishes and verifies the connection using the provided configuration.
	// This is the first step for all operations.
	Connect(ctx context.Context, cfg ConnectionCfg) error

	// Exec executes a command on the connected host.
	// Returns standard output, standard error, and a wrapped CommandError.
	Exec(ctx context.Context, cmd string, options *ExecOptions) (stdout, stderr []byte, err error)

	// Copy copies a local file or directory to the remote host.
	Copy(ctx context.Context, srcPath, dstPath string, options *FileTransferOptions) error

	// CopyContent writes a byte stream from memory directly to a remote file.
	// This is ideal for uploading rendered templates, avoiding local temporary files.
	CopyContent(ctx context.Context, content []byte, dstPath string, options *FileTransferOptions) error

	// Fetch retrieves a file or directory from the remote host to local.
	Fetch(ctx context.Context, remotePath, localPath string) error

	// Stat gets detailed status information of a remote file or directory.
	// If the file does not exist, it returns a FileStat with IsExist=false and nil error.
	Stat(ctx context.Context, path string) (*FileStat, error)

	// LookPath finds an executable file in the remote host's $PATH (similar to os.LookPath).
	// Returns an error if not found.
	LookPath(ctx context.Context, file string) (string, error)

	// GetOS probes and returns detailed information about the remote operating system.
	// Connector implementations should cache this result to avoid repeated probe commands.
	GetOS(ctx context.Context) (*OS, error)

	// IsConnected checks if the current connection is still active.
	IsConnected() bool

	// Close gracefully disconnects the connection and releases all resources.
	Close() error
}
