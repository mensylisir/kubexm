package common

// Working directory constants
const (
	DefaultWorkDir          = "/tmp/kubexm"    // Default working directory
	DefaultRemoteWorkDir    = "/tmp/kubexm"    // Default remote working directory
	DefaultLocalWorkDir     = ".kubexm"        // Default local working directory
	DefaultLogsDir          = "logs"           // Default logs directory
	DefaultConfigDir        = "configs"        // Default configuration directory
	DefaultCacheDir         = "cache"          // Default cache directory
	DefaultCertificatesDir  = "certificates"   // Default certificates directory
	DefaultBinariesDir      = "binaries"       // Default binaries directory
	DefaultTemplatesDir     = "templates"      // Default templates directory
	DefaultManifestsDir     = "manifests"      // Default manifests directory
	DefaultBackupDir        = "backup"         // Default backup directory
	DefaultArtifactsDir     = "artifacts"      // Default artifacts directory
	DefaultScriptsDir       = "scripts"        // Default scripts directory
	DefaultTmpDir           = "tmp"            // Default temporary directory
	
	// Temporary directory names
	DefaultTmpDirName       = ".kubexm_tmp"    // Default temporary directory name
	DefaultWorkDirName      = ".kubexm"        // Default work directory name (for backward compatibility)
	KubexmRootDirName      = ".kubexm"        // Root directory name (unified with DefaultLocalWorkDir)
)

// Component-specific artifact directories
const (
	// Artifact parent directories
	DefaultContainerRuntimeDir = "container_runtime" // Parent dir for different runtimes
	DefaultKubernetesDir       = "kubernetes"        // Parent dir for K8s components
	DefaultEtcdDir             = "etcd"              // Parent dir for etcd artifacts
	DefaultCNIDir              = "cni"               // Parent dir for CNI plugins
	DefaultLoadBalancerDir     = "loadbalancer"      // Parent dir for load balancers
	DefaultStorageDir          = "storage"           // Parent dir for storage components
	DefaultMonitoringDir       = "monitoring"        // Parent dir for monitoring components
	DefaultNetworkingDir       = "networking"        // Parent dir for networking components
	
	// Specific artifact directories
	ArtifactsEtcdDir            = "etcd"
	ArtifactsKubeDir            = "kube"
	ArtifactsCNIDir             = "cni"
	ArtifactsHelmDir            = "helm"
	ArtifactsDockerDir          = "docker"
	ArtifactsContainerdDir      = "containerd"
	ArtifactsRuncDir            = "runc"
	ArtifactsCrictlDir          = "crictl"
	ArtifactsCriDockerdDir      = "cri-dockerd"
	ArtifactsCalicoctlDir       = "calicoctl"
	ArtifactsRegistryDir        = "registry"
	ArtifactsComposeDir         = "compose"
	ArtifactsBuildDir           = "build"
	ArtifactsGenericBinariesDir = "generic"
	ArtifactsHAProxyDir         = "haproxy"
	ArtifactsNginxDir           = "nginx"
	ArtifactsKeepalivedDir      = "keepalived"
	ArtifactsKubeVIPDir         = "kube-vip"
	ArtifactsPrometheusDir      = "prometheus"
	ArtifactsGrafanaDir         = "grafana"
	ArtifactsFluentdDir         = "fluentd"
	ArtifactsElasticsearchDir   = "elasticsearch"
	ArtifactsKibanaDir          = "kibana"
)

