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
}

// PreflightConfigSpec holds configuration for preflight checks.
type PreflightConfigSpec struct {
	MinCPUCores   int    `yaml:"minCPUCores,omitempty"`
	MinMemoryMB   uint64 `yaml:"minMemoryMB,omitempty"`
	// DisableSwap being true means the DisableSwapStep should run.
	// If false, the step is skipped. If omitted, it's Go's default false.
	// The task factory can decide its default behavior if this section is omitted.
	DisableSwap   bool   `yaml:"disableSwap,omitempty"`
}

// KernelConfigSpec holds configuration for kernel module loading and sysctl parameters.
type KernelConfigSpec struct {
	Modules      []string          `yaml:"modules,omitempty"`      // Kernel modules to load
	SysctlParams map[string]string `yaml:"sysctlParams,omitempty"` // Sysctl parameters to set
	// SysctlConfigFilePath string `yaml:"sysctlConfigFilePath,omitempty"` // Path for sysctl config file
}

// ClusterSpec defines the desired state of the Kubernetes cluster.
type ClusterSpec struct {
	Global             GlobalSpec             `yaml:"global,omitempty"`
	Hosts              []HostSpec             `yaml:"hosts"`
	ContainerRuntime   *ContainerRuntimeSpec  `yaml:"containerRuntime,omitempty"` // Pointer
	Containerd       *ContainerdSpec       `yaml:"containerd,omitempty"`       // Pointer
	Etcd             *EtcdSpec             `yaml:"etcd,omitempty"`             // Pointer
	Kubernetes         KubernetesSpec         `yaml:"kubernetes,omitempty"` // Kept as struct, as K8s config is fundamental
	Network            NetworkSpec            `yaml:"network,omitempty"`    // Kept as struct
	HighAvailability   HighAvailabilitySpec   `yaml:"highAvailability,omitempty"` // Kept as struct
	Addons             []AddonSpec            `yaml:"addons,omitempty"`
	PreflightConfig    PreflightConfigSpec    `yaml:"preflight,omitempty"`    // Struct, not pointer
	KernelConfig       KernelConfigSpec       `yaml:"kernel,omitempty"`       // Struct, not pointer
}

// GlobalSpec contains settings applicable to the entire cluster or as defaults for hosts.
type GlobalSpec struct {
	User string `yaml:"user,omitempty"`; Port int `yaml:"port,omitempty"`; Password string `yaml:"password,omitempty"`;
	PrivateKeyPath string `yaml:"privateKeyPath,omitempty"`; ConnectionTimeout time.Duration `yaml:"connectionTimeout,omitempty"`;
	WorkDir string `yaml:"workDir,omitempty"`; Verbose bool `yaml:"verbose,omitempty"`; IgnoreErr bool `yaml:"ignoreErr,omitempty"`;
	SkipPreflight bool `yaml:"skipPreflight,omitempty"`
}
// HostSpec defines the configuration for a single host.
type HostSpec struct {
	Name string `yaml:"name"`; Address string `yaml:"address"`; InternalAddress string `yaml:"internalAddress,omitempty"`;
	Port int `yaml:"port,omitempty"`; User string `yaml:"user,omitempty"`; Password string `yaml:"password,omitempty"`;
	PrivateKey string `yaml:"privateKey,omitempty"`; PrivateKeyPath string `yaml:"privateKeyPath,omitempty"`;
	Roles []string `yaml:"roles,omitempty"`; Labels map[string]string `yaml:"labels,omitempty"`;
	Taints []TaintSpec `yaml:"taints,omitempty"`; Type string `yaml:"type,omitempty"`; WorkDir string `yaml:"workDir,omitempty"`
}
// TaintSpec defines a Kubernetes node taint.
type TaintSpec struct { Key string `yaml:"key"`; Value string `yaml:"value"`; Effect string `yaml:"effect"`}

// ContainerRuntimeSpec defines settings for the container runtime.
type ContainerRuntimeSpec struct {
	Type string `yaml:"type,omitempty"`; Version string `yaml:"version,omitempty"`
}
// ContainerdSpec defines specific settings for containerd.
type ContainerdSpec struct {
	Version string `yaml:"version,omitempty"`;
	RegistryMirrors map[string][]string `yaml:"registryMirrors,omitempty"` // Renamed from RegistryMirrorsConfig
	InsecureRegistries []string `yaml:"insecureRegistries,omitempty"`;
	// UseSystemdCgroup being bool means if omitted in YAML, it's false.
	// If we want "true unless explicitly false", it should be *bool or handled in SetDefaults/factory.
	UseSystemdCgroup bool `yaml:"useSystemdCgroup,omitempty"`;
	ExtraTomlConfig string `yaml:"extraTomlConfig,omitempty"`
	ConfigPath string `yaml:"configPath,omitempty"` // Added from previous full config
}
// EtcdSpec defines specific settings for etcd.
type EtcdSpec struct {
	Type string `yaml:"type,omitempty"`; Version string `yaml:"version,omitempty"`;
	Managed bool `yaml:"managed,omitempty"`; Nodes []string `yaml:"nodes,omitempty"`
}
// KubernetesSpec defines settings for Kubernetes components.
type KubernetesSpec struct {
	Version string `yaml:"version"`; ClusterName string `yaml:"clusterName,omitempty"`;
	APIServer APIServerSpec `yaml:"apiServer,omitempty"`; // Kept as struct, init by SetDefaults if nil
	ControllerManager CMKSpec `yaml:"controllerManager,omitempty"`;
	Scheduler SchedulerSpec `yaml:"scheduler,omitempty"`; Kubelet KubeletSpec `yaml:"kubelet,omitempty"`;
	KubeProxy KubeProxySpec `yaml:"kubeProxy,omitempty"`; PodSubnet string `yaml:"podSubnet,omitempty"`;
	ServiceSubnet string `yaml:"serviceSubnet,omitempty"`; FeatureGates map[string]bool `yaml:"featureGates,omitempty"`
}
// Sub-specs for Kubernetes components, kept as structs. SetDefaults will init if parent KubernetesSpec is not nil.
type APIServerSpec struct { /* Fields */ }
type CMKSpec struct { /* Fields */ }
type SchedulerSpec struct { /* Fields */ }
type KubeletSpec struct { /* Fields */ }
type KubeProxySpec struct { /* Fields */ }
// NetworkSpec defines settings for the CNI network plugin.
type NetworkSpec struct { Plugin string `yaml:"plugin,omitempty"`; Version string `yaml:"version,omitempty"`}
// HighAvailabilitySpec defines settings for HA.
type HighAvailabilitySpec struct { Type string `yaml:"type,omitempty"`}
// AddonSpec defines an addon to be deployed.
type AddonSpec struct { Name string `yaml:"name"`; Enabled bool `yaml:"enabled,omitempty"`}
