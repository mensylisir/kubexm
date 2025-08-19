package runner

import (
	"context"
	"k8s.io/apimachinery/pkg/api/resource"
	"os"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

type DiskInfo struct {
	Name       string
	Size       resource.Quantity
	Type       string
	MountPoint string
	Partitions []PartitionInfo
}

type PartitionInfo struct {
	Name       string
	Size       resource.Quantity
	MountPoint string
}

type Facts struct {
	OS               *connector.OS
	Hostname         string
	Kernel           string
	TotalMemory      resource.Quantity
	TotalCPU         resource.Quantity
	Disks            []DiskInfo
	TotalDisk        resource.Quantity
	IPv4Default      string
	IPv6Default      string
	PackageManager   *PackageInfo
	InitSystem       *ServiceInfo
	DefaultInterface string
}

type CPUInfo struct {
	ModelName      string `json:"modelName"`
	Architecture   string `json:"architecture"`
	Sockets        int    `json:"sockets"`
	CoresPerSocket int    `json:"coresPerSocket"`
	ThreadsPerCore int    `json:"threadsPerCore"`
	LogicalCount   int    `json:"logicalCount"`
}

type MemoryInfo struct {
	Total     resource.Quantity `json:"total"`
	SwapTotal resource.Quantity `json:"swapTotal"`
	SwapFree  resource.Quantity `json:"swapFree"`
}

type NetworkInterface struct {
	Name       string   `json:"name"`
	MACAddress string   `json:"macAddress"`
	IPv4       []string `json:"ipv4"`
	IPv6       []string `json:"ipv6"`
}

type SecurityProfile struct {
	AppArmorEnabled bool   `json:"appArmorEnabled"`
	SELinuxStatus   string `json:"seLinuxStatus"`
}

type HostFacts struct {
	OS                *connector.OS
	Hostname          string
	Kernel            string
	CPU               *CPUInfo
	TotalMemory       resource.Quantity
	Memory            *MemoryInfo
	Disks             []DiskInfo
	TotalDisk         resource.Quantity
	IPv4Default       string
	IPv6Default       string
	NetworkInterfaces []NetworkInterface
	PackageManager    *PackageInfo
	InitSystem        *ServiceInfo
	Security          *SecurityProfile
	SwapOn            bool
	KernelModules     map[string]bool
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

type Runner interface {
	GatherFacts(ctx context.Context, conn connector.Connector) (*Facts, error)
	GatherHostFacts(ctx context.Context, conn connector.Connector) (*HostFacts, error)
	DetermineSudo(ctx context.Context, conn connector.Connector, path string) (bool, error)
	Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error)
	OriginRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, string, error)
	MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string
	Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error)
	VerifyChecksum(ctx context.Context, conn connector.Connector, filePath, expectedChecksum, checksumType string, sudo bool) error
	RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) (stdout, stderr []byte, err error)
	RunInBackground(ctx context.Context, conn connector.Connector, cmd string, sudo bool) error
	RunRetry(ctx context.Context, conn connector.Connector, cmd string, sudo bool, retries int, delay time.Duration) (string, error)
	Download(ctx context.Context, conn connector.Connector, facts *Facts, url, destPath string, sudo bool) error
	Upload(ctx context.Context, conn connector.Connector, srcPath string, destPath string, sudo bool) error
	Fetch(ctx context.Context, conn connector.Connector, remotePath string, localPath string, sudo bool) error
	Extract(ctx context.Context, conn connector.Connector, facts *Facts, archivePath, destDir string, sudo bool, preserveOriginalArchive bool) error
	DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *Facts, url, destDir string, sudo bool) error
	Compress(ctx context.Context, conn connector.Connector, facts *Facts, archivePath string, sources []string, sudo bool) error
	ListArchiveContents(ctx context.Context, conn connector.Connector, facts *Facts, archivePath string, sudo bool) ([]string, error)
	Exists(ctx context.Context, conn connector.Connector, path string) (bool, error)
	ExistsWithOptions(ctx context.Context, conn connector.Connector, path string, opts *connector.StatOptions) (bool, error)
	IsDir(ctx context.Context, conn connector.Connector, path string) (bool, error)
	IsDirWithOptions(ctx context.Context, conn connector.Connector, path string, opts *connector.StatOptions) (bool, error)
	ReadFile(ctx context.Context, conn connector.Connector, path string) ([]byte, error)
	ReadFileWithOptions(ctx context.Context, conn connector.Connector, path string, opts *connector.FileTransferOptions) ([]byte, error)
	Move(ctx context.Context, conn connector.Connector, src, dest string, sudo bool) error
	CopyFile(ctx context.Context, conn connector.Connector, src, dest string, recursive bool, sudo bool) error
	Stat(ctx context.Context, conn connector.Connector, path string) (os.FileInfo, error)
	WriteFile(ctx context.Context, conn connector.Connector, content []byte, destPath, permissions string, sudo bool) error
	Mkdirp(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error
	Remove(ctx context.Context, conn connector.Connector, path string, sudo bool, recursive bool) error
	Chmod(ctx context.Context, conn connector.Connector, path, permissions string, sudo bool) error
	Chown(ctx context.Context, conn connector.Connector, path, owner, group string, recursive bool) error
	GetSHA256(ctx context.Context, conn connector.Connector, path string) (string, error)
	LookPath(ctx context.Context, conn connector.Connector, file string) (string, error)
	LookPathWithOptions(ctx context.Context, conn connector.Connector, file string, opts *connector.LookPathOptions) (string, error)
	IsPortOpen(ctx context.Context, conn connector.Connector, facts *Facts, port int) (bool, error)
	WaitForPort(ctx context.Context, conn connector.Connector, facts *Facts, port int, timeout time.Duration) error
	SetHostname(ctx context.Context, conn connector.Connector, facts *Facts, hostname string) error
	AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error
	EnsureHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error
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
	IsServiceEnabled(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) (bool, error)
	DaemonReload(ctx context.Context, conn connector.Connector, facts *Facts) error
	Render(ctx context.Context, conn connector.Connector, tmpl *template.Template, data interface{}, destPath, permissions string, sudo bool) error
	UserExists(ctx context.Context, conn connector.Connector, username string) (bool, error)
	GroupExists(ctx context.Context, conn connector.Connector, groupname string) (bool, error)
	AddUser(ctx context.Context, conn connector.Connector, username, group, shell string, homeDir string, createHome bool, systemUser bool) error
	AddGroup(ctx context.Context, conn connector.Connector, groupname string, systemGroup bool) error

	LoadModule(ctx context.Context, conn connector.Connector, facts *Facts, moduleName string, params ...string) error
	IsModuleLoaded(ctx context.Context, conn connector.Connector, moduleName string) (bool, error)
	ConfigureModuleOnBoot(ctx context.Context, conn connector.Connector, facts *Facts, moduleName string, params ...string) error
	SetSysctl(ctx context.Context, conn connector.Connector, key, value string, persistent bool) error
	SetTimezone(ctx context.Context, conn connector.Connector, facts *Facts, timezone string) error
	DisableSwap(ctx context.Context, conn connector.Connector, facts *Facts) error
	IsSwapEnabled(ctx context.Context, conn connector.Connector) (bool, error)
	EnsureMount(ctx context.Context, conn connector.Connector, device, mountPoint, fsType string, options []string, persistent bool) error
	Unmount(ctx context.Context, conn connector.Connector, mountPoint string, force bool, sudo bool) error
	IsMounted(ctx context.Context, conn connector.Connector, path string) (bool, error)
	MakeFilesystem(ctx context.Context, conn connector.Connector, device, fsType string, force bool) error
	CreateSymlink(ctx context.Context, conn connector.Connector, target, linkPath string, sudo bool) error
	GetDiskUsage(ctx context.Context, conn connector.Connector, path string) (total uint64, free uint64, used uint64, err error)
	TouchFile(ctx context.Context, conn connector.Connector, path string, sudo bool) error
	DisableFirewall(ctx context.Context, conn connector.Connector, facts *Facts) error
	GetInterfaceAddresses(ctx context.Context, conn connector.Connector, interfaceName string) (map[string][]string, error)
	ModifyUser(ctx context.Context, conn connector.Connector, username string, modifications UserModifications) error
	ConfigureSudoer(ctx context.Context, conn connector.Connector, sudoerName, content string) error
	SetUserPassword(ctx context.Context, conn connector.Connector, username, hashedPassword string) error
	GetUserInfo(ctx context.Context, conn connector.Connector, username string) (*UserInfo, error)
	DeployAndEnableService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName, configContent, configPath, permissions string, templateData interface{}) error
	Reboot(ctx context.Context, conn connector.Connector, timeout time.Duration) error
	RenderToString(ctx context.Context, tmpl *template.Template, data interface{}) (string, error)
	CreateVMTemplate(ctx context.Context, conn connector.Connector, name string, osVariant string, memoryMB uint, vcpus uint, diskPath string, diskSizeGB uint, network string, graphicsType string, cloudInitISOPath string) error
	StoragePoolExists(ctx context.Context, conn connector.Connector, poolName string) (bool, error)
	DeleteStoragePool(ctx context.Context, conn connector.Connector, poolName string) error
	VolumeExists(ctx context.Context, conn connector.Connector, poolName string, volName string) (bool, error)
	CloneVolume(ctx context.Context, conn connector.Connector, poolName string, origVolName string, newVolName string, newSizeGB uint, format string) error
	ResizeVolume(ctx context.Context, conn connector.Connector, poolName string, volName string, newSizeGB uint) error
	DeleteVolume(ctx context.Context, conn connector.Connector, poolName string, volName string) error
	CreateVolume(ctx context.Context, conn connector.Connector, poolName string, volName string, sizeGB uint, format string, backingVolName string, backingVolFormat string) error
	CreateCloudInitISO(ctx context.Context, conn connector.Connector, vmName string, isoDestPath string, userData string, metaData string, networkConfig string) error
	CreateVM(ctx context.Context, conn connector.Connector, facts *Facts, vmName string, memoryMB uint, vcpus uint, osVariant string, diskPaths []string, networkInterfaces []VMNetworkInterface, graphicsType string, cloudInitISOPath string, bootOrder []string, extraArgs []string) error
	VMExists(ctx context.Context, conn connector.Connector, vmName string) (bool, error)
	StartVM(ctx context.Context, conn connector.Connector, vmName string) error
	ShutdownVM(ctx context.Context, conn connector.Connector, vmName string, force bool, timeout time.Duration) error
	DestroyVM(ctx context.Context, conn connector.Connector, vmName string) error
	UndefineVM(ctx context.Context, conn connector.Connector, vmName string, deleteSnapshots bool, deleteStorage bool, storagePools []string) error
	GetVMState(ctx context.Context, conn connector.Connector, vmName string) (string, error)
	ListVMs(ctx context.Context, conn connector.Connector, all bool) ([]VMInfo, error)
	ImportVMTemplate(ctx context.Context, conn connector.Connector, name string, filePath string) error
	RefreshStoragePool(ctx context.Context, conn connector.Connector, poolName string) error
	CreateStoragePool(ctx context.Context, conn connector.Connector, name string, poolType string, targetPath string) error
	AttachDisk(ctx context.Context, conn connector.Connector, vmName string, diskPath string, targetDevice string, diskType string, driverType string) error
	DetachDisk(ctx context.Context, conn connector.Connector, vmName string, targetDeviceOrPath string) error
	SetVMMemory(ctx context.Context, conn connector.Connector, vmName string, memoryMB uint, current bool) error
	SetVMCPUs(ctx context.Context, conn connector.Connector, vmName string, vcpus uint, current bool) error
	AttachNetInterface(ctx context.Context, conn connector.Connector, vmName string, iface VMNetworkInterface, persistent bool) error
	DetachNetInterface(ctx context.Context, conn connector.Connector, vmName string, macAddress string, persistent bool) error
	ListNetInterfaces(ctx context.Context, conn connector.Connector, vmName string) ([]VMNetworkInterfaceDetail, error)
	CreateSnapshot(ctx context.Context, conn connector.Connector, vmName, snapshotName, description string, diskSpecs []VMSnapshotDiskSpec, noMetadata, halt, diskOnly, reuseExisting, quiesce, atomic bool) error
	DeleteSnapshot(ctx context.Context, conn connector.Connector, vmName, snapshotName string, children, metadata bool) error
	ListSnapshots(ctx context.Context, conn connector.Connector, vmName string) ([]VMSnapshotInfo, error)
	RevertToSnapshot(ctx context.Context, conn connector.Connector, vmName, snapshotName string, force, running bool) error
	GetVMInfo(ctx context.Context, conn connector.Connector, vmName string) (*VMDetails, error)
	GetVNCPort(ctx context.Context, conn connector.Connector, vmName string) (string, error)
	EnsureLibvirtDaemonRunning(ctx context.Context, conn connector.Connector, facts *Facts) error
	PullImage(ctx context.Context, conn connector.Connector, imageName string) error
	ImageExists(ctx context.Context, conn connector.Connector, imageName string) (bool, error)
	ListImages(ctx context.Context, conn connector.Connector, all bool) ([]ImageInfo, error)
	RemoveImage(ctx context.Context, conn connector.Connector, imageName string, force bool) error
	BuildImage(ctx context.Context, conn connector.Connector, dockerfilePath string, imageNameAndTag string, contextPath string, buildArgs map[string]string) error
	CreateContainer(ctx context.Context, conn connector.Connector, options ContainerCreateOptions) (string, error)
	ContainerExists(ctx context.Context, conn connector.Connector, containerNameOrID string) (bool, error)
	StartContainer(ctx context.Context, conn connector.Connector, containerNameOrID string) error
	StopContainer(ctx context.Context, conn connector.Connector, containerNameOrID string, timeout *time.Duration) error
	RestartContainer(ctx context.Context, conn connector.Connector, containerNameOrID string, timeout *time.Duration) error
	RemoveContainer(ctx context.Context, conn connector.Connector, containerNameOrID string, force bool, removeVolumes bool) error
	ListContainers(ctx context.Context, conn connector.Connector, all bool, filters map[string]string) ([]ContainerInfo, error)
	GetContainerLogs(ctx context.Context, conn connector.Connector, containerNameOrID string, options ContainerLogOptions) (string, error)
	GetContainerStats(ctx context.Context, conn connector.Connector, containerNameOrID string, stream bool) (<-chan ContainerStats, error)
	InspectContainer(ctx context.Context, conn connector.Connector, containerNameOrID string) (*ContainerDetails, error)
	PauseContainer(ctx context.Context, conn connector.Connector, containerNameOrID string) error
	UnpauseContainer(ctx context.Context, conn connector.Connector, containerNameOrID string) error
	ExecInContainer(ctx context.Context, conn connector.Connector, containerNameOrID string, cmd []string, user string, workDir string, tty bool) (string, error)
	CreateDockerNetwork(ctx context.Context, conn connector.Connector, name string, driver string, subnet string, gateway string, options map[string]string) error
	RemoveDockerNetwork(ctx context.Context, conn connector.Connector, networkNameOrID string) error
	ListDockerNetworks(ctx context.Context, conn connector.Connector, filters map[string]string) ([]DockerNetworkInfo, error)
	ConnectContainerToNetwork(ctx context.Context, conn connector.Connector, containerNameOrID string, networkNameOrID string, ipAddress string) error
	DisconnectContainerFromNetwork(ctx context.Context, conn connector.Connector, containerNameOrID string, networkNameOrID string, force bool) error
	CreateDockerVolume(ctx context.Context, conn connector.Connector, name string, driver string, driverOpts map[string]string, labels map[string]string) error
	RemoveDockerVolume(ctx context.Context, conn connector.Connector, volumeName string, force bool) error
	ListDockerVolumes(ctx context.Context, conn connector.Connector, filters map[string]string) ([]DockerVolumeInfo, error)
	InspectDockerVolume(ctx context.Context, conn connector.Connector, volumeName string) (*DockerVolumeDetails, error)
	DockerInfo(ctx context.Context, conn connector.Connector) (*DockerSystemInfo, error)
	DockerPrune(ctx context.Context, conn connector.Connector, pruneType string, filters map[string]string, all bool) (string, error)
	GetDockerDaemonConfig(ctx context.Context, conn connector.Connector) (*DockerDaemonOptions, error)
	ConfigureDockerDaemon(ctx context.Context, conn connector.Connector, opts DockerDaemonOptions, restartService bool) error
	EnsureDefaultDockerConfig(ctx context.Context, conn connector.Connector, facts *Facts, restartService bool) error
	EnsureDockerServiceFiles(ctx context.Context, conn connector.Connector, facts *Facts) error
	ConfigureDockerDropIn(ctx context.Context, conn connector.Connector, facts *Facts, content string) error
	EnsureCriDockerdService(ctx context.Context, conn connector.Connector, facts *Facts) error
	EnsureDockerService(ctx context.Context, c connector.Connector) error
	CtrListNamespaces(ctx context.Context, conn connector.Connector) ([]string, error)
	CtrListImages(ctx context.Context, conn connector.Connector, namespace string) ([]CtrImageInfo, error)
	CtrPullImage(ctx context.Context, conn connector.Connector, namespace, imageName string, allPlatforms bool, user string) error
	CtrRemoveImage(ctx context.Context, conn connector.Connector, namespace, imageName string) error
	CtrTagImage(ctx context.Context, conn connector.Connector, namespace, sourceImage, targetImage string) error
	CtrListContainers(ctx context.Context, conn connector.Connector, namespace string) ([]CtrContainerInfo, error)
	CtrRunContainer(ctx context.Context, conn connector.Connector, namespace string, opts ContainerdContainerCreateOptions) (string, error)
	CtrStopContainer(ctx context.Context, conn connector.Connector, namespace, containerID string, timeout time.Duration) error
	CtrRemoveContainer(ctx context.Context, conn connector.Connector, namespace, containerID string) error
	CtrExecInContainer(ctx context.Context, conn connector.Connector, namespace, containerID string, opts CtrExecOptions, cmd []string) (string, error)
	CtrImportImage(ctx context.Context, conn connector.Connector, namespace, filePath string, allPlatforms bool) error
	CtrExportImage(ctx context.Context, conn connector.Connector, namespace, imageName, outputFilePath string, allPlatforms bool) error
	CtrContainerInfo(ctx context.Context, conn connector.Connector, namespace, containerID string) (*CtrContainerInfo, error)
	CrictlListImages(ctx context.Context, conn connector.Connector, filters map[string]string) ([]CrictlImageInfo, error)
	CrictlPullImage(ctx context.Context, conn connector.Connector, imageName string, authCreds string, sandboxConfigPath string) error
	CrictlRemoveImage(ctx context.Context, conn connector.Connector, imageName string) error
	CrictlInspectImage(ctx context.Context, conn connector.Connector, imageName string) (*CrictlImageDetails, error)
	CrictlImageFSInfo(ctx context.Context, conn connector.Connector) ([]CrictlFSInfo, error)
	CrictlListPods(ctx context.Context, conn connector.Connector, filters map[string]string) ([]CrictlPodInfo, error)
	CrictlRunPodSandbox(ctx context.Context, conn connector.Connector, podSandboxConfigFile string, runtimeHandler string) (string, error)
	CrictlStopPodSandbox(ctx context.Context, conn connector.Connector, podID string) error
	CrictlRemovePodSandbox(ctx context.Context, conn connector.Connector, podID string) error
	CrictlInspectPod(ctx context.Context, conn connector.Connector, podID string) (*CrictlPodDetails, error)
	CrictlPodSandboxStatus(ctx context.Context, conn connector.Connector, podID string, verbose bool) (*CrictlPodDetails, error)
	CrictlCreateContainerInPod(ctx context.Context, conn connector.Connector, podID string, containerConfigFile string, podSandboxConfigFile string) (string, error)
	CrictlStartContainerInPod(ctx context.Context, conn connector.Connector, containerID string) error
	CrictlStopContainerInPod(ctx context.Context, conn connector.Connector, containerID string, timeout int64) error
	CrictlRemoveContainerInPod(ctx context.Context, conn connector.Connector, containerID string, force bool) error
	CrictlInspectContainerInPod(ctx context.Context, conn connector.Connector, containerID string) (*CrictlContainerDetails, error)
	CrictlContainerStatus(ctx context.Context, conn connector.Connector, containerID string, verbose bool) (*CrictlContainerDetails, error)
	CrictlLogsForContainer(ctx context.Context, conn connector.Connector, containerID string, opts CrictlLogOptions) (string, error)
	CrictlExecInContainerSync(ctx context.Context, conn connector.Connector, containerID string, timeout time.Duration, cmd []string) (stdout, stderr string, err error)
	CrictlExecInContainerAsync(ctx context.Context, conn connector.Connector, containerID string, cmd []string) (string, error)
	CrictlPortForward(ctx context.Context, conn connector.Connector, podID string, ports []string) (string, error)
	CrictlVersion(ctx context.Context, conn connector.Connector) (*CrictlVersionInfo, error)
	CrictlInfo(ctx context.Context, conn connector.Connector) (*CrictlRuntimeInfo, error)
	CrictlRuntimeConfig(ctx context.Context, conn connector.Connector) (string, error)
	CrictlStats(ctx context.Context, conn connector.Connector, resourceID string, outputFormat string) (string, error)
	CrictlPodStats(ctx context.Context, conn connector.Connector, outputFormat string, podID string) (string, error)
	ConfigureCrictl(ctx context.Context, conn connector.Connector, opts CrictlConfigOptions, configFilePath string) error
	EnsureDefaultContainerdConfig(ctx context.Context, conn connector.Connector, facts *Facts) error
	GetContainerdConfig(ctx context.Context, conn connector.Connector) (*ContainerdConfigOptions, error)
	ConfigureContainerd(ctx context.Context, conn connector.Connector, facts *Facts, opts ContainerdConfigOptions, restartService bool) error
	EnsureContainerdService(ctx context.Context, conn connector.Connector, facts *Facts) error
	ConfigureContainerdDropIn(ctx context.Context, conn connector.Connector, facts *Facts, content string) error
	HelmInstall(ctx context.Context, conn connector.Connector, releaseName, chartPath string, opts HelmInstallOptions) error
	HelmUninstall(ctx context.Context, conn connector.Connector, releaseName string, opts HelmUninstallOptions) error
	HelmList(ctx context.Context, conn connector.Connector, opts HelmListOptions) ([]HelmReleaseInfo, error)
	HelmStatus(ctx context.Context, conn connector.Connector, releaseName string, opts HelmStatusOptions) (*HelmReleaseInfo, error)
	HelmRepoAdd(ctx context.Context, conn connector.Connector, name, url string, opts HelmRepoAddOptions) error
	HelmRepoUpdate(ctx context.Context, conn connector.Connector, repoNames []string) error
	HelmSearchRepo(ctx context.Context, conn connector.Connector, keyword string, opts HelmSearchOptions) ([]HelmChartInfo, error)
	HelmPull(ctx context.Context, conn connector.Connector, chartPath string, opts HelmPullOptions) (string, error)
	HelmPackage(ctx context.Context, conn connector.Connector, chartPath string, opts HelmPackageOptions) (string, error)
	HelmVersion(ctx context.Context, conn connector.Connector) (*HelmVersionInfo, error)
	HelmUpgrade(ctx context.Context, conn connector.Connector, releaseName, chartPath string, opts HelmUpgradeOptions) error
	HelmRollback(ctx context.Context, conn connector.Connector, releaseName string, revision int, opts HelmRollbackOptions) error
	HelmHistory(ctx context.Context, conn connector.Connector, releaseName string, opts HelmHistoryOptions) ([]HelmReleaseRevisionInfo, error)
	HelmGetValues(ctx context.Context, conn connector.Connector, releaseName string, opts HelmGetOptions) (string, error)
	HelmGetManifest(ctx context.Context, conn connector.Connector, releaseName string, opts HelmGetOptions) (string, error)
	HelmGetHooks(ctx context.Context, conn connector.Connector, releaseName string, opts HelmGetOptions) (string, error)
	HelmTemplate(ctx context.Context, conn connector.Connector, releaseName, chartPath string, opts HelmTemplateOptions) (string, error)
	HelmDependencyUpdate(ctx context.Context, conn connector.Connector, chartPath string, opts HelmDependencyOptions) error
	HelmLint(ctx context.Context, conn connector.Connector, chartPath string, opts HelmLintOptions) (string, error)
	KubectlApply(ctx context.Context, conn connector.Connector, opts KubectlApplyOptions) (string, error)
	KubectlGet(ctx context.Context, conn connector.Connector, resourceType string, resourceName string, opts KubectlGetOptions) (string, error)
	KubectlDescribe(ctx context.Context, conn connector.Connector, resourceType string, resourceName string, opts KubectlDescribeOptions) (string, error)
	KubectlDelete(ctx context.Context, conn connector.Connector, resourceType string, resourceName string, opts KubectlDeleteOptions) error
	KubectlLogs(ctx context.Context, conn connector.Connector, podName string, opts KubectlLogOptions) (string, error)
	KubectlExec(ctx context.Context, conn connector.Connector, podName string, opts KubectlExecOptions, command ...string) (string, error)
	KubectlVersion(ctx context.Context, conn connector.Connector) (*KubectlVersionInfo, error)
	KubectlClusterInfo(ctx context.Context, conn connector.Connector, kubeconfigPath string) (string, error)
	KubectlGetNodes(ctx context.Context, conn connector.Connector, opts KubectlGetOptions) ([]KubectlNodeInfo, error)
	KubectlGetPods(ctx context.Context, conn connector.Connector, opts KubectlGetOptions) ([]KubectlPodInfo, error)
	KubectlGetServices(ctx context.Context, conn connector.Connector, opts KubectlGetOptions) ([]KubectlServiceInfo, error)
	KubectlGetDeployments(ctx context.Context, conn connector.Connector, opts KubectlGetOptions) ([]KubectlDeploymentInfo, error)
	KubectlGetResourceList(ctx context.Context, conn connector.Connector, resourceType string, opts KubectlGetOptions) ([]map[string]interface{}, error)
	KubectlRolloutStatus(ctx context.Context, conn connector.Connector, resourceType, resourceName string, opts KubectlRolloutOptions) (string, error)
	KubectlRolloutHistory(ctx context.Context, conn connector.Connector, resourceType, resourceName string, opts KubectlRolloutOptions) (string, error)
	KubectlRolloutUndo(ctx context.Context, conn connector.Connector, resourceType, resourceName string, toRevision int, opts KubectlRolloutOptions) error
	KubectlScale(ctx context.Context, conn connector.Connector, resourceType, resourceName string, replicas int32, opts KubectlScaleOptions) error
	KubectlConfigView(ctx context.Context, conn connector.Connector, opts KubectlConfigViewOptions) (*KubectlConfigInfo, error)
	KubectlConfigGetContexts(ctx context.Context, conn connector.Connector, kubeconfigPath string) ([]KubectlContextInfo, error)
	KubectlConfigUseContext(ctx context.Context, conn connector.Connector, contextName string, kubeconfigPath string) error
	KubectlConfigCurrentContext(ctx context.Context, conn connector.Connector, kubeconfigPath string) (string, error)
	KubectlTopNodes(ctx context.Context, conn connector.Connector, opts KubectlTopOptions) ([]KubectlMetricsInfo, error)
	KubectlTopPods(ctx context.Context, conn connector.Connector, opts KubectlTopOptions) ([]KubectlMetricsInfo, error)
	KubectlPortForward(ctx context.Context, conn connector.Connector, resourceType, resourceName string, ports []string, opts KubectlPortForwardOptions) error // Placeholder, as true port-forwarding is complex
	KubectlExplain(ctx context.Context, conn connector.Connector, resourceType string, opts KubectlExplainOptions) (string, error)
	KubectlDrainNode(ctx context.Context, conn connector.Connector, nodeName string, opts KubectlDrainOptions) error
	KubectlCordonNode(ctx context.Context, conn connector.Connector, nodeName string, opts KubectlCordonUncordonOptions) error
	KubectlUncordonNode(ctx context.Context, conn connector.Connector, nodeName string, opts KubectlCordonUncordonOptions) error
	KubectlTaintNode(ctx context.Context, conn connector.Connector, nodeName string, taints []string, opts KubectlTaintOptions) error
	KubectlCreateSecretGeneric(ctx context.Context, conn connector.Connector, namespace, name string, fromLiterals map[string]string, fromFiles map[string]string, opts KubectlCreateOptions) error
	KubectlCreateSecretDockerRegistry(ctx context.Context, conn connector.Connector, namespace, name, dockerServer, dockerUsername, dockerPassword, dockerEmail string, opts KubectlCreateOptions) error
	KubectlCreateSecretTLS(ctx context.Context, conn connector.Connector, namespace, name, certPath, keyPath string, opts KubectlCreateOptions) error
	KubectlCreateConfigMap(ctx context.Context, conn connector.Connector, namespace, name string, fromLiterals map[string]string, fromFiles map[string]string, fromEnvFile string, opts KubectlCreateOptions) error
	KubectlCreateServiceAccount(ctx context.Context, conn connector.Connector, namespace, name string, opts KubectlCreateOptions) error
	KubectlCreateRole(ctx context.Context, conn connector.Connector, namespace, name string, verbs, resources, resourceNames []string, opts KubectlCreateOptions) error
	KubectlCreateClusterRole(ctx context.Context, conn connector.Connector, name string, verbs, resources, resourceNames []string, aggregationRule string, opts KubectlCreateOptions) error
	KubectlCreateRoleBinding(ctx context.Context, conn connector.Connector, namespace, name, role, serviceAccount string, users, groups []string, opts KubectlCreateOptions) error
	KubectlCreateClusterRoleBinding(ctx context.Context, conn connector.Connector, name, clusterRole, serviceAccount string, users, groups []string, opts KubectlCreateOptions) error
	KubectlSetImage(ctx context.Context, conn connector.Connector, resourceType, resourceName, containerName, newImage string, opts KubectlSetOptions) error
	KubectlSetEnv(ctx context.Context, conn connector.Connector, resourceType, resourceName, containerName string, envVars map[string]string, removeEnvVars []string, fromSecret, fromConfigMap string, opts KubectlSetOptions) error
	KubectlSetResources(ctx context.Context, conn connector.Connector, resourceType, resourceName, containerName string, limits, requests map[string]string, opts KubectlSetOptions) error
	KubectlAutoscale(ctx context.Context, conn connector.Connector, resourceType, resourceName string, minReplicas, maxReplicas int32, cpuPercent int32, opts KubectlAutoscaleOptions) error
	KubectlCompletion(ctx context.Context, conn connector.Connector, shell string) (string, error)
	KubectlWait(ctx context.Context, conn connector.Connector, resourceType, resourceName string, condition string, opts KubectlWaitOptions) error
	KubectlLabel(ctx context.Context, conn connector.Connector, resourceType, resourceName string, labels map[string]string, overwrite bool, opts KubectlLabelOptions) error
	KubectlAnnotate(ctx context.Context, conn connector.Connector, resourceType, resourceName string, annotations map[string]string, overwrite bool, opts KubectlAnnotateOptions) error
	KubectlPatch(ctx context.Context, conn connector.Connector, resourceType, resourceName string, patchType, patchContent string, opts KubectlPatchOptions) error
}

