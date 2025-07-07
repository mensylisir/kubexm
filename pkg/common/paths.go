package common

const (
	// --- General Default Directories (some might be relative to KUBEXM work dir) ---
	// KubexmRootDirName is the default root directory name for kubexm operations.
	KubexmRootDirName = ".kubexm"
	// DefaultLogDirName is the default directory name for logs within the KubexmRootDirName.
	DefaultLogDirName = "logs"
	// DefaultCertsDir is the default directory name for certificates.
	DefaultCertsDir = "certs"
	// DefaultContainerRuntimeDir is the default directory for container runtime artifacts.
	DefaultContainerRuntimeDir = "container_runtime"
	// DefaultKubernetesDir is the default directory for kubernetes artifacts.
	DefaultKubernetesDir = "kubernetes"
	// DefaultEtcdDir is the default directory for etcd artifacts.
	DefaultEtcdDir = "etcd"

	// --- Kubexm Work Directories (typically under KUBEXM/.<clustername>/) ---
	// DefaultBinDir is for downloaded binaries.
	DefaultBinDir = "bin"
	// DefaultConfDir is for generated configuration files.
	DefaultConfDir = "conf"
	// DefaultScriptsDir is for temporary scripts.
	DefaultScriptsDir = "scripts"
	// DefaultBackupDir is for backup files.
	DefaultBackupDir = "backup"
)

// --- Kubernetes System Directories (Standard paths on nodes) ---
const (
	KubeletHomeDir          = "/var/lib/kubelet"
	KubernetesConfigDir     = "/etc/kubernetes"
	KubernetesManifestsDir  = "/etc/kubernetes/manifests"
	KubernetesPKIDir        = "/etc/kubernetes/pki"
	DefaultKubeconfigPath   = "/root/.kube/config" // Path for admin kubeconfig on control node/user machine

	// Kubernetes PKI file names (standard kubeadm layout within KubernetesPKIDir)
	CACertFileName                  = "ca.crt"
	CAKeyFileName                   = "ca.key"
	APIServerCertFileName           = "apiserver.crt"
	APIServerKeyFileName            = "apiserver.key"
	APIServerKubeletClientCertFileName = "apiserver-kubelet-client.crt"
	APIServerKubeletClientKeyFileName  = "apiserver-kubelet-client.key"
	FrontProxyCACertFileName        = "front-proxy-ca.crt"
	FrontProxyCAKeyFileName         = "front-proxy-ca.key"
	FrontProxyClientCertFileName    = "front-proxy-client.crt"
	FrontProxyClientKeyFileName     = "front-proxy-client.key"
	ServiceAccountPublicKeyFileName  = "sa.pub"
	ServiceAccountPrivateKeyFileName = "sa.key"
	APIServerEtcdClientCertFileName = "apiserver-etcd-client.crt" // Etcd specific PKI (if managed by kubeadm)
	APIServerEtcdClientKeyFileName  = "apiserver-etcd-client.key"

	// Kubernetes Config Files (standard kubeadm layout within KubernetesConfigDir)
	KubeadmConfigFileName       = "kubeadm-config.yaml"
	KubeletConfigFileName       = "kubelet.conf"
	KubeletSystemdEnvFileName   = "10-kubeadm.conf"      // Kubelet systemd drop-in
	ControllerManagerConfigFileName = "controller-manager.conf"
	SchedulerConfigFileName     = "scheduler.conf"
	AdminConfigFileName         = "admin.conf" // Kubeconfig for cluster admin

	// Static Pod manifests (within KubernetesManifestsDir)
	KubeAPIServerStaticPodFileName       = "kube-apiserver.yaml"
	KubeControllerManagerStaticPodFileName = "kube-controller-manager.yaml"
	KubeSchedulerStaticPodFileName      = "kube-scheduler.yaml"
	EtcdStaticPodFileName               = "etcd.yaml" // If kubeadm manages etcd
)

