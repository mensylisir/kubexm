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
	RunInBackground(ctx context.Context, conn connector.Connector, cmd string, sudo bool) error
	RunRetry(ctx context.Context, conn connector.Connector, cmd string, sudo bool, retries int, delay time.Duration) (string, error)
	Download(ctx context.Context, conn connector.Connector, facts *Facts, url, destPath string, sudo bool) error
	Extract(ctx context.Context, conn connector.Connector, facts *Facts, archivePath, destDir string, sudo bool) error
	DownloadAndExtract(ctx context.Context, conn connector.Connector, facts *Facts, url, destDir string, sudo bool) error
	Compress(ctx context.Context, conn connector.Connector, facts *Facts, archivePath string, sources []string, sudo bool) error
	ListArchiveContents(ctx context.Context, conn connector.Connector, facts *Facts, archivePath string, sudo bool) ([]string, error)
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
	Unmount(ctx context.Context, conn connector.Connector, mountPoint string, force bool, sudo bool) error
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

	// --- QEMU/libvirt Methods ---

	// CreateVMTemplate defines a new virtual machine configuration that can serve as a template.
	// It sets up the basic XML definition for a VM with specified parameters.
	// The associated disk image at diskPath should exist or be creatable; this method
	// will attempt to create a qcow2 disk image of diskSizeGB if diskPath does not exist.
	// conn: Connector to the host where libvirt is running.
	// name: Name for the VM template.
	// osVariant: OS variant string (e.g., "ubuntu22.04") for libvirt.
	// memoryMB: Memory for the VM in Megabytes.
	// vcpus: Number of virtual CPUs.
	// diskPath: Path on the host for the primary disk image.
	// diskSizeGB: Size of the primary disk in Gigabytes (used if creating the disk).
	// network: Name of the libvirt network to connect the VM to (e.g., "default").
	// graphicsType: Type of graphics device (e.g., "vnc", "spice", "none").
	// cloudInitISOPath: Optional path to a cloud-init ISO image for VM provisioning.
	CreateVMTemplate(ctx context.Context, conn connector.Connector, name string, osVariant string, memoryMB uint, vcpus uint, diskPath string, diskSizeGB uint, network string, graphicsType string, cloudInitISOPath string) error

	// ImportVMTemplate defines a new VM from an existing libvirt XML definition file.
	// conn: Connector to the host.
	// name: Name to assign to the imported VM (Note: libvirt might prioritize name within XML).
	// filePath: Path on the host to the XML file containing the VM definition.
	ImportVMTemplate(ctx context.Context, conn connector.Connector, name string, filePath string) error

	// RefreshStoragePool tells libvirt to refresh its view of a storage pool,
	// detecting any new or changed storage volumes.
	// conn: Connector to the host.
	// poolName: Name of the storage pool to refresh.
	RefreshStoragePool(ctx context.Context, conn connector.Connector, poolName string) error

	// CreateStoragePool defines and builds a new storage pool.
	// For "dir" type pools, it will create the targetPath if it doesn't exist.
	// The pool is set to autostart and is started after creation.
	// conn: Connector to the host.
	// name: Name for the new storage pool.
	// poolType: Type of the storage pool (e.g., "dir", "logical", "iscsi").
	// targetPath: Target path for the pool (e.g., directory path for "dir" type).
	CreateStoragePool(ctx context.Context, conn connector.Connector, name string, poolType string, targetPath string) error

	// StoragePoolExists checks if a storage pool with the given name is defined in libvirt.
	// conn: Connector to the host.
	// poolName: Name of the storage pool to check.
	StoragePoolExists(ctx context.Context, conn connector.Connector, poolName string) (bool, error)

	// DeleteStoragePool stops (destroys) and undefines a storage pool.
	// It does not delete the underlying storage (e.g., files in a "dir" pool) by default.
	// conn: Connector to the host.
	// poolName: Name of the storage pool to delete.
	DeleteStoragePool(ctx context.Context, conn connector.Connector, poolName string) error

	// VolumeExists checks if a storage volume exists within a given storage pool.
	// conn: Connector to the host.
	// poolName: Name of the storage pool.
	// volName: Name of the storage volume.
	VolumeExists(ctx context.Context, conn connector.Connector, poolName string, volName string) (bool, error)

	// CloneVolume creates a new storage volume by cloning an existing volume within the same pool.
	// conn: Connector to the host.
	// poolName: Name of the storage pool.
	// origVolName: Name of the original volume to clone.
	// newVolName: Name for the new cloned volume.
	// newSizeGB: Desired size for the new volume in Gigabytes. If 0, defaults to original size.
	//            Note: Actual resizing capability during clone depends on libvirt version and backend.
	//            A separate ResizeVolume call might be needed if the clone doesn't expand.
	// format: Format for the new volume (e.g., "qcow2").
	CloneVolume(ctx context.Context, conn connector.Connector, poolName string, origVolName string, newVolName string, newSizeGB uint, format string) error

	// ResizeVolume changes the capacity of an existing storage volume.
	// Currently supports expansion only.
	// conn: Connector to the host.
	// poolName: Name of the storage pool containing the volume.
	// volName: Name of the volume to resize.
	// newSizeGB: The new total size for the volume in Gigabytes.
	ResizeVolume(ctx context.Context, conn connector.Connector, poolName string, volName string, newSizeGB uint) error

	// DeleteVolume deletes a storage volume from a pool.
	// conn: Connector to the host.
	// poolName: Name of the storage pool.
	// volName: Name of the volume to delete.
	DeleteVolume(ctx context.Context, conn connector.Connector, poolName string, volName string) error

	// CreateVolume creates a new storage volume in a pool.
	// It can create a standalone volume or a volume based on a backing store (linked clone).
	// conn: Connector to the host.
	// poolName: Name of the storage pool.
	// volName: Name for the new volume.
	// sizeGB: Size of the volume in Gigabytes.
	// format: Format of the volume (e.g., "qcow2", "raw").
	// backingVolName: Optional name of the backing volume in the same pool. If empty, a standalone volume is created.
	// backingVolFormat: Optional format of the backing volume (e.g., "qcow2"). Required if backingVolName is provided.
	CreateVolume(ctx context.Context, conn connector.Connector, poolName string, volName string, sizeGB uint, format string, backingVolName string, backingVolFormat string) error

	// CreateCloudInitISO generates a cloud-init ISO image on the host.
	// It requires `genisoimage` or `mkisofs` to be installed on the target host.
	// conn: Connector to the host.
	// vmName: Name of the VM, used for temporary directory naming to ensure uniqueness.
	// isoDestPath: Full path on the host where the generated ISO should be saved.
	// userData: Content of the user-data file for cloud-init.
	// metaData: Content of the meta-data file for cloud-init.
	// networkConfig: Content of the network-config file for cloud-init (optional).
	CreateCloudInitISO(ctx context.Context, conn connector.Connector, vmName string, isoDestPath string, userData string, metaData string, networkConfig string) error

	// CreateVM defines and starts a new virtual machine based on provided parameters.
	// conn: Connector to the host.
	// vmName: Name for the new VM.
	// memoryMB: Memory for the VM in Megabytes.
	// vcpus: Number of virtual CPUs.
	// osVariant: OS variant string (e.g., "ubuntu22.04") for libvirt, helps in setting defaults.
	// diskPaths: Slice of paths on the host to disk images (e.g., qcow2 files). First disk is typically primary.
	// networkInterfaces: Slice of VMNetworkInterface configurations for network setup.
	// graphicsType: Type of graphics (e.g., "vnc", "spice", "none"). Defaults to "vnc".
	// cloudInitISOPath: Optional path to a cloud-init ISO for provisioning. If provided, "cdrom" is added to boot order.
	// bootOrder: Slice of boot devices (e.g., "hd", "cdrom"). Defaults to "hd".
	// extraArgs: Placeholder for future QEMU command-line passthrough arguments (not fully implemented in basic XML).
	CreateVM(ctx context.Context, conn connector.Connector, vmName string, memoryMB uint, vcpus uint, osVariant string, diskPaths []string, networkInterfaces []VMNetworkInterface, graphicsType string, cloudInitISOPath string, bootOrder []string, extraArgs []string) error

	// VMExists checks if a virtual machine with the given name is defined in libvirt.
	// conn: Connector to the host.
	// vmName: Name of the VM to check.
	VMExists(ctx context.Context, conn connector.Connector, vmName string) (bool, error)

	// StartVM starts a defined (but not running) virtual machine.
	// If the VM is already running, it does nothing.
	// conn: Connector to the host.
	// vmName: Name of the VM to start.
	StartVM(ctx context.Context, conn connector.Connector, vmName string) error

	// ShutdownVM attempts a graceful shutdown of a virtual machine.
	// If `force` is true, it will destroy the VM if graceful shutdown fails or times out.
	// conn: Connector to the host.
	// vmName: Name of the VM to shut down.
	// force: If true, forcefully destroy if graceful shutdown fails.
	// timeout: Duration to wait for graceful shutdown before forcing (if applicable) or returning timeout error.
	ShutdownVM(ctx context.Context, conn connector.Connector, vmName string, force bool, timeout time.Duration) error

	// DestroyVM forcefully stops (powers off) a virtual machine.
	// conn: Connector to the host.
	// vmName: Name of the VM to destroy.
	DestroyVM(ctx context.Context, conn connector.Connector, vmName string) error

	// UndefineVM removes the definition of a virtual machine from libvirt.
	// The VM must be shut off.
	// conn: Connector to the host.
	// vmName: Name of the VM to undefine.
	// deleteSnapshots: If true, attempts to delete all snapshots associated with the VM.
	// deleteStorage: If true, attempts to delete associated storage volumes. This is heuristic and relies on `storagePools`
	//                to correctly identify volumes if paths are used. Use with caution.
	// storagePools: A list of storage pool names to help identify volumes if `deleteStorage` is true and disks are path-based.
	UndefineVM(ctx context.Context, conn connector.Connector, vmName string, deleteSnapshots bool, deleteStorage bool, storagePools []string) error

	// GetVMState retrieves the current state of a virtual machine (e.g., "running", "shut off").
	// conn: Connector to the host.
	// vmName: Name of the VM.
	GetVMState(ctx context.Context, conn connector.Connector, vmName string) (string, error)

	// ListVMs lists virtual machines known to libvirt.
	// conn: Connector to the host.
	// all: If true, includes inactive (defined but not running) VMs. If false, lists only active VMs.
	ListVMs(ctx context.Context, conn connector.Connector, all bool) ([]VMInfo, error)

	// AttachDisk attaches a disk to a VM. Can be done live if VM and libvirt support it.
	// conn: Connector to the host.
	// vmName: Name of the VM.
	// diskPath: Path to the disk image file on the host.
	// targetDevice: Target device name in the VM (e.g., "vdb", "sdc").
	// diskType: Type of the disk source (e.g., "file", "block").
	// driverType: Disk driver type (e.g., "qcow2", "raw").
	AttachDisk(ctx context.Context, conn connector.Connector, vmName string, diskPath string, targetDevice string, diskType string, driverType string) error

	// DetachDisk detaches a disk from a VM. Can be done live.
	// conn: Connector to the host.
	// vmName: Name of the VM.
	// targetDeviceOrPath: The target device name (e.g., "vdb") or the source file path of the disk to detach.
	DetachDisk(ctx context.Context, conn connector.Connector, vmName string, targetDeviceOrPath string) error

	// SetVMMemory changes the memory allocation for a VM.
	// conn: Connector to the host.
	// vmName: Name of the VM.
	// memoryMB: New memory size in Megabytes.
	// current: If true, attempts to apply live if VM is running (and sets current config).
	//          If false, only sets config for next boot (and current if VM is running and it's supported).
	SetVMMemory(ctx context.Context, conn connector.Connector, vmName string, memoryMB uint, current bool) error

	// SetVMCPUs changes the number of virtual CPUs for a VM.
	// conn: Connector to the host.
	// vmName: Name of the VM.
	// vcpus: New number of vCPUs.
	// current: If true, attempts to apply live if VM is running (and sets current config).
	//          If false, only sets config for next boot (and current if VM is running and it's supported).
	SetVMCPUs(ctx context.Context, conn connector.Connector, vmName string, vcpus uint, current bool) error

	// --- Docker Methods ---

	// PullImage pulls a Docker image from a registry.
	// conn: Connector to the host where Docker daemon is running.
	// imageName: Name of the image to pull (e.g., "ubuntu:latest").
	PullImage(ctx context.Context, conn connector.Connector, imageName string) error

	// ImageExists checks if a Docker image exists locally on the host.
	// conn: Connector to the host.
	// imageName: Name of the image to check.
	ImageExists(ctx context.Context, conn connector.Connector, imageName string) (bool, error)

	// ListImages lists Docker images available locally on the host.
	// conn: Connector to the host.
	// all: If true, includes intermediate image layers. If false, shows top-level images.
	ListImages(ctx context.Context, conn connector.Connector, all bool) ([]ImageInfo, error)

	// RemoveImage removes a Docker image from the host.
	// conn: Connector to the host.
	// imageName: Name of the image to remove.
	// force: If true, forcefully remove the image (e.g., remove running containers using it).
	RemoveImage(ctx context.Context, conn connector.Connector, imageName string, force bool) error

	// BuildImage builds a Docker image from a Dockerfile.
	// Note: Context handling for local paths requires the client to create a tar stream of the context.
	// This implementation is simplified and might work best with URL contexts or require `r.Run("docker build ...")` for robustness with local file contexts.
	// conn: Connector to the host.
	// dockerfilePath: Path to the Dockerfile (can be relative to contextPath or a URL).
	// imageNameAndTag: Name and tag for the built image (e.g., "myimage:latest").
	// contextPath: Path to the build context (directory or URL to a git repo).
	// buildArgs: Map of build-time variables.
	BuildImage(ctx context.Context, conn connector.Connector, dockerfilePath string, imageNameAndTag string, contextPath string, buildArgs map[string]string) error

	// CreateContainer creates a new Docker container from an image.
	// conn: Connector to the host.
	// options: ContainerCreateOptions struct detailing container configuration.
	// Returns the ID of the created container.
	CreateContainer(ctx context.Context, conn connector.Connector, options ContainerCreateOptions) (string, error)

	// ContainerExists checks if a Docker container (by name or ID) exists on the host.
	// conn: Connector to the host.
	// containerNameOrID: Name or ID of the container.
	ContainerExists(ctx context.Context, conn connector.Connector, containerNameOrID string) (bool, error)

	// StartContainer starts an existing Docker container.
	// conn: Connector to the host.
	// containerNameOrID: Name or ID of the container to start.
	StartContainer(ctx context.Context, conn connector.Connector, containerNameOrID string) error

	// StopContainer stops a running Docker container.
	// conn: Connector to the host.
	// containerNameOrID: Name or ID of the container to stop.
	// timeout: Optional duration to wait for graceful stop before killing the container. If nil, Docker default is used.
	StopContainer(ctx context.Context, conn connector.Connector, containerNameOrID string, timeout *time.Duration) error

	// RestartContainer restarts a Docker container.
	// conn: Connector to the host.
	// containerNameOrID: Name or ID of the container to restart.
	// timeout: Optional duration to wait for stop before starting. If nil, Docker default is used.
	RestartContainer(ctx context.Context, conn connector.Connector, containerNameOrID string, timeout *time.Duration) error

	// RemoveContainer removes a Docker container from the host.
	// conn: Connector to the host.
	// containerNameOrID: Name or ID of the container to remove.
	// force: If true, forcefully remove a running container.
	// removeVolumes: If true, remove anonymous volumes associated with the container.
	RemoveContainer(ctx context.Context, conn connector.Connector, containerNameOrID string, force bool, removeVolumes bool) error

	// ListContainers lists Docker containers on the host.
	// conn: Connector to the host.
	// all: If true, lists all containers (including stopped). If false, only running.
	// filters: Map of filters to apply (e.g., {"status": "running"}).
	ListContainers(ctx context.Context, conn connector.Connector, all bool, filters map[string]string) ([]ContainerInfo, error)

	// GetContainerLogs retrieves logs from a Docker container.
	// Note: `options.Follow = true` is not suitable for this string-returning function signature;
	// it would require a streaming approach (e.g., returning a channel).
	// conn: Connector to the host.
	// containerNameOrID: Name or ID of the container.
	// options: ContainerLogOptions specifying how to retrieve logs.
	GetContainerLogs(ctx context.Context, conn connector.Connector, containerNameOrID string, options ContainerLogOptions) (string, error)

	// GetContainerStats retrieves a stream of live resource usage statistics for a container.
	// It returns a read-only channel from which `ContainerStats` can be received.
	// The channel will be closed when the stream ends (e.g., context cancelled, container stops if not streaming indefinitely).
	// The caller is responsible for consuming from the channel.
	// conn: Connector to the host.
	// containerNameOrID: Name or ID of the container.
	// stream: If true, continuously stream stats. If false, get a single snapshot.
	GetContainerStats(ctx context.Context, conn connector.Connector, containerNameOrID string, stream bool) (<-chan ContainerStats, error)

	// InspectContainer retrieves detailed information about a Docker container.
	// Returns nil if container not found (and no error).
	// conn: Connector to the host.
	// containerNameOrID: Name or ID of the container.
	InspectContainer(ctx context.Context, conn connector.Connector, containerNameOrID string) (*ContainerDetails, error)

	// PauseContainer pauses all processes within a running Docker container.
	// conn: Connector to the host.
	// containerNameOrID: Name or ID of the container to pause.
	PauseContainer(ctx context.Context, conn connector.Connector, containerNameOrID string) error

	// UnpauseContainer unpauses all processes within a paused Docker container.
	// conn: Connector to the host.
	// containerNameOrID: Name or ID of the container to unpause.
	UnpauseContainer(ctx context.Context, conn connector.Connector, containerNameOrID string) error

	// ExecInContainer executes a command inside a running Docker container.
	// conn: Connector to the host.
	// containerNameOrID: Name or ID of the container.
	// cmd: Command and arguments to execute.
	// user: Optional user to run the command as.
	// workDir: Optional working directory for the command.
	// tty: If true, allocate a pseudo-TTY. Affects output stream format.
	// Returns combined stdout/stderr output of the command.
	ExecInContainer(ctx context.Context, conn connector.Connector, containerNameOrID string, cmd []string, user string, workDir string, tty bool) (string, error)

	// CreateDockerNetwork creates a new Docker network.
	// conn: Connector to the host.
	// name: Name for the new network.
	// driver: Network driver (e.g., "bridge", "overlay"). Defaults to "bridge" if empty by Docker.
	// subnet: Optional subnet in CIDR format for IPAM configuration.
	// gateway: Optional gateway for the subnet.
	// options: Driver-specific options for the network.
	CreateDockerNetwork(ctx context.Context, conn connector.Connector, name string, driver string, subnet string, gateway string, options map[string]string) error

	// RemoveDockerNetwork removes a Docker network.
	// conn: Connector to the host.
	// networkNameOrID: Name or ID of the network to remove.
	RemoveDockerNetwork(ctx context.Context, conn connector.Connector, networkNameOrID string) error

	// ListDockerNetworks lists Docker networks on the host.
	// conn: Connector to the host.
	// filters: Map of filters to apply.
	ListDockerNetworks(ctx context.Context, conn connector.Connector, filters map[string]string) ([]DockerNetworkInfo, error)

	// ConnectContainerToNetwork connects a container to an existing Docker network.
	// conn: Connector to the host.
	// containerNameOrID: Name or ID of the container.
	// networkNameOrID: Name or ID of the network.
	// ipAddress: Optional static IPv4 address for the container on this network.
	ConnectContainerToNetwork(ctx context.Context, conn connector.Connector, containerNameOrID string, networkNameOrID string, ipAddress string) error

	// DisconnectContainerFromNetwork disconnects a container from a Docker network.
	// conn: Connector to the host.
	// containerNameOrID: Name or ID of the container.
	// networkNameOrID: Name or ID of the network.
	// force: If true, forcefully disconnect.
	DisconnectContainerFromNetwork(ctx context.Context, conn connector.Connector, containerNameOrID string, networkNameOrID string, force bool) error

	// CreateDockerVolume creates a new Docker volume.
	// conn: Connector to the host.
	// name: Name for the new volume.
	// driver: Volume driver (e.g., "local"). Defaults to "local" if empty by Docker.
	// driverOpts: Driver-specific options.
	// labels: Labels to apply to the volume.
	CreateDockerVolume(ctx context.Context, conn connector.Connector, name string, driver string, driverOpts map[string]string, labels map[string]string) error

	// RemoveDockerVolume removes a Docker volume.
	// conn: Connector to the host.
	// volumeName: Name of the volume to remove.
	// force: If true, forcefully remove the volume (e.g., if in use by a stopped container).
	RemoveDockerVolume(ctx context.Context, conn connector.Connector, volumeName string, force bool) error

	// ListDockerVolumes lists Docker volumes on the host.
	// conn: Connector to the host.
	// filters: Map of filters to apply.
	ListDockerVolumes(ctx context.Context, conn connector.Connector, filters map[string]string) ([]DockerVolumeInfo, error)

	// InspectDockerVolume retrieves detailed information about a Docker volume.
	// Returns nil if volume not found (and no error).
	// conn: Connector to the host.
	// volumeName: Name of the volume.
	InspectDockerVolume(ctx context.Context, conn connector.Connector, volumeName string) (*DockerVolumeDetails, error)

	// DockerInfo retrieves system-wide information about the Docker daemon.
	// conn: Connector to the host.
	DockerInfo(ctx context.Context, conn connector.Connector) (*DockerSystemInfo, error)

	// DockerPrune reclaims disk space by removing unused Docker resources.
	// conn: Connector to the host.
	// pruneType: Type of resource to prune ("containers", "images", "networks", "volumes", "system").
	// filters: Filters to apply to the prune operation (semantics vary by pruneType).
	// all: For images, `all=true` prunes all unused images, not just dangling ones. For system, influences image pruning.
	// Returns a summary string of actions taken and space reclaimed.
	DockerPrune(ctx context.Context, conn connector.Connector, pruneType string, filters map[string]string, all bool) (string, error)

	// --- Containerd/ctr Methods ---
	CtrListNamespaces(ctx context.Context, conn connector.Connector) ([]string, error)
	CtrListImages(ctx context.Context, conn connector.Connector, namespace string) ([]CtrImageInfo, error)
	CtrPullImage(ctx context.Context, conn connector.Connector, namespace, imageName string, allPlatforms bool, user string) error
	CtrRemoveImage(ctx context.Context, conn connector.Connector, namespace, imageName string) error
	CtrTagImage(ctx context.Context, conn connector.Connector, namespace, sourceImage, targetImage string) error
	CtrListContainers(ctx context.Context, conn connector.Connector, namespace string) ([]CtrContainerInfo, error)
	CtrRunContainer(ctx context.Context, conn connector.Connector, namespace string, opts ContainerdContainerCreateOptions) (string, error) // Returns container ID
	CtrStopContainer(ctx context.Context, conn connector.Connector, namespace, containerID string, timeout time.Duration) error
	CtrRemoveContainer(ctx context.Context, conn connector.Connector, namespace, containerID string) error
	CtrExecInContainer(ctx context.Context, conn connector.Connector, namespace, containerID string, opts CtrExecOptions, cmd []string) (string, error)
	CtrImportImage(ctx context.Context, conn connector.Connector, namespace, filePath string, allPlatforms bool) error
	CtrExportImage(ctx context.Context, conn connector.Connector, namespace, imageName, outputFilePath string, allPlatforms bool) error

	// --- Containerd/crictl Methods ---
	CrictlListImages(ctx context.Context, conn connector.Connector, filters map[string]string) ([]CrictlImageInfo, error)
	CrictlPullImage(ctx context.Context, conn connector.Connector, imageName string, authCreds string, sandboxConfigPath string) error
	CrictlRemoveImage(ctx context.Context, conn connector.Connector, imageName string) error
	CrictlInspectImage(ctx context.Context, conn connector.Connector, imageName string) (*CrictlImageDetails, error)
	CrictlImageFSInfo(ctx context.Context, conn connector.Connector) ([]CrictlFSInfo, error)
	CrictlListPods(ctx context.Context, conn connector.Connector, filters map[string]string) ([]CrictlPodInfo, error)
	CrictlRunPod(ctx context.Context, conn connector.Connector, podSandboxConfigFile string) (string, error) // Returns Pod ID
	CrictlStopPod(ctx context.Context, conn connector.Connector, podID string) error
	CrictlRemovePod(ctx context.Context, conn connector.Connector, podID string) error
	CrictlInspectPod(ctx context.Context, conn connector.Connector, podID string) (*CrictlPodDetails, error) // Define CrictlPodDetails
	CrictlCreateContainer(ctx context.Context, conn connector.Connector, podID string, containerConfigFile string, podSandboxConfigFile string) (string, error) // Returns Container ID
	CrictlStartContainer(ctx context.Context, conn connector.Connector, containerID string) error
	CrictlStopContainer(ctx context.Context, conn connector.Connector, containerID string, timeout int64) error // timeout in seconds
	CrictlRemoveContainerForce(ctx context.Context, conn connector.Connector, containerID string) error // crictl rm -f
	CrictlInspectContainer(ctx context.Context, conn connector.Connector, containerID string) (*CrictlContainerDetails, error) // Define CrictlContainerDetails
	CrictlLogs(ctx context.Context, conn connector.Connector, containerID string, opts CrictlLogOptions) (string, error)
	CrictlExec(ctx context.Context, conn connector.Connector, containerID string, timeout time.Duration, sync bool, cmd []string) (string, error)
	CrictlPortForward(ctx context.Context, conn connector.Connector, podID string, ports []string) (string, error) // Returns request ID or output
	CrictlVersion(ctx context.Context, conn connector.Connector) (*CrictlVersionInfo, error)
	CrictlRuntimeConfig(ctx context.Context, conn connector.Connector) (string, error) // Returns raw config string

	// --- Helm Methods ---
	HelmInstall(ctx context.Context, conn connector.Connector, releaseName, chartPath string, opts HelmInstallOptions) error
	HelmUninstall(ctx context.Context, conn connector.Connector, releaseName string, opts HelmUninstallOptions) error
	HelmList(ctx context.Context, conn connector.Connector, opts HelmListOptions) ([]HelmReleaseInfo, error)
	HelmStatus(ctx context.Context, conn connector.Connector, releaseName string, opts HelmStatusOptions) (*HelmReleaseInfo, error) // Single release status
	HelmRepoAdd(ctx context.Context, conn connector.Connector, name, url string, opts HelmRepoAddOptions) error
	HelmRepoUpdate(ctx context.Context, conn connector.Connector, repoNames []string) error
	HelmSearchRepo(ctx context.Context, conn connector.Connector, keyword string, opts HelmSearchOptions) ([]HelmChartInfo, error)
	HelmPull(ctx context.Context, conn connector.Connector, chartPath string, opts HelmPullOptions) (string, error) // Returns path to downloaded chart
	HelmPackage(ctx context.Context, conn connector.Connector, chartPath string, opts HelmPackageOptions) (string, error) // Returns path to packaged chart
	HelmVersion(ctx context.Context, conn connector.Connector) (*HelmVersionInfo, error)
}