type HelmInstallOptions struct {
	Namespace       string
	KubeconfigPath  string
	ValuesFiles     []string
	SetValues       []string
	Version         string
	CreateNamespace bool
	Wait            bool
	Timeout         time.Duration
	Atomic          bool
	DryRun          bool
	Devel           bool
	Description     string
	Sudo            bool
	Retries         int
	RetryDelay      time.Duration
}

type HelmUninstallOptions struct {
	Namespace      string
	KubeconfigPath string
	KeepHistory    bool
	Timeout        time.Duration
	DryRun         bool
	Sudo           bool
}

type HelmListOptions struct {
	Namespace      string
	KubeconfigPath string
	AllNamespaces  bool
	Filter         string
	Selector       string
	Max            int
	Offset         int
	ByDate         bool
	SortReverse    bool
	Deployed       bool
	Failed         bool
	Pending        bool
	Uninstalled    bool
	Uninstalling   bool
	Sudo           bool
}

type HelmReleaseInfo struct {
	Name       string                 `json:"name"`
	Namespace  string                 `json:"namespace"`
	Revision   string                 `json:"revision"`
	Updated    string                 `json:"updated"`
	Status     string                 `json:"status"`
	Chart      string                 `json:"chart"`
	AppVersion string                 `json:"app_version"`
	Notes      string                 `json:"notes,omitempty"`
	Config     map[string]interface{} `json:"config,omitempty"`
	Manifest   string                 `json:"manifest,omitempty"`
	Version    int                    `json:"version"`
}

