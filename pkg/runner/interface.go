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
	Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error
	GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error)
	LookPath(ctx context.Context, conn connector.Connector, file string) (string, error)
	IsPortOpen(ctx context.Context, conn connector.Connector, facts *Facts, port int) (bool, error)
	WaitForPort(ctx context.Context, conn connector.Connector, facts *Facts, port int, timeout time.Duration) error
	SetHostname(ctx context.Context, conn connector.Connector, facts *Facts, hostname string) error
	AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error
	InstallPackages(ctx context.Context, conn connector.Connector, facts *Facts, packages ...string) error
	RemovePackages(ctx context.Context, conn connector.Connector, facts *Facts, packages ...string) error
	UpdatePackageCache(ctx context.Context, conn connector.Connector, facts *Facts) error
	IsPackageInstalled(ctx context.Context, conn connector.Connector, facts *Facts, packageName string) (bool, error)
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

	// --- System & Kernel ---
	LoadModule(ctx context.Context, conn connector.Connector, facts *Facts, moduleName string, params ...string) error
	IsModuleLoaded(ctx context.Context, conn connector.Connector, moduleName string) (bool, error)
	ConfigureModuleOnBoot(ctx context.Context, conn connector.Connector, facts *Facts, moduleName string, params ...string) error
	SetSysctl(ctx context.Context, conn connector.Connector, key, value string, persistent bool) error
	SetTimezone(ctx context.Context, conn connector.Connector, facts *Facts, timezone string) error
	DisableSwap(ctx context.Context, conn connector.Connector, facts *Facts) error
	IsSwapEnabled(ctx context.Context, conn connector.Connector) (bool, error)

	// --- Filesystem & Storage ---
	EnsureMount(ctx context.Context, conn connector.Connector, device, mountPoint, fsType string, options []string, persistent bool) error
	IsMounted(ctx context.Context, conn connector.Connector, path string) (bool, error)
	MakeFilesystem(ctx context.Context, conn connector.Connector, device, fsType string, force bool) error
	CreateSymlink(ctx context.Context, conn connector.Connector, target, linkPath string, sudo bool) error
	GetDiskUsage(ctx context.Context, conn connector.Connector, path string) (total uint64, free uint64, used uint64, err error)
	TouchFile(ctx context.Context, conn connector.Connector, path string, sudo bool) error

	// --- Networking ---
	DisableFirewall(ctx context.Context, conn connector.Connector, facts *Facts) error
	GetInterfaceAddresses(ctx context.Context, conn connector.Connector, interfaceName string) (map[string][]string, error)

	// --- User & Permissions ---
	ModifyUser(ctx context.Context, conn connector.Connector, username string, modifications UserModifications) error
	ConfigureSudoer(ctx context.Context, conn connector.Connector, sudoerName, content string) error
	SetUserPassword(ctx context.Context, conn connector.Connector, username, hashedPassword string) error
	GetUserInfo(ctx context.Context, conn connector.Connector, username string) (*UserInfo, error)

	// --- High-Level ---
	DeployAndEnableService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName, configContent, configPath, permissions string, templateData interface{}) error
	Reboot(ctx context.Context, conn connector.Connector, timeout time.Duration) error
	RenderToString(ctx context.Context, tmpl *template.Template, data interface{}) (string, error) // Different from Render, no conn/destPath
}

// UserModifications defines the set of attributes that can be changed for a user.
// Pointers are used for string fields to distinguish between a requested empty value (not usually applicable for these fields)
// and a value that should not be changed (nil pointer).
type UserModifications struct {
	NewUsername         *string  // New login name (-l NEW_LOGIN)
	NewPrimaryGroup     *string  // New primary group name or GID (-g GROUP)
	AppendToGroups      []string // Groups to add the user to (appends to existing secondary groups -aG GROUP1,GROUP2)
	SetSecondaryGroups  []string // Explicitly set secondary groups (replaces all existing secondary groups -G GROUP1,GROUP2)
	NewShell            *string  // New login shell (-s SHELL)
	NewHomeDir          *string  // New home directory path (-d HOME_DIR)
	MoveHomeDirContents bool     // If NewHomeDir is set, move contents from old home to new home (requires -m flag with -d)
	NewComment          *string  // New GECOS comment field (-c COMMENT)
}

// UserInfo holds detailed information about a user.
type UserInfo struct {
	Username string
	UID      string
	GID      string
	Comment  string   // GECOS field
	HomeDir  string
	Shell    string
	Groups   []string // List of group names the user belongs to
}
