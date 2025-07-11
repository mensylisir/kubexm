package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"net"
	"regexp"
	"strings"
	"time"
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
	// Status field can be added here if needed by Kubebuilder/controller-gen
	// Status            ClusterStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// ClusterSpec defines the desired state of the Kubernetes cluster.
type ClusterSpec struct {
	Type                 string                    `json:"type,omitempty" yaml:"type,omitempty"` // Added from design doc, was common.KubernetesDeploymentType
	Hosts                []HostSpec                `json:"hosts" yaml:"hosts"`
	RoleGroups           *RoleGroupsSpec           `json:"roleGroups,omitempty" yaml:"roleGroups,omitempty"`
	Global               *GlobalSpec               `json:"global,omitempty" yaml:"global,omitempty"`
	System               *SystemSpec               `json:"system,omitempty" yaml:"system,omitempty"`
	Kubernetes           *KubernetesConfig         `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
	Etcd                 *EtcdConfig               `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	DNS                  DNS                       `json:"dns,omitempty" yaml:"dns,omitempty"` // Changed from *DNS and casing
	ContainerRuntime     *ContainerRuntimeConfig   `json:"containerRuntime,omitempty" yaml:"containerRuntime,omitempty"`
	Network              *NetworkConfig            `json:"network,omitempty" yaml:"network,omitempty"`
	ControlPlaneEndpoint *ControlPlaneEndpointSpec `json:"controlPlaneEndpoint,omitempty" yaml:"controlPlaneEndpoint,omitempty"`
	HighAvailability     *HighAvailabilityConfig   `json:"highAvailability,omitempty" yaml:"highAvailability,omitempty"`
	Storage              *StorageConfig            `json:"storage,omitempty" yaml:"storage,omitempty"`
	Registry             *RegistryConfig           `json:"registry,omitempty" yaml:"registry,omitempty"`
	Addons               []string                  `json:"addons,omitempty" yaml:"addons,omitempty"`
	Preflight            *PreflightConfig          `json:"preflight,omitempty" yaml:"preflight,omitempty"`
	// HostsFileContent from existing file is not in the design doc's ClusterSpec, assuming it's removed or handled differently.
	// HostsCount from existing file is a helper, not part of spec.
}