// --- Helm Supporting Structs ---

type HelmInstallOptions struct {
	Namespace       string   // Namespace to install the release into
	KubeconfigPath  string   // Path to kubeconfig file on the target host
	ValuesFiles     []string // List of paths to values files
	SetValues       []string // List of set values (e.g., "key1=value1,key2.subkey=value2")
	Version         string   // Specify chart version
	CreateNamespace bool     // Whether to create the namespace if it doesn't exist
	Wait            bool     // If true, will wait until all Pods, PVCs, Services, and minimum number of Pods of a Deployment, StatefulSet, or ReplicaSet are in a ready state
	Timeout         time.Duration // Time to wait for any individual Kubernetes operation (like Jobs for hooks)
	Atomic          bool     // If true, installation process purges chart on fail. The --wait flag will be set automatically if --atomic is used
	DryRun          bool     // Simulate an install
	Devel           bool     // Use development versions, too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored.
	Description     string   // Add a custom description
	Sudo            bool     // If helm command itself needs sudo
	Retries         int      // Number of retries for the command execution
	RetryDelay      time.Duration // Delay between retries
}

type HelmUninstallOptions struct {
	Namespace      string        // Namespace of the release
	KubeconfigPath string        // Path to kubeconfig file
	KeepHistory    bool          // If true, remove all associated resources and mark the release as deleted, but retain the release history
	Timeout        time.Duration // Time to wait for any individual Kubernetes operation (like Jobs for hooks)
	DryRun         bool          // Simulate an uninstall
	Sudo           bool
}