type HelmStatusOptions struct {
	Namespace      string
	KubeconfigPath string
	Revision       int
	ShowDesc       bool
	Sudo           bool
}

type HelmRepoAddOptions struct {
	Username        string
	Password        string
	CAFile          string
	CertFile        string
	KeyFile         string
	Insecure        bool
	ForceUpdate     bool
	PassCredentials bool
	Sudo            bool
}

type HelmSearchOptions struct {
	Regexp       bool
	Devel        bool
	Version      string
	Versions     bool
	OutputFormat string
	Sudo         bool
}

type HelmChartInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	AppVersion  string `json:"app_version"`
	Description string `json:"description"`
}

type HelmPullOptions struct {
	Destination     string
	Prov            bool
	Untar           bool
	UntarDir        string
	Verify          bool
	Keyring         string
	Version         string
	CAFile          string
	CertFile        string
	KeyFile         string
	Insecure        bool
	Devel           bool
	PassCredentials bool
	Username        string
	Password        string
	Sudo            bool
}

type HelmPackageOptions struct {
	Destination      string
	Sign             bool
	Key              string
	Keyring          string
	PassphraseFile   string
	Version          string
	AppVersion       string
	DependencyUpdate bool
	Sudo             bool
}

type HelmUpgradeOptions struct {
	HelmInstallOptions
	Install       bool
	Force         bool
	ResetValues   bool
	ReuseValues   bool
	CleanupOnFail bool
	MaxHistory    int
}