// HostSpec defines the configuration for a single host.
type HostSpec struct {
	Name            string            `json:"name" yaml:"name"`
	Address         string            `json:"address" yaml:"address"`
	InternalAddress string            `json:"internalAddress,omitempty" yaml:"internalAddress,omitempty"`
	Port            int               `json:"port,omitempty" yaml:"port,omitempty"`
	User            string            `json:"user,omitempty" yaml:"user,omitempty"`
	Password        string            `json:"password,omitempty" yaml:"password,omitempty"`
	PrivateKey      string            `json:"privateKey,omitempty" yaml:"privateKey,omitempty"` // Added from design doc
	PrivateKeyPath  string            `json:"privateKeyPath,omitempty" yaml:"privateKeyPath,omitempty"`
	Roles           []string          `json:"roles,omitempty" yaml:"roles,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Taints          []TaintSpec       `json:"taints,omitempty" yaml:"taints,omitempty"`
	Type            string            `json:"type,omitempty" yaml:"type,omitempty"` // Was common.HostConnectionType
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

// SystemSpec defines system-level configuration.
type SystemSpec struct {
	NTPServers         []string          `json:"ntpServers,omitempty" yaml:"ntpServers,omitempty"`
	Timezone           string            `json:"timezone,omitempty" yaml:"timezone,omitempty"`
	RPMs               []string          `json:"rpms,omitempty" yaml:"rpms,omitempty"`
	Debs               []string          `json:"debs,omitempty" yaml:"debs,omitempty"`
	PackageManager     string            `json:"packageManager,omitempty" yaml:"packageManager,omitempty"`
	PreInstallScripts  []string          `json:"preInstallScripts,omitempty" yaml:"preInstall,omitempty"`
	PostInstallScripts []string          `json:"postInstallScripts,omitempty" yaml:"postInstall,omitempty"`
	SkipConfigureOS    bool              `json:"skipConfigureOS,omitempty" yaml:"skipConfigureOS,omitempty"`
	Modules            []string          `json:"modules,omitempty" yaml:"modules,omitempty"`
	SysctlParams       map[string]string `json:"sysctlParams,omitempty" yaml:"sysctlParams,omitempty"`
}

// GlobalSpec contains settings applicable to the entire cluster or as defaults for hosts.
type GlobalSpec struct {
	User              string        `json:"user,omitempty" yaml:"user,omitempty"`
	Port              int           `json:"port,omitempty" yaml:"port,omitempty"`
	Password          string        `json:"password,omitempty" yaml:"password,omitempty"`
	PrivateKey        string        `json:"privateKey,omitempty" yaml:"privateKey,omitempty"` // Added from design doc
	PrivateKeyPath    string        `json:"privateKeyPath,omitempty" yaml:"privateKeyPath,omitempty"`
	ConnectionTimeout time.Duration `json:"connectionTimeout,omitempty" yaml:"connectionTimeout,omitempty"`
	WorkDir           string        `json:"workDir,omitempty" yaml:"workDir,omitempty"`
	// HostWorkDir from design doc is missing here, assuming WorkDir is for local and remote is defaulted/inferred.
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
	// cfg.SetGroupVersionKind(SchemeGroupVersion.WithKind("Cluster")) // Will be set by K8s machinery

	if cfg.Spec.Type == "" {
		cfg.Spec.Type = common.ClusterTypeKubeXM // Default to kubexm type
	}

	if cfg.Spec.Global == nil {
		cfg.Spec.Global = &GlobalSpec{}
	}
	g := cfg.Spec.Global
	if g.Port == 0 {
		g.Port = 22 // common.DefaultSSHPort
	}
	if g.ConnectionTimeout == 0 {
		g.ConnectionTimeout = 30 * time.Second // common.DefaultConnectionTimeout
	}
	if g.WorkDir == "" {
		g.WorkDir = "/tmp/kubexms_work" // Or common.DefaultRemoteWorkDir / common.DefaultWorkDirName logic
	}

	for i := range cfg.Spec.Hosts {
		host := &cfg.Spec.Hosts[i]
		if host.Port == 0 && g != nil {
			host.Port = g.Port
		}
		if host.User == "" && g != nil {
			host.User = g.User
		}
		// Assuming PrivateKey takes precedence over PrivateKeyPath if both somehow set.
		// Defaulting logic might need to be smarter if PrivateKey is directly provided.
		if host.PrivateKeyPath == "" && host.PrivateKey == "" && g != nil {
			host.PrivateKeyPath = g.PrivateKeyPath
			host.PrivateKey = g.PrivateKey
		}
		if host.Type == "" {
			host.Type = "ssh" // common.HostConnectionTypeSSH
		}
		if host.Arch == "" {
			host.Arch = "amd64" // common.DefaultArch
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
	// Specific defaults for Docker/Containerd are called within SetDefaults_ContainerRuntimeConfig

	if cfg.Spec.Etcd == nil {
		cfg.Spec.Etcd = &EtcdConfig{}
	}
	SetDefaults_EtcdConfig(cfg.Spec.Etcd)

	if cfg.Spec.RoleGroups == nil {
		cfg.Spec.RoleGroups = &RoleGroupsSpec{}
	}
	// SetDefaults_RoleGroupsSpec might be needed if RoleGroupsSpec has defaults.

	if cfg.Spec.ControlPlaneEndpoint == nil {
		cfg.Spec.ControlPlaneEndpoint = &ControlPlaneEndpointSpec{}
	}
	SetDefaults_ControlPlaneEndpointSpec(cfg.Spec.ControlPlaneEndpoint)

	if cfg.Spec.System == nil {
		cfg.Spec.System = &SystemSpec{}
	}
	SetDefaults_SystemSpec(cfg.Spec.System)

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

	if cfg.Spec.Addons == nil {
		cfg.Spec.Addons = []string{}
	}

	if cfg.Spec.Storage == nil {
		cfg.Spec.Storage = &StorageConfig{}
	}
	SetDefaults_StorageConfig(cfg.Spec.Storage)

	if cfg.Spec.Registry == nil {
		cfg.Spec.Registry = &RegistryConfig{}
	}
	SetDefaults_RegistryConfig(cfg.Spec.Registry)

	// Assuming DNS struct has a SetDefaults_DNS function if needed
	// SetDefaults_DNS(&cfg.Spec.DNS) // For non-pointer DNS field
}

// SetDefaults_SystemSpec sets default values for SystemSpec.
func SetDefaults_SystemSpec(cfg *SystemSpec) {
	if cfg == nil {
		return
	}
	if cfg.NTPServers == nil {
		cfg.NTPServers = []string{}
	}
	if cfg.RPMs == nil {
		cfg.RPMs = []string{}
	}
	if cfg.Debs == nil {
		cfg.Debs = []string{}
	}
	if cfg.Modules == nil {
		cfg.Modules = []string{}
	}
	if cfg.SysctlParams == nil {
		cfg.SysctlParams = make(map[string]string)
	}
	if cfg.PreInstallScripts == nil {
		cfg.PreInstallScripts = []string{}
	}
	if cfg.PostInstallScripts == nil {
		cfg.PostInstallScripts = []string{}
	}
}

// Validate_Cluster validates the Cluster configuration.
func Validate_Cluster(cfg *Cluster) error {
	verrs := &ValidationErrors{}
	// GroupVersion and Kind are usually set by K8s API server or client libraries
	// if cfg.APIVersion != SchemeGroupVersion.Group+"/"+SchemeGroupVersion.Version {
	// 	verrs.Add("apiVersion: must be %s/%s, got %s", SchemeGroupVersion.Group, SchemeGroupVersion.Version, cfg.APIVersion)
	// }
	// if cfg.Kind != "Cluster" {
	// 	verrs.Add("kind: must be Cluster, got %s", cfg.Kind)
	// }
	if strings.TrimSpace(cfg.ObjectMeta.Name) == "" {
		verrs.Add("metadata.name: cannot be empty")
	}

	validClusterTypes := []string{common.ClusterTypeKubeXM, common.ClusterTypeKubeadm, ""} // Allow empty for default
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
			// Basic validation for IP or hostname
			if net.ParseIP(host.Address) == nil { // Not an IP
				// Check if it's a valid hostname (simple regex, can be more complex)
				if matched, _ := regexp.MatchString(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`, host.Address); !matched {
					verrs.Add("%s.address: '%s' is not a valid IP address or hostname", pathPrefix, host.Address)
				}
			}
		}
		if host.Port != 0 && (host.Port <= 0 || host.Port > 65535) { // Port 0 is defaulted
			verrs.Add("%s.port: %d is invalid, must be between 1 and 65535", pathPrefix, host.Port)
		}
		if host.User == "" && host.Type != "local" { // User can be empty for local type
             // User can be defaulted from GlobalSpec as well. Actual check should be post-defaulting.
             // This check is simplified here.
        }

		if host.Type != "local" && host.Type != "ssh" && host.Type != "" {
			verrs.Add("%s.type: invalid host type '%s', must be 'local', 'ssh' or empty for default", pathPrefix, host.Type)
		}
		if host.Type == "ssh" {
			if host.Password == "" && host.PrivateKey == "" && host.PrivateKeyPath == "" {
				verrs.Add("%s: no SSH authentication method provided for ssh host type", pathPrefix)
			}
		}
		// Arch validation could use common.SupportedArches
	}

	if cfg.Spec.ContainerRuntime != nil {
		Validate_ContainerRuntimeConfig(cfg.Spec.ContainerRuntime, verrs, "spec.containerRuntime")
	} else {
		verrs.Add("spec.containerRuntime: section is required")
	}


	if cfg.Spec.Etcd != nil {
		Validate_EtcdConfig(cfg.Spec.Etcd, verrs, "spec.etcd")
	} else {
		verrs.Add("spec.etcd: section is required")
	}

	// RoleGroups validation can be complex, ensuring hosts listed exist in spec.hosts
	// Validate_RoleGroupsSpec(cfg.Spec.RoleGroups, verrs, "spec.roleGroups", hostNames)


	if cfg.Spec.ControlPlaneEndpoint != nil {
		Validate_ControlPlaneEndpointSpec(cfg.Spec.ControlPlaneEndpoint, verrs, "spec.controlPlaneEndpoint")
	} else {
		verrs.Add("spec.controlPlaneEndpoint: section is required for HA or accessible clusters")
	}

	if cfg.Spec.System != nil {
		Validate_SystemSpec(cfg.Spec.System, verrs, "spec.system")
	}

	if cfg.Spec.Kubernetes != nil {
		Validate_KubernetesConfig(cfg.Spec.Kubernetes, verrs, "spec.kubernetes")
	} else {
		verrs.Add("spec.kubernetes: section is required")
	}

	if cfg.Spec.Network != nil {
		Validate_NetworkConfig(cfg.Spec.Network, verrs, "spec.network", cfg.Spec.Kubernetes)
	} else {
		verrs.Add("spec.network: section is required")
	}

	if cfg.Spec.HighAvailability != nil {
		Validate_HighAvailabilityConfig(cfg.Spec.HighAvailability, verrs, "spec.highAvailability")
	}
	if cfg.Spec.Preflight != nil {
		Validate_PreflightConfig(cfg.Spec.Preflight, verrs, "spec.preflight")
	}

	if cfg.Spec.Storage != nil {
		Validate_StorageConfig(cfg.Spec.Storage, verrs, "spec.storage")
	}
	if cfg.Spec.Registry != nil {
		Validate_RegistryConfig(cfg.Spec.Registry, verrs, "spec.registry")
	}
	// Validate DNS
	// Validate_DNS(&cfg.Spec.DNS, verrs, "spec.dns") // For non-pointer DNS field

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

// DeepCopy methods have been removed as per instruction.
// They should be generated by controller-gen.

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

// All placeholder types and functions below were causing redeclaration errors
// and have been removed. The actual definitions reside in their respective
// xxx_types.go files within the same package.
// For example, KubernetesConfig, EtcdConfig, etc., and their SetDefaults/Validate functions
// are expected to be in kubernetes_types.go, etcd_types.go etc.
// The simplified DeepCopy methods from the prompt are also removed,
// relying on controller-gen for correct generation.
// SchemeGroupVersion needs to be defined, typically in groupversion_info.go
var SchemeGroupVersion = metav1.GroupVersion{Group: "kubexms.io", Version: "v1alpha1"}
