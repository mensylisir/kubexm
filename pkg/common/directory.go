package common

const (
	DefaultWorkDir         = "/tmp/kubexm"
	DefaultRemoteWorkDir   = "/tmp/kubexm"
	DefaultLocalWorkDir    = ".kubexm"
	DefaultLogsDir         = "logs"
	DefaultConfigDir       = "configs"
	DefaultCacheDir        = "cache"
	DefaultCertificatesDir = "certificates"
	DefaultCertsDir        = "certs"
	DefaultBinariesDir     = "binaries"
	DefaultTemplatesDir    = "templates"
	DefaultManifestsDir    = "manifests"
	DefaultBackupDir       = "backup"
	DefaultArtifactsDir    = "artifacts"
	DefaultScriptsDir      = "scripts"
	DefaultTmpDir          = "tmp"

	// Temporary directory names
	DefaultTmpDirName     = ".kubexm_tmp"
	DefaultWorkDirName    = ".kubexm"
	KubexmRootDirName     = ".kubexm"
	KUBEXM                = ".kubexm"
	DefaultLogDirName     = "logs"
	DefaultBinDirName     = "bin"
	DefaultConfDirName    = "conf"
	DefaultScriptsDirName = "scripts"
	DefaultBackupDirName  = "backup"
)

const (
	DefaultContainerRuntimeDir = "container_runtime"
	DefaultKubernetesDir       = "kubernetes"
	DefaultEtcdDir             = "etcd"
	DefaultCNIDir              = "cni"
	DefaultLoadBalancerDir     = "loadbalancer"
	DefaultStorageDir          = "storage"
	DefaultMonitoringDir       = "monitoring"
	DefaultNetworkingDir       = "networking"

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
	DefaultInstallPrefix = "/usr/local"
	DefaultBinDir        = DefaultInstallPrefix + "/bin"
	DefaultEtcDir        = DefaultInstallPrefix + "/etc"
	DefaultLibDir        = DefaultInstallPrefix + "/lib"
	DefaultShareDir      = DefaultInstallPrefix + "/share"
	DefaultVarDir        = "/var"
	DefaultOptDir        = "/opt"
	DefaultSbinDir       = "/usr/local/sbin"
	DefaultCNIBin        = "/opt/cni/bin"

	// System configuration directories
	DefaultSystemEtcDir  = "/etc"
	DefaultSystemVarDir  = "/var"
	DefaultSystemOptDir  = "/opt"
	DefaultSystemUsrDir  = "/usr"
	DefaultSystemLibDir  = "/lib"
	DefaultSystemBinDir  = "/bin"
	DefaultSystemSbinDir = "/sbin"

	// Runtime directories
	DefaultRuntimeDir           = "/run"
	DefaultSystemdRuntimeDir    = "/run/systemd"
	DefaultContainerdRuntimeDir = "/run/containerd"
	DefaultDockerRuntimeDir     = "/var/run/docker"
	DefaultKubeletRuntimeDir    = "/var/lib/kubelet"
	DefaultEtcdRuntimeDir       = "/var/lib/etcd"
)

// Log directory constants
const (
	// System log directories
	DefaultSystemLogDir   = "/var/log"          // System log directory
	DefaultJournalDir     = "/var/log/journal"  // Systemd journal directory
	DefaultAuditLogDir    = "/var/log/audit"    // Audit log directory
	DefaultKernelLogDir   = "/var/log/kernel"   // Kernel log directory
	DefaultSecurityLogDir = "/var/log/security" // Security log directory

	// Application log directories
	DefaultKubexmLogDir     = "/var/log/kubexm"     // Kubexm log directory
	DefaultKubernetesLogDir = "/var/log/kubernetes" // Kubernetes log directory
	DefaultEtcdLogDir       = "/var/log/etcd"       // Etcd log directory
	DefaultContainerdLogDir = "/var/log/containerd" // Containerd log directory
	DefaultDockerLogDir     = "/var/log/docker"     // Docker log directory
	DefaultHAProxyLogDir    = "/var/log/haproxy"    // HAProxy log directory
	DefaultNginxLogDir      = "/var/log/nginx"      // Nginx log directory
	DefaultKeepalivedLogDir = "/var/log/keepalived" // Keepalived log directory

	// Pod log directories
	DefaultPodLogDir       = "/var/log/pods"       // Pod log directory
	DefaultContainerLogDir = "/var/log/containers" // Container log directory
	DefaultAuditPolicyDir  = "/etc/kubernetes/audit"
	DefaultAuditPolicyFile = "/etc/kubernetes/audit/audit-policy.yaml" // Audit policy directory
	DefaultAuditLogFile    = "/var/log/audit/audit.log"                // Audit log file
)

// Cache and temporary directory constants
const (
	// Cache directories
	DefaultSystemCacheDir     = "/var/cache"            // System cache directory
	DefaultUserCacheDir       = "/root/.cache"          // User cache directory
	DefaultKubexmCacheDir     = "/var/cache/kubexm"     // Kubexm cache directory
	DefaultHelmCacheDir       = "/root/.cache/helm"     // Helm cache directory
	DefaultDockerCacheDir     = "/var/cache/docker"     // Docker cache directory
	DefaultKubeletCacheDir    = "/var/cache/kubelet"    // Kubelet cache directory
	DefaultContainerdCacheDir = "/var/cache/containerd" // Containerd cache directory

	// Temporary directories
	DefaultSystemTmpDir   = "/tmp"                  // System temporary directory
	DefaultUserTmpDir     = "/tmp"                  // User temporary directory
	DefaultKubexmTmpDir   = "/tmp/kubexm"           // Kubexm temporary directory
	DefaultBuildTmpDir    = "/tmp/kubexm/build"     // Build temporary directory
	DefaultDownloadTmpDir = "/tmp/kubexm/downloads" // Download temporary directory
	DefaultExtractTmpDir  = "/tmp/kubexm/extract"   // Extract temporary directory
	DefaultUploadTmpDir   = "/tmp/kubexm/upload"    // Upload temporary directory
)