type HelmRollbackOptions struct {
	Namespace      string
	KubeconfigPath string
	Timeout        time.Duration
	Wait           bool
	CleanupOnFail  bool
	DryRun         bool
	Force          bool
	NoHooks        bool
	RecreatePods   bool
	Sudo           bool
}

type HelmHistoryOptions struct {
	Namespace      string
	KubeconfigPath string
	OutputFormat   string
	Max            int
	Sudo           bool
}

type HelmReleaseRevisionInfo struct {
	Revision    int    `json:"revision"`
	Updated     string `json:"updated"`
	Status      string `json:"status"`
	Chart       string `json:"chart"`
	AppVersion  string `json:"app_version"`
	Description string `json:"description"`
}

type HelmGetOptions struct {
	Namespace      string
	KubeconfigPath string
	Revision       int
	AllValues      bool
	Sudo           bool
}

type HelmTemplateOptions struct {
	Namespace       string
	KubeconfigPath  string
	ValuesFiles     []string
	SetValues       []string
	ReleaseName     string
	CreateNamespace bool
	ShowOnly        []string
	SkipCrds        bool
	Validate        bool
	IncludeCrds     bool
	IsUpgrade       bool
	Sudo            bool
}

type HelmDependencyOptions struct {
	Keyring     string
	SkipRefresh bool
	Verify      bool
	Sudo        bool
}

