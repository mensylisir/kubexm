package v1alpha1

import (
	"fmt"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"strings"
	"time"
	"github.com/mensylisir/kubexm/pkg/util"
	"github.com/mensylisir/kubexm/pkg/util/validation"
	"github.com/mensylisir/kubexm/pkg/common" // Import common package
)

// Note: ClusterTypeKubeXM and ClusterTypeKubeadm constants are now defined in pkg/common/constants.go
// const (
// // ClusterTypeKubeXM indicates a cluster where core components (kube-apiserver,
// // kube-controller-manager, kube-scheduler, kube-proxy) are deployed as binaries.
// ClusterTypeKubeXM = "kubexm"
//
// // ClusterTypeKubeadm indicates a cluster where core components are deployed as static Pods
// // managed by kubeadm.
// ClusterTypeKubeadm = "kubeadm"
// )

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
	Type                 string                    `json:"type,omitempty" yaml:"type,omitempty"` // Cluster deployment type, e.g., "kubexm", "kubeadm"
	Hosts                []HostSpec                `json:"hosts" yaml:"hosts"`
	RoleGroups           *RoleGroupsSpec           `json:"roleGroups,omitempty" yaml:"roleGroups,omitempty"`
	Global               *GlobalSpec               `json:"global,omitempty" yaml:"global,omitempty"`
	System               *SystemSpec               `json:"system,omitempty" yaml:"system,omitempty"`
	Kubernetes           *KubernetesConfig         `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
	Etcd                 *EtcdConfig               `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	DNS                  DNS                       `json:"dns,omitempty" yaml:"dns,omitempty"`
	ContainerRuntime     *ContainerRuntimeConfig   `json:"containerRuntime,omitempty" yaml:"containerRuntime,omitempty"`
	Network              *NetworkConfig            `json:"network,omitempty" yaml:"network,omitempty"`
	ControlPlaneEndpoint *ControlPlaneEndpointSpec `json:"controlPlaneEndpoint,omitempty" yaml:"controlPlaneEndpoint,omitempty"`
	HighAvailability     *HighAvailabilityConfig   `json:"highAvailability,omitempty" yaml:"highAvailability,omitempty"`
	Storage              *StorageConfig            `json:"storage,omitempty" yaml:"storage,omitempty"`
	Registry             *RegistryConfig           `json:"registry,omitempty" yaml:"registry,omitempty"`
	Addons               []string                  `json:"addons,omitempty" yaml:"addons,omitempty"`
	Preflight            *PreflightConfig          `json:"preflight,omitempty" yaml:"preflight,omitempty"`
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

	if cfg.Spec.Type == "" { // Added default for ClusterSpec.Type
		cfg.Spec.Type = common.ClusterTypeKubeXM
	}

	if cfg.Spec.Global == nil {
		cfg.Spec.Global = &GlobalSpec{}
	}
	g := cfg.Spec.Global
	if g.Port == 0 {
		g.Port = common.DefaultSSHPort
	}
	if g.ConnectionTimeout == 0 {
		g.ConnectionTimeout = 30 * time.Second
	}
	if g.WorkDir == "" {
		g.WorkDir = common.DefaultWorkDir
	}

	for i := range cfg.Spec.Hosts {
		host := &cfg.Spec.Hosts[i]
		if host.Port == 0 && g != nil {
			host.Port = g.Port // Inherits from global default if global.Port was 0 and then defaulted, or if global.Port was set
		} else if host.Port == 0 { // If global is nil or global.Port was not set (remains 0)
			host.Port = common.DefaultSSHPort
		}

		if host.User == "" && g != nil {
			host.User = g.User
		}
		if host.PrivateKeyPath == "" && g != nil {
			host.PrivateKeyPath = g.PrivateKeyPath
		}
		if host.Type == "" {
			host.Type = common.HostTypeSSH
		}
		if host.Arch == "" {
			host.Arch = common.DefaultArch
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

	SetDefaults_DNS(&cfg.Spec.DNS)
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

// Validate_SystemSpec validates SystemSpec.
func Validate_SystemSpec(cfg *SystemSpec, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	for i, ntp := range cfg.NTPServers {
		if strings.TrimSpace(ntp) == "" {
			verrs.Add(fmt.Sprintf("%s.ntpServers[%d]", pathPrefix, i), "NTP server address cannot be empty")
		}
	}
	if cfg.Timezone != "" && strings.TrimSpace(cfg.Timezone) == "" {
		verrs.Add(pathPrefix+".timezone", "cannot be only whitespace if specified")
	}
	for i, rpm := range cfg.RPMs {
		if strings.TrimSpace(rpm) == "" {
			verrs.Add(fmt.Sprintf("%s.rpms[%d]", pathPrefix, i), "RPM package name cannot be empty")
		}
	}
	for i, deb := range cfg.Debs {
		if strings.TrimSpace(deb) == "" {
			verrs.Add(fmt.Sprintf("%s.debs[%d]", pathPrefix, i), "DEB package name cannot be empty")
		}
	}
	for i, module := range cfg.Modules {
		if strings.TrimSpace(module) == "" {
			verrs.Add(fmt.Sprintf("%s.modules[%d]", pathPrefix, i), "module name cannot be empty")
		}
	}
	for key, val := range cfg.SysctlParams {
		if strings.TrimSpace(key) == "" {
			verrs.Add(pathPrefix+".sysctlParams", fmt.Sprintf("sysctl key cannot be empty (value: '%s')", val))
		}
	}
	if cfg.PackageManager != "" && strings.TrimSpace(cfg.PackageManager) == "" {
		verrs.Add(pathPrefix+".packageManager", "cannot be only whitespace if specified")
	}
	for i, script := range cfg.PreInstallScripts {
		if strings.TrimSpace(script) == "" {
			verrs.Add(fmt.Sprintf("%s.preInstallScripts[%d]", pathPrefix, i), "script cannot be empty")
		}
	}
	for i, script := range cfg.PostInstallScripts {
		if strings.TrimSpace(script) == "" {
			verrs.Add(fmt.Sprintf("%s.postInstallScripts[%d]", pathPrefix, i), "script cannot be empty")
		}
	}
}

// Validate_Cluster validates the Cluster configuration.
func Validate_Cluster(cfg *Cluster) error {
	verrs := &validation.ValidationErrors{}
	if cfg.APIVersion != SchemeGroupVersion.Group+"/"+SchemeGroupVersion.Version {
		verrs.Add("apiVersion", fmt.Sprintf("must be %s/%s, got %s", SchemeGroupVersion.Group, SchemeGroupVersion.Version, cfg.APIVersion))
	}
	if cfg.Kind != "Cluster" {
		verrs.Add("kind", fmt.Sprintf("must be Cluster, got %s", cfg.Kind))
	}
	if strings.TrimSpace(cfg.ObjectMeta.Name) == "" {
		verrs.Add("metadata.name", "cannot be empty")
	}

	// Validate ClusterSpec.Type
	validClusterTypes := []string{common.ClusterTypeKubeXM, common.ClusterTypeKubeadm}
	if !util.ContainsString(validClusterTypes, cfg.Spec.Type) {
		verrs.Add("spec.type", fmt.Sprintf("invalid cluster type '%s', must be one of %v", cfg.Spec.Type, validClusterTypes))
	}

	if cfg.Spec.Global != nil {
		g := cfg.Spec.Global
		if g.Port != 0 && (g.Port <= 0 || g.Port > 65535) {
			verrs.Add("spec.global.port", fmt.Sprintf("%d is invalid, must be between 1 and 65535 or 0 for default", g.Port))
		}
	}
	if len(cfg.Spec.Hosts) == 0 {
		verrs.Add("spec.hosts", "must contain at least one host")
	}
	hostNames := make(map[string]bool)
	for i, host := range cfg.Spec.Hosts {
		pathPrefix := fmt.Sprintf("spec.hosts[%d:%s]", i, host.Name)
		if strings.TrimSpace(host.Name) == "" {
			pathPrefix = fmt.Sprintf("spec.hosts[%d]", i)
			verrs.Add(pathPrefix+".name", "cannot be empty")
		} else {
			if _, exists := hostNames[host.Name]; exists {
				verrs.Add(pathPrefix+".name", fmt.Sprintf("'%s' is duplicated", host.Name))
			}
			hostNames[host.Name] = true
		}
		if strings.TrimSpace(host.Address) == "" {
			verrs.Add(pathPrefix+".address", "cannot be empty")
		} else if !util.IsValidIP(host.Address) && !util.IsValidDomainName(host.Address) {
			verrs.Add(pathPrefix+".address", fmt.Sprintf("'%s' is not a valid IP address or hostname", host.Address))
		}
		if host.Port <= 0 || host.Port > 65535 {
			verrs.Add(pathPrefix+".port", fmt.Sprintf("%d is invalid, must be between 1 and 65535", host.Port))
		}
		if strings.TrimSpace(host.User) == "" {
			verrs.Add(pathPrefix+".user", "cannot be empty (after defaults)")
		}
		if strings.ToLower(host.Type) != common.HostTypeLocal {
			if host.Password == "" && host.PrivateKey == "" && host.PrivateKeyPath == "" {
				verrs.Add(pathPrefix, "no SSH authentication method provided for non-local host")
			}
		}
	}

	if cfg.Spec.ContainerRuntime != nil {
		Validate_ContainerRuntimeConfig(cfg.Spec.ContainerRuntime, verrs, "spec.containerRuntime")
		if cfg.Spec.ContainerRuntime.Type == ContainerRuntimeContainerd {
			if cfg.Spec.ContainerRuntime.Containerd == nil {
				verrs.Add("spec.containerRuntime.containerd", fmt.Sprintf("must be defined if containerRuntime.type is '%s'", ContainerRuntimeContainerd))
			} else {
				Validate_ContainerdConfig(cfg.Spec.ContainerRuntime.Containerd, verrs, "spec.containerRuntime.containerd")
			}
		}
	}

	if cfg.Spec.Etcd != nil {
		Validate_EtcdConfig(cfg.Spec.Etcd, verrs, "spec.etcd")
	} else {
		verrs.Add("spec.etcd", "section is required")
	}

	if cfg.Spec.RoleGroups != nil {
		Validate_RoleGroupsSpec(cfg.Spec.RoleGroups, verrs, "spec.roleGroups")
		if !verrs.HasErrors() {
			allHostNames := make(map[string]bool)
			for _, h := range cfg.Spec.Hosts {
				allHostNames[h.Name] = true
			}
			validateRoleGroupHostExistence := func(roleHosts []string, rolePath string) {
				for _, hostName := range roleHosts {
					if !allHostNames[hostName] {
						verrs.Add(rolePath, fmt.Sprintf("host '%s' is not defined in spec.hosts", hostName))
					}
				}
			}
			rgPath := "spec.roleGroups"
			if cfg.Spec.RoleGroups.Master.Hosts != nil {
				validateRoleGroupHostExistence(cfg.Spec.RoleGroups.Master.Hosts, rgPath+".master.hosts")
			}
			if cfg.Spec.RoleGroups.Worker.Hosts != nil {
				validateRoleGroupHostExistence(cfg.Spec.RoleGroups.Worker.Hosts, rgPath+".worker.hosts")
			}
			if cfg.Spec.RoleGroups.Etcd.Hosts != nil {
				validateRoleGroupHostExistence(cfg.Spec.RoleGroups.Etcd.Hosts, rgPath+".etcd.hosts")
			}
			if cfg.Spec.RoleGroups.LoadBalancer.Hosts != nil {
				validateRoleGroupHostExistence(cfg.Spec.RoleGroups.LoadBalancer.Hosts, rgPath+".loadbalancer.hosts")
			}
			if cfg.Spec.RoleGroups.Storage.Hosts != nil {
				validateRoleGroupHostExistence(cfg.Spec.RoleGroups.Storage.Hosts, rgPath+".storage.hosts")
			}
			if cfg.Spec.RoleGroups.Registry.Hosts != nil {
				validateRoleGroupHostExistence(cfg.Spec.RoleGroups.Registry.Hosts, rgPath+".registry.hosts")
			}
			for i, customRole := range cfg.Spec.RoleGroups.CustomRoles {
				if customRole.Hosts != nil {
					customRolePath := fmt.Sprintf("%s.customRoles[%d:%s].hosts", rgPath, i, customRole.Name)
					validateRoleGroupHostExistence(customRole.Hosts, customRolePath)
				}
			}
		}
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
		verrs.Add("spec.kubernetes", "section is required")
	}

	if cfg.Spec.HighAvailability != nil {
		Validate_HighAvailabilityConfig(cfg.Spec.HighAvailability, verrs, "spec.highAvailability")
		// Enhanced HA validation related to Roles and ControlPlaneEndpoint
		if cfg.Spec.HighAvailability.Enabled != nil && *cfg.Spec.HighAvailability.Enabled &&
			cfg.Spec.HighAvailability.External != nil &&
			(cfg.Spec.HighAvailability.External.Type == common.ExternalLBTypeKubexmKH || cfg.Spec.HighAvailability.External.Type == common.ExternalLBTypeKubexmKN) {

			foundLBRole := false
			if cfg.Spec.RoleGroups != nil && cfg.Spec.RoleGroups.LoadBalancer.Hosts != nil && len(cfg.Spec.RoleGroups.LoadBalancer.Hosts) > 0 {
				foundLBRole = true
			} else {
				for _, host := range cfg.Spec.Hosts {
					if util.ContainsString(host.Roles, common.RoleLoadBalancer) {
						foundLBRole = true
						break
					}
				}
			}
			if !foundLBRole {
				verrs.Add("spec.highAvailability.external", fmt.Sprintf("type '%s' requires at least one host with role '%s' or hosts defined in roleGroups.loadbalancer", cfg.Spec.HighAvailability.External.Type, common.RoleLoadBalancer))
			}

			if cfg.Spec.ControlPlaneEndpoint == nil || strings.TrimSpace(cfg.Spec.ControlPlaneEndpoint.Address) == "" {
				verrs.Add("spec.controlPlaneEndpoint.address", fmt.Sprintf("must be set to the VIP address when HA type is '%s'", cfg.Spec.HighAvailability.External.Type))
			}
		}
	}
	if cfg.Spec.Preflight != nil {
		Validate_PreflightConfig(cfg.Spec.Preflight, verrs, "spec.preflight")
	}

	if cfg.Spec.Addons != nil {
		for i, addonName := range cfg.Spec.Addons {
			if strings.TrimSpace(addonName) == "" {
				verrs.Add(fmt.Sprintf("spec.addons[%d]", i), "addon name cannot be empty")
			}
		}
	}

	if cfg.Spec.Network != nil {
		Validate_NetworkConfig(cfg.Spec.Network, verrs, "spec.network", cfg.Spec.Kubernetes)
	} else {
		verrs.Add("spec.network", "section is required")
	}
	if cfg.Spec.Storage != nil {
		Validate_StorageConfig(cfg.Spec.Storage, verrs, "spec.storage")
	}
	if cfg.Spec.Registry != nil {
		Validate_RegistryConfig(cfg.Spec.Registry, verrs, "spec.registry")
	}

	Validate_DNS(&cfg.Spec.DNS, verrs, "spec.dns")

	if verrs.HasErrors() {
		return verrs
	}
	return nil
}

// DeepCopyObject implements runtime.Object.
func (c *Cluster) DeepCopyObject() runtime.Object {
	if c == nil {
		return nil
	}
	out := new(Cluster)
	c.DeepCopyInto(out)
	return out
}

// DeepCopyInto is a manually implemented deepcopy function.
func (in *Cluster) DeepCopyInto(out *Cluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
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

// DeepCopyInto for ClusterSpec - this would need to be generated or carefully written
func (in *ClusterSpec) DeepCopyInto(out *ClusterSpec) {
	*out = *in
	if in.Hosts != nil {
		inHosts, outHosts := &in.Hosts, &out.Hosts
		*outHosts = make([]HostSpec, len(*inHosts))
		for i := range *inHosts {
			(*inHosts)[i].DeepCopyInto(&(*outHosts)[i])
		}
	}
	if in.RoleGroups != nil {
		inRoleGroups, outRoleGroups := &in.RoleGroups, &out.RoleGroups
		*outRoleGroups = new(RoleGroupsSpec)
		(*inRoleGroups).DeepCopyInto(*outRoleGroups)
	}
	if in.Global != nil {
		inGlobal, outGlobal := &in.Global, &out.Global
		*outGlobal = new(GlobalSpec)
		**outGlobal = **inGlobal
	}
	if in.System != nil {
		inSystem, outSystem := &in.System, &out.System
		*outSystem = new(SystemSpec)
		(*inSystem).DeepCopyInto(*outSystem)
	}
	if in.Kubernetes != nil {
		out.Kubernetes = new(KubernetesConfig)
		in.Kubernetes.DeepCopyInto(out.Kubernetes)
	}
	if in.Etcd != nil {
		out.Etcd = new(EtcdConfig)
		in.Etcd.DeepCopyInto(out.Etcd)
	}
	in.DNS.DeepCopyInto(&out.DNS) // DNS is a value type in ClusterSpec

	if in.ContainerRuntime != nil {
		out.ContainerRuntime = new(ContainerRuntimeConfig)
		in.ContainerRuntime.DeepCopyInto(out.ContainerRuntime)
	}
	if in.Network != nil {
		out.Network = new(NetworkConfig)
		in.Network.DeepCopyInto(out.Network)
	}
	if in.ControlPlaneEndpoint != nil {
		out.ControlPlaneEndpoint = new(ControlPlaneEndpointSpec)
		// ControlPlaneEndpointSpec is a simple struct with value types, direct assignment is fine after new.
		*out.ControlPlaneEndpoint = *in.ControlPlaneEndpoint
	}
	if in.HighAvailability != nil {
		out.HighAvailability = new(HighAvailabilityConfig)
		in.HighAvailability.DeepCopyInto(out.HighAvailability)
	}
	if in.Storage != nil {
		out.Storage = new(StorageConfig)
		in.Storage.DeepCopyInto(out.Storage)
	}
	if in.Registry != nil {
		out.Registry = new(RegistryConfig)
		in.Registry.DeepCopyInto(out.Registry)
	}
	if in.Addons != nil {
		inAddons, outAddons := &in.Addons, &out.Addons
		*outAddons = make([]string, len(*inAddons))
		copy(*outAddons, *inAddons)
	}
	if in.Preflight != nil {
		out.Preflight = new(PreflightConfig)
		in.Preflight.DeepCopyInto(out.Preflight)
	}
}

// DeepCopyInto for HostSpec
func (in *HostSpec) DeepCopyInto(out *HostSpec) {
	*out = *in
	if in.Roles != nil {
		in, out := &in.Roles, &out.Roles
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Taints != nil {
		in, out := &in.Taints, &out.Taints
		*out = make([]TaintSpec, len(*in))
		copy(*out, *in) // TaintSpec is simple
	}
}

// DeepCopyInto for RoleGroupsSpec
func (in *RoleGroupsSpec) DeepCopyInto(out *RoleGroupsSpec) {
	*out = *in
	// Master, Worker, Etcd, LoadBalancer, Storage, Registry are value types within RoleGroupsSpec
	// if their underlying Host lists need deep copy, it would be handled here.
	// For now, assuming direct copy is okay or their specific types handle it.
	// Example for MasterRoleSpec if it had complex fields:
	// in.Master.DeepCopyInto(&out.Master)
	if in.CustomRoles != nil {
		in, out := &in.CustomRoles, &out.CustomRoles
		*out = make([]CustomRoleSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i]) // Assuming CustomRoleSpec has DeepCopyInto
		}
	}
}

// DeepCopyInto for CustomRoleSpec
func (in *CustomRoleSpec) DeepCopyInto(out *CustomRoleSpec) {
	*out = *in
	if in.Hosts != nil {
		in, out := &in.Hosts, &out.Hosts
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopyInto for SystemSpec
func (in *SystemSpec) DeepCopyInto(out *SystemSpec) {
	*out = *in
	if in.NTPServers != nil {
		in, out := &in.NTPServers, &out.NTPServers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.RPMs != nil {
		in, out := &in.RPMs, &out.RPMs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Debs != nil {
		in, out := &in.Debs, &out.Debs
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.PreInstallScripts != nil {
		in, out := &in.PreInstallScripts, &out.PreInstallScripts
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.PostInstallScripts != nil {
		in, out := &in.PostInstallScripts, &out.PostInstallScripts
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Modules != nil {
		in, out := &in.Modules, &out.Modules
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.SysctlParams != nil {
		in, out := &in.SysctlParams, &out.SysctlParams
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
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

func Validate_RoleGroupsSpec(cfg *RoleGroupsSpec, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	validateRoleSpecHosts := func(hosts []string, roleName string, pPrefix string) {
		for i, hostName := range hosts {
			if strings.TrimSpace(hostName) == "" {
				// Corrected path for predefined roles
				verrs.Add(fmt.Sprintf("%s.%s.hosts[%d]", pPrefix, roleName, i), "hostname cannot be empty")
			}
		}
	}

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

	if cfg.CustomRoles != nil {
		customRoleNames := make(map[string]bool)
		predefinedRoles := []string{
			common.RoleMaster, common.RoleWorker, common.RoleEtcd,
			common.RoleLoadBalancer, common.RoleStorage, common.RoleRegistry,
		}
		for i, customRole := range cfg.CustomRoles {
			// Use customRole.Name in the path for better identification
			customRolePathPrefix := fmt.Sprintf("%s.customRoles[%d:%s]", pathPrefix, i, customRole.Name)
			if strings.TrimSpace(customRole.Name) == "" {
				// If name is empty, use index for path
				customRolePathPrefixForEmptyName := fmt.Sprintf("%s.customRoles[%d]", pathPrefix, i)
				verrs.Add(customRolePathPrefixForEmptyName+".name", "custom role name cannot be empty")
			} else {
				if util.ContainsString(predefinedRoles, customRole.Name) {
					verrs.Add(customRolePathPrefix+".name", fmt.Sprintf("custom role name '%s' conflicts with a predefined role name", customRole.Name))
				}
				if _, exists := customRoleNames[customRole.Name]; exists {
					verrs.Add(customRolePathPrefix+".name", fmt.Sprintf("custom role name '%s' is duplicated", customRole.Name))
				}
				customRoleNames[customRole.Name] = true
				// Validate hosts for this custom role
				if customRole.Hosts != nil {
					// Pass the specific path for this custom role's hosts
					validateRoleSpecHosts(customRole.Hosts, "hosts", customRolePathPrefix)
				}
			}
		}
	}
}
