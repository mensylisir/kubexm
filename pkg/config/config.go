package config

import (
	"time" // Still needed for GlobalSpec if it had time.Duration and was kept, but GlobalSpec will be from v1alpha1
	// Import v1alpha1 types
	"{{MODULE_NAME}}/pkg/apis/kubexms/v1alpha1"
	// metav1 is needed if config.Metadata is converted to metav1.ObjectMeta directly here
	// For now, config.Metadata is simple.
)

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

// ClusterSpec defines the desired state of the Kubernetes cluster, primarily using v1alpha1 types.
// YAML tags are added here if the YAML field name differs from the v1alpha1 struct field name (which uses JSON tags).
// For most fields, we assume the YAML parser (yaml.v3) can use the JSON tags from v1alpha1 types.
type ClusterSpec struct {
	RoleGroups           *v1alpha1.RoleGroupsSpec          `yaml:"roleGroups,omitempty"`
	ControlPlaneEndpoint *v1alpha1.ControlPlaneEndpointSpec `yaml:"controlPlaneEndpoint,omitempty"`
	System               *v1alpha1.SystemSpec              `yaml:"system,omitempty"`

	Global             *v1alpha1.GlobalSpec             `yaml:"global,omitempty"`
	Hosts              []v1alpha1.HostSpec              `yaml:"hosts"` // Assuming HostSpec in YAML is identical to v1alpha1.HostSpec
	ContainerRuntime   *v1alpha1.ContainerRuntimeConfig `yaml:"containerRuntime,omitempty"`
	Containerd         *v1alpha1.ContainerdConfig       `yaml:"containerd,omitempty"`
	Etcd               *v1alpha1.EtcdConfig             `yaml:"etcd,omitempty"`
	Kubernetes         *v1alpha1.KubernetesConfig       `yaml:"kubernetes,omitempty"`
	Network            *v1alpha1.NetworkConfig          `yaml:"network,omitempty"`
	Storage            *v1alpha1.StorageConfig          `yaml:"storage,omitempty"`
	Registry           *v1alpha1.RegistryConfig         `yaml:"registry,omitempty"`
	HighAvailability   *v1alpha1.HighAvailabilityConfig `yaml:"highAvailability,omitempty"`
	// Addons field in v1alpha1.ClusterSpec is []v1alpha1.AddonConfig
	// Assuming AddonSpec in config.go was a placeholder for this.
	Addons             []v1alpha1.AddonConfig           `yaml:"addons,omitempty"`
	// PreflightConfig in v1alpha1.ClusterSpec is *v1alpha1.PreflightConfig
	Preflight          *v1alpha1.PreflightConfig        `yaml:"preflight,omitempty"`
	// KernelConfig in v1alpha1.ClusterSpec is *v1alpha1.KernelConfig
	Kernel             *v1alpha1.KernelConfig           `yaml:"kernel,omitempty"`
	// OS field in v1alpha1.ClusterSpec is *v1alpha1.OSConfig
	OS                 *v1alpha1.OSConfig               `yaml:"os,omitempty"`
}

// Note: Definitions for RoleGroupsSpec, ControlPlaneEndpointSpec, SystemSpec,
// KubernetesConfigHolder, EtcdConfigHolder, NetworkConfigHolder, StorageConfigHolder, RegistryConfigHolder,
// GlobalSpec, HostSpec, TaintSpec, ContainerRuntimeSpec, ContainerdSpec, PreflightConfigSpec, KernelConfigSpec,
// HighAvailabilityConfigSpec, AddonSpec, and their sub-structs (like MasterRoleSpec, etc.)
// are removed as their roles are now taken by direct usage of v1alpha1 types or they are no longer needed.