type HelmLintOptions struct {
	Strict        bool
	ValuesFiles   []string
	SetValues     []string
	Quiet         bool
	WithSubcharts bool
	Namespace     string
	KubeVersion   string
	Sudo          bool
}

type HelmVersionInfo struct {
	Version      string `json:"version"`
	GitCommit    string `json:"gitCommit"`
	GitTreeState string `json:"gitTreeState"`
	GoVersion    string `json:"goVersion"`
}

type KubectlApplyOptions struct {
	KubeconfigPath string
	Namespace      string
	Force          bool
	Prune          bool
	Selector       string
	DryRun         string
	Validate       bool
	Filenames      []string
	FileContent    string
	Recursive      bool
	Sudo           bool
}

type KubectlGetOptions struct {
	KubeconfigPath string
	Namespace      string
	AllNamespaces  bool
	OutputFormat   string
	Selector       string
	FieldSelector  string
	Watch          bool
	IgnoreNotFound bool
	ChunkSize      int64
	LabelColumns   []string
	ShowLabels     bool
	Sudo           bool
}

type KubectlDescribeOptions struct {
	KubeconfigPath string
	Namespace      string
	Selector       string
	ShowEvents     bool
	Sudo           bool
}

type KubectlDeleteOptions struct {
	KubeconfigPath string
	Namespace      string
	Force          bool
	GracePeriod    *int64
	Timeout        time.Duration
	Wait           bool
	Selector       string
	Filenames      []string
	FileContent    string
	Recursive      bool
	IgnoreNotFound bool
	Cascade        string
	Sudo           bool
}

type KubectlLogOptions struct {
	KubeconfigPath string
	Namespace      string
	Container      string
	Follow         bool
	Previous       bool
	SinceTime      string
	SinceSeconds   *int64
	TailLines      *int64
	LimitBytes     *int64
	Timestamps     bool
	Sudo           bool
}

type KubectlExecOptions struct {
	KubeconfigPath string
	Namespace      string
	Container      string
	Stdin          bool
	TTY            bool
	CommandTimeout time.Duration
	Sudo           bool
}

type KubectlVersionInfo struct {
	ClientVersion struct {
		Major        string `json:"major"`
		Minor        string `json:"minor"`
		GitVersion   string `json:"gitVersion"`
		GitCommit    string `json:"gitCommit"`
		GitTreeState string `json:"gitTreeState"`
		BuildDate    string `json:"buildDate"`
		GoVersion    string `json:"goVersion"`
		Compiler     string `json:"compiler"`
		Platform     string `json:"platform"`
	} `json:"clientVersion"`
	ServerVersion *struct {
		Major        string `json:"major"`
		Minor        string `json:"minor"`
		GitVersion   string `json:"gitVersion"`
		GitCommit    string `json:"gitCommit"`
		GitTreeState string `json:"gitTreeState"`
		BuildDate    string `json:"buildDate"`
		GoVersion    string `json:"goVersion"`
		Compiler     string `json:"compiler"`
		Platform     string `json:"platform"`
	} `json:"serverVersion,omitempty"`
}

type KubectlNodeInfo struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name              string            `json:"name"`
		UID               string            `json:"uid"`
		CreationTimestamp string            `json:"creationTimestamp"`
		Labels            map[string]string `json:"labels"`
		Annotations       map[string]string `json:"annotations"`
	} `json:"metadata"`
	Spec struct {
		PodCIDR       string `json:"podCIDR"`
		ProviderID    string `json:"providerID"`
		Unschedulable bool   `json:"unschedulable,omitempty"`
	} `json:"spec"`
	Status struct {
		Capacity    map[string]string `json:"capacity"`
		Allocatable map[string]string `json:"allocatable"`
		Conditions  []struct {
			Type               string `json:"type"`
			Status             string `json:"status"`
			LastHeartbeatTime  string `json:"lastHeartbeatTime"`
			LastTransitionTime string `json:"lastTransitionTime"`
			Reason             string `json:"reason"`
			Message            string `json:"message"`
		} `json:"conditions"`
		Addresses []struct {
			Type    string `json:"type"`
			Address string `json:"address"`
		} `json:"addresses"`
		NodeInfo struct {
			MachineID               string `json:"machineID"`
			SystemUUID              string `json:"systemUUID"`
			BootID                  string `json:"bootID"`
			KernelVersion           string `json:"kernelVersion"`
			OSImage                 string `json:"osImage"`
			ContainerRuntimeVersion string `json:"containerRuntimeVersion"`
			KubeletVersion          string `json:"kubeletVersion"`
			KubeProxyVersion        string `json:"kubeProxyVersion"`
		} `json:"nodeInfo"`
	} `json:"status"`
}

type KubectlPodInfo struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		UID               string            `json:"uid"`
		CreationTimestamp string            `json:"creationTimestamp"`
		Labels            map[string]string `json:"labels"`
		Annotations       map[string]string `json:"annotations"`
		OwnerReferences   []struct {
			APIVersion string `json:"apiVersion"`
			Kind       string `json:"kind"`
			Name       string `json:"name"`
			UID        string `json:"uid"`
		} `json:"ownerReferences,omitempty"`
	} `json:"metadata"`
	Spec struct {
		NodeName   string `json:"nodeName"`
		Containers []struct {
			Name  string `json:"name"`
			Image string `json:"image"`
		} `json:"containers"`
	} `json:"spec"`
	Status struct {
		Phase             string `json:"phase"`
		HostIP            string `json:"hostIP"`
		PodIP             string `json:"podIP"`
		StartTime         string `json:"startTime,omitempty"`
		ContainerStatuses []struct {
			Name         string                 `json:"name"`
			State        map[string]interface{} `json:"state"`
			LastState    map[string]interface{} `json:"lastState,omitempty"`
			Ready        bool                   `json:"ready"`
			RestartCount int32                  `json:"restartCount"`
			Image        string                 `json:"image"`
			ImageID      string                 `json:"imageID"`
			ContainerID  string                 `json:"containerID"`
		} `json:"containerStatuses,omitempty"`
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions,omitempty"`
	} `json:"status"`
}

type KubectlServiceInfo struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		UID               string            `json:"uid"`
		CreationTimestamp string            `json:"creationTimestamp"`
		Labels            map[string]string `json:"labels"`
		Annotations       map[string]string `json:"annotations"`
	} `json:"metadata"`
	Spec struct {
		Ports []struct {
			Name       string `json:"name,omitempty"`
			Protocol   string `json:"protocol"`
			Port       int32  `json:"port"`
			TargetPort any    `json:"targetPort"`
			NodePort   int32  `json:"nodePort,omitempty"`
		} `json:"ports"`
		Selector        map[string]string `json:"selector,omitempty"`
		ClusterIP       string            `json:"clusterIP"`
		ClusterIPs      []string          `json:"clusterIPs,omitempty"`
		Type            string            `json:"type"`
		SessionAffinity string            `json:"sessionAffinity"`
		ExternalIPs     []string          `json:"externalIPs,omitempty"`
		LoadBalancerIP  string            `json:"loadBalancerIP,omitempty"`
	} `json:"spec"`
	Status struct {
		LoadBalancer struct {
			Ingress []struct {
				IP       string `json:"ip,omitempty"`
				Hostname string `json:"hostname,omitempty"`
			} `json:"ingress,omitempty"`
		} `json:"loadBalancer,omitempty"`
	} `json:"status,omitempty"`
}

