package v1alpha1

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"net"
	"regexp"
	"strings"
	"time"
)

const (
	// ClusterTypeKubeXM indicates a cluster where core components (kube-apiserver,
	// kube-controller-manager, kube-scheduler, kube-proxy) are deployed as binaries.
	ClusterTypeKubeXM = "kubexm"

	// ClusterTypeKubeadm indicates a cluster where core components are deployed as static Pods
	// managed by kubeadm.
	ClusterTypeKubeadm = "kubeadm"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=clusters,scope=Namespaced,shortName=kc
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.kubernetes.version",description="Kubernetes Version"
// +kubebuilder:printcolumn:name="Hosts",type="integer",JSONPath=".spec.hostsCount",description="Number of hosts"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Cluster is the top-level configuration object.
type Cluster struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec              ClusterSpec `json:"spec,omitempty" yaml:"spec,omitempty"`
}

// ClusterSpec defines the desired state of the Kubernetes cluster.
type ClusterSpec struct {
	Type                 string                    `json:"type,omitempty" yaml:"type,omitempty"` // "kubexm" or "kubeadm"
	RoleGroups           *RoleGroupsSpec           `json:"roleGroups,omitempty" yaml:"roleGroups,omitempty"`
	ControlPlaneEndpoint *ControlPlaneEndpointSpec `json:"controlPlaneEndpoint,omitempty" yaml:"controlPlaneEndpoint,omitempty"`
	System               *SystemSpec               `json:"system,omitempty" yaml:"system,omitempty"`
	Global               *GlobalSpec               `json:"global,omitempty" yaml:"global,omitempty"`
	Hosts                []HostSpec                `json:"hosts" yaml:"hosts"`
	ContainerRuntime     *ContainerRuntimeConfig   `json:"containerRuntime,omitempty" yaml:"containerRuntime,omitempty"`
	Etcd                 *EtcdConfig               `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	Kubernetes           *KubernetesConfig         `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
	Network              *NetworkConfig            `json:"network,omitempty" yaml:"network,omitempty"`
	HighAvailability     *HighAvailabilityConfig   `json:"highAvailability,omitempty" yaml:"highAvailability,omitempty"`
	Preflight            *PreflightConfig          `json:"preflight,omitempty" yaml:"preflight,omitempty"`
	Kernel               *KernelConfig             `json:"kernel,omitempty" yaml:"kernel,omitempty"`
	Storage              *StorageConfig            `json:"storage,omitempty" yaml:"storage,omitempty"`
	Registry             *RegistryConfig           `json:"registry,omitempty" yaml:"registry,omitempty"`
	OS                   *OSConfig                 `json:"os,omitempty" yaml:"os,omitempty"`
	Addons               []AddonConfig             `json:"addons,omitempty" yaml:"addons,omitempty"`
}

// HostSpec defines the configuration for a single host.
type HostSpec struct {
	Name            string            `json:"name" yaml:"name"`
	Address         string            `json:"address" yaml:"address"`
	InternalAddress string            `json:"internalAddress,omitempty" yaml:"internalAddress,omitempty"`
	Port            int               `json:"port,omitempty" yaml:"port,omitempty"`
	User            string            `json:"user,omitempty" yaml:"user,omitempty"`
	Password        string            `json:"password,omitempty" yaml:"password,omitempty"`
	PrivateKey      string            `json:"privateKey,omitempty" yaml:"privateKey,omitempty"`
	PrivateKeyPath  string            `json:"privateKeyPath,omitempty" yaml:"privateKeyPath,omitempty"`
	Roles           []string          `json:"roles,omitempty" yaml:"roles,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Taints          []TaintSpec       `json:"taints,omitempty" yaml:"taints,omitempty"`
	Type            string            `json:"type,omitempty" yaml:"type,omitempty"`
	Arch            string            `json:"arch,omitempty" yaml:"arch,omitempty"`
}
// RoleGroupsSpec defines the different groups of nodes in the cluster.
type RoleGroupsSpec struct {
	Master       MasterRoleSpec       `json:"master,omitempty" yaml:"master,omitempty"`
	Worker       WorkerRoleSpec       `json:"worker,omitempty" yaml:"worker,omitempty"`
	Etcd         EtcdRoleSpec         `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	LoadBalancer LoadBalancerRoleSpec `json:"loadbalancer,omitempty" yaml:"loadbalancer,omitempty"`
	Storage      StorageRoleSpec      `json:"storage,omitempty" yaml:"storage,omitempty"`
	Registry     RegistryRoleSpec     `json:"registry,omitempty" yaml:"registry,omitempty"`
	CustomRoles  []CustomRoleSpec     `json:"customRoles,omitempty" yaml:"customRoles,omitempty"`
}

