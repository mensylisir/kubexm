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
	Hosts                []HostSpec                `json:"hosts" yaml:"hosts"`
	RoleGroups           *RoleGroupsSpec           `json:"roleGroups,omitempty" yaml:"roleGroups,omitempty"`
	Global               *GlobalSpec               `json:"global,omitempty" yaml:"global,omitempty"`
	System               *SystemSpec               `json:"system,omitempty" yaml:"system,omitempty"`
	Kubernetes           *KubernetesConfig         `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
	Etcd                 *EtcdConfig               `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	DNS                  DNS                       `yaml:"dns" json:"dns,omitempty"`
	ContainerRuntime     *ContainerRuntimeConfig   `json:"containerRuntime,omitempty" yaml:"containerRuntime,omitempty"`
	Network              *NetworkConfig            `json:"network,omitempty" yaml:"network,omitempty"`
	ControlPlaneEndpoint *ControlPlaneEndpointSpec `json:"controlPlaneEndpoint,omitempty" yaml:"controlPlaneEndpoint,omitempty"`
	HighAvailability     *HighAvailabilityConfig   `json:"highAvailability,omitempty" yaml:"highAvailability,omitempty"` // This might be deprecated or merged into ControlPlaneEndpoint
	Storage              *StorageConfig            `json:"storage,omitempty" yaml:"storage,omitempty"`
	Registry             *RegistryConfig           `json:"registry,omitempty" yaml:"registry,omitempty"`
	Addons               []string                  `json:"addons,omitempty" yaml:"addons,omitempty"`
	Preflight            *PreflightConfig          `json:"preflight,omitempty" yaml:"preflight,omitempty"`
	// Additional fields from YAML not explicitly in existing structs will be added here or to relevant sub-specs.
}

// HostSpec defines the configuration for a single host.
// Note: 'arch' field was already present.
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
// It now incorporates fields previously in OSConfig and KernelConfig.
type SystemSpec struct {
	// NTP servers for time synchronization. Corresponds to `system.ntpServers` in YAML.
	NTPServers         []string `json:"ntpServers,omitempty" yaml:"ntpServers,omitempty"`
	// Timezone to set on hosts. Corresponds to `system.timezone` in YAML.
	Timezone           string   `json:"timezone,omitempty" yaml:"timezone,omitempty"`
	// RPM packages to install. Corresponds to `system.rpms` in YAML.
	RPMs               []string `json:"rpms,omitempty" yaml:"rpms,omitempty"`
	// DEB packages to install. Corresponds to `system.debs` in YAML.
	Debs               []string `json:"debs,omitempty" yaml:"debs,omitempty"`

	// PackageManager allows specifying the package manager to use, overriding auto-detection.
	PackageManager     string   `json:"packageManager,omitempty" yaml:"packageManager,omitempty"`
	// PreInstallScripts are commands/scripts to run before main component installation.
	// YAML tag "preInstall" as per 21-其他说明.md.
	PreInstallScripts  []string `json:"preInstallScripts,omitempty" yaml:"preInstall,omitempty"`
	// PostInstallScripts are commands/scripts to run after main component installation.
	// YAML tag "postInstall" as per 21-其他说明.md.
	PostInstallScripts []string `json:"postInstallScripts,omitempty" yaml:"postInstall,omitempty"`
	// SkipConfigureOS, if true, skips OS configuration steps like NTP, timezone. Defaults to false.
	SkipConfigureOS    bool     `json:"skipConfigureOS,omitempty" yaml:"skipConfigureOS,omitempty"`

	// Modules is a list of kernel modules to be loaded. (From former KernelConfig)
	Modules            []string          `json:"modules,omitempty" yaml:"modules,omitempty"`
	// SysctlParams is a map of sysctl parameters to set. (From former KernelConfig)
	SysctlParams       map[string]string `json:"sysctlParams,omitempty" yaml:"sysctlParams,omitempty"`
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

	// cfg.Spec.Type was removed

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
	SetDefaults_SystemSpec(cfg.Spec.System) // Call the new centralized System defaults

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

	// SetDefaults_KernelConfig and SetDefaults_OSConfig calls are removed
	// Their logic is now part of SetDefaults_SystemSpec.

	// Addons in ClusterSpec is now []string, so no SetDefaults_AddonConfig directly here.
	// If individual addons had complex types and defaults, that would be handled differently.
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

	// Set defaults for DNS. cfg.Spec.DNS is not a pointer.
	SetDefaults_DNS(&cfg.Spec.DNS)
	// OS field removed from ClusterSpec
}