type KubectlDeploymentInfo struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name              string            `json:"name"`
		Namespace         string            `json:"namespace"`
		UID               string            `json:"uid"`
		CreationTimestamp string            `json:"creationTimestamp"`
		Labels            map[string]string `json:"labels"`
		Annotations       map[string]string `json:"annotations"`
		Generation        int64             `json:"generation"`
	} `json:"metadata"`
	Spec struct {
		Replicas *int32 `json:"replicas"`
		Selector struct {
			MatchLabels map[string]string `json:"matchLabels"`
		} `json:"selector"`
		Template struct {
		} `json:"template"`
		Strategy struct {
			Type          string `json:"type"`
			RollingUpdate *struct {
				MaxUnavailable any `json:"maxUnavailable,omitempty"`
				MaxSurge       any `json:"maxSurge,omitempty"`
			} `json:"rollingUpdate,omitempty"`
		} `json:"strategy"`
		MinReadySeconds         int32  `json:"minReadySeconds,omitempty"`
		RevisionHistoryLimit    *int32 `json:"revisionHistoryLimit,omitempty"`
		Paused                  bool   `json:"paused,omitempty"`
		ProgressDeadlineSeconds *int32 `json:"progressDeadlineSeconds,omitempty"`
	} `json:"spec"`
	Status struct {
		ObservedGeneration  int64 `json:"observedGeneration,omitempty"`
		Replicas            int32 `json:"replicas,omitempty"`
		UpdatedReplicas     int32 `json:"updatedReplicas,omitempty"`
		ReadyReplicas       int32 `json:"readyReplicas,omitempty"`
		AvailableReplicas   int32 `json:"availableReplicas,omitempty"`
		UnavailableReplicas int32 `json:"unavailableReplicas,omitempty"`
		Conditions          []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions,omitempty"`
	} `json:"status,omitempty"`
}

type KubectlConfigInfo struct {
	APIVersion string `json:"apiVersion"`
	Clusters   []struct {
		Name    string `json:"name"`
		Cluster struct {
			Server                   string `json:"server"`
			CertificateAuthorityData string `json:"certificate-authority-data,omitempty"`
		} `json:"cluster"`
	} `json:"clusters"`
	Contexts []struct {
		Name    string `json:"name"`
		Context struct {
			Cluster   string `json:"cluster"`
			User      string `json:"user"`
			Namespace string `json:"namespace,omitempty"`
		} `json:"context"`
	} `json:"contexts"`
	CurrentContext string                 `json:"current-context"`
	Kind           string                 `json:"kind"`
	Preferences    map[string]interface{} `json:"preferences"`
	Users          []struct {
		Name string `json:"name"`
		User struct {
			ClientCertificateData string `json:"client-certificate-data,omitempty"`
			ClientKeyData         string `json:"client-key-data,omitempty"`
			Token                 string `json:"token,omitempty"`
		} `json:"user"`
	} `json:"users"`
}

type KubectlContextInfo struct {
	Name      string `json:"name"`
	Cluster   string `json:"cluster"`
	AuthInfo  string `json:"user"`
	Namespace string `json:"namespace,omitempty"`
	Current   bool
}

type KubectlMetricsInfo struct {
	Metadata struct {
		Name              string    `json:"name"`
		CreationTimestamp time.Time `json:"timestamp"`
	} `json:"metadata"`
	Timestamp  string                        `json:"timestamp"`
	Window     string                        `json:"window"`
	Containers []KubectlContainerMetricsInfo `json:"containers,omitempty"`
	CPU        struct {
		UsageNanoCores string `json:"usageNanoCores,omitempty"`
		UsageCoreNanos *int64 `json:"-"`
	} `json:"cpu,omitempty"`
	Memory struct {
		UsageBytes       string `json:"usageBytes,omitempty"`
		UsageBytesParsed *int64 `json:"-"`
	} `json:"memory,omitempty"`
}

type KubectlContainerMetricsInfo struct {
	Name string `json:"name"`
	CPU  struct {
		UsageNanoCores string `json:"usageNanoCores,omitempty"`
		UsageCoreNanos *int64 `json:"-"`
	} `json:"cpu"`
	Memory struct {
		UsageBytes       string `json:"usageBytes,omitempty"`
		UsageBytesParsed *int64 `json:"-"`
	} `json:"memory"`
}

type ContainerdConfigOptions struct {
	Version         *int                     `toml:"version,omitempty" json:"version,omitempty"`
	Root            *string                  `toml:"root,omitempty" json:"root,omitempty"`
	State           *string                  `toml:"state,omitempty" json:"state,omitempty"`
	OOMScore        *int                     `toml:"oom_score,omitempty" json:"oom_score,omitempty"`
	GRPC            *ContainerdGRPCConfig    `toml:"grpc,omitempty" json:"grpc,omitempty"`
	Metrics         *ContainerdMetricsConfig `toml:"metrics,omitempty" json:"metrics,omitempty"`
	DisabledPlugins *[]string                `toml:"disabled_plugins,omitempty" json:"disabled_plugins,omitempty"`
	PluginConfigs   *map[string]interface{}  `toml:"plugins,omitempty" json:"plugins,omitempty"`
	RegistryMirrors map[string][]string      `toml:"-" json:"-"`
}

type ContainerdGRPCConfig struct {
	Address        *string `toml:"address,omitempty" json:"address,omitempty"`
	UID            *int    `toml:"uid,omitempty" json:"uid,omitempty"`
	GID            *int    `toml:"gid,omitempty" json:"gid,omitempty"`
	MaxRecvMsgSize *int    `toml:"max_recv_message_size,omitempty" json:"max_recv_message_size,omitempty"`
	MaxSendMsgSize *int    `toml:"max_send_message_size,omitempty" json:"max_send_message_size,omitempty"`
}

type ContainerdMetricsConfig struct {
	Address       *string `toml:"address,omitempty" json:"address,omitempty"`
	GRPCHistogram *bool   `toml:"grpc_histogram,omitempty" json:"grpc_histogram,omitempty"`
}

type CtrImageInfo struct {
	Name   string
	Digest string
	Size   string
	OSArch string
	Labels map[string]string
}

type CtrContainerInfo struct {
	ID      string
	Image   string
	Runtime string
	Status  string
	Labels  map[string]string
}

type ContainerdContainerCreateOptions struct {
	ImageName      string
	ContainerID    string
	Snapshotter    string
	ConfigPath     string
	Runtime        string
	NetHost        bool
	TTY            bool
	Env            []string
	Mounts         []string
	Command        []string
	Labels         map[string]string
	RemoveExisting bool
	Privileged     bool
	ReadOnlyRootFS bool
	User           string
	Cwd            string
	Platforms      []string
}

type CtrExecOptions struct {
	TTY  bool
	User string
	Cwd  string
}

type CrictlImageInfo struct {
	ID          string   `json:"id"`
	RepoTags    []string `json:"repoTags"`
	RepoDigests []string `json:"repoDigests"`
	Size        string   `json:"size"`
	UID         *int64   `json:"uid"`
	Username    string   `json:"username"`
}

type CrictlImageDetails struct {
	Status struct {
		ID          string   `json:"id"`
		RepoTags    []string `json:"repoTags"`
		RepoDigests []string `json:"repoDigests"`
		Size        string   `json:"size"`
		Username    string   `json:"username"`
		UID         *int64   `json:"uid"`
	} `json:"status"`
	Info map[string]interface{} `json:"info"`
}

type CrictlFSInfo struct {
	Timestamp int64 `json:"timestamp"`
	FsID      struct {
		Mountpoint string `json:"mountpoint"`
	} `json:"fsId"`
	UsedBytes  string `json:"usedBytes"`
	InodesUsed string `json:"inodesUsed"`
}

type CrictlPodInfo struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Namespace      string            `json:"namespace"`
	Attempt        uint32            `json:"attempt"`
	State          string            `json:"state"`
	CreatedAt      string            `json:"createdAt"`
	Labels         map[string]string `json:"labels"`
	Annotations    map[string]string `json:"annotations"`
	RuntimeHandler string            `json:"runtimeHandler"`
}

type CrictlPodDetails struct {
	Status struct {
		ID       string `json:"id"`
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			Attempt   uint32 `json:"attempt"`
			UID       string `json:"uid"`
		} `json:"metadata"`
		State     string `json:"state"`
		CreatedAt string `json:"createdAt"`
		Network   struct {
			IP string `json:"ip"`
		} `json:"network"`
		Linux struct {
			Namespaces struct {
				Options struct {
					Network string `json:"network"`
					Pid     string `json:"pid"`
					Ipc     string `json:"ipc"`
				} `json:"options"`
			} `json:"namespaces"`
		} `json:"linux"`
		Labels         map[string]string `json:"labels"`
		Annotations    map[string]string `json:"annotations"`
		RuntimeHandler string            `json:"runtimeHandler"`
	} `json:"status"`
	Info map[string]interface{} `json:"info"`
}