type HelmListOptions struct {
	Namespace      string            // Scope this list to a specific namespace
	KubeconfigPath string            // Path to kubeconfig file
	AllNamespaces  bool              // List releases across all namespaces
	Filter         string            // A regular expression (Perl compatible) to filter the list (e.g., `helm list --filter 'myrelease.+`)
	Selector       string            // Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)
	Max            int               // Maximum number of releases to fetch (0 for no limit)
	Offset         int               // Next release index in the list, used to offset from start value
	ByDate         bool              // Sort by release date
	SortReverse    bool              // Sort in reverse order (implies --by-date)
	Deployed       bool              // Show deployed releases. If no other is specified, this will be automatically enabled
	Failed         bool              // Show failed releases
	Pending        bool              // Show pending releases
	Uninstalled    bool              // Show uninstalled releases (if 'helm list --uninstalled')
	Uninstalling   bool              // Show releases that are currently uninstalling
	Sudo           bool
}

type HelmReleaseInfo struct { // Based on `helm list -o json` and `helm status -o json`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
	Revision     string `json:"revision"` // string because it can be large
	Updated      string `json:"updated"`  // Timestamp string e.g. "2023-10-27 10:30:00.123 -0700 MST"
	Status       string `json:"status"`   // e.g. "deployed", "failed", "pending-install"
	Chart        string `json:"chart"`    // Chart name with version e.g. "nginx-1.12.3"
	AppVersion   string `json:"app_version"` // Application version from the chart
	Notes        string `json:"notes,omitempty"` // Only from `helm status`
	Config       map[string]interface{} `json:"config,omitempty"` // User-supplied values, only from `helm status`
	Manifest     string `json:"manifest,omitempty"` // Rendered manifest, only from `helm status` (can be huge)
	Version      int    `json:"version"` // Alias for revision, sometimes present
}


