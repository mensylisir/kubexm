package config

import "time"

// APIVersion and Kind are standard fields for Kubernetes-style configuration objects.
const (
	DefaultAPIVersion = "kubexms.io/v1alpha1"
	ClusterKind     = "Cluster"
	// Add other Kinds if more top-level config objects are envisioned
)

// Cluster is the top-level configuration object, typically parsed from cluster.yaml.
type Cluster struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       ClusterSpec `yaml:"spec"`
}

// Metadata holds common metadata for the cluster.
type Metadata struct {
	Name string `yaml:"name"`
	// Annotations map[string]string `yaml:"annotations,omitempty"`
	// Labels      map[string]string `yaml:"labels,omitempty"`
	// CreationTimestamp time.Time `yaml:"creationTimestamp,omitempty"` // Usually set by system, not user
}

// ClusterSpec defines the desired state of the Kubernetes cluster.
type ClusterSpec struct {
	Global             GlobalSpec             `yaml:"global,omitempty"`
	Hosts              []HostSpec             `yaml:"hosts"` // Must have at least one host
	ContainerRuntime   *ContainerRuntimeSpec  `yaml:"containerRuntime,omitempty"` // Pointer to be optional
	Containerd       *ContainerdSpec       `yaml:"containerd,omitempty"`       // Specific settings if containerd is used
	Etcd               *EtcdSpec             `yaml:"etcd,omitempty"`             // Specific settings for Etcd
	Kubernetes         *KubernetesSpec       `yaml:"kubernetes,omitempty"`
	Network            *NetworkSpec            `yaml:"network,omitempty"`
	HighAvailability   *HighAvailabilitySpec   `yaml:"highAvailability,omitempty"`
	Addons             []AddonSpec            `yaml:"addons,omitempty"`
	// PreflightConfig    *PreflightConfigSpec    `yaml:"preflightConfig,omitempty"` // Specific preflight controls
	// UpgradeConfig      *UpgradeConfigSpec      `yaml:"upgradeConfig,omitempty"`   // For cluster upgrades
}

// GlobalSpec contains settings applicable to the entire cluster or as defaults for hosts.
type GlobalSpec struct {
	User              string        `yaml:"user,omitempty"`              // Default SSH user
	Port              int           `yaml:"port,omitempty"`              // Default SSH port
	Password          string        `yaml:"password,omitempty"`          // Default SSH password (use with caution)
	PrivateKeyPath    string        `yaml:"privateKeyPath,omitempty"`    // Default path to SSH private key
	ConnectionTimeout time.Duration `yaml:"connectionTimeout,omitempty"` // e.g., "30s"
	WorkDir           string        `yaml:"workDir,omitempty"`           // Default remote work directory
	Verbose           bool          `yaml:"verbose,omitempty"`
	IgnoreErr         bool          `yaml:"ignoreErr,omitempty"`
	SkipPreflight     bool          `yaml:"skipPreflight,omitempty"`     // Global flag to skip all preflight checks
	// SudoPassword   string        `yaml:"sudoPassword,omitempty"`   // If sudo needs password (use with extreme caution)
}

// HostSpec defines the configuration for a single host.
type HostSpec struct {
	Name            string            `yaml:"name"`       // Required, unique
	Address         string            `yaml:"address"`    // Required, IP or FQDN for connection
	InternalAddress string            `yaml:"internalAddress,omitempty"` // Internal/private IP
	Port            int               `yaml:"port,omitempty"`            // Overrides GlobalSpec.Port
	User            string            `yaml:"user,omitempty"`            // Overrides GlobalSpec.User
	Password        string            `yaml:"password,omitempty"`        // Host-specific password
	PrivateKey      string            `yaml:"privateKey,omitempty"`      // Base64 encoded private key content
	PrivateKeyPath  string            `yaml:"privateKeyPath,omitempty"`  // Host-specific path, overrides global
	// BastionHost    *BastionSpec      `yaml:"bastion,omitempty"`       // Bastion/jump host config
	Roles           []string          `yaml:"roles,omitempty"`           // e.g., ["master", "etcd", "worker"]
	Labels          map[string]string `yaml:"labels,omitempty"`          // Custom labels
	Taints          []TaintSpec       `yaml:"taints,omitempty"`          // Kubernetes taints for the node
	Type            string            `yaml:"type,omitempty"`            // "ssh" (default) or "local"
	WorkDir         string            `yaml:"workDir,omitempty"`         // Host-specific, overrides global
}

// TaintSpec defines a Kubernetes node taint.
type TaintSpec struct {
	Key    string `yaml:"key"`
	Value  string `yaml:"value"`
	Effect string `yaml:"effect"` // NoSchedule, PreferNoSchedule, NoExecute
}

// BastionSpec defines connection details for a bastion/jump host. (Placeholder from 1.md)
// type BastionSpec struct {
// 	Address        string `yaml:"address"`
// 	Port           int    `yaml:"port,omitempty"`
// 	User           string `yaml:"user"`
// 	Password       string `yaml:"password,omitempty"`
// 	PrivateKeyPath string `yaml:"privateKeyPath,omitempty"`
// }

// ContainerRuntimeSpec defines settings for the container runtime.
type ContainerRuntimeSpec struct {
	Type    string `yaml:"type,omitempty"`    // "containerd" (default), "docker" (not fully supported by modern K8s)
	Version string `yaml:"version,omitempty"` // e.g., "1.6.9"
}

