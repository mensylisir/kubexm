package connector

import (
	"context"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"golang.org/x/crypto/ssh"
	"io/fs"
	"time"
)

type OS struct {
	ID         string // e.g., "ubuntu", "centos", "windows"
	VersionID  string // e.g., "20.04", "7", "10.0.19042"
	PrettyName string // e.g., "Ubuntu 20.04.3 LTS"
	Codename   string // e.g., "focal", "bionic"
	Arch       string // e.g., "amd64", "arm64"
	Kernel     string // e.g., "5.4.0-80-generic"
}

type BastionCfg struct {
	Host            string              `json:"host,omitempty" yaml:"host,omitempty"`
	Port            int                 `json:"port,omitempty" yaml:"port,omitempty"`
	User            string              `json:"user,omitempty" yaml:"user,omitempty"`
	Password        string              `json:"password,omitempty" yaml:"password,omitempty"`
	PrivateKey      []byte              `json:"-" yaml:"-"`
	PrivateKeyPath  string              `json:"privateKeyPath,omitempty" yaml:"privateKeyPath,omitempty"`
	Timeout         time.Duration       `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	HostKeyCallback ssh.HostKeyCallback `json:"-" yaml:"-"`
}

type ProxyCfg struct {
	URL string `json:"url,omitempty" yaml:"url,omitempty"`
}

type ConnectionCfg struct {
	Host            string
	Port            int
	User            string
	Password        string
	PrivateKey      []byte
	PrivateKeyPath  string
	Timeout         time.Duration
	BastionCfg      *BastionCfg
	ProxyCfg        *ProxyCfg
	HostKeyCallback ssh.HostKeyCallback `json:"-" yaml:"-"`
}

type FileStat struct {
	Name    string
	Size    int64
	Mode    fs.FileMode
	ModTime time.Time
	IsDir   bool
	IsExist bool
}

type Connector interface {
	Connect(ctx context.Context, cfg ConnectionCfg) error
	Exec(ctx context.Context, cmd string, opts *ExecOptions) (stdout, stderr []byte, err error)
	Upload(ctx context.Context, localPath, remotePath string, opts *FileTransferOptions) error
	Download(ctx context.Context, remotePath, localPath string, opts *FileTransferOptions) error
	Fetch(ctx context.Context, remotePath, localPath string, opts *FileTransferOptions) error
	CopyContent(ctx context.Context, content []byte, destPath string, options *FileTransferOptions) error
	Stat(ctx context.Context, path string) (*FileStat, error)
	StatWithOptions(ctx context.Context, path string, opts *StatOptions) (*FileStat, error)
	LookPath(ctx context.Context, file string) (string, error)
	LookPathWithOptions(ctx context.Context, file string, opts *LookPathOptions) (string, error)
	Close() error
	IsConnected() bool
	GetOS(ctx context.Context) (*OS, error)
	ReadFile(ctx context.Context, path string) ([]byte, error)
	ReadFileWithOptions(ctx context.Context, path string, opts *FileTransferOptions) ([]byte, error)
	WriteFile(ctx context.Context, content []byte, destPath string, options *FileTransferOptions) error
	Mkdir(ctx context.Context, path string, perm string) error
	Remove(ctx context.Context, path string, opts RemoveOptions) error
	GetFileChecksum(ctx context.Context, path string, checksumType string) (string, error)
	GetConnectionConfig() ConnectionCfg
}

type Host interface {
	GetName() string
	SetName(name string)
	GetAddress() string
	SetAddress(str string)
	GetInternalAddress() string
	GetInternalIPv4Address() string
	GetInternalIPv6Address() string
	SetInternalAddress(str string)
	GetPort() int
	SetPort(port int)
	GetUser() string
	SetUser(u string)
	GetPassword() string
	SetPassword(password string)
	GetPrivateKey() string
	SetPrivateKey(privateKey string)
	GetPrivateKeyPath() string
	SetPrivateKeyPath(path string)
	GetArch() string
	SetArch(arch string)
	GetTimeout() int64
	SetTimeout(timeout int64)
	GetRoles() []string
	SetRoles(roles []string)
	IsRole(role string) bool
	GetHostSpec() v1alpha1.HostSpec
}

type dialSSHFunc func(ctx context.Context, cfg ConnectionCfg, connectTimeout time.Duration) (*ssh.Client, *ssh.Client, error)
