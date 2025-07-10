package common

// This file defines constants related to file system paths used within Kubexm.

const (
	// --- General Default Directory Names for Kubexm Local Workstation (machine running kubexm) ---
	// These define the structure within the main Kubexm work directory (e.g., $(pwd)/.kubexm/${cluster_name}/).

	// KubexmRootDirName is the top-level directory created by kubexm in the current working directory (e.g., ".kubexm")
	// if no global work_dir is specified in config. The full path would be like $(pwd)/.kubexm.
	KubexmRootDirName = ".kubexm"

	// DefaultLogDirName is the subdirectory for log files within a specific cluster's work directory.
	// e.g., $(pwd)/.kubexm/${cluster_name}/logs
	DefaultLogDirName = "logs"

	// DefaultCertsDir is the subdirectory for generated certificates within a specific cluster's work directory
	// on the machine running Kubexm. This is where Kubexm will store CA certs and other PKI assets
	// it generates locally before distribution.
	// e.g., $(pwd)/.kubexm/${cluster_name}/certs
	DefaultCertsDir = "certs"

	// DefaultArtifactsDir is a general subdirectory for downloaded artifacts within a specific cluster's work directory
	// on the machine running Kubexm. Specific components will have subdirectories under this.
	// e.g., $(pwd)/.kubexm/${cluster_name}/artifacts
	DefaultArtifactsDir = "artifacts"

	// DefaultBinDirName is the subdirectory within a component's artifact path (under DefaultArtifactsDir) for binaries.
	// e.g., $(pwd)/.kubexm/${cluster_name}/artifacts/etcd/${etcd_version}/${arch}/bin
	DefaultBinDirName = "bin"
	// DefaultConfDirName is the subdirectory for generated configuration files locally before upload, if needed,
	// within a component's artifact path.
	// e.g., $(pwd)/.kubexm/${cluster_name}/artifacts/etcd/${etcd_version}/${arch}/conf
	DefaultConfDirName = "conf"
	// DefaultScriptsDirName is for temporary scripts generated locally before upload, if needed,
	// within a component's artifact path.
	DefaultScriptsDirName = "scripts"
	// DefaultBackupDirName is for backup files, potentially within a cluster's work directory or a global backup path.
	// e.g., $(pwd)/.kubexm/${cluster_name}/backup
	DefaultBackupDirName = "backup"

	// Component-specific artifact parent directories under DefaultArtifactsDir and a component type directory.
	// These names should ideally match the BinaryType strings from types.go for consistency in path construction.
	// Example path structure: ${GlobalWorkDir}/${ClusterName}/artifacts/${ComponentTypeDir}/${ComponentName}/${Version}/${Arch}
	// These are names for directories on the Kubexm local machine where artifacts are staged.
	DefaultContainerRuntimeDir = "container_runtime" // Parent dir for different runtimes like docker, containerd
	DefaultKubernetesDir       = "kubernetes"        // Parent dir for different K8s components like kubelet, kubeadm
	DefaultEtcdDir             = "etcd"              // Parent dir for etcd artifacts

	ArtifactsEtcdDir             = "etcd"
	ArtifactsKubeDir             = "kube"
	ArtifactsCNIDir              = "cni"
	ArtifactsHelmDir             = "helm"
	ArtifactsDockerDir           = "docker"
	ArtifactsContainerdDir       = "containerd"
	ArtifactsRuncDir             = "runc"
	ArtifactsCrictlDir           = "crictl"
	ArtifactsCriDockerdDir       = "cri-dockerd"
	ArtifactsCalicoctlDir        = "calicoctl"
	ArtifactsRegistryDir         = "registry"
	ArtifactsComposeDir          = "compose"
	ArtifactsBuildDir            = "build"
	ArtifactsGenericBinariesDir  = "generic"
)

// --- Standard Paths on Target Nodes ---
// These constants define well-known paths on the actual cluster nodes (masters, workers, etcd nodes).
const (
	KubeletHomeDir         = "/var/lib/kubelet"          // Default directory for Kubelet data on a node.
	KubernetesConfigDir    = "/etc/kubernetes"           // Main directory for Kubernetes configuration files on a node.
	KubernetesManifestsDir = "/etc/kubernetes/manifests" // Directory for static Pod manifests on control-plane nodes.
	KubernetesPKIDir       = "/etc/kubernetes/pki"       // Directory for Kubernetes PKI assets on a node.
	DefaultKubeconfigPath  = "/root/.kube/config"        // Standard path for the admin kubeconfig on a user's machine or control node.

	CAKeyFileName                      = "ca.key"                         // Cluster CA private key.
	APIServerCertFileName              = "apiserver.crt"                  // API server certificate.
	APIServerKeyFileName               = "apiserver.key"                  // API server private key.
	APIServerKubeletClientCertFileName = "apiserver-kubelet-client.crt" // Cert for API server to connect to Kubelets.
	APIServerKubeletClientKeyFileName  = "apiserver-kubelet-client.key"  // Key for API server to connect to Kubelets.
	FrontProxyCACertFileName           = "front-proxy-ca.crt"           // Front proxy CA certificate.
	FrontProxyCAKeyFileName            = "front-proxy-ca.key"           // Front proxy CA private key.
	FrontProxyClientCertFileName       = "front-proxy-client.crt"       // Front proxy client certificate.
	FrontProxyClientKeyFileName        = "front-proxy-client.key"       // Front proxy client private key.
	ServiceAccountPublicKeyFileName    = "sa.pub"                       // Service account public key.
	ServiceAccountPrivateKeyFileName   = "sa.key"                       // Service account private key.
	APIServerEtcdClientCertFileName    = "apiserver-etcd-client.crt"    // Cert for API server to connect to Etcd.
	APIServerEtcdClientKeyFileName     = "apiserver-etcd-client.key"     // Key for API server to connect to Etcd.

	KubeadmConfigFileName               = "kubeadm-config.yaml"     // Kubeadm configuration file.
	KubeletKubeconfigFileName           = "kubelet.conf"            // Kubelet's kubeconfig file name.
	KubeletSystemdEnvFileName           = "10-kubeadm.conf"         // Kubelet systemd drop-in environment file name.
	ControllerManagerKubeconfigFileName = "controller-manager.conf" // Controller Manager's kubeconfig file name.
	SchedulerKubeconfigFileName         = "scheduler.conf"          // Scheduler's kubeconfig file name.
	AdminKubeconfigFileName             = "admin.conf"              // Kubeconfig for cluster admin.
	KubeProxyKubeconfigFileName         = "kube-proxy.conf"         // Kube-proxy's kubeconfig file name.

	KubeAPIServerStaticPodFileName       = "kube-apiserver.yaml"
	KubeControllerManagerStaticPodFileName = "kube-controller-manager.yaml"
	KubeSchedulerStaticPodFileName      = "kube-scheduler.yaml"
	EtcdStaticPodFileName               = "etcd.yaml"
)