// ContainerdSpec defines specific settings for containerd.
// This would be used if ContainerRuntimeSpec.Type is "containerd".
type ContainerdSpec struct {
	// Version is often tied to ContainerRuntimeSpec.Version, but can be specific if needed.
	// ConfigFilePath string `yaml:"configFilePath,omitempty"` // Path to config.toml
	RegistryMirrors    map[string][]string `yaml:"registryMirrors,omitempty"`   // Maps registry to list of mirror endpoints
	InsecureRegistries []string            `yaml:"insecureRegistries,omitempty"` // List of insecure registries (e.g. "myregistry.corp:5000")
	UseSystemdCgroup   bool                `yaml:"useSystemdCgroup,omitempty"`  // Default: true for K8s
	ExtraTomlConfig    string              `yaml:"extraTomlConfig,omitempty"`   // Raw TOML string to append/merge
}

// EtcdSpec defines settings for the etcd cluster.
type EtcdSpec struct {
	Type    string   `yaml:"type,omitempty"`    // "stacked" (on masters, default), "external"
	Version string   `yaml:"version,omitempty"` // e.g., "v3.5.9"
	Nodes   []string `yaml:"nodes,omitempty"`   // List of hostnames/IPs for etcd nodes, if type is "external" or to specify members.
	// DataDir string   `yaml:"dataDir,omitempty"` // Default: /var/lib/etcd or similar
	// External *ExternalEtcdSpec `yaml:"external,omitempty"` // Config if type is "external"
	// Backup   *EtcdBackupSpec   `yaml:"backup,omitempty"`
	// ExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
}

// ExternalEtcdSpec defines connection details for an external etcd cluster.
// type ExternalEtcdSpec struct {
// 	Endpoints []string `yaml:"endpoints"`
// 	CaFile    string   `yaml:"caFile"`
// 	CertFile  string   `yaml:"certFile"`
// 	KeyFile   string   `yaml:"keyFile"`
// }

// KubernetesSpec defines settings for Kubernetes components.
type KubernetesSpec struct {
	Version          string            `yaml:"version"` // e.g., "v1.25.3"
	ClusterName      string            `yaml:"clusterName,omitempty"` // Default: Metadata.Name
	APIServer        *APIServerSpec     `yaml:"apiServer,omitempty"` // Pointer to be optional
	ControllerManager *CMKSpec          `yaml:"controllerManager,omitempty"`
	Scheduler        *SchedulerSpec     `yaml:"scheduler,omitempty"`
	Kubelet          *KubeletSpec       `yaml:"kubelet,omitempty"`
	KubeProxy        *KubeProxySpec     `yaml:"kubeProxy,omitempty"`
	PodSubnet        string            `yaml:"podSubnet,omitempty"`     // e.g., "10.244.0.0/16"
	ServiceSubnet    string            `yaml:"serviceSubnet,omitempty"` // e.g., "10.96.0.0/12"
	FeatureGates     map[string]bool   `yaml:"featureGates,omitempty"`
}

// APIServerSpec for kube-apiserver settings.
type APIServerSpec struct {
	// CertSANs []string `yaml:"certSANs,omitempty"` // Extra SANs for API server cert
	// ExtraArgs map[string]string `yaml:"extraArgs,omitempty"`
	// AdmissionPlugins []string `yaml:"admissionPlugins,omitempty"`
}
// CMKSpec for kube-controller-manager settings.
type CMKSpec struct { /* ExtraArgs, etc. */ }
// SchedulerSpec for kube-scheduler settings.
type SchedulerSpec struct { /* ExtraArgs, etc. */ }
// KubeletSpec for kubelet settings.
type KubeletSpec struct { /* ExtraArgs, cgroupDriver (auto-detect from containerd), etc. */ }
// KubeProxySpec for kube-proxy settings.
type KubeProxySpec struct { /* Mode (iptables/ipvs), ExtraArgs, etc. */ }


// NetworkSpec defines settings for the CNI network plugin.
type NetworkSpec struct {
	Plugin  string `yaml:"plugin,omitempty"` // "calico", "flannel", "cilium", etc.
	Version string `yaml:"version,omitempty"`
	// Calico   *CalicoSpec   `yaml:"calico,omitempty"`
	// Flannel  *FlannelSpec  `yaml:"flannel,omitempty"`
	// Cilium   *CiliumSpec   `yaml:"cilium,omitempty"`
}

// HighAvailabilitySpec defines settings for HA (e.g., VIP, load balancers).
type HighAvailabilitySpec struct {
	Type string `yaml:"type,omitempty"` // "keepalived", "externalL4" etc.
	// VIP  string `yaml:"vip,omitempty"`
	// Keepalived *KeepalivedSpec `yaml:"keepalived,omitempty"`
}

// AddonSpec defines an addon to be deployed.
type AddonSpec struct {
	Name    string                 `yaml:"name"` // e.g., "coredns", "metrics-server"
	Enabled bool                   `yaml:"enabled,omitempty"` // Default true if listed
	// Version string                 `yaml:"version,omitempty"`
	// Config  map[string]interface{} `yaml:"config,omitempty"` // Addon-specific config
}

// TODO: Define further nested specs like CalicoSpec, ExternalEtcdSpec, KeepalivedSpec, etc.
// as per the details in markdown files or common requirements. For now, they are commented out
// to keep this initial full definition manageable, but their place in the hierarchy is shown.
// The "no placeholder functionality" applies to the fields directly used by currently
// implemented logic (runtime, factories). These detailed sub-specs are for future expansion.