type HelmStatusOptions struct {
	Namespace      string // Namespace of the release
	KubeconfigPath string // Path to kubeconfig file
	Revision       int    // If set, display the status of the named release at a specific revision
	ShowDesc       bool   // If true, display the description given to the release
	Sudo           bool
}

type HelmRepoAddOptions struct {
	Username       string // Chart repository username
	Password       string // Chart repository password
	CAFile         string // Verify certificates of HTTPS-enabled servers using this CA bundle
	CertFile       string // Identify HTTPS client using this SSL certificate file
	KeyFile        string // Identify HTTPS client using this SSL key file
	Insecure       bool   // Skip TLS certificate checks for the repository
	ForceUpdate    bool   // Replace the repository if it already exists
	PassCredentials bool  // Pass credentials to all domains
	Sudo           bool
}

type HelmSearchOptions struct { // For `helm search repo`
	Regexp      bool   // Use regular expressions for searching
	Devel       bool   // Use development versions, too (equivalent to version '>0.0.0-0')
	Version     string // Specify a version constraint for the chart version (e.g. "~1.0.0")
	Versions    bool   // Show all versions of charts (equivalent to --version '>')
	OutputFormat string // Output format: table, json, yaml. Default is table.
	Sudo        bool
}

type HelmChartInfo struct { // Based on `helm search repo -o json`
	Name        string `json:"name"`        // e.g. "stable/nginx-ingress"
	Version     string `json:"version"`     // e.g. "1.41.3"
	AppVersion  string `json:"app_version"` // e.g. "0.30.0"
	Description string `json:"description"`
}

type HelmPullOptions struct {
	Destination    string // Location to write the chart. If this and tardir are specified, tardir is appended to destination
	Prov           bool   // Fetch the provenance file, but don't perform verification
	Untar          bool   // If set to true, pull the chart then untar it in tardir
	UntarDir       string // If untar is specified, this flag specifies the directory to untar the chart after downloading it (default ".")
	Verify         bool   // Verify the package against its signature
	Keyring        string // Keyring containing public keys (default "$HOME/.gnupg/pubring.gpg")
	Version        string // Specify a version constraint for the chart version. If this is not specified, the latest version is downloaded
	CAFile         string // Verify certificates of HTTPS-enabled servers using this CA bundle
	CertFile       string // Identify HTTPS client using this SSL certificate file
	KeyFile        string // Identify HTTPS client using this SSL key file
	Insecure       bool   // Skip TLS certificate checks for the repository
	Devel          bool   // Use development versions too. Equivalent to version '>0.0.0-0'. If --version is set, this is ignored.
	PassCredentials bool  // Pass credentials to all domains
	Username       string // Chart repository username
	Password       string // Chart repository password
	Sudo           bool
}