// SetDefaults_SystemSpec sets default values for SystemSpec.
// Incorporates logic from former SetDefaults_OSConfig and SetDefaults_KernelConfig.
func SetDefaults_SystemSpec(cfg *SystemSpec) {
	if cfg == nil {
		return
	}
	// Defaults from OSConfig
	if cfg.NTPServers == nil {
		cfg.NTPServers = []string{}
	}
	// Timezone: No default, let OS default prevail if not set by user.
	if cfg.RPMs == nil {
		cfg.RPMs = []string{}
	}
	if cfg.Debs == nil {
		cfg.Debs = []string{}
	}
	// SkipConfigureOS (bool) defaults to false (its zero value).

	// Defaults from KernelConfig
	if cfg.Modules == nil {
		cfg.Modules = []string{}
	}
	if cfg.SysctlParams == nil {
		cfg.SysctlParams = make(map[string]string)
	}
	// Example default sysctl param:
	// if _, exists := cfg.SysctlParams["net.bridge.bridge-nf-call-iptables"]; !exists {
	//    cfg.SysctlParams["net.bridge.bridge-nf-call-iptables"] = "1"
	// }

	// Defaults for new fields in SystemSpec
	if cfg.PreInstallScripts == nil {
		cfg.PreInstallScripts = []string{}
	}
	if cfg.PostInstallScripts == nil {
		cfg.PostInstallScripts = []string{}
	}
	// PackageManager: No default, allow auto-detection by runner if empty.
}

