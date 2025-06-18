package config

import "time"

// This package is a placeholder to allow other packages to compile.
// It will be properly implemented as a separate module later, likely with YAML/JSON parsing.

// Metadata holds common metadata for configuration objects.
type Metadata struct {
	Name string `yaml:"name,omitempty"` // Example: cluster name
	// TODO: Add other common fields like UID, CreationTimestamp, Annotations, Labels etc.
}

// Cluster holds the overall cluster configuration.
// It's the root object when parsing a cluster configuration file.
type Cluster struct {
	APIVersion string   `yaml:"apiVersion,omitempty"` // Example: "kubexms.io/v1alpha1"
	Kind       string   `yaml:"kind,omitempty"`       // Example: "Cluster"
	Metadata   Metadata `yaml:"metadata,omitempty"`   // Cluster metadata
	Spec       ClusterSpec `yaml:"spec,omitempty"`
}

// ClusterSpec defines the specification of the cluster.
type ClusterSpec struct {
	Hosts            []HostSpec            `yaml:"hosts,omitempty"`
	Global           GlobalSpec            `yaml:"global,omitempty"`
	ContainerRuntime *ContainerRuntimeSpec `yaml:"containerRuntime,omitempty"` // For selecting runtime type (containerd, docker)
	Containerd       *ContainerdSpec       `yaml:"containerd,omitempty"`       // Specific settings if containerd is used
	Etcd             *EtcdSpec             `yaml:"etcd,omitempty"`             // Specific settings for Etcd
	// TODO: Add other cluster-wide configurations:
	// Kubernetes *KubernetesSpec `yaml:"kubernetes,omitempty"`
	// Network    *NetworkSpec    `yaml:"network,omitempty"` // For CNI plugin choice and config
	// Addons     *AddonsSpec     `yaml:"addons,omitempty"`
}

// GlobalSpec contains global configurations applicable to the cluster deployment.
// These can often be overridden by host-specific settings in HostSpec.
type GlobalSpec struct {
	ConnectionTimeout time.Duration `yaml:"connectionTimeout,omitempty"` // Example: "30s"
	WorkDir           string        `yaml:"workDir,omitempty"`           // Default work directory on remote hosts
	Verbose           bool          `yaml:"verbose,omitempty"`           // Global verbosity setting
	IgnoreErr         bool          `yaml:"ignoreErr,omitempty"`         // Global ignore error setting
	User              string        `yaml:"user,omitempty"`              // Default SSH user if not set per host
	Port              int           `yaml:"port,omitempty"`              // Default SSH port if not set per host
	PrivateKeyPath    string        `yaml:"privateKeyPath,omitempty"`    // Default private key path if not set per host
	// SkipPreflight  bool        `yaml:"skipPreflight,omitempty"`   // Example: if preflight module needs a global skip flag
}

// HostSpec defines the configuration for a single host in the cluster.
type HostSpec struct {
	Name            string            `yaml:"name"` // Mandatory
	Address         string            `yaml:"address"` // Mandatory
	InternalAddress string            `yaml:"internalAddress,omitempty"`
	Port            int               `yaml:"port,omitempty"`            // Overrides GlobalSpec.Port
	User            string            `yaml:"user,omitempty"`            // Overrides GlobalSpec.User
	Password        string            `yaml:"password,omitempty"`        // For password-based SSH
	PrivateKey      []byte            `yaml:"-"`                         // Raw private key content (usually not directly in YAML, loaded from path)
	PrivateKeyPath  string            `yaml:"privateKeyPath,omitempty"`  // Overrides GlobalSpec.PrivateKeyPath
	Roles           []string          `yaml:"roles,omitempty"`           // List of roles for the host (e.g., "master", "worker", "etcd")
	Labels          map[string]string `yaml:"labels,omitempty"`          // Custom labels for flexible grouping
	Type            string            `yaml:"type,omitempty"`            // Connection type: "ssh", "local". Defaults to "ssh".
	WorkDir         string            `yaml:"workDir,omitempty"`         // Host-specific work directory, overrides GlobalSpec.WorkDir
	// TODO: Add host-specific overrides for other global settings if needed (e.g. SudoPassword)
}

// ContainerRuntimeSpec defines settings for the container runtime.
type ContainerRuntimeSpec struct {
	Type string `yaml:"type,omitempty"` // "containerd", "docker", etc. Default could be "containerd".
	// Version string `yaml:"version,omitempty"` // Common version, or use specific type's version
	// Socket string `yaml:"socket,omitempty"` // e.g., /run/containerd/containerd.sock
	// TODO: Add other common settings like sandbox image (pause image)
}

// ContainerdSpec defines specific settings for the containerd runtime.
// This would be used if ContainerRuntimeSpec.Type is "containerd".
type ContainerdSpec struct {
	Version string `yaml:"version,omitempty"`
	// RegistryMirrorsConfig holds registry mirror configurations.
	// Key: Registry domain (e.g., "docker.io"). Value: List of mirror URLs.
	RegistryMirrorsConfig map[string][]string `yaml:"registryMirrors,omitempty"`
	// UseSystemdCgroup can be a pointer to distinguish between unset (use default) vs explicitly set.
	// For simplicity, using bool here; factory logic can interpret zero-value vs. explicit.
	UseSystemdCgroup   bool     `yaml:"useSystemdCgroup,omitempty"` // Defaults to true in step/task if not specified
	InsecureRegistries []string `yaml:"insecureRegistries,omitempty"`
	ExtraTomlConfig    string   `yaml:"extraTomlConfig,omitempty"`    // Arbitrary additional TOML to append/merge
	ConfigPath         string   `yaml:"configPath,omitempty"`         // Override default /etc/containerd/config.toml
}

// EtcdSpec defines specific settings for etcd.
type EtcdSpec struct {
    Managed bool     `yaml:"managed,omitempty"` // If kubexms should manage etcd deployment (vs. external)
    Type    string   `yaml:"type,omitempty"`    // "stacked" (on masters) or "external"
    Version string   `yaml:"version,omitempty"` // e.g., "v3.5.9"
    Nodes   []string `yaml:"nodes,omitempty"`   // List of hostnames/IPs for etcd nodes.
                                               // Used for initial cluster string, and to determine join logic.
    // TODO: Add more etcd configurations:
    // DataDir string `yaml:"dataDir,omitempty"`
    // InitialClusterToken string `yaml:"initialClusterToken,omitempty"`
    // CertSANs []string `yaml:"certSANs,omitempty"` // Extra SANs for etcd certs
    // ExternalEtcd *ExternalEtcdSpec `yaml:"external,omitempty"` // If type is "external"
}

// TODO: Define KubernetesSpec, NetworkSpec (CalicoSpec, FlannelSpec etc.), AddonsSpec.
// Example:
// type KubernetesSpec struct {
//     Version string `yaml:"version"` // e.g., "v1.25.3"
//     APIServer APIServerSpec `yaml:"apiServer"`
//     KubeProxy KubeProxySpec `yaml:"kubeProxy"`
//     // ... other components ...
// }
// type NetworkSpec struct {
//     Plugin string `yaml:"plugin"` // "calico", "flannel", "cilium"
//     PodCIDR string `yaml:"podCidr"`
//     ServiceCIDR string `yaml:"serviceCidr"`
//     // PluginSpecificConfig map[string]interface{} `yaml:",inline"`
// }