const (
	EtcdDefaultWalDir         = "/var/lib/etcd/wal" // Etcd default WAL directory.
	EtcdDefaultConfDirTarget  = "/etc/etcd"         // Target Etcd configuration directory on nodes.
	EtcdDefaultPKIDirTarget   = "/etc/etcd/pki"     // Target Etcd PKI directory on nodes for binary installs.
	EtcdEnvFileTarget         = "/etc/etcd.env"     // Target path for etcd environment file for binary installs.
)

const (
	ContainerdDefaultConfDirTarget     = "/etc/containerd"                         // Target Containerd configuration directory on nodes.
	ContainerdDefaultConfigFileTarget  = "/etc/containerd/config.toml"             // Target Containerd main config file on nodes.
	ContainerdDefaultSystemdFile       = "/etc/systemd/system/containerd.service"  // Default systemd file path for Containerd.

	DockerDefaultConfDirTarget     = "/etc/docker"                         // Target Docker configuration directory on nodes.
	DockerDefaultConfigFileTarget  = "/etc/docker/daemon.json"             // Target Docker daemon config file on nodes.
	DockerDefaultSystemdFile       = "/lib/systemd/system/docker.service"  // Default systemd file path for Docker (can be distro-specific).

	CniDockerdSystemdFile     = "/etc/systemd/system/cri-dockerd.service" // Default systemd file path for CRI-Dockerd.
)

const (
	KeepalivedDefaultConfDirTarget     = "/etc/keepalived"                         // Target Keepalived configuration directory on nodes.
	KeepalivedDefaultConfigFileTarget  = "/etc/keepalived/keepalived.conf"       // Target Keepalived main config file on nodes.
	KeepalivedDefaultSystemdFile       = "/etc/systemd/system/keepalived.service"  // Default systemd file path for Keepalived.

	HAProxyDefaultConfDirTarget      = "/etc/haproxy"                          // Target HAProxy configuration directory on nodes.
	HAProxyDefaultConfigFileTarget   = "/etc/haproxy/haproxy.cfg"              // Target HAProxy main config file on nodes.
	HAProxyDefaultSystemdFile        = "/etc/systemd/system/haproxy.service"   // Default systemd file path for HAProxy.

	KubeVIPManifestFileName = "kube-vip.yaml" // Kube-VIP static pod manifest file name.
)

const (
	SysctlDefaultConfFileTarget    = "/etc/sysctl.conf"                      // Target sysctl main configuration file.
	ModulesLoadDefaultDirTarget    = "/etc/modules-load.d"                 // Target directory for kernel modules to load on boot.
	KubernetesSysctlConfFileTarget = "/etc/sysctl.d/99-kubernetes-cri.conf"  // Common target path for Kubernetes-specific sysctl settings.
	KubeletSystemdDropinDirTarget  = "/etc/systemd/system/kubelet.service.d" // Target directory for Kubelet systemd drop-in files.
)

const (
	DefaultCNIConfDirTarget = "/etc/cni/net.d" // Target directory for CNI configuration files.
	DefaultCNIBinDirTarget  = "/opt/cni/bin"   // Target directory for CNI plugin binaries.
)

const (
	DefaultHelmHome  = "/root/.helm"       // Default Helm home directory.
	DefaultHelmCache = "/root/.cache/helm" // Default Helm cache directory.
)

const (
	KubeletKubeconfigPathTarget          = "/etc/kubernetes/kubelet.conf"
	KubeletBootstrapKubeconfigPathTarget = "/etc/kubernetes/bootstrap-kubelet.conf"
	KubeletConfigYAMLPathTarget          = "/var/lib/kubelet/config.yaml"
	KubeletFlagsEnvPathTarget            = "/var/lib/kubelet/kubeadm-flags.env"
	KubeletPKIDirTarget                  = "/var/lib/kubelet/pki"
)
