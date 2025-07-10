package v1alpha1

import (
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/util"
	"github.com/mensylisir/kubexm/pkg/util/validation"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=clusters,scope=Namespaced,shortName=kc
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type",description="Cluster Type"
// +kubebuilder:printcolumn:name="Version",type="string",JSONPath=".spec.kubernetes.version",description="Kubernetes Version"
// +kubebuilder:printcolumn:name="Hosts",type="integer",JSONPath=".spec.hostsCount",description="Number of hosts"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Cluster is the top-level configuration object.
type Cluster struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Spec              ClusterSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status            ClusterStatus `json:"status,omitempty" yaml:"status,omitempty"`
}

// ClusterSpec defines the desired state of the Kubernetes cluster.
// +k8s:deepcopy-gen=true
type ClusterSpec struct {
	// Type specifies the overall cluster deployment type (e.g., "kubexm", "kubeadm").
	// It influences high-level deployment strategies.
	// Defaults to common.KubernetesDeploymentTypeKubexm.
	Type common.KubernetesDeploymentType `json:"type,omitempty" yaml:"type,omitempty"`

	Hosts      []HostSpec      `json:"hosts" yaml:"hosts"`
	RoleGroups *RoleGroupsSpec `json:"roleGroups,omitempty" yaml:"roleGroups,omitempty"`
	Global     *GlobalSpec     `json:"global,omitempty" yaml:"global,omitempty"`

	ControlPlaneEndpoint *ControlPlaneEndpointSpec `json:"controlPlaneEndpoint,omitempty" yaml:"controlPlaneEndpoint,omitempty"`
	System               *SystemSpec               `json:"system,omitempty" yaml:"system,omitempty"`
	Kubernetes           *KubernetesConfig         `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"` // ContainerRuntime is now solely within KubernetesConfig
	Etcd                 *EtcdConfig               `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	Network              *NetworkConfig            `json:"network,omitempty" yaml:"network,omitempty"`
	Storage              *StorageConfig            `json:"storage,omitempty" yaml:"storage,omitempty"`
	Registry             *RegistryConfig           `json:"registry,omitempty" yaml:"registry,omitempty"`
	Addons               []string                  `json:"addons,omitempty" yaml:"addons,omitempty"`
	Dns                  *DNS                      `json:"dns,omitempty" yaml:"dns,omitempty"`
	Preflight            *PreflightConfig          `json:"preflight,omitempty" yaml:"preflight,omitempty"`
	HighAvailability     *HighAvailabilityConfig   `json:"highAvailability,omitempty" yaml:"highAvailability,omitempty"`

	// HostsFileContent allows specifying content to be appended to /etc/hosts on all nodes.
	// Corresponds to the 'host:' field in the YAML example.
	HostsFileContent string `json:"hostsFileContent,omitempty" yaml:"host,omitempty"`

	// HostsCount is a calculated field representing the number of hosts.
	// Not directly from YAML, but useful for kubectl printing.
	HostsCount int `json:"hostsCount,omitempty" yaml:"-"` // yaml:"-" to exclude from marshalling
}

// ClusterStatus defines the observed state of Cluster
// +k8s:deepcopy-gen=true
type ClusterStatus struct {
	Conditions     []ClusterCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	Version        string             `json:"version,omitempty" yaml:"version,omitempty"`
	APIServerReady bool               `json:"apiServerReady,omitempty" yaml:"apiServerReady,omitempty"`
	EtcdReady      bool               `json:"etcdReady,omitempty" yaml:"etcdReady,omitempty"`
	NodeCounts     NodeCounts         `json:"nodeCounts,omitempty" yaml:"nodeCounts,omitempty"`
}

// ClusterCondition contains details for the current condition of this cluster.
type ClusterCondition struct {
	Type               string                 `json:"type" yaml:"type"`
	Status             metav1.ConditionStatus `json:"status" yaml:"status"`
	LastTransitionTime metav1.Time            `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	Reason             string                 `json:"reason,omitempty" yaml:"reason,omitempty"`
	Message            string                 `json:"message,omitempty" yaml:"message,omitempty"`
}