// System directory constants
const (
	// Standard system directories
	DefaultInstallPrefix     = "/usr/local"                    // Default installation prefix
	DefaultBinDir           = DefaultInstallPrefix + "/bin"    // Default binary directory
	DefaultEtcDir           = DefaultInstallPrefix + "/etc"    // Default configuration directory
	DefaultLibDir           = DefaultInstallPrefix + "/lib"    // Default library directory
	DefaultShareDir         = DefaultInstallPrefix + "/share"  // Default share directory
	DefaultVarDir           = "/var"                           // Default variable data directory
	DefaultOptDir           = "/opt"                           // Default optional software directory
	DefaultSbinDir          = "/usr/local/sbin"               // Default system binary directory
	
	// System configuration directories
	DefaultSystemEtcDir     = "/etc"                          // System configuration directory
	DefaultSystemVarDir     = "/var"                          // System variable data directory
	DefaultSystemOptDir     = "/opt"                          // System optional software directory
	DefaultSystemUsrDir     = "/usr"                          // System user directory
	DefaultSystemLibDir     = "/lib"                          // System library directory
	DefaultSystemBinDir     = "/bin"                          // System binary directory
	DefaultSystemSbinDir    = "/sbin"                         // System system binary directory
	
	// Runtime directories
	DefaultRuntimeDir       = "/run"                          // Default runtime directory
	DefaultSystemdRuntimeDir = "/run/systemd"                 // Systemd runtime directory
	DefaultContainerdRuntimeDir = "/run/containerd"           // Containerd runtime directory
	DefaultDockerRuntimeDir = "/var/run/docker"               // Docker runtime directory
	DefaultKubeletRuntimeDir = "/var/lib/kubelet"             // Kubelet runtime directory
	DefaultEtcdRuntimeDir   = "/var/lib/etcd"                 // Etcd runtime directory
)

// Log directory constants
const (
	// System log directories
	DefaultSystemLogDir     = "/var/log"                      // System log directory
	DefaultJournalDir       = "/var/log/journal"              // Systemd journal directory
	DefaultAuditLogDir      = "/var/log/audit"                // Audit log directory
	DefaultKernelLogDir     = "/var/log/kernel"               // Kernel log directory
	DefaultSecurityLogDir   = "/var/log/security"             // Security log directory
	
	// Application log directories
	DefaultKubexmLogDir     = "/var/log/kubexm"               // Kubexm log directory
	DefaultKubernetesLogDir = "/var/log/kubernetes"           // Kubernetes log directory
	DefaultEtcdLogDir       = "/var/log/etcd"                 // Etcd log directory
	DefaultContainerdLogDir = "/var/log/containerd"           // Containerd log directory
	DefaultDockerLogDir     = "/var/log/docker"               // Docker log directory
	DefaultHAProxyLogDir    = "/var/log/haproxy"              // HAProxy log directory
	DefaultNginxLogDir      = "/var/log/nginx"                // Nginx log directory
	DefaultKeepalivedLogDir = "/var/log/keepalived"           // Keepalived log directory
	
	// Pod log directories
	DefaultPodLogDir        = "/var/log/pods"                 // Pod log directory
	DefaultContainerLogDir  = "/var/log/containers"           // Container log directory
	DefaultAuditPolicyDir   = "/etc/kubernetes/audit"         // Audit policy directory
	DefaultAuditLogFile     = "/var/log/audit/audit.log"      // Audit log file
)

// Cache and temporary directory constants
const (
	// Cache directories
	DefaultSystemCacheDir   = "/var/cache"                    // System cache directory
	DefaultUserCacheDir     = "/root/.cache"                  // User cache directory
	DefaultKubexmCacheDir   = "/var/cache/kubexm"             // Kubexm cache directory
	DefaultHelmCacheDir     = "/root/.cache/helm"             // Helm cache directory
	DefaultDockerCacheDir   = "/var/cache/docker"             // Docker cache directory
	DefaultKubeletCacheDir  = "/var/cache/kubelet"            // Kubelet cache directory
	DefaultContainerdCacheDir = "/var/cache/containerd"       // Containerd cache directory
	
	// Temporary directories
	DefaultSystemTmpDir     = "/tmp"                          // System temporary directory
	DefaultUserTmpDir       = "/tmp"                          // User temporary directory
	DefaultKubexmTmpDir     = "/tmp/kubexm"                   // Kubexm temporary directory
	DefaultBuildTmpDir      = "/tmp/build"                    // Build temporary directory
	DefaultDownloadTmpDir   = "/tmp/downloads"                // Download temporary directory
	DefaultExtractTmpDir    = "/tmp/extract"                  // Extract temporary directory
	DefaultUploadTmpDir     = "/tmp/upload"                   // Upload temporary directory
)