type CrictlContainerDetails struct {
	Status struct {
		ID       string `json:"id"`
		Metadata struct {
			Name    string `json:"name"`
			Attempt uint32 `json:"attempt"`
		} `json:"metadata"`
		State      string `json:"state"`
		CreatedAt  string `json:"createdAt"`
		StartedAt  string `json:"startedAt"`
		FinishedAt string `json:"finishedAt"`
		ExitCode   int32  `json:"exitCode"`
		Image      struct {
			Image string `json:"image"`
			ID    string `json:"id"`
		} `json:"image"`
		ImageRef    string            `json:"imageRef"`
		Reason      string            `json:"reason"`
		Message     string            `json:"message"`
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
		Mounts      []struct {
			ContainerPath  string `json:"containerPath"`
			HostPath       string `json:"hostPath"`
			Readonly       bool   `json:"readonly"`
			Propagation    string `json:"propagation"`
			SelinuxRelabel bool   `json:"selinuxRelabel"`
		} `json:"mounts"`
		LogPath string `json:"logPath"`
	} `json:"status"`
	Pid  int                    `json:"pid"`
	Info map[string]interface{} `json:"info"`
}

type CrictlLogOptions struct {
	Follow     bool
	TailLines  *int64
	Since      string
	Timestamps bool
	Latest     bool
	NumLines   *int64
}

type CrictlVersionInfo struct {
	Version           string
	RuntimeName       string
	RuntimeVersion    string
	RuntimeApiVersion string
}

type DockerDaemonOptions struct {
	LogDriver              *string                   `json:"log-driver,omitempty"`
	LogOpts                *map[string]string        `json:"log-opts,omitempty"`
	StorageDriver          *string                   `json:"storage-driver,omitempty"`
	StorageOpts            *[]string                 `json:"storage-opts,omitempty"`
	RegistryMirrors        *[]string                 `json:"registry-mirrors,omitempty"`
	InsecureRegistries     *[]string                 `json:"insecure-registries,omitempty"`
	ExecOpts               *[]string                 `json:"exec-opts,omitempty"`
	Bridge                 *string                   `json:"bridge,omitempty"`
	Bip                    *string                   `json:"bip,omitempty"`
	FixedCIDR              *string                   `json:"fixed-cidr,omitempty"`
	DefaultGateway         *string                   `json:"default-gateway,omitempty"`
	DNS                    *[]string                 `json:"dns,omitempty"`
	IPTables               *bool                     `json:"iptables,omitempty"`
	Experimental           *bool                     `json:"experimental,omitempty"`
	Debug                  *bool                     `json:"debug,omitempty"`
	APICorsHeader          *string                   `json:"api-cors-header,omitempty"`
	Hosts                  *[]string                 `json:"hosts,omitempty"`
	UserlandProxy          *bool                     `json:"userland-proxy,omitempty"`
	LiveRestore            *bool                     `json:"live-restore,omitempty"`
	CgroupParent           *string                   `json:"cgroup-parent,omitempty"`
	DefaultRuntime         *string                   `json:"default-runtime,omitempty"`
	Runtimes               *map[string]DockerRuntime `json:"runtimes,omitempty"`
	Graph                  *string                   `json:"graph,omitempty"`
	DataRoot               *string                   `json:"data-root,omitempty"`
	MaxConcurrentDownloads *int                      `json:"max-concurrent-downloads,omitempty"`
	MaxConcurrentUploads   *int                      `json:"max-concurrent-uploads,omitempty"`
	ShutdownTimeout        *int                      `json:"shutdown-timeout,omitempty"`
}

type DockerRuntime struct {
	Path        string   `json:"path"`
	RuntimeArgs []string `json:"runtimeArgs,omitempty"`
}

type ImageInfo struct {
	ID          string
	RepoTags    []string
	Created     string
	Size        int64
	VirtualSize int64
}

type ContainerPortMapping struct {
	HostIP        string
	HostPort      string
	ContainerPort string
	Protocol      string
}

type ContainerMount struct {
	Type        string
	Source      string
	Destination string
	Mode        string
}

type ContainerCreateOptions struct {
	ImageName        string
	ContainerName    string
	Ports            []ContainerPortMapping
	Volumes          []ContainerMount
	EnvVars          []string
	Command          []string
	Entrypoint       []string
	WorkingDir       string
	User             string
	RestartPolicy    string
	NetworkMode      string
	ExtraHosts       []string
	Labels           map[string]string
	Privileged       bool
	CapAdd           []string
	CapDrop          []string
	Resources        ContainerResources
	HealthCheck      *ContainerHealthCheck
	AutoRemove       bool
	VolumesFrom      []string
	SecurityOpt      []string
	Sysctls          map[string]string
	DNSServers       []string
	DNSSearchDomains []string
}

type ContainerResources struct {
	CPUShares   int64
	Memory      int64
	NanoCPUs    int64
	PidsLimit   int64
	BlkioWeight uint16
}

type ContainerHealthCheck struct {
	Test        []string
	Interval    time.Duration
	Timeout     time.Duration
	Retries     int
	StartPeriod time.Duration
}

type ContainerInfo struct {
	ID      string
	Names   []string
	Image   string
	ImageID string
	Command string
	Created int64
	State   string
	Status  string
	Ports   []ContainerPortMapping
	Labels  map[string]string
	Mounts  []ContainerMount
}

type ContainerLogOptions struct {
	Follow     bool
	Tail       string
	Since      string
	Until      string
	Timestamps bool
	Details    bool
	ShowStdout bool
	ShowStderr bool
}

type ContainerStats struct {
	CPUPercentage    float64
	MemoryUsageBytes uint64
	MemoryLimitBytes uint64
	NetworkRxBytes   uint64
	NetworkTxBytes   uint64
	BlockReadBytes   uint64
	BlockWriteBytes  uint64
	PidsCurrent      uint64
	Error            error
}

type ContainerDetails struct {
	ID              string
	Created         string
	Path            string
	Args            []string
	State           *ContainerState
	Image           string
	ResolvConfPath  string
	HostnamePath    string
	HostsPath       string
	LogPath         string
	Name            string
	RestartCount    int
	Driver          string
	Platform        string
	MountLabel      string
	ProcessLabel    string
	AppArmorProfile string
	ExecIDs         []string
	HostConfig      *HostConfig
	GraphDriver     *GraphDriverData
	Mounts          []ContainerMount
	Config          *ContainerConfig
	NetworkSettings *NetworkSettings
}

type ContainerState struct {
	Status     string
	Running    bool
	Paused     bool
	Restarting bool
	OOMKilled  bool
	Dead       bool
	Pid        int
	ExitCode   int
	Error      string
	StartedAt  string
	FinishedAt string
}

type HostConfig struct {
	NetworkMode   string
	RestartPolicy struct {
		Name              string
		MaximumRetryCount int
	}
	PortBindings map[string][]ContainerPortMapping
	Resources    ContainerResources
	Privileged   bool
	AutoRemove   bool
}

type GraphDriverData struct {
	Name string
	Data map[string]string
}

type ContainerConfig struct {
	Hostname     string
	Domainname   string
	User         string
	AttachStdin  bool
	AttachStdout bool
	AttachStderr bool
	ExposedPorts map[string]struct{}
	Tty          bool
	OpenStdin    bool
	StdinOnce    bool
	Env          []string
	Cmd          []string
	Image        string
	Volumes      map[string]struct{}
	WorkingDir   string
	Entrypoint   []string
	Labels       map[string]string
	Healthcheck  *ContainerHealthCheck
}

type NetworkSettings struct {
	Bridge                 string
	SandboxID              string
	HairpinMode            bool
	LinkLocalIPv6Address   string
	LinkLocalIPv6PrefixLen int
	Ports                  map[string][]ContainerPortMapping
	SandboxKey             string
	SecondaryIPAddresses   []string
	SecondaryIPv6Addresses []string
	EndpointID             string
	Gateway                string
	GlobalIPv6Address      string
	GlobalIPv6PrefixLen    int
	IPAddress              string
	IPPrefixLen            int
	IPv6Gateway            string
	MacAddress             string
	Networks               map[string]*EndpointSettings
}

type EndpointSettings struct {
	IPAMConfig          *EndpointIPAMConfig
	Links               []string
	Aliases             []string
	NetworkID           string
	EndpointID          string
	Gateway             string
	IPAddress           string
	IPPrefixLen         int
	IPv6Gateway         string
	GlobalIPv6Address   string
	GlobalIPv6PrefixLen int
	MacAddress          string
	DriverOpts          map[string]string
}

type EndpointIPAMConfig struct {
	IPv4Address  string
	IPv6Address  string
	LinkLocalIPs []string
}

type DockerNetworkInfo struct {
	ID         string
	Name       string
	Driver     string
	Scope      string
	EnableIPv6 bool
	Subnets    []string
	Gateways   []string
	Containers map[string]string
}

type DockerVolumeInfo struct {
	Name       string
	Driver     string
	Mountpoint string
	Labels     map[string]string
	Scope      string
}