// MasterRoleSpec defines the configuration for master nodes.
type MasterRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

// WorkerRoleSpec defines the configuration for worker nodes.
type WorkerRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

// EtcdRoleSpec defines the configuration for etcd nodes.
type EtcdRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

// LoadBalancerRoleSpec defines the configuration for load balancer nodes.
type LoadBalancerRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

// StorageRoleSpec defines the configuration for storage nodes.
type StorageRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

// RegistryRoleSpec defines the configuration for registry nodes.
type RegistryRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

// CustomRoleSpec defines a custom role group.
type CustomRoleSpec struct {
	Name  string   `json:"name" yaml:"name"`
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

// ControlPlaneEndpointSpec is now defined in endpoint_types.go.
// The type *ControlPlaneEndpointSpec for ClusterSpec.ControlPlaneEndpoint will refer to that definition
// as they are in the same package.

// SystemSpec defines system-level configuration.
type SystemSpec struct {
	PackageManager     string   `json:"packageManager,omitempty" yaml:"packageManager,omitempty"`
	NTPServers         []string `json:"ntpServers,omitempty" yaml:"ntpServers,omitempty"`
	Timezone           string   `json:"timezone,omitempty" yaml:"timezone,omitempty"`
	RPMs               []string `json:"rpms,omitempty" yaml:"rpms,omitempty"`
	Debs               []string `json:"debs,omitempty" yaml:"debs,omitempty"`
	PreInstallScripts  []string `json:"preInstallScripts,omitempty" yaml:"preInstall,omitempty"`
	PostInstallScripts []string `json:"postInstallScripts,omitempty" yaml:"postInstall,omitempty"`
	SkipConfigureOS    bool     `json:"skipConfigureOS,omitempty" yaml:"skipConfigureOS,omitempty"`
}

// GlobalSpec contains settings applicable to the entire cluster or as defaults for hosts.
type GlobalSpec struct {
	User              string        `json:"user,omitempty" yaml:"user,omitempty"`
	Port              int           `json:"port,omitempty" yaml:"port,omitempty"`
	Password          string        `json:"password,omitempty" yaml:"password,omitempty"`
	PrivateKey        string        `json:"privateKey,omitempty" yaml:"privateKey,omitempty"`
	PrivateKeyPath    string        `json:"privateKeyPath,omitempty" yaml:"privateKeyPath,omitempty"`
	ConnectionTimeout time.Duration `json:"connectionTimeout,omitempty" yaml:"connectionTimeout,omitempty"`
	WorkDir           string        `json:"workDir,omitempty" yaml:"workDir,omitempty"`
	Verbose           bool          `json:"verbose,omitempty" yaml:"verbose,omitempty"`
	IgnoreErr         bool          `json:"ignoreErr,omitempty" yaml:"ignoreErr,omitempty"`
	SkipPreflight     bool          `json:"skipPreflight,omitempty" yaml:"skipPreflight,omitempty"`
}

// TaintSpec defines a Kubernetes node taint.
type TaintSpec struct {
	Key    string `json:"key" yaml:"key"`
	Value  string `json:"value" yaml:"value"`
	Effect string `json:"effect" yaml:"effect"`
}

// SetDefaults_Cluster sets default values for the Cluster configuration.
func SetDefaults_Cluster(cfg *Cluster) {
	if cfg == nil {
		return
	}
	cfg.SetGroupVersionKind(SchemeGroupVersion.WithKind("Cluster"))

	if cfg.Spec.Type == "" {
		cfg.Spec.Type = ClusterTypeKubeXM // Default to kubexm type
	}

	if cfg.Spec.Global == nil {
		cfg.Spec.Global = &GlobalSpec{}
	}
	g := cfg.Spec.Global
	if g.Port == 0 {
		g.Port = 22
	}
	if g.ConnectionTimeout == 0 {
		g.ConnectionTimeout = 30 * time.Second
	}
	if g.WorkDir == "" {
		g.WorkDir = "/tmp/kubexms_work"
	}

	for i := range cfg.Spec.Hosts {
		host := &cfg.Spec.Hosts[i]
		if host.Port == 0 && g != nil {
			host.Port = g.Port
		}
		if host.User == "" && g != nil {
			host.User = g.User
		}
		if host.PrivateKeyPath == "" && g != nil {
			host.PrivateKeyPath = g.PrivateKeyPath
		}
		if host.Type == "" {
			host.Type = "ssh"
		}
		if host.Arch == "" {
			host.Arch = "amd64"
		}
		if host.Labels == nil {
			host.Labels = make(map[string]string)
		}
		if host.Taints == nil {
			host.Taints = []TaintSpec{}
		}
		if host.Roles == nil {
			host.Roles = []string{}
		}
	}

	if cfg.Spec.ContainerRuntime == nil {
		cfg.Spec.ContainerRuntime = &ContainerRuntimeConfig{}
	}
	SetDefaults_ContainerRuntimeConfig(cfg.Spec.ContainerRuntime)
	if cfg.Spec.ContainerRuntime.Type == ContainerRuntimeContainerd {
		if cfg.Spec.ContainerRuntime.Containerd == nil {
			cfg.Spec.ContainerRuntime.Containerd = &ContainerdConfig{}
		}
		SetDefaults_ContainerdConfig(cfg.Spec.ContainerRuntime.Containerd)
	}

	if cfg.Spec.Etcd == nil {
		cfg.Spec.Etcd = &EtcdConfig{}
	}
	SetDefaults_EtcdConfig(cfg.Spec.Etcd)

	if cfg.Spec.RoleGroups == nil {
		cfg.Spec.RoleGroups = &RoleGroupsSpec{}
	}
	if cfg.Spec.ControlPlaneEndpoint == nil {
		cfg.Spec.ControlPlaneEndpoint = &ControlPlaneEndpointSpec{} // This will use the one from endpoint_types.go
	}
	// Call SetDefaults_ControlPlaneEndpointSpec for the endpoint
	SetDefaults_ControlPlaneEndpointSpec(cfg.Spec.ControlPlaneEndpoint)

	if cfg.Spec.System == nil {
		cfg.Spec.System = &SystemSpec{}
	}
	if cfg.Spec.Kubernetes == nil {
		cfg.Spec.Kubernetes = &KubernetesConfig{}
	}
	SetDefaults_KubernetesConfig(cfg.Spec.Kubernetes, cfg.ObjectMeta.Name)
	if cfg.Spec.Network == nil {
		cfg.Spec.Network = &NetworkConfig{}
	}
	SetDefaults_NetworkConfig(cfg.Spec.Network)
	if cfg.Spec.HighAvailability == nil {
		cfg.Spec.HighAvailability = &HighAvailabilityConfig{}
	}
	SetDefaults_HighAvailabilityConfig(cfg.Spec.HighAvailability)
	if cfg.Spec.Preflight == nil {
		cfg.Spec.Preflight = &PreflightConfig{}
	}
	SetDefaults_PreflightConfig(cfg.Spec.Preflight)
	if cfg.Spec.Kernel == nil {
		cfg.Spec.Kernel = &KernelConfig{}
	}
	SetDefaults_KernelConfig(cfg.Spec.Kernel)
	if cfg.Spec.Addons == nil {
		cfg.Spec.Addons = []AddonConfig{}
	}
	for i := range cfg.Spec.Addons {
		SetDefaults_AddonConfig(&cfg.Spec.Addons[i])
	}
	if cfg.Spec.Storage == nil {
		cfg.Spec.Storage = &StorageConfig{}
	}
	SetDefaults_StorageConfig(cfg.Spec.Storage)
	if cfg.Spec.Registry == nil {
		cfg.Spec.Registry = &RegistryConfig{}
	}
	SetDefaults_RegistryConfig(cfg.Spec.Registry)
	if cfg.Spec.OS == nil {
		cfg.Spec.OS = &OSConfig{}
	}
	SetDefaults_OSConfig(cfg.Spec.OS)
}

// Validate_Cluster validates the Cluster configuration.
func Validate_Cluster(cfg *Cluster) error {
	verrs := &ValidationErrors{}
	if cfg.APIVersion != SchemeGroupVersion.Group+"/"+SchemeGroupVersion.Version {
		verrs.Add("apiVersion: must be %s/%s, got %s", SchemeGroupVersion.Group, SchemeGroupVersion.Version, cfg.APIVersion)
	}
	if cfg.Kind != "Cluster" {
		verrs.Add("kind: must be Cluster, got %s", cfg.Kind)
	}
	if strings.TrimSpace(cfg.ObjectMeta.Name) == "" {
		verrs.Add("metadata.name: cannot be empty")
	}

	validClusterTypes := []string{ClusterTypeKubeXM, ClusterTypeKubeadm}
	isValidClusterType := false
	for _, vt := range validClusterTypes {
		if cfg.Spec.Type == vt {
			isValidClusterType = true
			break
		}
	}
	if !isValidClusterType {
		verrs.Add("spec.type: invalid cluster type '%s', must be one of %v", cfg.Spec.Type, validClusterTypes)
	}

	if cfg.Spec.Global != nil {
		g := cfg.Spec.Global
		if g.Port != 0 && (g.Port <= 0 || g.Port > 65535) {
			verrs.Add("spec.global.port: %d is invalid, must be between 1 and 65535 or 0 for default", g.Port)
		}
	}
	if len(cfg.Spec.Hosts) == 0 {
		verrs.Add("spec.hosts: must contain at least one host")
	}
	hostNames := make(map[string]bool)
	for i, host := range cfg.Spec.Hosts {
		pathPrefix := fmt.Sprintf("spec.hosts[%d:%s]", i, host.Name)
		if strings.TrimSpace(host.Name) == "" {
			pathPrefix = fmt.Sprintf("spec.hosts[%d]", i)
			verrs.Add("%s.name: cannot be empty", pathPrefix)
		} else {
			if _, exists := hostNames[host.Name]; exists {
				verrs.Add("%s.name: '%s' is duplicated", pathPrefix, host.Name)
			}
			hostNames[host.Name] = true
		}
		if strings.TrimSpace(host.Address) == "" {
			verrs.Add("%s.address: cannot be empty", pathPrefix)
		} else {
			if net.ParseIP(host.Address) == nil {
				if matched, _ := regexp.MatchString(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`, host.Address); !matched {
					verrs.Add("%s.address: '%s' is not a valid IP address or hostname", pathPrefix, host.Address)
				}
			}
		}
		if host.Port <= 0 || host.Port > 65535 {
			verrs.Add("%s.port: %d is invalid, must be between 1 and 65535", pathPrefix, host.Port)
		}
		if strings.TrimSpace(host.User) == "" {
			verrs.Add("%s.user: cannot be empty (after defaults)", pathPrefix)
		}
		if strings.ToLower(host.Type) != "local" {
			if host.Password == "" && host.PrivateKey == "" && host.PrivateKeyPath == "" {
				verrs.Add("%s: no SSH authentication method provided for non-local host", pathPrefix)
			}
		}
	}

	if cfg.Spec.ContainerRuntime != nil {
		Validate_ContainerRuntimeConfig(cfg.Spec.ContainerRuntime, verrs, "spec.containerRuntime")
		if cfg.Spec.ContainerRuntime.Type == ContainerRuntimeContainerd {
			if cfg.Spec.ContainerRuntime.Containerd == nil {
				verrs.Add("spec.containerRuntime.containerd: must be defined if containerRuntime.type is '%s'", ContainerRuntimeContainerd)
			} else {
				Validate_ContainerdConfig(cfg.Spec.ContainerRuntime.Containerd, verrs, "spec.containerRuntime.containerd")
			}
		}
	}

	if cfg.Spec.Etcd != nil {
		Validate_EtcdConfig(cfg.Spec.Etcd, verrs, "spec.etcd")
	} else {
		verrs.Add("spec.etcd: section is required")
	}

	if cfg.Spec.RoleGroups != nil {
		Validate_RoleGroupsSpec(cfg.Spec.RoleGroups, verrs, "spec.roleGroups")
	}
	if cfg.Spec.ControlPlaneEndpoint != nil {
		Validate_ControlPlaneEndpointSpec(cfg.Spec.ControlPlaneEndpoint, verrs, "spec.controlPlaneEndpoint")
	}
	if cfg.Spec.System != nil {
		Validate_SystemSpec(cfg.Spec.System, verrs, "spec.system")
	}

	if cfg.Spec.Kubernetes != nil {
		Validate_KubernetesConfig(cfg.Spec.Kubernetes, verrs, "spec.kubernetes")
	} else {
		verrs.Add("spec.kubernetes: section is required")
	}

	if cfg.Spec.HighAvailability != nil {
		Validate_HighAvailabilityConfig(cfg.Spec.HighAvailability, verrs, "spec.highAvailability")
	}
	if cfg.Spec.Preflight != nil {
		Validate_PreflightConfig(cfg.Spec.Preflight, verrs, "spec.preflight")
	}
	if cfg.Spec.Kernel != nil {
		Validate_KernelConfig(cfg.Spec.Kernel, verrs, "spec.kernel")
	}
	if cfg.Spec.Addons != nil {
		for i := range cfg.Spec.Addons {
			addonNameForPath := cfg.Spec.Addons[i].Name
			if addonNameForPath == "" {
				addonNameForPath = fmt.Sprintf("index_%d", i)
			}
			addonPathPrefix := fmt.Sprintf("spec.addons[%s]", addonNameForPath)
			Validate_AddonConfig(&cfg.Spec.Addons[i], verrs, addonPathPrefix)
		}
	}
	if cfg.Spec.Network != nil {
		Validate_NetworkConfig(cfg.Spec.Network, verrs, "spec.network", cfg.Spec.Kubernetes)
	} else {
		verrs.Add("spec.network: section is required")
	}
	if cfg.Spec.Storage != nil {
		Validate_StorageConfig(cfg.Spec.Storage, verrs, "spec.storage")
	}
	if cfg.Spec.Registry != nil {
		Validate_RegistryConfig(cfg.Spec.Registry, verrs, "spec.registry")
	}
	if cfg.Spec.OS != nil {
		Validate_OSConfig(cfg.Spec.OS, verrs, "spec.os")
	}

	if !verrs.IsEmpty() {
		return verrs
	}
	return nil
}

// ValidationErrors defines a type to collect multiple validation errors.
type ValidationErrors struct{ Errors []string }

// Add records an error.
func (ve *ValidationErrors) Add(format string, args ...interface{}) {
	ve.Errors = append(ve.Errors, fmt.Sprintf(format, args...))
}

// Error returns a concatenated string of all errors, or a default message if none.
func (ve *ValidationErrors) Error() string {
	if len(ve.Errors) == 0 {
		return "no validation errors"
	}
	return strings.Join(ve.Errors, "; ")
}

// IsEmpty checks if any errors were recorded.
func (ve *ValidationErrors) IsEmpty() bool { return len(ve.Errors) == 0 }

// DeepCopyObject implements runtime.Object.
func (c *Cluster) DeepCopyObject() runtime.Object {
	if c == nil {
		return nil
	}
	out := new(Cluster)
	c.DeepCopyInto(out)
	return out
}

// DeepCopyInto is a manually implemented deepcopy function, copying the receiver, writing into out.
// WARNING: This is a simplified implementation. For full correctness, especially with nested pointers and slices,
// a code generator (like controller-gen) should be used to create these methods.
func (in *Cluster) DeepCopyInto(out *Cluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)

	// Create a new ClusterSpec and copy primitive fields
	newSpec := ClusterSpec{
		// Copy primitive types directly
	}

	// Deep copy pointer fields in Spec
	if in.Spec.RoleGroups != nil {
		newSpec.RoleGroups = new(RoleGroupsSpec)
		*newSpec.RoleGroups = *in.Spec.RoleGroups // Shallow copy for sub-fields of RoleGroupsSpec for now
	}
	if in.Spec.ControlPlaneEndpoint != nil {
		newSpec.ControlPlaneEndpoint = new(ControlPlaneEndpointSpec)
		*newSpec.ControlPlaneEndpoint = *in.Spec.ControlPlaneEndpoint
	}
	if in.Spec.System != nil {
		newSpec.System = new(SystemSpec)
		*newSpec.System = *in.Spec.System
	}
	if in.Spec.Global != nil {
		newSpec.Global = new(GlobalSpec)
		*newSpec.Global = *in.Spec.Global
	}
	// Deep copy slice of HostSpec
	if in.Spec.Hosts != nil {
		newSpec.Hosts = make([]HostSpec, len(in.Spec.Hosts))
		for i := range in.Spec.Hosts {
			// HostSpec also needs a DeepCopyInto if it has complex fields
			newSpec.Hosts[i] = in.Spec.Hosts[i] // Shallow copy of HostSpec contents for now
		}
	}
    if in.Spec.ContainerRuntime != nil {
        newSpec.ContainerRuntime = new(ContainerRuntimeConfig)
        // Assuming ContainerRuntimeConfig has DeepCopyInto or is simple enough for shallow
        *newSpec.ContainerRuntime = *in.Spec.ContainerRuntime
    }
    if in.Spec.Etcd != nil {
        newSpec.Etcd = new(EtcdConfig)
        *newSpec.Etcd = *in.Spec.Etcd
    }
    if in.Spec.Kubernetes != nil {
        newSpec.Kubernetes = new(KubernetesConfig)
        *newSpec.Kubernetes = *in.Spec.Kubernetes
    }
    if in.Spec.Network != nil {
        newSpec.Network = new(NetworkConfig)
        *newSpec.Network = *in.Spec.Network
    }
    if in.Spec.HighAvailability != nil {
        newSpec.HighAvailability = new(HighAvailabilityConfig)
        *newSpec.HighAvailability = *in.Spec.HighAvailability
    }
    if in.Spec.Preflight != nil {
        newSpec.Preflight = new(PreflightConfig)
        *newSpec.Preflight = *in.Spec.Preflight
    }
    if in.Spec.Kernel != nil {
        newSpec.Kernel = new(KernelConfig)
        *newSpec.Kernel = *in.Spec.Kernel
    }
    if in.Spec.Storage != nil {
        newSpec.Storage = new(StorageConfig)
        *newSpec.Storage = *in.Spec.Storage
    }
    if in.Spec.Registry != nil {
        newSpec.Registry = new(RegistryConfig)
        *newSpec.Registry = *in.Spec.Registry
    }
    if in.Spec.OS != nil {
        newSpec.OS = new(OSConfig)
        *newSpec.OS = *in.Spec.OS
    }
	if in.Spec.Addons != nil {
		newSpec.Addons = make([]AddonConfig, len(in.Spec.Addons))
		for i := range in.Spec.Addons {
			// AddonConfig also needs DeepCopyInto if complex
			newSpec.Addons[i] = in.Spec.Addons[i] // Shallow copy of AddonConfig contents
		}
	}
	out.Spec = newSpec
}

// DeepCopy is a deepcopy function, copying the receiver, creating a new Cluster.
func (in *Cluster) DeepCopy() *Cluster {
	if in == nil {
		return nil
	}
	out := new(Cluster)
	in.DeepCopyInto(out)
	return out
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

// DeepCopyObject implements runtime.Object.
func (cl *ClusterList) DeepCopyObject() runtime.Object {
	if cl == nil {
		return nil
	}
	out := new(ClusterList)
	cl.DeepCopyInto(out)
	return out
}

// DeepCopyInto is a manually implemented copy for compilation.
func (in *ClusterList) DeepCopyInto(out *ClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		inItems := in.Items
		out.Items = make([]Cluster, len(inItems))
		for i := range inItems {
			inItems[i].DeepCopyInto(&out.Items[i])
		}
	}
}

// DeepCopy is a deepcopy function, copying the receiver, creating a new ClusterList.
func (in *ClusterList) DeepCopy() *ClusterList {
	if in == nil {
		return nil
	}
	out := new(ClusterList)
	in.DeepCopyInto(out)
	return out
}

// All placeholder types and functions below were causing redeclaration errors
// and have been removed. The actual definitions reside in their respective
// xxx_types.go files within the same package.