type HelmPackageOptions struct {
	Destination  string   // Location to write the chart archive (default ".")
	Sign         bool     // Use a GPG key to sign this package
	Key          string   // Name of the GPG key to use when signing
	Keyring      string   // Keyring containing private keys (default "$HOME/.gnupg/secring.gpg")
	PassphraseFile string // Location of a file containing the GPG passphrase
	Version      string   // Set the package version. Overrides the version in Chart.yaml
	AppVersion   string   // Set the appVersion. Overrides the appVersion in Chart.yaml
	DependencyUpdate bool // Update chart dependencies before packaging
	Sudo         bool
}

type HelmVersionInfo struct { // Based on `helm version -o json`
	Version    string `json:"version"`    // e.g. "v3.7.0"
	GitCommit  string `json:"gitCommit"`
	GitTreeState string `json:"gitTreeState"`
	GoVersion  string `json:"goVersion"`
}

// --- Kubectl Supporting Structs ---

type KubectlApplyOptions struct {
	KubeconfigPath string   // Path to kubeconfig file
	Namespace      string   // Namespace for the apply operation
	Force          bool     // Force apply updates
	Prune          bool     // Prune unmanaged resources
	Selector       string   // Selector (label query) to filter on for pruning
	DryRun         string   // "none", "client", or "server". Default "none".
	Validate       bool     // Validate schemas. Default true. (For older kubectl, might be string "true"/"false")
	Filenames      []string // List of filenames, URLs, or '-' for stdin. If using stdin, FileContent must be provided.
	FileContent    string   // Content to apply if Filenames contains '-'.
	Recursive      bool     // If true, process directory recursively.
	Sudo           bool
}

type KubectlGetOptions struct {
	KubeconfigPath string   // Path to kubeconfig file
	Namespace      string   // Namespace for the get operation
	AllNamespaces  bool     // If true, get from all namespaces
	OutputFormat   string   // Output format: json, yaml, wide, name, custom-columns=..., go-template=...
	Selector       string   // Selector (label query) to filter on
	FieldSelector  string   // Selector (field query) to filter on
	Watch          bool     // Watch for changes
	IgnoreNotFound bool     // If true, ignore "not found" errors
	ChunkSize      int64    // Return large lists in chunks rather than all at once. (0 for no chunking)
	LabelColumns   []string // Additional columns to display for wide output
	ShowLabels     bool     // When printing, show all labels as columns
	Sudo           bool
}

type KubectlDescribeOptions struct {
	KubeconfigPath string // Path to kubeconfig file
	Namespace      string // Namespace for the describe operation
	Selector       string // Selector (label query) to filter on
	ShowEvents     bool   // Include events in describe output (default true)
	Sudo           bool
}

type KubectlDeleteOptions struct {
	KubeconfigPath string   // Path to kubeconfig file
	Namespace      string   // Namespace for the delete operation
	Force          bool     // Force deletion ofgrace period 0
	GracePeriod    *int64   // Period of time in seconds given to the resource to terminate gracefully. Ignored if negative.
	Timeout        time.Duration // The length of time to wait before giving up on a delete, zero means determine a timeout from the grace period
	Wait           bool     // If true, wait for resources to be gone before returning. This may be slow.
	Selector       string   // Selector (label query) to filter on
	Filenames      []string // List of filenames, URLs, or '-' for stdin.
	FileContent    string   // Content to delete if Filenames contains '-'.
	Recursive      bool     // If true, process directory recursively.
	IgnoreNotFound bool     // If true, ignore "not found" errors
	Cascade        string   // "true", "false", or "orphan". If true, cascade the deletion of the resources managed by this resource (e.g. Pods created by a ReplicaSet).
	Sudo           bool
}

type KubectlLogOptions struct {
	KubeconfigPath string // Path to kubeconfig file
	Namespace      string // Namespace of the pod
	Container      string // Container name within the pod
	Follow         bool   // Follow the log stream
	Previous       bool   // Print the logs for the previous instance of the container in a pod if it exists
	SinceTime      string // Only return logs newer than a specific date (RFC3339)
	SinceSeconds   *int64 // Only return logs newer than a relative duration like 5s, 2m, or 1h.
	TailLines      *int64 // If set, the number of lines from the end of the logs to show.
	LimitBytes     *int64 // If set, the maximum number of bytes to read from the log.
	Timestamps     bool   // Include timestamps on each line in the log output
	Sudo           bool
}

type KubectlExecOptions struct {
	KubeconfigPath string        // Path to kubeconfig file
	Namespace      string        // Namespace of the pod
	Container      string        // Container name within the pod
	Stdin          bool          // Pass stdin to the container
	TTY            bool          // Allocate a pseudo-TTY
	CommandTimeout time.Duration // Timeout for the exec command itself (not for the process in container unless it's interactive and this runner handles it)
	Sudo           bool
}

type KubectlVersionInfo struct { // Based on `kubectl version -o json`
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
	ServerVersion *struct { // ServerVersion can be null if server is unreachable
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

// KubectlNodeInfo is a simplified struct for `kubectl get nodes -o json` items
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
		PodCIDR           string `json:"podCIDR"`
		ProviderID        string `json:"providerID"`
		Unschedulable     bool   `json:"unschedulable,omitempty"`
		// Taints might be here
	} `json:"spec"`
	Status struct {
		Capacity    map[string]string `json:"capacity"`
		Allocatable map[string]string `json:"allocatable"`
		Conditions  []struct {
			Type               string `json:"type"` // e.g., Ready, MemoryPressure
			Status             string `json:"status"` // True, False, Unknown
			LastHeartbeatTime  string `json:"lastHeartbeatTime"`
			LastTransitionTime string `json:"lastTransitionTime"`
			Reason             string `json:"reason"`
			Message            string `json:"message"`
		} `json:"conditions"`
		Addresses []struct {
			Type    string `json:"type"` // e.g., InternalIP, Hostname
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
		// Images, DaemonEndpoints might be here
	} `json:"status"`
}

// KubectlPodInfo is a simplified struct for `kubectl get pods -o json` items
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
			// Ports, Env, Resources, VolumeMounts etc.
		} `json:"containers"`
		// Volumes, RestartPolicy, TerminationGracePeriodSeconds etc.
	} `json:"spec"`
	Status struct {
		Phase             string `json:"phase"` // Pending, Running, Succeeded, Failed, Unknown
		HostIP            string `json:"hostIP"`
		PodIP             string `json:"podIP"`
		StartTime         string `json:"startTime,omitempty"`
		ContainerStatuses []struct {
			Name        string `json:"name"`
			State       map[string]interface{} `json:"state"` // e.g. {"running":{"startedAt":"..."}} or {"terminated":{"exitCode":0,...}}
			LastState   map[string]interface{} `json:"lastState,omitempty"`
			Ready       bool   `json:"ready"`
			RestartCount int32  `json:"restartCount"`
			Image       string `json:"image"`
			ImageID     string `json:"imageID"`
			ContainerID string `json:"containerID"` // e.g. containerd://<hash>
		} `json:"containerStatuses,omitempty"`
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
			// LastProbeTime, LastTransitionTime
		} `json:"conditions,omitempty"`
		// QOSClass
	} `json:"status"`
}


// KubectlServiceInfo for `kubectl get svc -o json`
// KubectlDeploymentInfo for `kubectl get deploy -o json`
// KubectlRolloutOptions, KubectlScaleOptions, KubectlPortForwardOptions, KubectlConfigViewOptions, KubectlContextInfo, KubectlMetricsInfo, KubectlContainerMetricsInfo
// would also need to be defined based on their respective kubectl command flags and JSON outputs.
// For brevity, these are listed as comments but would be fully fleshed out.


// --- Containerd/ctr Supporting Structs ---

type CtrImageInfo struct {
	Name   string   // Image name (e.g., docker.io/library/alpine:latest)
	Digest string   // Image digest (e.g., sha256:...)
	Size   string   // Human-readable size (e.g., "2.83 MiB")
	OSArch string   // OS/Architecture (e.g., linux/amd64)
	Labels map[string]string
}

type CtrContainerInfo struct {
	ID      string
	Image   string
	Runtime string // e.g., io.containerd.runc.v2
	Status  string // e.g., RUNNING, STOPPED, CREATED - Needs parsing from `ctr c list`
	Labels  map[string]string
}

