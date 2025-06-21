package v1alpha1

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
// +kubebuilder:printcolumn:name="Hosts",type="integer",JSONPath=".spec.hostsCount",description="Number of hosts" // Example, need field
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Cluster is the top-level configuration object.
type Cluster struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec ClusterSpec `json:"spec,omitempty" yaml:"spec,omitempty"`
	// Status ClusterStatus `json:"status,omitempty"` // Add when status is defined
}

// ClusterSpec defines the desired state of the Kubernetes cluster.
type ClusterSpec struct {
	// RoleGroups defines the different groups of nodes in the cluster.
	RoleGroups *RoleGroupsSpec `json:"roleGroups,omitempty" yaml:"roleGroups,omitempty"`
	// ControlPlaneEndpoint defines the endpoint for the Kubernetes API server.
	ControlPlaneEndpoint *ControlPlaneEndpointSpec `json:"controlPlaneEndpoint,omitempty" yaml:"controlPlaneEndpoint,omitempty"`
	// System contains system-level configuration.
	System *SystemSpec `json:"system,omitempty" yaml:"system,omitempty"`

	Global *GlobalSpec `json:"global,omitempty" yaml:"global,omitempty"`
	Hosts  []HostSpec  `json:"hosts" yaml:"hosts"`

	// Component configurations - will be pointers to specific config types
	ContainerRuntime *ContainerRuntimeConfig `json:"containerRuntime,omitempty" yaml:"containerRuntime,omitempty"`
	Etcd             *EtcdConfig             `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	Kubernetes       *KubernetesConfig       `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
	Network          *NetworkConfig          `json:"network,omitempty" yaml:"network,omitempty"`
	HighAvailability *HighAvailabilityConfig `json:"highAvailability,omitempty" yaml:"highAvailability,omitempty"`
	Preflight        *PreflightConfig        `json:"preflight,omitempty" yaml:"preflight,omitempty"`
	Kernel           *KernelConfig           `json:"kernel,omitempty" yaml:"kernel,omitempty"`
	Storage          *StorageConfig          `json:"storage,omitempty" yaml:"storage,omitempty"`
	Registry         *RegistryConfig         `json:"registry,omitempty" yaml:"registry,omitempty"`
	OS               *OSConfig               `json:"os,omitempty" yaml:"os,omitempty"`
	Addons           []AddonConfig           `json:"addons,omitempty" yaml:"addons,omitempty"`
	// HostsCount int `json:"hostsCount,omitempty"` // Example for printcolumn
}

