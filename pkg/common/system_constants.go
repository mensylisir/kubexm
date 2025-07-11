package common

// System configuration constants
const (
	// Default system paths
	DefaultSystemdPath      = "/etc/systemd/system"           // Default systemd service directory
	DefaultSystemdDropinPath = "/etc/systemd/system/%s.service.d" // Default systemd dropin directory
	DefaultBinPath          = "/usr/local/bin"                // Default binary installation path
	DefaultConfigPath       = "/etc/kubexm"                   // Default system configuration path
	DefaultCniPath          = "/opt/cni/bin"                  // Default CNI plugin path
	DefaultCniConfigPath    = "/etc/cni/net.d"                // Default CNI configuration path
	DefaultKubeletPath      = "/var/lib/kubelet"              // Default kubelet data directory
	DefaultEtcdPath         = "/var/lib/etcd"                 // Default etcd data directory
	DefaultContainerdPath   = "/var/lib/containerd"           // Default containerd data directory
	DefaultDockerPath       = "/var/lib/docker"               // Default docker data directory
	DefaultKubeConfigPath   = "/etc/kubernetes"               // Default kubeconfig directory
	DefaultPKIPath          = "/etc/kubernetes/pki"           // Default PKI directory
	DefaultManifestPath     = "/etc/kubernetes/manifests"     // Default static manifest directory
	DefaultLogPath          = "/var/log/kubexm"               // Default log directory
	
	// Default configuration files
	DefaultKubeConfigFile   = "admin.conf"                    // Default kubeconfig file name
	DefaultKubeletConfig    = "kubelet.conf"                  // Default kubelet config file name
	DefaultEtcdConfig       = "etcd.conf"                     // Default etcd config file name
	DefaultContainerdConfig = "config.toml"                   // Default containerd config file name
	DefaultDockerConfig     = "daemon.json"                   // Default docker daemon config file name
	DefaultCniConfig        = "10-kubexm.conf"                // Default CNI config file name
	DefaultHAProxyConfig    = "haproxy.cfg"                   // Default HAProxy config file name
	DefaultNginxConfig      = "nginx.conf"                    // Default Nginx config file name
	DefaultKeepalivedConfig = "keepalived.conf"               // Default Keepalived config file name
)

// Security and permission constants
const (
	// File permissions (following Kubernetes security best practices)
	DefaultFilePermission        = 0644  // Default file permission
	DefaultDirPermission         = 0755  // Default directory permission
	DefaultSecretFilePermission  = 0600  // Default secret file permission
	DefaultSecretDirPermission   = 0700  // Default secret directory permission
	DefaultExecutablePermission  = 0755  // Default executable permission
	DefaultConfigFilePermission  = 0644  // Default config file permission
	DefaultPrivateKeyPermission  = 0600  // Default private key permission
	DefaultCertificatePermission = 0644  // Default certificate permission
	
	// Default users and groups
	DefaultKubeletUser       = "root"     // Default kubelet user
	DefaultEtcdUser          = "root"     // Default etcd user
	DefaultContainerdUser    = "root"     // Default containerd user
	DefaultDockerUser        = "root"     // Default docker user
	DefaultKubeUser          = "root"     // Default kubernetes user
	DefaultSystemUser        = "root"     // Default system user
	
	// Default timeouts and intervals
	DefaultConnectionTimeout = 30         // Default connection timeout in seconds
	DefaultReadTimeout      = 30          // Default read timeout in seconds
	DefaultWriteTimeout     = 30          // Default write timeout in seconds
	DefaultRetryCount       = 3           // Default retry count
	DefaultRetryInterval    = 5           // Default retry interval in seconds
	DefaultHealthCheckInterval = 30       // Default health check interval in seconds
	DefaultBackupInterval   = 24          // Default backup interval in hours
)

// Resource limits and requirements constants
const (
	// Default resource requirements
	DefaultMinMemoryMB      = 2048        // Default minimum memory in MB
	DefaultMinCPUCores      = 2           // Default minimum CPU cores
	DefaultMinDiskGB        = 20          // Default minimum disk space in GB
	DefaultMaxPods          = 110         // Default maximum pods per node
	DefaultMaxPodPidsLimit  = 4096        // Default maximum pids per pod
	DefaultMaxOpenFiles     = 1000000     // Default maximum open files
	DefaultMaxMapCount      = 262144      // Default vm.max_map_count
	DefaultMaxUserInstances = 8192        // Default systemd user instances
	
	// Default cluster sizing
	DefaultMaxNodes         = 5000        // Default maximum nodes in cluster
	DefaultMaxNamespaces    = 1000        // Default maximum namespaces
	DefaultMaxServices      = 5000        // Default maximum services
	DefaultMaxIngresses     = 1000        // Default maximum ingresses
	DefaultMaxPVs           = 1000        // Default maximum persistent volumes
	DefaultMaxPVCs          = 1000        // Default maximum persistent volume claims
)

// Architecture and platform constants
const (
	// Supported architectures
	DefaultArch             = "amd64"     // Default architecture
	ArchAMD64              = "amd64"      // AMD64 architecture
	ArchARM64              = "arm64"      // ARM64 architecture
	ArchPPC64LE            = "ppc64le"    // PowerPC 64-bit little-endian
	ArchS390X              = "s390x"      // IBM System/390 architecture
	
	// Supported operating systems
	DefaultOS              = "linux"      // Default operating system
	OSLinux                = "linux"      // Linux operating system
	OSDarwin               = "darwin"     // macOS operating system
	OSWindows              = "windows"    // Windows operating system
	
	// Supported distributions
	DistroUbuntu           = "ubuntu"     // Ubuntu distribution
	DistroCentOS           = "centos"     // CentOS distribution
	DistroRHEL             = "rhel"       // Red Hat Enterprise Linux
	DistroDebian           = "debian"     // Debian distribution
	DistroFedora           = "fedora"     // Fedora distribution
	DistroSUSE             = "suse"       // openSUSE distribution
	DistroPhoton           = "photon"     // VMware Photon OS
	DistroFlatcar          = "flatcar"    // Flatcar Linux
	DistroAmazonLinux      = "amzn"       // Amazon Linux
)