// ContainerdContainerCreateOptions mirrors relevant fields from `ctr run` or `ctr c create`
type ContainerdContainerCreateOptions struct {
	ImageName     string   // Image to use
	ContainerID   string   // ID for the new container
	Snapshotter   string   // Snapshotter to use (e.g., "overlayfs")
	ConfigPath    string   // Path to OCI spec file (optional, ctr can generate)
	Runtime       string   // Runtime to use (e.g., "io.containerd.runc.v2")
	NetHost       bool     // Use host network
	TTY           bool     // Allocate TTY
	Env           []string // Environment variables "KEY=value"
	Mounts        []string // Mounts in "type=TYPE,src=SRC,dst=DST,options=OPT" format
	Command       []string // Command to run
	Labels        map[string]string
	RemoveExisting bool    // Remove container with same ID if it exists
	Privileged    bool
	ReadOnlyRootFS bool
	User          string // user[:group]
	Cwd           string // Working directory
	Platforms     []string // For multi-platform images, e.g. "linux/amd64"
}

type CtrExecOptions struct {
	TTY  bool
	User string // user[:group]
	Cwd  string
}


// --- Containerd/crictl Supporting Structs ---

type CrictlImageInfo struct {
	ID          string   `json:"id"`
	RepoTags    []string `json:"repoTags"`
	RepoDigests []string `json:"repoDigests"`
	Size        string   `json:"size"` // crictl outputs size as string e.g. "5.57MB"
	UID         *int64   `json:"uid"`  // User ID to run the image as
	Username    string   `json:"username"`
}

type CrictlImageDetails struct { // Based on `crictl inspecti`
	Status struct {
		ID          string   `json:"id"`
		RepoTags    []string `json:"repoTags"`
		RepoDigests []string `json:"repoDigests"`
		Size        string   `json:"size"`
		Username    string   `json:"username"`
		UID         *int64   `json:"uid"`
	} `json:"status"`
	Info map[string]interface{} `json:"info"` // Raw JSON info from image config
}

type CrictlFSInfo struct {
	Timestamp int64 `json:"timestamp"`
	FsID struct {
		Mountpoint string `json:"mountpoint"`
	} `json:"fsId"`
	UsedBytes  string `json:"usedBytes"` // e.g., "1.23GB"
	InodesUsed string `json:"inodesUsed"`
}

type CrictlPodInfo struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Namespace      string            `json:"namespace"`
	Attempt        uint32            `json:"attempt"`
	State          string            `json:"state"` // e.g., "SANDBOX_READY", "SANDBOX_NOTREADY"
	CreatedAt      string            `json:"createdAt"` // Timestamp string
	Labels         map[string]string `json:"labels"`
	Annotations    map[string]string `json:"annotations"`
	RuntimeHandler string            `json:"runtimeHandler"`
}

type CrictlPodDetails struct { // Based on `crictl inspectp`
	Status struct {
		ID             string            `json:"id"`
		Metadata struct {
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
			Attempt   uint32 `json:"attempt"`
			UID       string `json:"uid"`
		} `json:"metadata"`
		State          string            `json:"state"`
		CreatedAt      string            `json:"createdAt"`
		Network struct {
			IP       string `json:"ip"`
			// AdditionalIPs might be present
		} `json:"network"`
		Linux struct {
			Namespaces struct {
				Options struct {
					Network string `json:"network"` // "POD", "NODE"
					Pid     string `json:"pid"`     // "POD", "NODE", "TARGET" (for container)
					Ipc     string `json:"ipc"`     // "POD", "NODE"
				} `json:"options"`
			} `json:"namespaces"`
		} `json:"linux"`
		Labels         map[string]string `json:"labels"`
		Annotations    map[string]string `json:"annotations"`
		RuntimeHandler string            `json:"runtimeHandler"`
	} `json:"status"`
	Info map[string]interface{} `json:"info"` // Raw JSON from runtime
}


type CrictlContainerDetails struct { // Based on `crictl inspect` (for containers)
	Status struct {
		ID       string `json:"id"`
		Metadata struct {
			Name    string `json:"name"`
			Attempt uint32 `json:"attempt"`
		} `json:"metadata"`
		State       string            `json:"state"` // e.g., "CONTAINER_RUNNING", "CONTAINER_EXITED"
		CreatedAt   string            `json:"createdAt"`
		StartedAt   string            `json:"startedAt"`
		FinishedAt  string            `json:"finishedAt"`
		ExitCode    int32             `json:"exitCode"`
		Image struct {
			Image string `json:"image"` // Image name
			ID    string `json:"id"`    // Image ID
		} `json:"image"`
		ImageRef    string            `json:"imageRef"` // Image ID (same as Image.ID usually)
		Reason      string            `json:"reason"`
		Message     string            `json:"message"`
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
		Mounts      []struct {
			ContainerPath  string `json:"containerPath"`
			HostPath       string `json:"hostPath"`
			Readonly       bool   `json:"readonly"`
			Propagation    string `json:"propagation"` // e.g. "PROPAGATION_PRIVATE"
			SelinuxRelabel bool   `json:"selinuxRelabel"`
		} `json:"mounts"`
		LogPath     string `json:"logPath"`
	} `json:"status"`
	Pid  int                    `json:"pid"`
	Info map[string]interface{} `json:"info"` // Raw JSON from runtime
}


type CrictlLogOptions struct { // Based on `crictl logs` flags
	Follow     bool   // -f, --follow
	TailLines  *int64 // -t, --tail (use pointer to distinguish 0 from not set)
	Since      string // --since (duration like 10s, 1m, or RFC3339Nano timestamp)
	Timestamps bool   // --timestamps
	Latest     bool   // --latest (deprecated, use tail)
	NumLines   *int64 // -l, --lines (alternative to tail)
}

type CrictlVersionInfo struct { // Based on `crictl version`
	Version           string
	RuntimeName       string
	RuntimeVersion    string
	RuntimeApiVersion string
}


// --- Docker Supporting Structs ---

// ImageInfo holds basic information about a Docker image.
type ImageInfo struct {
	ID          string   // ID of the image.
	RepoTags    []string // Repository tags associated with the image.
	Created     string   // Timestamp string of image creation time.
	Size        int64    // Size of the image in bytes.
	VirtualSize int64    // Virtual size of the image in bytes (size on disk).
}

// ContainerPortMapping defines port mappings for a container.
type ContainerPortMapping struct {
	HostIP        string // Host IP address to bind to.
	HostPort      string // Host port number.
	ContainerPort string // Container port number.
	Protocol      string // Protocol (e.g., "tcp", "udp").
}

// ContainerMount defines a mount point for a container.
type ContainerMount struct {
	Type        string // Type of mount (e.g., "bind", "volume", "tmpfs").
	Source      string // Path on the host (for bind mounts) or name of the volume.
	Destination string // Path inside the container where the mount is visible.
	Mode        string // Mount mode (e.g., "ro" for read-only, "rw" for read-write).
}

// ContainerCreateOptions encapsulates parameters for creating a Docker container.
type ContainerCreateOptions struct {
	ImageName        string            // Name of the image to use for the container.
	ContainerName    string            // Optional name for the container; Docker generates one if empty.
	Ports            []ContainerPortMapping // Port mappings.
	Volumes          []ContainerMount  // Volume mounts.
	EnvVars          []string          // Environment variables in "VAR=value" format.
	Command          []string          // Command to run in the container (overrides image's CMD).
	Entrypoint       []string          // Entrypoint for the container (overrides image's ENTRYPOINT).
	WorkingDir       string            // Working directory inside the container.
	User             string            // Username or UID to run commands as inside the container.
	RestartPolicy    string            // Restart policy (e.g., "no", "on-failure:3", "always", "unless-stopped").
	NetworkMode      string            // Network mode (e.g., "bridge", "host", "none", "container:<name|id>", "<network_name>").
	ExtraHosts       []string          // Hosts to add to the container's /etc/hosts file ("hostname:ip").
	Labels           map[string]string // Labels to apply to the container.
	Privileged       bool              // If true, run the container in privileged mode.
	CapAdd           []string          // Linux capabilities to add to the container.
	CapDrop          []string          // Linux capabilities to drop from the container.
	Resources        ContainerResources // CPU, Memory limits/reservations.
	HealthCheck      *ContainerHealthCheck // Health check configuration for the container.
	AutoRemove       bool              // If true, automatically remove the container when it exits (--rm flag).
	VolumesFrom      []string          // Mount volumes from the specified container(s).
	SecurityOpt      []string          // Security options (e.g., "apparmor:unconfined", "seccomp=unconfined").
	Sysctls          map[string]string // Kernel parameters (sysctls) to set in the container.
	DNSServers       []string          // Custom DNS servers for the container.
	DNSSearchDomains []string          // Custom DNS search domains for the container.
}