// RoleGroupsSpec defines the different groups of nodes in the cluster.
type RoleGroupsSpec struct {
	Master       MasterRoleSpec       `json:"master,omitempty" yaml:"master,omitempty"`
	Worker       WorkerRoleSpec       `json:"worker,omitempty" yaml:"worker,omitempty"`
	Etcd         EtcdRoleSpec         `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	LoadBalancer LoadBalancerRoleSpec `json:"loadBalancer,omitempty" yaml:"loadBalancer,omitempty"` // YAML name from example: loadbalancer
	Storage      StorageRoleSpec      `json:"storage,omitempty" yaml:"storage,omitempty"`
	// Registry       RegistryRoleSpec `json:"registry,omitempty" yaml:"registry,omitempty"` // Assuming a registry role might exist
	CustomRoles []CustomRoleSpec `json:"customRoles,omitempty" yaml:"customRoles,omitempty"`
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

// CustomRoleSpec defines a custom role group.
type CustomRoleSpec struct {
	Name  string   `json:"name" yaml:"name"`
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

// ControlPlaneEndpointSpec defines the endpoint for the Kubernetes API server.
// YAML fields from description: internalLoadbalancer, externalDNS, domain, address, port
type ControlPlaneEndpointSpec struct {
	Host                 string `json:"host,omitempty" yaml:"address,omitempty"` // Maps to 'address' in YAML
	Port                 int    `json:"port,omitempty" yaml:"port,omitempty"`
	InternalLoadbalancer string `json:"internalLoadbalancer,omitempty" yaml:"internalLoadbalancer,omitempty"` // New field
	ExternalDNS          bool   `json:"externalDNS,omitempty" yaml:"externalDNS,omitempty"`                   // New field
	Domain               string `json:"domain,omitempty" yaml:"domain,omitempty"`                             // New field
}

// SystemSpec defines system-level configuration.
// YAML fields from description: ntpServers, timezone, rpms, debs, preInstall, postInstall, skipConfigureOS
type SystemSpec struct {
	PackageManager     string   `json:"packageManager,omitempty" yaml:"packageManager,omitempty"`   // e.g., "apt", "yum"
	NTPServers         []string `json:"ntpServers,omitempty" yaml:"ntpServers,omitempty"`           // New field
	Timezone           string   `json:"timezone,omitempty" yaml:"timezone,omitempty"`               // New field
	RPMs               []string `json:"rpms,omitempty" yaml:"rpms,omitempty"`                       // New field for RPM package names
	Debs               []string `json:"debs,omitempty" yaml:"debs,omitempty"`                       // New field for Deb package names
	PreInstallScripts  []string `json:"preInstallScripts,omitempty" yaml:"preInstall,omitempty"`    // New field, assuming list of script paths or inline
	PostInstallScripts []string `json:"postInstallScripts,omitempty" yaml:"postInstall,omitempty"`  // New field
	SkipConfigureOS    bool     `json:"skipConfigureOS,omitempty" yaml:"skipConfigureOS,omitempty"` // New field
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
}

// TaintSpec defines a Kubernetes node taint.
type TaintSpec struct {
	Key    string `json:"key" yaml:"key"`
	Value  string `json:"value" yaml:"value"`
	Effect string `json:"effect" yaml:"effect"`
}

// Placeholder structs for component configs (initial version)
// Removed placeholder ContainerRuntimeConfig
// Removed placeholder ContainerdConfig
// Removed placeholder EtcdConfig
// Removed placeholder KubernetesConfig (should be in kubernetes_types.go)
// Removed placeholder HighAvailabilityConfig (should be in ha_types.go)
// Removed placeholder PreflightConfig (should be in preflight_types.go)
// Removed placeholder KernelConfig (should be in kernel_types.go)
// Removed placeholder NetworkConfig
// Removed placeholder AddonConfig

// SetDefaults_Cluster sets default values for the Cluster configuration.
func SetDefaults_Cluster(cfg *Cluster) {
	if cfg == nil {
		return
	}
	cfg.SetGroupVersionKind(SchemeGroupVersion.WithKind("Cluster"))
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

	// Initialize component configs if nil (initial placeholder logic)
	// Integrate ContainerRuntime and Containerd defaulting
	if cfg.Spec.ContainerRuntime == nil {
		cfg.Spec.ContainerRuntime = &ContainerRuntimeConfig{}
	}
	SetDefaults_ContainerRuntimeConfig(cfg.Spec.ContainerRuntime)
	if cfg.Spec.ContainerRuntime.Type == ContainerRuntimeContainerd {
		if cfg.Spec.Containerd == nil {
			cfg.Spec.Containerd = &ContainerdConfig{}
		}
		SetDefaults_ContainerdConfig(cfg.Spec.Containerd)
	}

	// Integrate EtcdConfig defaulting
	if cfg.Spec.Etcd == nil {
		cfg.Spec.Etcd = &EtcdConfig{}
	}
	SetDefaults_EtcdConfig(cfg.Spec.Etcd) // Assuming SetDefaults_EtcdConfig exists

	// SetDefaults for RoleGroups, ControlPlaneEndpoint, System
	if cfg.Spec.RoleGroups == nil {
		cfg.Spec.RoleGroups = &RoleGroupsSpec{}
	}
	// SetDefaults_RoleGroupsSpec(cfg.Spec.RoleGroups) // If it exists
	if cfg.Spec.ControlPlaneEndpoint == nil {
		cfg.Spec.ControlPlaneEndpoint = &ControlPlaneEndpointSpec{}
	}
	// SetDefaults_ControlPlaneEndpointSpec(cfg.Spec.ControlPlaneEndpoint) // If it exists
	if cfg.Spec.System == nil {
		cfg.Spec.System = &SystemSpec{}
	}
	// SetDefaults_SystemSpec(cfg.Spec.System) // If it exists

	if cfg.Spec.Kubernetes == nil {
		cfg.Spec.Kubernetes = &KubernetesConfig{}
	}
	SetDefaults_KubernetesConfig(cfg.Spec.Kubernetes, cfg.ObjectMeta.Name) // Assuming SetDefaults_KubernetesConfig exists

	if cfg.Spec.Network == nil {
		cfg.Spec.Network = &NetworkConfig{}
	}
	SetDefaults_NetworkConfig(cfg.Spec.Network) // Assuming SetDefaults_NetworkConfig exists

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
	for i := range cfg.Spec.Addons { // Iterate by index to modify items in place
		SetDefaults_AddonConfig(&cfg.Spec.Addons[i])
	}

	if cfg.Spec.Storage == nil {
		cfg.Spec.Storage = &StorageConfig{}
	}
	SetDefaults_StorageConfig(cfg.Spec.Storage) // Assuming SetDefaults_StorageConfig exists

	if cfg.Spec.Registry == nil {
		cfg.Spec.Registry = &RegistryConfig{}
	}
	SetDefaults_RegistryConfig(cfg.Spec.Registry) // Assuming SetDefaults_RegistryConfig exists

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
	// Initial basic validation for Kubernetes (will be expanded when kubernetes_types.go is integrated)
	// This specific validation is now moved to Validate_KubernetesConfig
	// if cfg.Spec.Kubernetes != nil && strings.TrimSpace(cfg.Spec.Kubernetes.Version) == "" {
	//    verrs.Add("spec.kubernetes.version: cannot be empty")
	// }

	// Integrate ContainerRuntime and Containerd validation
	if cfg.Spec.ContainerRuntime != nil {
		Validate_ContainerRuntimeConfig(cfg.Spec.ContainerRuntime, verrs, "spec.containerRuntime")
		if cfg.Spec.ContainerRuntime.Type == ContainerRuntimeContainerd {
			if cfg.Spec.Containerd == nil {
				verrs.Add("spec.containerd: must be defined if containerRuntime.type is '%s'", ContainerRuntimeContainerd)
			} else {
				Validate_ContainerdConfig(cfg.Spec.Containerd, verrs, "spec.containerd")
			}
		}
	}

	// Integrate EtcdConfig validation
	if cfg.Spec.Etcd != nil { // Changed to pointer, so check for nil
		Validate_EtcdConfig(cfg.Spec.Etcd, verrs, "spec.etcd") // Assuming Validate_EtcdConfig exists
	} else {
		verrs.Add("spec.etcd: section is required") // Or handle if truly optional
	}

	// Validate RoleGroups, ControlPlaneEndpoint, System
	if cfg.Spec.RoleGroups != nil {
		Validate_RoleGroupsSpec(cfg.Spec.RoleGroups, verrs, "spec.roleGroups") // Assuming Validate_RoleGroupsSpec will be created
	} // else { verrs.Add("spec.roleGroups: section is required"); } // If mandatory
	if cfg.Spec.ControlPlaneEndpoint != nil {
		Validate_ControlPlaneEndpointSpec(cfg.Spec.ControlPlaneEndpoint, verrs, "spec.controlPlaneEndpoint") // Assuming Validate_ControlPlaneEndpointSpec will be created
	} // else { verrs.Add("spec.controlPlaneEndpoint: section is required"); } // If mandatory
	if cfg.Spec.System != nil {
		Validate_SystemSpec(cfg.Spec.System, verrs, "spec.system") // Assuming Validate_SystemSpec will be created
	} // else { verrs.Add("spec.system: section is required"); } // If mandatory

	// Integrate KubernetesConfig validation
	if cfg.Spec.Kubernetes != nil { // Changed to pointer
		Validate_KubernetesConfig(cfg.Spec.Kubernetes, verrs, "spec.kubernetes") // Assuming Validate_KubernetesConfig exists
	} else {
		verrs.Add("spec.kubernetes: section is required")
	}

	// Integrate HighAvailability validation
	if cfg.Spec.HighAvailability != nil {
		Validate_HighAvailabilityConfig(cfg.Spec.HighAvailability, verrs, "spec.highAvailability")
	}

	// Integrate Preflight validation
	if cfg.Spec.Preflight != nil {
		Validate_PreflightConfig(cfg.Spec.Preflight, verrs, "spec.preflight")
	}

	// Integrate Kernel validation
	if cfg.Spec.Kernel != nil {
		Validate_KernelConfig(cfg.Spec.Kernel, verrs, "spec.kernel")
	}

	if cfg.Spec.Addons != nil { // Check if Addons slice itself is nil
		for i := range cfg.Spec.Addons {
			addonNameForPath := cfg.Spec.Addons[i].Name
			if addonNameForPath == "" { // Handle case where addon name might be empty during validation
				addonNameForPath = fmt.Sprintf("index_%d", i)
			}
			addonPathPrefix := fmt.Sprintf("spec.addons[%s]", addonNameForPath)
			Validate_AddonConfig(&cfg.Spec.Addons[i], verrs, addonPathPrefix)
		}
	}

	// Integrate NetworkConfig validation
	if cfg.Spec.Network != nil { // Changed to pointer
		Validate_NetworkConfig(cfg.Spec.Network, verrs, "spec.network", cfg.Spec.Kubernetes) // Assuming Validate_NetworkConfig exists
	} else {
		verrs.Add("spec.network: section is required")
	}

	// Integrate StorageConfig validation
	if cfg.Spec.Storage != nil { // Changed to pointer
		Validate_StorageConfig(cfg.Spec.Storage, verrs, "spec.storage") // Assuming Validate_StorageConfig exists
	} // else { verrs.Add("spec.storage: section is required"); } // If optional, no error

	// Integrate RegistryConfig validation
	if cfg.Spec.Registry != nil { // Changed to pointer
		Validate_RegistryConfig(cfg.Spec.Registry, verrs, "spec.registry") // Assuming Validate_RegistryConfig exists
	} // else { verrs.Add("spec.registry: section is required"); } // If optional, no error

	if cfg.Spec.OS != nil {
		Validate_OSConfig(cfg.Spec.OS, verrs, "spec.os")
	}

	if !verrs.IsEmpty() {
		return verrs
	}
	return nil
}

// ValidationErrors (simple version, can be moved to a common errors file)
type ValidationErrors struct{ Errors []string }

func (ve *ValidationErrors) Add(format string, args ...interface{}) {
	ve.Errors = append(ve.Errors, fmt.Sprintf(format, args...))
}
func (ve *ValidationErrors) Error() string {
	if len(ve.Errors) == 0 {
		return "no validation errors"
	}
	return strings.Join(ve.Errors, "; ")
}
func (ve *ValidationErrors) IsEmpty() bool { return len(ve.Errors) == 0 }

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}