type DockerVolumeDetails struct {
	Name       string
	Driver     string
	Mountpoint string
	Status     map[string]string
	Labels     map[string]string
	Scope      string
	Options    map[string]string
	CreatedAt  string
}

type DockerSystemInfo struct {
	ID                string
	Containers        int
	ContainersRunning int
	ContainersPaused  int
	ContainersStopped int
	Images            int
	Driver            string
	DriverStatus      [][2]string
	Plugins           struct {
		Volume  []string
		Network []string
	}
	MemoryLimit        bool
	SwapLimit          bool
	KernelMemory       bool
	CPUCfsPeriod       bool
	CPUCfsQuota        bool
	CPUShares          bool
	CPUSet             bool
	PidsLimit          bool
	IPv4Forwarding     bool
	BridgeNfIptables   bool
	BridgeNfIp6tables  bool
	Debug              bool
	NFd                int
	OomKillDisable     bool
	NGoroutines        int
	SystemTime         string
	LoggingDriver      string
	CgroupDriver       string
	CgroupVersion      string
	NEventsListener    int
	KernelVersion      string
	OperatingSystem    string
	OSType             string
	Architecture       string
	IndexServerAddress string
	RegistryConfig     *RegistryConfig
	NCPU               int
	MemTotal           int64
	ServerVersion      string
}

type RegistryConfig struct {
	IndexConfigs map[string]*IndexInfo
}

type IndexInfo struct {
	Name     string
	Mirrors  []string
	Secure   bool
	Official bool
}

type UserModifications struct {
	NewUsername         *string
	NewPrimaryGroup     *string
	AppendToGroups      []string
	SetSecondaryGroups  []string
	NewShell            *string
	NewHomeDir          *string
	MoveHomeDirContents bool
	NewComment          *string
	LockPassword        bool
	UnlockPassword      bool
	ExpireDate          *string
}

type UserInfo struct {
	Username       string
	UID            string
	GID            string
	Comment        string
	HomeDir        string
	Shell          string
	Groups         []string
	PasswordStatus string
}

type VMNetworkInterface struct {
	Type       string
	Source     string
	Model      string
	MACAddress string
}

type VMInfo struct {
	Name   string
	State  string
	CPUs   int
	Memory uint
	UUID   string
	Error  string `json:"error,omitempty"`
}

type VMNetworkInterfaceDetail struct {
	VMNetworkInterface
	InterfaceName string
	State         string
}

type VMSnapshotDiskSpec struct {
	Name       string
	Snapshot   string
	DriverType string
	File       string
}

type VMSnapshotInfo struct {
	Name        string
	Description string
	CreatedAt   string
	State       string
	HasParent   bool
	Children    []string
	Disks       map[string]string
}

type VMInterfaceAddress struct {
	Addr   string `json:"addr"`
	Prefix int    `json:"prefix"`
}

type VMInterfaceInfo struct {
	Name        string               `json:"name"`
	MAC         string               `json:"mac"`
	Source      string               `json:"source,omitempty"`
	IPAddresses []VMInterfaceAddress `json:"ip-addresses,omitempty"`
	DeviceName  string               `json:"deviceName"`
}

type VMBlockDeviceInfo struct {
	Device     string `json:"device"`
	Type       string `json:"type"`
	SourceFile string `json:"source-file,omitempty"`
	DriverType string `json:"driver-type,omitempty"`
	TargetBus  string `json:"target-bus,omitempty"`
	Size       uint64 `json:"size-bytes,omitempty"`
}

type VMDetails struct {
	VMInfo
	OSVariant         string
	DomainType        string
	Architecture      string
	EmulatorPath      string
	Graphics          []VMGraphicsInfo
	Disks             []VMBlockDeviceInfo
	NetworkInterfaces []VMInterfaceInfo
	PersistentConfig  bool
	Autostart         bool
	EffectiveMemory   uint
	EffectiveVCPUs    uint
	RawXML            string
}

type VMGraphicsInfo struct {
	Type     string
	Port     string
	Listen   string
	Password string
	Keymap   string
}

type CrictlRuntimeInfo struct {
	Config struct {
		Containerd struct {
			Snapshotter string `json:"snapshotter"`
			Runtimes    map[string]struct {
				Type        string `json:"runtimeType"`
				Engine      string `json:"runtimeEngine"`
				Root        string `json:"runtimeRoot"`
				SandboxMode string `json:"sandboxMode"`
			} `json:"runtimes"`
		} `json:"containerd"`
	} `json:"config"`
	Status map[string]interface{} `json:"status"`
}

type KubectlTopOptions struct {
	KubeconfigPath string
	Namespace      string
	AllNamespaces  bool
	Selector       string
	Containers     bool
	SortBy         string
	UseHeapster    bool
	Sudo           bool
}

type KubectlExplainOptions struct {
	KubeconfigPath string
	APIVersion     string
	Recursive      bool
	Sudo           bool
}

type KubectlDrainOptions struct {
	KubeconfigPath           string
	Force                    bool
	GracePeriod              int
	IgnoreDaemonSets         bool
	DeleteLocalData          bool
	Selector                 string
	Timeout                  time.Duration
	DisableEviction          bool
	SkipWaitForDeleteTimeout int
	Sudo                     bool
}

type KubectlCordonUncordonOptions struct {
	KubeconfigPath string
	Selector       string
	Sudo           bool
}

type KubectlTaintOptions struct {
	KubeconfigPath string
	Selector       string
	Overwrite      bool
	All            bool
	Sudo           bool
}

type KubectlCreateOptions struct {
	KubeconfigPath string
	DryRun         string
	Validate       bool
	Sudo           bool
}

type KubectlSetOptions struct {
	KubeconfigPath string
	Namespace      string
	All            bool
	Selector       string
	Local          bool
	DryRun         string
	Sudo           bool
}

// KubectlRolloutOptions defines options for kubectl rollout commands.
type KubectlRolloutOptions struct {
	KubeconfigPath string
	Namespace      string
	Watch          bool
	Timeout        time.Duration
	Sudo           bool
	ToRevision     int // For Undo
	// Add other common rollout flags if needed, e.g. DryRun
}

// KubectlScaleOptions defines options for kubectl scale commands.
type KubectlScaleOptions struct {
	KubeconfigPath  string
	Namespace       string
	CurrentReplicas *int32
	ResourceVersion *string
	Timeout         time.Duration
	Sudo            bool
	// Add other common scale flags if needed
}

// KubectlConfigViewOptions defines options for kubectl config view.
type KubectlConfigViewOptions struct {
	KubeconfigPath string
	Minify         bool
	Raw            bool
	OutputFormat   string // Typically "json" or "yaml" for programmatic use
	Sudo           bool
	// Add other common config view flags if needed
}

// KubectlPortForwardOptions defines options for kubectl port-forward.
type KubectlPortForwardOptions struct {
	KubeconfigPath    string
	Namespace         string
	Address           []string // Addresses to listen on (e.g., "localhost", "127.0.0.1")
	PodRunningTimeout time.Duration
	Sudo              bool
	// Add other common port-forward flags if needed, e.g. StopChannel, ReadyChannel for more programmatic control
}

type KubectlAutoscaleOptions struct {
	KubeconfigPath string
	Namespace      string
	Name           string
	DryRun         string
	Sudo           bool
}

type KubectlWaitOptions struct {
	KubeconfigPath string
	Namespace      string
	AllNamespaces  bool
	Selector       string
	FieldSelector  string
	For            string
	Timeout        time.Duration
	Sudo           bool
}

type KubectlLabelOptions struct {
	KubeconfigPath string
	Namespace      string
	AllNamespaces  bool
	Selector       string
	Overwrite      bool
	Local          bool
	DryRun         string
	ListLabels     bool
	Timeout        time.Duration
	Sudo           bool
}

type KubectlAnnotateOptions struct {
	KubeconfigPath  string
	Namespace       string
	AllNamespaces   bool
	Selector        string
	Overwrite       bool
	Local           bool
	DryRun          string
	ListAnnotations bool
	Timeout         time.Duration
	Sudo            bool
}

type KubectlPatchOptions struct {
	KubeconfigPath string
	Namespace      string
	Local          bool
	DryRun         string
	Sudo           bool
}

type CrictlConfigOptions struct {
	RuntimeEndpoint   string `yaml:"runtime-endpoint,omitempty" json:"runtime-endpoint,omitempty"`
	ImageEndpoint     string `yaml:"image-endpoint,omitempty" json:"image-endpoint,omitempty"`
	Timeout           *int   `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	Debug             *bool  `yaml:"debug,omitempty" json:"debug,omitempty"`
	PullImageOnCreate *bool  `yaml:"pull-image-on-create,omitempty" json:"pull-image-on-create,omitempty"`
}
