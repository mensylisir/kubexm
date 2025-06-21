package runner

import (
	"context"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// Facts, PackageInfo, ServiceInfo etc. structure definitions
type Facts struct {
	OS             *connector.OS
	Hostname       string
	Kernel         string
	TotalMemory    uint64 // in MiB
	TotalCPU       int
	IPv4Default    string
	IPv6Default    string
	PackageManager *PackageInfo
	InitSystem     *ServiceInfo
}
type PackageManagerType string
const (
	PackageManagerUnknown PackageManagerType = "unknown"
	PackageManagerApt     PackageManagerType = "apt"
	PackageManagerYum     PackageManagerType = "yum"
	PackageManagerDnf     PackageManagerType = "dnf"
)
type PackageInfo struct {
	Type          PackageManagerType
	UpdateCmd     string
	InstallCmd    string
	RemoveCmd     string
	PkgQueryCmd   string
	CacheCleanCmd string
}
type InitSystemType string
const (
	InitSystemUnknown InitSystemType = "unknown"
	InitSystemSystemd InitSystemType = "systemd"
	InitSystemSysV    InitSystemType = "sysvinit"
)
type ServiceInfo struct {
	Type            InitSystemType
	StartCmd        string
	StopCmd         string
	EnableCmd       string
	DisableCmd      string
	RestartCmd      string
	IsActiveCmd     string
	IsEnabledCmd    string // Added for completeness based on typical needs
	DaemonReloadCmd string
}


// Runner interface defines a complete, stateless host operation service library.
type Runner interface {
	GatherFacts(ctx context.Context, conn connector.Connector) (*Facts, error)
	Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error)
	MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string
	Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error)
	RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) (stdout, stderr []byte, err error)
	Download(ctx context.Context, conn connector.Connector, facts *Facts, url, destPath string, sudo bool) error
	Extract(ctx context.Context, conn connector.Connector, facts *Facts, archivePath, destDir string, sudo bool) error
	DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *Facts, url, destDir string, sudo bool) error
	Exists(ctx context.Context, conn connector.Connector, path string) (bool, error)
	IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error)
	ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error)
	WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error
	Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error
	Remove(ctx context.Context, conn connector.Connector, path string, sudo bool) error
	Chmod(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error
	Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool, sudo bool) error // Added sudo to chown
	GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error)
	LookPath(ctx context.Context, conn connector.Connector, file string) (string, error)
	IsPortOpen(ctx context.Context, conn connector.Connector, facts *Facts, port int) (bool, error)
	WaitForPort(ctx context.Context, conn connector.Connector, facts *Facts, port int, timeout time.Duration) error
	SetHostname(ctx context.Context, conn connector.Connector, facts *Facts, hostname string) error
	AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error
	InstallPackages(ctx context.Context, conn connector.Connector, facts *Facts, packages ...string) error
	RemovePackages(ctx context.Context, conn connector.Connector, facts *Facts, packages ...string) error
	UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *Facts) error
	IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *Facts, packageName string) (installed bool, version string, err error) // return installed status, version, error
	AddRepository(ctx context.Context, conn connector.Connector, facts *Facts, repoConfig string, isFilePath bool) error
	StartService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error
	StopService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error
	RestartService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error
	EnableService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error
	DisableService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error
	IsServiceActive(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) (bool, error)
	DaemonReload(ctx context.Context, conn connector.Connector, facts *Facts) error
	Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error
	UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error)
	GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error)
	AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error
	AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error
}