// ContainerResources defines CPU and memory constraints for a container.
type ContainerResources struct {
	CPUShares   int64  // Relative CPU shares (weight).
	Memory      int64  // Memory limit in bytes (0 for unlimited).
	NanoCPUs    int64  // CPU quota in units of 1e-9 CPUs (e.g., 0.5 CPUs = 500,000,000).
	PidsLimit   int64  // PID limit for the container (Linux only). 0 or -1 for unlimited (kernel default may vary).
	BlkioWeight uint16 // Block IO weight (relative weight), range 10 to 1000. 0 to disable.
}

// ContainerHealthCheck defines health check parameters for a container.
type ContainerHealthCheck struct {
	Test        []string      // Command to run to check health (e.g., ["CMD", "curl", "-f", "http://localhost/health"]).
	Interval    time.Duration // Time between running the check.
	Timeout     time.Duration // Maximum time to allow one check to run.
	Retries     int           // Number of consecutive failures needed to consider the container unhealthy.
	StartPeriod time.Duration // Start period for the container to initialize before health checks begin counting retries.
}


// ContainerInfo holds basic information about a Docker container, typically from a list operation.
type ContainerInfo struct {
	ID      string   // ID of the container.
	Names   []string // Names associated with the container.
	Image   string   // Image name used to create the container.
	ImageID string   // ID of the image.
	Command string   // Command being run.
	Created int64    // Unix timestamp of container creation time.
	State   string   // Current state of the container (e.g., "running", "exited", "created").
	Status  string   // Additional status information (e.g., "Up 2 hours", "Exited (0) 5 minutes ago").
	Ports   []ContainerPortMapping // Port mappings active for the container.
	Labels  map[string]string    // Labels applied to the container.
	Mounts  []ContainerMount     // Mounts configured for the container.
}

// ContainerLogOptions specifies how to retrieve logs from a container.
type ContainerLogOptions struct {
	Follow     bool   // If true, stream logs. Note: string return type of GetContainerLogs is not ideal for Follow=true.
	Tail       string // Number of lines to show from the end of the logs (e.g., "all", "100").
	Since      string // Show logs since a specific timestamp (e.g., "2013-01-02T13:23:37Z") or relative duration (e.g., "42m").
	Until      string // Show logs before a specific timestamp or relative duration.
	Timestamps bool   // If true, include timestamps in log output.
	Details    bool   // If true, show extra details provided to logs (rarely used, driver-dependent).
	ShowStdout bool   // If true, retrieve stdout logs. Defaults to false if neither stdout/stderr is true.
	ShowStderr bool   // If true, retrieve stderr logs. Defaults to false if neither stdout/stderr is true.
}

// ContainerStats holds live resource usage statistics for a container.
type ContainerStats struct {
	CPUPercentage    float64 // Calculated CPU usage percentage across all cores.
	MemoryUsageBytes uint64  // Current memory usage in bytes.
	MemoryLimitBytes uint64  // Memory limit for the container in bytes.
	NetworkRxBytes   uint64  // Cumulative network bytes received across all interfaces.
	NetworkTxBytes   uint64  // Cumulative network bytes transmitted across all interfaces.
	BlockReadBytes   uint64  // Cumulative block I/O bytes read from block devices.
	BlockWriteBytes  uint64  // Cumulative block I/O bytes written to block devices.
	PidsCurrent      uint64  // Current number of PIDs (processes/threads) in the container.
	Error            error   // Used to propagate errors from the stats stream itself (e.g., decoding error).
}

// ContainerDetails provides detailed information about a container, typically from an "inspect" operation.
// This is a simplified representation; Docker's inspect output is very rich.
type ContainerDetails struct {
	ID              string    // Full ID of the container.
	Created         string    // Timestamp of container creation.
	Path            string    // Path to the command being run.
	Args            []string  // Arguments to the command.
	State           *ContainerState // Detailed state of the container.
	Image           string    // Image ID (sha256 hash) the container was created from.
	ResolvConfPath  string    // Path to the container's resolv.conf file.
	HostnamePath    string    // Path to the container's hostname file.
	HostsPath       string    // Path to the container's hosts file.
	LogPath         string    // Path to the container's log file (driver-dependent).
	Name            string    // Name of the container.
	RestartCount    int       // Number of times the container has been restarted.
	Driver          string    // Storage driver used for the container.
	Platform        string    // Platform of the container (e.g., "linux").
	MountLabel      string    // Mount label for SELinux.
	ProcessLabel    string    // Process label for SELinux.
	AppArmorProfile string    // AppArmor profile name.
	ExecIDs         []string  // List of exec instance IDs running in the container.
	HostConfig      *HostConfig // Host-specific configuration applied to the container.
	GraphDriver     *GraphDriverData // Information about the storage driver.
	Mounts          []ContainerMount // List of mounts configured for the container (reflects actual runtime mounts).
	Config          *ContainerConfig // Container's base configuration as provided at creation time.
	NetworkSettings *NetworkSettings // Network settings for the container, including IP addresses and connected networks.
}

// ContainerState holds detailed information about a container's state.
type ContainerState struct {
	Status     string // Human-readable status (e.g., "running", "exited", "paused").
	Running    bool   // True if the container is currently running.
	Paused     bool   // True if the container is paused.
	Restarting bool   // True if the container is in the process of restarting.
	OOMKilled  bool   // True if the container was killed by OOM killer.
	Dead       bool   // True if the container is dead (Docker internal state).
	Pid        int    // Process ID of the container's main process on the host.
	ExitCode   int    // Exit code of the container if it has exited.
	Error      string // Error message if the container failed to start.
	StartedAt  string // Timestamp when the container was last started.
	FinishedAt string // Timestamp when the container last finished.
}

// HostConfig is a simplified representation of Docker's HostConfig structure,
// containing host-specific configurations for a container.
type HostConfig struct {
	NetworkMode   string // Network mode for the container.
	RestartPolicy struct { // Restart policy.
		Name              string // Policy name (e.g., "on-failure").
		MaximumRetryCount int    // Max number of retries for "on-failure".
	}
	PortBindings map[string][]ContainerPortMapping // Port bindings. Key: "containerPort/protocol".
	Resources    ContainerResources // Resource constraints.
	Privileged   bool // True if container runs in privileged mode.
	AutoRemove   bool // True if container should be removed on exit.
	// Add other commonly used fields from Docker's HostConfig as needed.
	// e.g., Binds, CapAdd, CapDrop, SecurityOpt, etc.
}

// GraphDriverData holds information about the storage driver used for a container.
type GraphDriverData struct {
	Name string            // Name of the storage driver.
	Data map[string]string // Driver-specific data.
}

// ContainerConfig is a simplified representation of Docker's container configuration,
// as provided at creation time.
type ContainerConfig struct {
	Hostname     string   // Hostname of the container.
	Domainname   string   // Domain name for the container.
	User         string   // User that commands run as inside the container.
	AttachStdin  bool     // True if stdin is attached.
	AttachStdout bool     // True if stdout is attached.
	AttachStderr bool     // True if stderr is attached.
	ExposedPorts map[string]struct{} // Ports exposed by the Dockerfile (e.g. "80/tcp": {}).
	Tty          bool     // True if a TTY is allocated.
	OpenStdin    bool     // True if stdin is kept open even if not attached.
	StdinOnce    bool     // True if stdin is closed after the first write.
	Env          []string // Environment variables.
	Cmd          []string // Command to run.
	Image        string   // Image name specified at create time (not the ID).
	Volumes      map[string]struct{} // Volumes defined in the Dockerfile (e.g. "/var/www": {}).
	WorkingDir   string   // Working directory.
	Entrypoint   []string // Entrypoint.
	Labels       map[string]string // Labels.
	Healthcheck  *ContainerHealthCheck // Healthcheck configuration from Dockerfile or create options.
}

