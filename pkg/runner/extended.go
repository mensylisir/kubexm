package runner

import (
	"context"
	"os"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// ExtendedRunner adds comprehensive operations for system administration,
// Kubernetes, containers, networking, security, monitoring, and more.
type ExtendedRunner interface {
	// ==================== System Administration ====================

	// System Information
	GetSystemUptime(ctx context.Context, conn connector.Connector) (time.Duration, error)
	GetSystemLoad(ctx context.Context, conn connector.Connector) (float64, float64, float64, error)
	GetSystemMemoryUsage(ctx context.Context, conn connector.Connector) (used, total, available uint64, err error)
	GetSystemCPUUsage(ctx context.Context, conn connector.Connector) (float64, error)
	GetSystemDiskIO(ctx context.Context, conn connector.Connector) (readBytes, writeBytes uint64, err error)
	GetSystemNetworkIO(ctx context.Context, conn connector.Connector) (rxBytes, txBytes uint64, err error)

	// System Operations
	SetSystemTime(ctx context.Context, conn connector.Connector, timezone string) error
	GetSystemTime(ctx context.Context, conn connector.Connector) (time.Time, error)
	SyncSystemTime(ctx context.Context, conn connector.Connector, ntpServer string) error
	ConfigureNTP(ctx context.Context, conn connector.Connector, ntpServers []string, isServer bool) error
	ConfigureTimezone(ctx context.Context, conn connector.Connector, timezone string) error

	// System Limits
	GetSystemLimits(ctx context.Context, conn connector.Connector) (maxFiles, maxProcs uint64, err error)
	SetSystemLimit(ctx context.Context, conn connector.Connector, limitType string, soft, hard uint64) error
	ConfigureULimits(ctx context.Context, conn connector.Connector, limits map[string]string) error

	// Kernel Parameters
	GetKernelParameter(ctx context.Context, conn connector.Connector, param string) (string, error)
	SetKernelParameter(ctx context.Context, conn connector.Connector, param, value string) error
	GetAllKernelParameters(ctx context.Context, conn connector.Connector) (map[string]string, error)
	ConfigureSysctl(ctx context.Context, conn connector.Connector, params map[string]string) error

	// ==================== Kubernetes Operations ====================

	// Kubernetes Cluster
	GetKubernetesVersion(ctx context.Context, conn connector.Connector) (string, error)
	GetKubernetesNodes(ctx context.Context, conn connector.Connector, kubeconfigPath string) ([]string, error)
	GetKubernetesPods(ctx context.Context, conn connector.Connector, namespace, kubeconfigPath string) ([]string, error)
	GetKubernetesServices(ctx context.Context, conn connector.Connector, namespace, kubeconfigPath string) ([]string, error)
	GetKubernetesEndpoints(ctx context.Context, conn connector.Connector, namespace, kubeconfigPath string) ([]string, error)

	// Kubernetes Resources
	ApplyKubernetesManifest(ctx context.Context, conn connector.Connector, manifest string, kubeconfigPath string, namespace string) error
	DeleteKubernetesResource(ctx context.Context, conn connector.Connector, resourceType, name, namespace, kubeconfigPath string) error
	GetKubernetesResource(ctx context.Context, conn connector.Connector, resourceType, name, namespace, kubeconfigPath string, output string) (string, error)
	ScaleKubernetesDeployment(ctx context.Context, conn connector.Connector, deployment, namespace, replicas string, kubeconfigPath string) error
	RolloutRestartKubernetesDeployment(ctx context.Context, conn connector.Connector, deployment, namespace, kubeconfigPath string) error

	// Kubernetes Node Operations
	DrainKubernetesNode(ctx context.Context, conn connector.Connector, nodeName, kubeconfigPath string, options map[string]string) error
	CordonKubernetesNode(ctx context.Context, conn connector.Connector, nodeName, kubeconfigPath string) error
	UncordonKubernetesNode(ctx context.Context, conn connector.Connector, nodeName, kubeconfigPath string) error
	LabelKubernetesNode(ctx context.Context, conn connector.Connector, nodeName, labels map[string]string, kubeconfigPath string) error
	TaintKubernetesNode(ctx context.Context, conn connector.Connector, nodeName string, taints []string, kubeconfigPath string) error

	// Kubernetes Certificates
	CheckKubernetesCertExpiration(ctx context.Context, conn connector.Connector, certPath string) (time.Duration, error)
	RotateKubernetesCert(ctx context.Context, conn connector.Connector, certPath, newCertPath string) error

	// ==================== Container Operations ====================

	// Container Management
	ListContainers(ctx context.Context, conn connector.Connector, runtime string, all bool) ([]string, error)
	StartContainer(ctx context.Context, conn connector.Connector, containerName, runtime string) error
	StopContainer(ctx context.Context, conn connector.Connector, containerName, runtime string, timeout time.Duration) error
	RestartContainer(ctx context.Context, conn connector.Connector, containerName, runtime string, timeout time.Duration) error
	RemoveContainer(ctx context.Context, conn connector.Connector, containerName, runtime string, force bool) error
	GetContainerIP(ctx context.Context, conn connector.Connector, containerName, runtime string) (string, error)
	GetContainerLogs(ctx context.Context, conn connector.Connector, containerName, runtime string, tailLines int) (string, error)
	ExecuteInContainer(ctx context.Context, conn connector.Connector, containerName, runtime string, cmd []string) (string, string, error)

	// Container Images
	ListContainerImages(ctx context.Context, conn connector.Connector, runtime string) ([]string, error)
	PullContainerImage(ctx context.Context, conn connector.Connector, imageName, runtime string) error
	RemoveContainerImage(ctx context.Context, conn connector.Connector, imageName, runtime string, force bool) error
	TagContainerImage(ctx context.Context, conn connector.Connector, sourceImage, targetImage, runtime string) error
	BuildContainerImage(ctx context.Context, conn connector.Connector, dockerfilePath, imageName, contextPath string, buildArgs map[string]string) error

	// Container Networks
	ListContainerNetworks(ctx context.Context, conn connector.Connector, runtime string) ([]string, error)
	CreateContainerNetwork(ctx context.Context, conn connector.Connector, networkName, driver, subnet, gateway string, options map[string]string) error
	RemoveContainerNetwork(ctx context.Context, conn connector.Connector, networkName, runtime string) error
	ConnectContainerToNetwork(ctx context.Context, conn connector.Connector, containerName, networkName, runtime string, ipAddress string) error
	DisconnectContainerFromNetwork(ctx context.Context, conn connector.Connector, containerName, networkName, runtime string, force bool) error

	// Container Volumes
	ListContainerVolumes(ctx context.Context, conn connector.Connector, runtime string) ([]string, error)
	CreateContainerVolume(ctx context.Context, conn connector.Connector, volumeName, driver string, driverOpts, labels map[string]string) error
	RemoveContainerVolume(ctx context.Context, conn connector.Connector, volumeName, runtime string, force bool) error

	// ==================== Network Operations ====================

	// Network Configuration
	GetNetworkInterfaces(ctx context.Context, conn connector.Connector) ([]NetworkInterface, error)
	ConfigureNetworkInterface(ctx context.Context, conn connector.Connector, ifaceName string, config map[string]string) error
	EnableNetworkInterface(ctx context.Context, conn connector.Connector, ifaceName string) error
	DisableNetworkInterface(ctx context.Context, conn connector.Connector, ifaceName string) error
	AddNetworkRoute(ctx context.Context, conn connector.Connector, destination, gateway, ifaceName string) error
	DeleteNetworkRoute(ctx context.Context, conn connector.Connector, destination, gateway string) error

	// DNS Configuration
	GetDNSConfig(ctx context.Context, conn connector.Connector) ([]string, error)
	SetDNSConfig(ctx context.Context, conn connector.Connector, nameservers []string, searchDomains []string) error
	AddDNSEntry(ctx context.Context, conn connector.Connector, hostname, ipAddress string) error
	RemoveDNSEntry(ctx context.Context, conn connector.Connector, hostname string) error
	TestDNSResolution(ctx context.Context, conn connector.Connector, hostname string) (time.Duration, error)

	// Firewall Configuration
	GetFirewallStatus(ctx context.Context, conn connector.Connector) (enabled bool, err error)
	EnableFirewall(ctx context.Context, conn connector.Connector) error
	DisableFirewall(ctx context.Context, conn connector.Connector) error
	AddFirewallRule(ctx context.Context, conn connector.Connector, rule string) error
	RemoveFirewallRule(ctx context.Context, conn connector.Connector, rule string) error
	ListFirewallRules(ctx context.Context, conn connector.Connector) ([]string, error)

	// Network Testing
	TestPortConnectivity(ctx context.Context, conn connector.Connector, host string, port int, timeout time.Duration) (bool, error)
	Traceroute(ctx context.Context, conn connector.Connector, destination string, maxHops int) ([]string, error)
	MTUTest(ctx context.Context, conn connector.Connector, host string, mtu int) (bool, error)
	BandwidthTest(ctx context.Context, conn connector.Connector, host string, port int, duration time.Duration) (uint64, error)

	// ==================== Security Operations ====================

	// User and Group Management
	ListUsers(ctx context.Context, conn connector.Connector) ([]string, error)
	ListGroups(ctx context.Context, conn connector.Connector) ([]string, error)
	GetUserInfo(ctx context.Context, conn connector.Connector, username string) (*UserInfo, error)
	CreateUser(ctx context.Context, conn connector.Connector, username, password, shell string, groups []string) error
	DeleteUser(ctx context.Context, conn connector.Connector, username string, removeHome bool) error
	ModifyUser(ctx context.Context, conn connector.Connector, username string, changes UserModifications) error
	CreateGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error
	DeleteGroup(ctx context.Context, conn connector.Connector, groupname string) error
	AddUserToGroup(ctx context.Context, conn connector.Connector, username, groupname string) error
	RemoveUserFromGroup(ctx context.Context, conn connector.Connector, username, groupname string) error

	// SSH Configuration
	GetSSHConfig(ctx context.Context, conn connector.Connector) (map[string]string, error)
	SetSSHConfig(ctx context.Context, conn connector.Connector, config map[string]string) error
	GetSSHAuthorizedKeys(ctx context.Context, conn connector.Connector, username string) ([]string, error)
	AddSSHAuthorizedKey(ctx context.Context, conn connector.Connector, username, key string) error
	RemoveSSHAuthorizedKey(ctx context.Context, conn connector.Connector, username, key string) error

	// Security Hardening
	CheckSELinuxStatus(ctx context.Context, conn connector.Connector) (string, error)
	SetSELinuxMode(ctx context.Context, conn connector.Connector, mode string) error
	CheckAppArmorStatus(ctx context.Context, conn connector.Connector) (bool, error)
	ConfigureAppArmorProfile(ctx context.Context, conn connector.Connector, profileName, profileContent string) error

	// Certificate Management
	GenerateSelfSignedCert(ctx context.Context, conn connector.Connector, commonName, org string, days int) (certPEM, keyPEM []byte, err error)
	GenerateCSR(ctx context.Context, conn connector.Connector, commonName, org string, sans []string) (csrPEM, keyPEM []byte, err error)
	SignCert(ctx context.Context, conn connector.Connector, csrPEM, caCertPEM, caKeyPEM []byte, days int) (certPEM []byte, err error)
	VerifyCert(ctx context.Context, conn connector.Connector, certPEM, caCertPEM []byte) (bool, error)
	GetCertExpiration(ctx context.Context, conn connector.Connector, certPEM []byte) (time.Time, error)

	// ==================== Monitoring Operations ====================

	// Process Monitoring
	ListProcesses(ctx context.Context, conn connector.Connector) ([]ProcessInfo, error)
	GetProcessInfo(ctx context.Context, conn connector.Connector, pid int) (*ProcessInfo, error)
	FindProcessByName(ctx context.Context, conn connector.Connector, processName string) ([]int, error)
	KillProcess(ctx context.Context, conn connector.Connector, pid int, signal string) error
	TopProcesses(ctx context.Context, conn connector.Connector, sortBy string, limit int) ([]ProcessInfo, error)

	// Resource Monitoring
	GetSystemResources(ctx context.Context, conn connector.Connector) (*SystemResources, error)
	GetProcessResources(ctx context.Context, conn connector.Connector, pid int) (*ProcessResources, error)
	GetIOStats(ctx context.Context, conn connector.Connector) (*IOStats, error)
	GetNetworkStats(ctx context.Context, conn connector.Connector) (*NetworkStats, error)

	// Log Monitoring
	TailLog(ctx context.Context, conn connector.Connector, logPath string, lines int) ([]string, error)
	GrepLog(ctx context.Context, conn connector.Connector, logPath, pattern string, tailLines int) ([]string, error)
	RotateLog(ctx context.Context, conn connector.Connector, logPath string, maxSize int64, maxFiles int) error
	CollectSystemLogs(ctx context.Context, conn connector.Connector, since time.Time) ([]string, error)

	// Health Checks
	HealthCheckCPU(ctx context.Context, conn connector.Connector, threshold float64) (bool, error)
	HealthCheckMemory(ctx context.Context, conn connector.Connector, threshold float64) (bool, error)
	HealthCheckDisk(ctx context.Context, conn connector.Connector, threshold float64) (bool, error)
	HealthCheckService(ctx context.Context, conn connector.Connector, serviceName string) (bool, error)

	// ==================== Backup Operations ====================

	// File Backup
	BackupDirectory(ctx context.Context, conn connector.Connector, sourcePath, destPath string, excludePatterns []string) error
	RestoreDirectory(ctx context.Context, conn connector.Connector, backupPath, destPath string) error
	IncrementalBackup(ctx context.Context, conn connector.Connector, sourcePath, destPath, lastBackupTime string) error

	// Database Backup
	BackupMySQL(ctx context.Context, conn connector.Connector, database, destPath string, options map[string]string) error
	RestoreMySQL(ctx context.Context, conn connector.Connector, database, backupPath string) error
	BackupPostgreSQL(ctx context.Context, conn connector.Connector, database, destPath string, options map[string]string) error
	RestorePostgreSQL(ctx context.Context, conn connector.Connector, database, backupPath string) error

	// Snapshot Management
	CreateSnapshot(ctx context.Context, conn connector.Connector, volumePath, snapshotPath string) error
	RestoreSnapshot(ctx context.Context, conn connector.Connector, snapshotPath, volumePath string) error
	ListSnapshots(ctx context.Context, conn connector.Connector, volumePath string) ([]string, error)
	DeleteSnapshot(ctx context.Context, conn connector.Connector, snapshotPath string) error

	// ==================== Package Management ====================

	// Repository Management
	AddRepository(ctx context.Context, conn connector.Connector, repoType, name, url string) error
	RemoveRepository(ctx context.Context, conn connector.Connector, repoType, name string) error
	UpdateRepositoryCache(ctx context.Context, conn connector.Connector) error
	ListRepositories(ctx context.Context, conn connector.Connector, repoType string) ([]string, error)

	// Package Operations
	SearchPackage(ctx context.Context, conn connector.Connector, packageName string) ([]string, error)
	GetPackageInfo(ctx context.Context, conn connector.Connector, packageName string) (*PackageInfo, error)
	InstallPackageVersion(ctx context.Context, conn connector.Connector, packageName, version string) error
	RemovePackage(ctx context.Context, conn connector.Connector, packageName string, purge bool) error
	UpgradePackage(ctx context.Context, conn connector.Connector, packageName string) error
	UpgradeAllPackages(ctx context.Context, conn connector.Connector) error

	// ==================== File Operations ====================

	// Advanced File Operations
	FindFiles(ctx context.Context, conn connector.Connector, path, pattern string, maxDepth int) ([]string, error)
	CountFiles(ctx context.Context, conn connector.Connector, path string) (int64, error)
	GetFileTree(ctx context.Context, conn connector.Connector, path string, maxDepth int) ([]string, error)
	CompareFiles(ctx context.Context, conn connector.Connector, file1, file2 string) (bool, error)

	// File Permissions
	GetFileOwner(ctx context.Context, conn connector.Connector, path string) (string, string, error)
	SetFileOwner(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error
	GetFilePermissions(ctx context.Context, conn connector.Connector, path string) (os.FileMode, error)
	SetFilePermissions(ctx context.Context, conn connector.Connector, path string, mode os.FileMode, recursive bool) error
	ChangeFileMode(ctx context.Context, conn connector.Connector, path, mode string, recursive bool) error

	// ==================== Archive Operations ====================

	// Advanced Archive Operations
	CreateTarball(ctx context.Context, conn connector.Connector, archivePath string, sources []string, compression string) error
	ExtractTarball(ctx context.Context, conn connector.Connector, archivePath, destPath string) error
	CreateZipArchive(ctx context.Context, conn connector.Connector, archivePath string, sources []string) error
	ExtractZipArchive(ctx context.Context, conn connector.Connector, archivePath, destPath string) error
	ListArchiveContents(ctx context.Context, conn connector.Connector, archivePath string) ([]string, error)

	// ==================== Encryption Operations ====================

	// File Encryption
	EncryptFile(ctx context.Context, conn connector.Connector, sourcePath, destPath string, password string) error
	DecryptFile(ctx context.Context, conn connector.Connector, sourcePath, destPath string, password string) error
	GenerateEncryptionKey(ctx context.Context, conn connector.Connector) ([]byte, error)

	// Data Encryption
	EncryptData(ctx context.Context, conn connector.Connector, data []byte, key []byte) ([]byte, error)
	DecryptData(ctx context.Context, conn connector.Connector, encryptedData []byte, key []byte) ([]byte, error)
	HashData(ctx context.Context, conn connector.Connector, data []byte, algorithm string) (string, error)

	// ==================== Template Operations ====================

	// Template Rendering
	RenderTemplate(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error
	RenderToString(ctx context.Context, tmpl *template.Template, data interface{}) (string, error)
	RenderWithFunctions(ctx context.Context, conn connector.Connector, templateContent string, data interface{}, destPath, permissions string, sudo bool, funcMap map[string]interface{}) error

	// ==================== SSH Operations ====================

	// SSH Key Management
	GenerateSSHKey(ctx context.Context, conn connector.Connector, keyType string, bits int) ([]byte, []byte, error)
	DeploySSHKey(ctx context.Context, conn connector.Connector, username, pubKey string) error
	GetSSHKnownHosts(ctx context.Context, conn connector.Connector) ([]string, error)
	AddSSHKnownHost(ctx context.Context, conn connector.Connector, host, key string) error

	// ==================== Container Runtime ====================

	// Containerd Operations
	ReloadContainerd(ctx context.Context, conn connector.Connector) error
	GetContainerdConfig(ctx context.Context, conn connector.Connector) (string, error)
	SetContainerdConfig(ctx context.Context, conn connector.Connector, config string) error
	ctrImages(ctx context.Context, conn connector.Connector, namespace string) ([]string, error)
	ctrPullImage(ctx context.Context, conn connector.Connector, namespace, image string) error

	// CRI-O Operations
	ReloadCRIO(ctx context.Context, conn connector.Connector) error
	GetCRIOConfig(ctx context.Context, conn connector.Connector) (string, error)
	SetCRIOConfig(ctx context.Context, conn connector.Connector, config string) error

	// ==================== Storage Operations ====================

	// LVM Operations
	CreateLV(ctx context.Context, conn connector.Connector, vgName, lvName string, size uint64) error
	RemoveLV(ctx context.Context, conn connector.Connector, vgName, lvName string) error
	ExtendLV(ctx context.Context, conn connector.Connector, vgName, lvName string, size uint64) error
	ReduceLV(ctx context.Context, conn connector.Connector, vgName, lvName string, size uint64) error
	CreateVG(ctx context.Context, conn connector.Connector, vgName string, devices []string) error
	RemoveVG(ctx context.Context, conn connector.Connector, vgName string) error
	ExtendVG(ctx context.Context, conn connector.Connector, vgName string, devices []string) error
	GetVGInfo(ctx context.Context, conn connector.Connector, vgName string) (map[string]string, error)

	// Mount Operations
	GetMounts(ctx context.Context, conn connector.Connector) ([]MountInfo, error)
	Remount(ctx context.Context, conn connector.Connector, mountPoint string) error
	BindMount(ctx context.Context, conn connector.Connector, source, target string, readOnly bool) error
	UnmountAll(ctx context.Context, conn connector.Connector, target string) error

	// ==================== Utility Operations ====================

	// Random Operations
	GenerateRandomString(ctx context.Context, conn connector.Connector, length int) (string, error)
	GenerateUUID(ctx context.Context, conn connector.Connector) (string, error)

	// Hash Operations
	CalculateFileHash(ctx context.Context, conn connector.Connector, filePath, algorithm string) (string, error)
	CalculateDirectoryHash(ctx context.Context, conn connector.Connector, dirPath, algorithm string) (string, error)

	// Compression
	CompressData(ctx context.Context, conn connector.Connector, data []byte, algorithm string) ([]byte, error)
	DecompressData(ctx context.Context, conn connector.Connector, data []byte, algorithm string) ([]byte, error)

	// JSON/YAML Operations
	ParseJSON(ctx context.Context, conn connector.Connector, content string) (map[string]interface{}, error)
	ToJSON(ctx context.Context, conn connector.Connector, data interface{}) (string, error)
	ParseYAML(ctx context.Context, conn connector.Connector, content string) (map[string]interface{}, error)
	ToYAML(ctx context.Context, conn connector.Connector, data interface{}) (string, error)
}

// ==================== Additional Types ====================

type ProcessInfo struct {
	PID     int
	PPID    int
	User    string
	CPU     float64
	Memory  float64
	Command string
	Args    string
	Start   string
	TTY     string
	Status  string
	Time    string
}

type SystemResources struct {
	CPU       float64
	Memory    float64
	Disk      float64
	NetworkRX uint64
	NetworkTX uint64
}

type ProcessResources struct {
	PID        int
	CPUPercent float64
	MemoryRSS  uint64
	MemoryVMS  uint64
	ReadBytes  uint64
	WriteBytes uint64
	OpenFiles  int
	OpenPorts  []int
}

type IOStats struct {
	ReadOps    uint64
	WriteOps   uint64
	ReadBytes  uint64
	WriteBytes uint64
	ReadTime   uint64
	WriteTime  uint64
}

type NetworkStats struct {
	RXBytes uint64
	TXBytes uint64
	RXPkts  uint64
	TXPkts  uint64
	RXErrs  uint64
	TXErrs  uint64
	RXDrops uint64
	TXDrops uint64
}

type MountInfo struct {
	Device     string
	MountPoint string
	FSType     string
	Options    string
	Dump       int
	Pass       int
}