// Validate_SystemSpec validates SystemSpec.
// Incorporates logic from former Validate_OSConfig and Validate_KernelConfig.
func Validate_SystemSpec(cfg *SystemSpec, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	// Validations from OSConfig
	for i, ntp := range cfg.NTPServers {
		if strings.TrimSpace(ntp) == "" {
			verrs.Add("%s.ntpServers[%d]: NTP server address cannot be empty", pathPrefix, i)
		}
		// Could add validation for hostname/IP format for NTP servers
	}
	if cfg.Timezone != "" && strings.TrimSpace(cfg.Timezone) == "" { // Check if set to only whitespace
		verrs.Add("%s.timezone: cannot be only whitespace if specified", pathPrefix)
		// Could validate against a list of known timezones if necessary (complex)
	}
	for i, rpm := range cfg.RPMs {
		if strings.TrimSpace(rpm) == "" {
			verrs.Add("%s.rpms[%d]: RPM package name cannot be empty", pathPrefix, i)
		}
	}
	for i, deb := range cfg.Debs {
		if strings.TrimSpace(deb) == "" {
			verrs.Add("%s.debs[%d]: DEB package name cannot be empty", pathPrefix, i)
		}
	}
	// SkipConfigureOS (bool) has no specific validation other than type.

	// Validations from KernelConfig
	for i, module := range cfg.Modules {
		if strings.TrimSpace(module) == "" {
			verrs.Add("%s.modules[%d]: module name cannot be empty", pathPrefix, i)
		}
	}
	for key, val := range cfg.SysctlParams {
		if strings.TrimSpace(key) == "" {
			verrs.Add("%s.sysctlParams: sysctl key cannot be empty (value: '%s')", pathPrefix, val)
		}
		// Could also validate that val is not empty if that's a requirement
	}

	// Validations for new fields in SystemSpec
	if cfg.PackageManager != "" && strings.TrimSpace(cfg.PackageManager) == "" { // Check if set to only whitespace
		verrs.Add("%s.packageManager: cannot be only whitespace if specified", pathPrefix)
	}
	for i, script := range cfg.PreInstallScripts {
		if strings.TrimSpace(script) == "" {
			verrs.Add("%s.preInstallScripts[%d]: script cannot be empty", pathPrefix, i)
		}
	}
	for i, script := range cfg.PostInstallScripts {
		if strings.TrimSpace(script) == "" {
			verrs.Add("%s.postInstallScripts[%d]: script cannot be empty", pathPrefix, i)
		}
	}
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

	// cfg.Spec.Type was removed, so its validation is also removed.
	// The type of Kubernetes deployment (kubexm or kubeadm) is now solely determined by KubernetesConfig.Type.

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
		Validate_SystemSpec(cfg.Spec.System, verrs, "spec.system") // Call the new centralized System validation
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
	// Validate_KernelConfig and Validate_OSConfig calls are removed.
	// Their logic will be part of Validate_SystemSpec called earlier for cfg.Spec.System.

	// Addons in ClusterSpec is now []string. Validation might involve checking if addon names are known/valid if there's a predefined list.
	// For now, just ensure no empty strings if the list itself isn't empty.
	if cfg.Spec.Addons != nil { // It's defaulted to []string{}, so never nil
		for i, addonName := range cfg.Spec.Addons {
			if strings.TrimSpace(addonName) == "" {
				verrs.Add("spec.addons[%d]: addon name cannot be empty", i)
			}
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

	// Validate DNS. cfg.Spec.DNS is not a pointer.
	Validate_DNS(&cfg.Spec.DNS, verrs, "spec.dns")
	// OS field removed from ClusterSpec

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
// IMPORTANT: The DeepCopyObject, DeepCopy, and DeepCopyInto methods should be generated by controller-gen.
// The implementations below are simplified and may not be fully correct for all nested types.
// Ensure controller-gen is run to generate the official deepcopy methods.
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
// The comment above this func block provides guidance on using controller-gen.
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
    if in.Spec.Storage != nil {
        newSpec.Storage = new(StorageConfig)
        *newSpec.Storage = *in.Spec.Storage
    }
    if in.Spec.Registry != nil {
        newSpec.Registry = new(RegistryConfig)
        *newSpec.Registry = *in.Spec.Registry
    }
	if in.Spec.Addons != nil {
		newSpec.Addons = make([]string, len(in.Spec.Addons))
		copy(newSpec.Addons, in.Spec.Addons)
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

// Validate_RoleGroupsSpec validates the RoleGroupsSpec.
// It performs structural checks on the defined roles and their host lists.
// Cross-validation against ClusterSpec.Hosts (e.g., ensuring hostnames exist)
// is typically done in Validate_Cluster.
func Validate_RoleGroupsSpec(cfg *RoleGroupsSpec, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		// RoleGroupsSpec is optional, so nil is acceptable.
		return
	}

	validateRoleSpecHosts := func(hosts []string, roleName string, pPrefix string) {
		for i, hostName := range hosts {
			if strings.TrimSpace(hostName) == "" {
				verrs.Add("%s.%s.hosts[%d]: hostname cannot be empty", pPrefix, roleName, i)
			}
		}
	}

	// Validate predefined roles
	if cfg.Master.Hosts != nil {
		validateRoleSpecHosts(cfg.Master.Hosts, "master", pathPrefix)
	}
	if cfg.Worker.Hosts != nil {
		validateRoleSpecHosts(cfg.Worker.Hosts, "worker", pathPrefix)
	}
	if cfg.Etcd.Hosts != nil {
		validateRoleSpecHosts(cfg.Etcd.Hosts, "etcd", pathPrefix)
	}
	if cfg.LoadBalancer.Hosts != nil {
		validateRoleSpecHosts(cfg.LoadBalancer.Hosts, "loadbalancer", pathPrefix)
	}
	if cfg.Storage.Hosts != nil {
		validateRoleSpecHosts(cfg.Storage.Hosts, "storage", pathPrefix)
	}
	if cfg.Registry.Hosts != nil {
		validateRoleSpecHosts(cfg.Registry.Hosts, "registry", pathPrefix)
	}

	// Validate CustomRoles
	if cfg.CustomRoles != nil {
		customRoleNames := make(map[string]bool)
		for i, customRole := range cfg.CustomRoles {
			customRolePathPrefix := fmt.Sprintf("%s.customRoles[%d]", pathPrefix, i) // Corrected to use string(i) for index
			if strings.TrimSpace(customRole.Name) == "" {
				verrs.Add("%s.name: custom role name cannot be empty", customRolePathPrefix)
			} else {
				if _, exists := customRoleNames[customRole.Name]; exists {
					verrs.Add("%s.name: custom role name '%s' is duplicated", customRolePathPrefix, customRole.Name)
				}
				customRoleNames[customRole.Name] = true
			}
			if customRole.Hosts != nil {
				// It seems there was a copy-paste error in the original diff for zz_placeholder_validations.go
				// The call to validateRoleSpecHosts for customRole.Hosts was missing. Adding it here.
				validateRoleSpecHosts(customRole.Hosts, customRole.Name, customRolePathPrefix)
			}
		}
	}
}