// NodeCounts contains the count of nodes in various states.
type NodeCounts struct {
	Total   int `json:"total,omitempty" yaml:"total,omitempty"`
	Ready   int `json:"ready,omitempty" yaml:"ready,omitempty"`
	Master  int `json:"master,omitempty" yaml:"master,omitempty"`
	Worker  int `json:"worker,omitempty" yaml:"worker,omitempty"`
	Etcd    int `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	Storage int `json:"storage,omitempty" yaml:"storage,omitempty"`
}

// HostSpec defines the configuration for a single host.
// +k8s:deepcopy-gen=true
type HostSpec struct {
	Name            string            `json:"name" yaml:"name"`
	Address         string            `json:"address" yaml:"address"`
	InternalAddress string            `json:"internalAddress,omitempty" yaml:"internalAddress,omitempty"`
	Port            int               `json:"port,omitempty" yaml:"port,omitempty"`
	User            string            `json:"user,omitempty" yaml:"user,omitempty"`
	Password        string            `json:"password,omitempty" yaml:"password,omitempty"`
	PrivateKey      string            `json:"privateKey,omitempty" yaml:"privateKey,omitempty"`
	PrivateKeyPath  string            `json:"privateKeyPath,omitempty" yaml:"privateKeyPath,omitempty"`
	Arch            string            `json:"arch,omitempty" yaml:"arch,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Taints          []TaintSpec       `json:"taints,omitempty" yaml:"taints,omitempty"`
	Roles           []string          `json:"roles,omitempty" yaml:"roles,omitempty"`
	// Type defines the connection type (e.g., "ssh", "local").
	// Defaults to common.HostConnectionTypeSSH.
	Type common.HostConnectionType `json:"type,omitempty" yaml:"type,omitempty"`
}

// RoleGroupsSpec defines the different groups of nodes in the cluster.
// +k8s:deepcopy-gen=true
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
// +k8s:deepcopy-gen=true
type MasterRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

// WorkerRoleSpec defines the configuration for worker nodes.
// +k8s:deepcopy-gen=true
type WorkerRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

// EtcdRoleSpec defines the configuration for etcd nodes.
// +k8s:deepcopy-gen=true
type EtcdRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

// LoadBalancerRoleSpec defines the configuration for load balancer nodes.
// +k8s:deepcopy-gen=true
type LoadBalancerRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

// StorageRoleSpec defines the configuration for storage nodes.
// +k8s:deepcopy-gen=true
type StorageRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