// --- Etcd System Directories & Files (Standard paths on nodes for binary installs) ---
const (
	EtcdDefaultDataDir      = "/var/lib/etcd"       // Etcd default data directory
	EtcdDefaultWalDir       = "/var/lib/etcd/wal"   // Etcd default WAL directory
	EtcdDefaultConfDir      = "/etc/etcd"           // Etcd configuration directory
	EtcdDefaultPKIDir       = "/etc/etcd/pki"       // Etcd PKI directory (for binary installs)
	EtcdDefaultBinDir       = "/usr/local/bin"      // Etcd binaries default install directory (shared with other bins)
	EtcdDefaultSystemdFile  = "/etc/systemd/system/etcd.service" // Etcd systemd service file
	EtcdDefaultConfFile     = "/etc/etcd/etcd.conf.yml" // Example etcd config file name (can vary)

	// Etcd PKI file names (for binary installs, within EtcdDefaultPKIDir)
	EtcdServerCert          = "server.crt"
	EtcdServerKey           = "server.key"
	EtcdPeerCert            = "peer.crt"
	EtcdPeerKey             = "peer.key"
	// EtcdCaCert is often the same as Kubernetes CA or a dedicated Etcd CA
	// EtcdCaCert              = "ca.crt"
	// EtcdCaKey               = "ca.key"
)

// --- Container Runtime Directories & Files (Standard paths on nodes) ---
const (
	// Containerd
	ContainerdDefaultConfDir      = "/etc/containerd"
	ContainerdDefaultConfigFile   = "/etc/containerd/config.toml"
	ContainerdDefaultSocketPath   = "/run/containerd/containerd.sock"
	ContainerdDefaultSystemdFile  = "/etc/systemd/system/containerd.service"
	// Runc (often placed in a common bin dir like /usr/local/bin)

	// Docker
	DockerDefaultConfDir      = "/etc/docker"
	DockerDefaultConfigFile   = "/etc/docker/daemon.json"
	DockerDefaultDataRoot     = "/var/lib/docker"
	DockerDefaultSocketPath   = "/var/run/docker.sock"
	DockerDefaultSystemdFile  = "/lib/systemd/system/docker.service" // Path can vary by distro

	// CRI-Dockerd (for Docker with recent Kubernetes)
	CniDockerdSocketPath      = "/var/run/cri-dockerd.sock"
	CniDockerdSystemdFile     = "/etc/systemd/system/cri-dockerd.service"
)

// --- High Availability (HA) Component Paths ---
const (
	// Keepalived
	KeepalivedDefaultConfDir      = "/etc/keepalived"
	KeepalivedDefaultConfigFile   = "/etc/keepalived/keepalived.conf"
	KeepalivedDefaultSystemdFile  = "/etc/systemd/system/keepalived.service"

	// HAProxy
	HAProxyDefaultConfDir       = "/etc/haproxy"
	HAProxyDefaultConfigFile    = "/etc/haproxy/haproxy.cfg"
	HAProxyDefaultSystemdFile   = "/etc/systemd/system/haproxy.service"

	// Kube-VIP
	KubeVIPManifestFileName = "kube-vip.yaml" // Kube-VIP static pod manifest
)

// --- System Configuration Paths ---
const (
	SysctlDefaultConfFile     = "/etc/sysctl.conf"
	ModulesLoadDefaultDir     = "/etc/modules-load.d"
	// KubernetesSysctlConfFile is a common pattern for K8s sysctl settings.
	KubernetesSysctlConfFile  = "/etc/sysctl.d/99-kubernetes-cri.conf"
	KubeletSystemdDropinDir   = "/etc/systemd/system/kubelet.service.d"
)

// --- CNI Related Paths ---
const (
	DefaultCNIConfDir = "/etc/cni/net.d"
	DefaultCNIBinDir  = "/opt/cni/bin"
)

// --- Helm Related Paths ---
const (
	DefaultHelmHome  = "/root/.helm" // Or user specific path
	DefaultHelmCache = "/root/.cache/helm" // Or user specific path
)