// NetworkSettings holds detailed network configuration and runtime state for a container.
type NetworkSettings struct {
	Bridge                 string   // Name of the bridge interface on the host if using default bridge network.
	SandboxID              string   // ID of the network sandbox.
	HairpinMode            bool     // True if hairpin NAT is enabled.
	LinkLocalIPv6Address   string   // IPv6 link-local address.
	LinkLocalIPv6PrefixLen int      // Prefix length for IPv6 link-local address.
	Ports                  map[string][]ContainerPortMapping // Runtime port mappings.
	SandboxKey             string   // Key for the network sandbox.
	SecondaryIPAddresses   []string // Array of secondary IPv4 addresses.
	SecondaryIPv6Addresses []string // Array of secondary IPv6 addresses.
	EndpointID             string   // ID of the container's endpoint in the default network.
	Gateway                string   // Gateway IP address for the default network.
	GlobalIPv6Address      string   // Global IPv6 address.
	GlobalIPv6PrefixLen    int      // Prefix length for global IPv6 address.
	IPAddress              string   // IPv4 address in the default network.
	IPPrefixLen            int      // Prefix length for IPv4 address.
	IPv6Gateway            string   // IPv6 gateway address.
	MacAddress             string   // MAC address for the default network interface.
	Networks               map[string]*EndpointSettings // Network settings for each network the container is connected to, keyed by network name or ID.
}

// EndpointSettings holds configuration for a container's network endpoint in a specific network.
type EndpointSettings struct {
	IPAMConfig          *EndpointIPAMConfig // IPAM configuration for this endpoint.
	Links               []string // Links to other containers in this network.
	Aliases             []string // Aliases for this container in this network.
	NetworkID           string   // ID of the network.
	EndpointID          string   // ID of this endpoint.
	Gateway             string   // Gateway IP address for this network.
	IPAddress           string   // IPv4 address in this network.
	IPPrefixLen         int      // Prefix length for IPv4 address in this network.
	IPv6Gateway         string   // IPv6 gateway address for this network.
	GlobalIPv6Address   string   // Global IPv6 address in this network.
	GlobalIPv6PrefixLen int      // Prefix length for global IPv6 address.
	MacAddress          string   // MAC address for this endpoint.
	DriverOpts          map[string]string // Driver-specific options for this endpoint.
}

// EndpointIPAMConfig holds IPAM (IP Address Management) configuration for a network endpoint.
type EndpointIPAMConfig struct {
	IPv4Address  string   // Static IPv4 address.
	IPv6Address  string   // Static IPv6 address.
	LinkLocalIPs []string // List of link-local IP addresses.
}


// DockerNetworkInfo holds information about a Docker network.
type DockerNetworkInfo struct {
	ID         string   // ID of the network.
	Name       string   // Name of the network.
	Driver     string   // Driver used for the network (e.g., "bridge", "overlay").
	Scope      string   // Scope of the network (e.g., "local", "swarm", "global").
	EnableIPv6 bool     // True if IPv6 is enabled on this network.
	Subnets    []string // List of subnets in CIDR format associated with this network's IPAM configurations.
	Gateways   []string // List of gateways associated with this network's IPAM configurations.
	Containers map[string]string // Map of container ID to container name for containers connected to this network.
}

// DockerVolumeInfo holds information about a Docker volume.
type DockerVolumeInfo struct {
	Name       string            // Name of the volume.
	Driver     string            // Driver used for the volume (e.g., "local").
	Mountpoint string            // Path on the host where the volume data is stored.
	Labels     map[string]string // Labels applied to the volume.
	Scope      string            // Scope of the volume (e.g., "local", "global").
}

// DockerVolumeDetails provides detailed information about a Docker volume, typically from an "inspect" operation.
type DockerVolumeDetails struct {
	Name       string            // Name of the volume.
	Driver     string            // Driver used for the volume.
	Mountpoint string            // Path on the host where the volume data is stored.
	Status     map[string]string // Driver-specific status information (can be nil).
	Labels     map[string]string // Labels applied to the volume.
	Scope      string            // Scope of the volume.
	Options    map[string]string // Driver options used when the volume was created.
	CreatedAt  string            // Timestamp of when the volume was created.
}

// DockerSystemInfo holds general information about the Docker daemon.
// This is a selection of fields commonly found in the output of `docker info`.
type DockerSystemInfo struct {
	ID                string      // Unique ID of the Docker daemon.
	Containers        int         // Total number of containers managed by the daemon.
	ContainersRunning int         // Number of containers currently running.
	ContainersPaused  int         // Number of containers currently paused.
	ContainersStopped int         // Number of containers currently stopped.
	Images            int         // Total number of images known to the daemon.
	Driver            string      // Storage driver being used (e.g., "overlay2", "aufs").
	DriverStatus      [][2]string // Key-value pairs describing the status of the storage driver.
	Plugins           struct {    // Information about installed plugins.
		Volume  []string // List of volume plugin names.
		Network []string // List of network plugin names.
		// Add other plugin types as needed (e.g., Authorization, Log)
	}
	MemoryLimit       bool   // True if memory limit support is enabled for containers.
	SwapLimit         bool   // True if swap limit support is enabled for containers.
	KernelMemory      bool   // True if kernel memory limit support is enabled (deprecated, use KernelMemoryTCP).
	CPUCfsPeriod      bool   // True if CPU CFS period support is enabled.
	CPUCfsQuota       bool   // True if CPU CFS quota support is enabled.
	CPUShares         bool   // True if CPU shares support is enabled.
	CPUSet            bool   // True if CPU set support (pinning to specific CPUs) is enabled.
	PidsLimit         bool   // True if PIDs limit support for containers is enabled.
	IPv4Forwarding    bool   // True if IPv4 forwarding is enabled on the host.
	BridgeNfIptables  bool   // True if bridge netfilter iptables is enabled (required for Docker networking).
	BridgeNfIp6tables bool   // True if bridge netfilter ip6tables is enabled.
	Debug             bool   // True if the Docker daemon is running in debug mode.
	NFd               int    // Number of file descriptors currently used by the daemon process.
	OomKillDisable    bool   // True if OOM kill disable support is enabled for containers.
	NGoroutines       int    // Number of active goroutines in the daemon process.
	SystemTime        string // Current system time on the daemon host, in RFC3339Nano format.
	LoggingDriver     string // Default logging driver for containers (e.g., "json-file").
	CgroupDriver      string // Cgroup driver used by the daemon (e.g., "cgroupfs", "systemd").
	CgroupVersion     string // Cgroup version in use by the host system (e.g., "1", "2").
	NEventsListener   int    // Number of event listeners registered with the daemon.
	KernelVersion     string // Kernel version of the host operating system.
	OperatingSystem   string // Operating system of the host (e.g., "Docker Desktop", "Ubuntu 20.04.3 LTS").
	OSType            string // OS type (e.g., "linux", "windows").
	Architecture      string // Hardware architecture of the host (e.g., "x86_64", "aarch64").
	IndexServerAddress string // Default registry server address (usually "https://index.docker.io/v1/").
	RegistryConfig    *RegistryConfig // Information about configured Docker registries, including mirrors and insecure registries.
	NCPU              int    // Number of logical CPUs available to the daemon.
	MemTotal          int64  // Total physical memory on the host in bytes.
	ServerVersion     string // Version of the Docker server (daemon).
	// Add more fields as needed from `docker info` (e.g. SecurityOptions, Runtimes, LiveRestoreEnabled).
}

// RegistryConfig mirrors parts of the Docker daemon's registry configuration,
// such as mirrors and insecure registry settings.
type RegistryConfig struct {
	IndexConfigs map[string]*IndexInfo // Configuration for specific registry indexes, keyed by registry hostname (e.g., "docker.io").
	// InsecureRegistryCIDRs, etc. can be added if detailed insecure registry info is needed.
}

// IndexInfo holds information about a specific Docker registry index, including its mirrors and security status.
type IndexInfo struct {
	Name     string   // Name of the registry (e.g., "docker.io").
	Mirrors  []string // List of configured mirror URLs for this registry.
	Secure   bool     // True if the registry is considered secure (HTTPS with valid certificate). Note: This model's `Secure` is inverted from Docker API's `Secure` field.
	Official bool     // True if this is an official Docker registry (e.g., Docker Hub).
}


// VMNetworkInterface defines network interface parameters for a VM
type VMNetworkInterface struct {
	Type       string // e.g., "network", "bridge", "direct"
	Source     string // e.g., "default" (libvirt network), "br0" (bridge name), "eth0" (direct interface)
	Model      string // e.g., "virtio"
	MACAddress string // Optional, specific MAC address
}

// VMInfo holds basic information about a virtual machine
type VMInfo struct {
	Name   string
	State  string // e.g., "running", "shut off", "paused"
	CPUs   int
	Memory uint // in MB
	UUID   string
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