// RegistryRoleSpec defines the configuration for registry nodes.
// +k8s:deepcopy-gen=true
type RegistryRoleSpec struct {
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

// CustomRoleSpec defines a custom role group.
// +k8s:deepcopy-gen=true
type CustomRoleSpec struct {
	Name  string   `json:"name" yaml:"name"`
	Hosts []string `json:"hosts,omitempty" yaml:"hosts,omitempty"`
}

// SystemSpec defines system-level configuration.
// +k8s:deepcopy-gen=true
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
// +k8s:deepcopy-gen=true
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
// +k8s:deepcopy-gen=true
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
	// cfg.SetGroupVersionKind(SchemeGroupVersion.WithKind("Cluster")) // This will be set by the K8s machinery

	if cfg.Spec.Type == "" {
		cfg.Spec.Type = common.KubernetesDeploymentTypeKubexm // Use typed constant
	}

	if cfg.Spec.Global == nil {
		cfg.Spec.Global = &GlobalSpec{}
	}
	g := cfg.Spec.Global
	if g.Port == 0 {
		g.Port = common.DefaultSSHPort
	}
	if g.ConnectionTimeout == 0 {
		g.ConnectionTimeout = common.DefaultConnectionTimeout
	}
	// WorkDir default is handled by runtime.Builder

	cfg.Spec.HostsCount = len(cfg.Spec.Hosts)

	for i := range cfg.Spec.Hosts {
		host := &cfg.Spec.Hosts[i]
		if host.Port == 0 {
			if g != nil && g.Port != 0 {
				host.Port = g.Port
			} else {
				host.Port = common.DefaultSSHPort
			}
		}
		if host.User == "" && g != nil {
			host.User = g.User
		}
		if host.PrivateKeyPath == "" && g != nil {
			host.PrivateKeyPath = g.PrivateKeyPath
		}
		if host.Type == "" {
			host.Type = common.HostConnectionTypeSSH // Use typed constant
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

	if cfg.Spec.RoleGroups == nil {
		cfg.Spec.RoleGroups = &RoleGroupsSpec{}
	}
	SetDefaults_RoleGroupsSpec(cfg.Spec.RoleGroups)

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
	SetDefaults_KubernetesConfig(cfg.Spec.Kubernetes, cfg.ObjectMeta.Name) // This will call SetDefaults_ContainerRuntimeConfig

	if cfg.Spec.Etcd == nil {
		cfg.Spec.Etcd = &EtcdConfig{}
	}
	SetDefaults_EtcdConfig(cfg.Spec.Etcd)

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

	if cfg.Spec.Dns == nil {
		cfg.Spec.Dns = &DNS{}
	}
	SetDefaults_DNS(cfg.Spec.Dns)
}

// SetDefaults_RoleGroupsSpec sets default values for RoleGroupsSpec
func SetDefaults_RoleGroupsSpec(cfg *RoleGroupsSpec) {
	if cfg == nil {
		return
	}
	if cfg.Master.Hosts == nil {
		cfg.Master.Hosts = []string{}
	}
	if cfg.Worker.Hosts == nil {
		cfg.Worker.Hosts = []string{}
	}
	if cfg.Etcd.Hosts == nil {
		cfg.Etcd.Hosts = []string{}
	}
	if cfg.LoadBalancer.Hosts == nil {
		cfg.LoadBalancer.Hosts = []string{}
	}
	if cfg.Storage.Hosts == nil {
		cfg.Storage.Hosts = []string{}
	}
	if cfg.Registry.Hosts == nil {
		cfg.Registry.Hosts = []string{}
	}
	if cfg.CustomRoles == nil {
		cfg.CustomRoles = []CustomRoleSpec{}
	}
	for i := range cfg.CustomRoles {
		if cfg.CustomRoles[i].Hosts == nil {
			cfg.CustomRoles[i].Hosts = []string{}
		}
	}
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
	defaultSysctl := map[string]string{
		"net.bridge.bridge-nf-call-iptables":  "1",
		"net.ipv4.ip_forward":                 "1",
		"net.bridge.bridge-nf-call-ip6tables": "1",
		"fs.inotify.max_user_watches":         "524288",
		"fs.inotify.max_user_instances":       "512",
		"vm.max_map_count":                    "262144",
	}
	for k, v := range defaultSysctl {
		if _, exists := cfg.SysctlParams[k]; !exists {
			cfg.SysctlParams[k] = v
		}
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
		if strings.TrimSpace(val) == "" {
			verrs.Add(pathPrefix+".sysctlParams", fmt.Sprintf("sysctl value for key '%s' cannot be empty", key))
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
	// APIVersion and Kind validation is often handled by K8s machinery when CRD is applied/retrieved.
	// However, explicit check can be useful if this object is constructed manually.
	// if cfg.APIVersion != SchemeGroupVersion.Group+"/"+SchemeGroupVersion.Version {
	// 	verrs.Add("apiVersion", fmt.Sprintf("must be %s/%s, got %s", SchemeGroupVersion.Group, SchemeGroupVersion.Version, cfg.APIVersion))
	// }
	// if cfg.Kind != "Cluster" {
	// 	verrs.Add("kind", fmt.Sprintf("must be Cluster, got %s", cfg.Kind))
	// }
	if strings.TrimSpace(cfg.ObjectMeta.Name) == "" {
		verrs.Add("metadata.name", "cannot be empty")
	}

	// Validate Spec.Type against common.KubernetesDeploymentType values
	if cfg.Spec.Type != common.KubernetesDeploymentTypeKubexm && cfg.Spec.Type != common.KubernetesDeploymentTypeKubeadm && cfg.Spec.Type != "" {
		verrs.Add("spec.type", fmt.Sprintf("invalid cluster type '%s', must be '%s' or '%s' or empty for default",
			cfg.Spec.Type, common.KubernetesDeploymentTypeKubexm, common.KubernetesDeploymentTypeKubeadm))
	}

	if cfg.Spec.Global != nil {
		g := cfg.Spec.Global
		if g.Port != 0 && (g.Port < 1 || g.Port > 65535) {
			verrs.Add("spec.global.port", fmt.Sprintf("%d is invalid, must be between 1 and 65535 or 0 for default", g.Port))
		}
		if g.ConnectionTimeout < 0 {
			verrs.Add("spec.global.connectionTimeout", "cannot be negative")
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
		if host.Port < 1 || host.Port > 65535 { // Port 0 is handled by defaulting
			verrs.Add(pathPrefix+".port", fmt.Sprintf("%d is invalid, must be between 1 and 65535", host.Port))
		}
		// User can be empty if global user is set or if it defaults to root (handled by connector)
		if host.Type != common.HostConnectionTypeLocal && host.Type != "" { // Allow empty for default
			if host.Password == "" && host.PrivateKey == "" && host.PrivateKeyPath == "" {
				verrs.Add(pathPrefix, "no SSH authentication method provided for non-local host")
			}
		}
		if host.Arch != "" && !util.ContainsString(common.SupportedArches, host.Arch) {
			verrs.Add(pathPrefix+".arch", fmt.Sprintf("unsupported architecture '%s', must be one of %v", host.Arch, common.SupportedArches))
		}
		// Validate HostSpec.Type
		if host.Type != common.HostConnectionTypeSSH && host.Type != common.HostConnectionTypeLocal && host.Type != "" {
			verrs.Add(pathPrefix+".type", fmt.Sprintf("invalid host type '%s', must be '%s' or '%s' or empty for default",
				host.Type, common.HostConnectionTypeSSH, common.HostConnectionTypeLocal))
		}
		for ti, taint := range host.Taints {
			taintPathPrefix := fmt.Sprintf("%s.taints[%d]", pathPrefix, ti)
			if strings.TrimSpace(taint.Key) == "" {
				verrs.Add(taintPathPrefix+".key", "taint key cannot be empty")
			}
			if !util.ContainsString(common.ValidTaintEffects, taint.Effect) {
				verrs.Add(taintPathPrefix+".effect", fmt.Sprintf("invalid taint effect '%s', must be one of %v", taint.Effect, common.ValidTaintEffects))
			}
		}
	}

	if cfg.Spec.Kubernetes == nil {
		verrs.Add("spec.kubernetes", "section is required")
	} else {
		Validate_KubernetesConfig(cfg.Spec.Kubernetes, verrs, "spec.kubernetes")
	}

	if cfg.Spec.Etcd == nil {
		verrs.Add("spec.etcd", "section is required")
	} else {
		Validate_EtcdConfig(cfg.Spec.Etcd, verrs, "spec.etcd")
	}

	if cfg.Spec.Network == nil {
		verrs.Add("spec.network", "section is required")
	} else {
		// Assuming KubernetesConfig is not nil due to above check or defaulting
		Validate_NetworkConfig(cfg.Spec.Network, verrs, "spec.network", cfg.Spec.Kubernetes)
	}

	if cfg.Spec.RoleGroups != nil {
		Validate_RoleGroupsSpec(cfg.Spec.RoleGroups, verrs, "spec.roleGroups", hostNames)
	}
	if cfg.Spec.ControlPlaneEndpoint != nil {
		Validate_ControlPlaneEndpointSpec(cfg.Spec.ControlPlaneEndpoint, verrs, "spec.controlPlaneEndpoint")
	}
	if cfg.Spec.System != nil {
		Validate_SystemSpec(cfg.Spec.System, verrs, "spec.system")
	}
	if cfg.Spec.HighAvailability != nil {
		Validate_HighAvailabilityConfig(cfg.Spec.HighAvailability, verrs, "spec.highAvailability")
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
	if cfg.Spec.Storage != nil {
		Validate_StorageConfig(cfg.Spec.Storage, verrs, "spec.storage")
	}
	if cfg.Spec.Registry != nil {
		Validate_RegistryConfig(cfg.Spec.Registry, verrs, "spec.registry")
	}
	if cfg.Spec.Dns != nil {
		Validate_DNS(cfg.Spec.Dns, verrs, "spec.dns")
	}

	if verrs.HasErrors() {
		return verrs
	}
	return nil
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterList contains a list of Cluster
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

// Validate_RoleGroupsSpec validates RoleGroupsSpec.
// It now also checks if hosts defined in role groups actually exist in the main Hosts list.
func Validate_RoleGroupsSpec(cfg *RoleGroupsSpec, verrs *validation.ValidationErrors, pathPrefix string, definedHostNames map[string]bool) {
	if cfg == nil {
		return
	}
	validateRoleSpecHosts := func(hosts []string, roleName string, pPrefix string) {
		for i, hostName := range hosts {
			currentHostPath := fmt.Sprintf("%s.%s.hosts[%d]", pPrefix, roleName, i)
			if strings.TrimSpace(hostName) == "" {
				verrs.Add(currentHostPath, "hostname cannot be empty")
			} else if definedHostNames != nil { // Only check existence if definedHostNames is provided
				expandedHosts, err := util.ExpandHostRange(hostName)
				if err != nil {
					verrs.Add(currentHostPath, fmt.Sprintf("invalid host range format '%s': %v", hostName, err))
					continue
				}
				for _, eh := range expandedHosts {
					if _, exists := definedHostNames[eh]; !exists {
						verrs.Add(currentHostPath, fmt.Sprintf("host '%s' (from range '%s') is not defined in spec.hosts", eh, hostName))
					}
				}
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
			customRolePathPrefix := fmt.Sprintf("%s.customRoles[%d:%s]", pathPrefix, i, customRole.Name)
			if strings.TrimSpace(customRole.Name) == "" {
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
				if customRole.Hosts != nil {
					validateRoleSpecHosts(customRole.Hosts, "hosts", customRolePathPrefix)
				}
			}
		}
	}
}
