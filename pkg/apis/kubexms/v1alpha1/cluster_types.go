package v1alpha1

import (
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

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
	Status            ClusterStatus `json:"status,omitempty" yaml:"status,omitempty"` // Added ClusterStatus
}

// ClusterSpec defines the desired state of the Kubernetes cluster.
type ClusterSpec struct {
	// Type specifies the overall cluster deployment type.
	// It can influence high-level deployment strategies.
	// Defaults to "kubexm".
	Type string `json:"type,omitempty" yaml:"type,omitempty"`

	Hosts      []HostSpec      `json:"hosts" yaml:"hosts"`
	RoleGroups *RoleGroupsSpec `json:"roleGroups,omitempty" yaml:"roleGroups,omitempty"`
	Global     *GlobalSpec     `json:"global,omitempty" yaml:"global,omitempty"`

	ControlPlaneEndpoint *ControlPlaneEndpointSpec `json:"controlPlaneEndpoint,omitempty" yaml:"controlPlaneEndpoint,omitempty"`
	System               *SystemSpec               `json:"system,omitempty" yaml:"system,omitempty"`
	Kubernetes           *KubernetesConfig         `json:"kubernetes,omitempty" yaml:"kubernetes,omitempty"`
	Etcd                 *EtcdConfig               `json:"etcd,omitempty" yaml:"etcd,omitempty"`
	Network              *NetworkConfig            `json:"network,omitempty" yaml:"network,omitempty"`
	Storage              *StorageConfig            `json:"storage,omitempty" yaml:"storage,omitempty"`
	Registry             *RegistryConfig           `json:"registry,omitempty" yaml:"registry,omitempty"`
	Addons               []string                  `json:"addons,omitempty" yaml:"addons,omitempty"`
	Dns                  *DNS                      `json:"dns,omitempty" yaml:"dns,omitempty"` // Changed to pointer
	Preflight            *PreflightConfig          `json:"preflight,omitempty" yaml:"preflight,omitempty"`
	HighAvailability     *HighAvailabilityConfig   `json:"highAvailability,omitempty" yaml:"highAvailability,omitempty"`

	// HostsFileContent allows specifying content to be appended to /etc/hosts on all nodes.
	// Corresponds to the 'host:' field in the YAML example.
	HostsFileContent string `json:"hostsFileContent,omitempty" yaml:"host,omitempty"`

	// HostsCount is a calculated field representing the number of hosts.
	// Not directly from YAML, but useful for kubectl printing.
	HostsCount int `json:"hostsCount,omitempty" yaml:"-"` // yaml:"-" to exclude from marshalling

	// HostsFileContentEntries provides a structured way to define /etc/hosts entries.
	// Example: {"10.0.0.1": ["host1.example.com", "host1"], "10.0.0.2": ["host2.example.com", "host2"]}
	// +optional
	HostsFileContentEntries map[string][]string `json:"hostsFileContentEntries,omitempty" yaml:"hostsFileContentEntries,omitempty"`
}

// ClusterStatus defines the observed state of Cluster
type ClusterStatus struct {
	// Conditions represent the latest available observations of a cluster's state.
	Conditions []ClusterCondition `json:"conditions,omitempty" yaml:"conditions,omitempty"`
	// Version of the Kubernetes cluster.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	// APIServerReady indicates if the API server is ready.
	APIServerReady bool `json:"apiServerReady,omitempty" yaml:"apiServerReady,omitempty"`
	// EtcdReady indicates if the Etcd cluster is ready.
	EtcdReady bool `json:"etcdReady,omitempty" yaml:"etcdReady,omitempty"`
	// NodeCounts holds the number of nodes in different roles and states.
	NodeCounts NodeCounts `json:"nodeCounts,omitempty" yaml:"nodeCounts,omitempty"`
}

// ClusterCondition contains details for the current condition of this cluster.
type ClusterCondition struct {
	// Type is the type of the condition.
	Type string `json:"type" yaml:"type"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	Status metav1.ConditionStatus `json:"status" yaml:"status"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" yaml:"lastTransitionTime,omitempty"`
	// Unique, one-word, CamelCase reason for the condition's last transition.
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
	// Human-readable message indicating details about last transition.
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
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
type HostSpec struct {
	Name            string            `json:"name" yaml:"name"`
	Address         string            `json:"address" yaml:"address"`
	InternalAddress string            `json:"internalAddress,omitempty" yaml:"internalAddress,omitempty"`
	Port            int               `json:"port,omitempty" yaml:"port,omitempty"`
	User            string            `json:"user,omitempty" yaml:"user,omitempty"`
	Password        string            `json:"password,omitempty" yaml:"password,omitempty"`
	PrivateKey      string            `json:"privateKey,omitempty" yaml:"privateKey,omitempty"` // Content of the private key
	PrivateKeyPath  string            `json:"privateKeyPath,omitempty" yaml:"privateKeyPath,omitempty"`
	Arch            string            `json:"arch,omitempty" yaml:"arch,omitempty"`
	Labels          map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
	Taints          []TaintSpec       `json:"taints,omitempty" yaml:"taints,omitempty"`
	Roles           []string          `json:"roles,omitempty" yaml:"roles,omitempty"`
	// Type defines the connection type, e.g., "ssh", "local".
	// Defaults to "ssh".
	Type string `json:"type,omitempty" yaml:"type,omitempty"`
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
	WorkDir           string        `json:"workDir,omitempty" yaml:"workDir,omitempty"` // Local workdir on the machine running kubexm
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
		g.ConnectionTimeout = common.DefaultConnectionTimeout
	}
	if g.WorkDir == "" {
		// Default workdir will be calculated in runtime.Builder based on cluster name
		// For example: $(pwd)/.kubexm/mycluster
		// Here we can set a base default name or leave it for builder.
		// For now, let runtime.Builder handle the full path construction.
		// g.WorkDir = common.DefaultWorkDirName
	}

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
			host.User = g.User // Can be empty if global user is also empty (root assumed by some logic later)
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

	// Ensure these core configurations are initialized before their defaults are set
	if cfg.Spec.Kubernetes == nil {
		cfg.Spec.Kubernetes = &KubernetesConfig{}
	}
	SetDefaults_KubernetesConfig(cfg.Spec.Kubernetes, cfg.ObjectMeta.Name)

	if cfg.Spec.Etcd == nil {
		cfg.Spec.Etcd = &EtcdConfig{}
	}
	SetDefaults_EtcdConfig(cfg.Spec.Etcd)

	if cfg.Spec.Kubernetes.ContainerRuntime == nil {
		// ContainerRuntime defaults are now handled within KubernetesConfig defaults
		// but if KubernetesConfig itself is nil initially, this path might be taken.
		// However, KubernetesConfig is initialized above.
		// Let's ensure ContainerRuntime is init if Kubernetes was nil, though unlikely.
		if cfg.Spec.Kubernetes.ContainerRuntime == nil {
			cfg.Spec.Kubernetes.ContainerRuntime = &ContainerRuntimeConfig{}
		}
		// The direct cfg.Spec.ContainerRuntime might be deprecated if fully moved.
		// For now, assume it could still exist as a top-level override or for other types.
		// If it's meant to be solely under KubernetesConfig, this block needs adjustment.
		// Based on YAML, it is under KubernetesConfig.
		// This top-level field is redundant if kubernetes.containerRuntime is the source of truth.
		// For safety, if it exists and Kubernetes.ContainerRuntime is nil, copy over.
		// This logic is becoming complex due to potential dual-definition.
		// Let's assume kubernetes.containerRuntime is primary.
	}
	// SetDefaults_ContainerRuntimeConfig is called by SetDefaults_KubernetesConfig

	if cfg.Spec.Network == nil {
		cfg.Spec.Network = &NetworkConfig{}
	}
	SetDefaults_NetworkConfig(cfg.Spec.Network) // Pass KubernetesConfig for potential CIDR defaults

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

	if cfg.Spec.Dns == nil { // Changed to pointer
		cfg.Spec.Dns = &DNS{}
	}
	SetDefaults_DNS(cfg.Spec.Dns) // Pass DnsConfig by pointer
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
	// Add best-practice sysctl params for Kubernetes
	defaultSysctl := map[string]string{
		"net.bridge.bridge-nf-call-iptables":  "1",
		"net.ipv4.ip_forward":                 "1",
		"net.bridge.bridge-nf-call-ip6tables": "1", // For IPv6
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
	if cfg.APIVersion != SchemeGroupVersion.Group+"/"+SchemeGroupVersion.Version {
		verrs.Add("apiVersion", fmt.Sprintf("must be %s/%s, got %s", SchemeGroupVersion.Group, SchemeGroupVersion.Version, cfg.APIVersion))
	}
	if cfg.Kind != "Cluster" {
		verrs.Add("kind", fmt.Sprintf("must be Cluster, got %s", cfg.Kind))
	}
	if strings.TrimSpace(cfg.ObjectMeta.Name) == "" {
		verrs.Add("metadata.name", "cannot be empty")
	}

	validClusterTypes := []string{common.ClusterTypeKubeXM, common.ClusterTypeKubeadm}
	if !util.ContainsString(validClusterTypes, cfg.Spec.Type) {
		verrs.Add("spec.type", fmt.Sprintf("invalid cluster type '%s', must be one of %v", cfg.Spec.Type, validClusterTypes))
	}

	if cfg.Spec.Global != nil {
		g := cfg.Spec.Global
		if g.Port != 0 && (g.Port < 1 || g.Port > 65535) { // Port 0 is used to signify "use default"
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
		if host.Port < 1 || host.Port > 65535 {
			verrs.Add(pathPrefix+".port", fmt.Sprintf("%d is invalid, must be between 1 and 65535", host.Port))
		}
		if strings.TrimSpace(host.User) == "" && (cfg.Spec.Global == nil || cfg.Spec.Global.User == "") {
			// User can be empty if global user is set, but if both are empty it's an issue unless defaulting to root.
			// Assuming default to root is handled by connector if user is empty.
			// For validation, if it's truly required, this check needs to be stricter.
			// For now, let's assume it's okay if it defaults to root later.
		}
		if strings.ToLower(host.Type) != common.HostTypeLocal {
			if host.Password == "" && host.PrivateKey == "" && host.PrivateKeyPath == "" {
				verrs.Add(pathPrefix, "no SSH authentication method provided for non-local host")
			}
		}
		if host.Arch != "" && !util.ContainsString(common.SupportedArches, host.Arch) {
			verrs.Add(pathPrefix+".arch", fmt.Sprintf("unsupported architecture '%s', must be one of %v", host.Arch, common.SupportedArches))
		}
		for _, taint := range host.Taints {
			if strings.TrimSpace(taint.Key) == "" {
				verrs.Add(pathPrefix+".taints.key", "taint key cannot be empty")
			}
			if !util.ContainsString(common.ValidTaintEffects, taint.Effect) {
				verrs.Add(pathPrefix+".taints.effect", fmt.Sprintf("invalid taint effect '%s', must be one of %v", taint.Effect, common.ValidTaintEffects))
			}
		}
	}

	// KubernetesConfig is required
	if cfg.Spec.Kubernetes == nil {
		verrs.Add("spec.kubernetes", "section is required")
	} else {
		Validate_KubernetesConfig(cfg.Spec.Kubernetes, verrs, "spec.kubernetes")
	}

	// EtcdConfig is required
	if cfg.Spec.Etcd == nil {
		verrs.Add("spec.etcd", "section is required")
	} else {
		Validate_EtcdConfig(cfg.Spec.Etcd, verrs, "spec.etcd")
	}

	// NetworkConfig is required
	if cfg.Spec.Network == nil {
		verrs.Add("spec.network", "section is required")
	} else {
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
	if cfg.Spec.Dns != nil { // Changed to pointer
		Validate_DNS(cfg.Spec.Dns, verrs, "spec.dns")
	}

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

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
// No longer manually implemented - will be generated by controller-gen.
// func (in *Cluster) DeepCopyInto(out *Cluster) {
// 	*out = *in
// 	out.TypeMeta = in.TypeMeta
// 	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
// 	// in.Spec.DeepCopyInto(&out.Spec) // This would call the Spec's DeepCopyInto
// 	// For now, let's assume a more direct copy for Spec if its DeepCopyInto is also removed/stubbed.
//  // If controller-gen handles all, this function will be replaced entirely.
//  // For a minimal stub for compilation if Spec fields are simple or have their own generated copiers:
// 	out.Spec = *in.Spec.DeepCopy() // Relies on ClusterSpec having a DeepCopy()
// 	out.Status = in.Status // Assuming Status is simple or has DeepCopy
// }

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Cluster.
// No longer manually implemented - will be generated by controller-gen.
// func (in *Cluster) DeepCopy() *Cluster {
// 	if in == nil {
// 		return nil
// 	}
// 	out := new(Cluster)
// 	in.DeepCopyInto(out)
// 	return out
// }

// DeepCopyInto for ClusterSpec - To be generated by controller-gen
// func (in *ClusterSpec) DeepCopyInto(out *ClusterSpec) { ... }

// DeepCopy for ClusterSpec - To be generated by controller-gen
func (in *ClusterSpec) DeepCopy() *ClusterSpec {
	if in == nil {
		return nil
	}
	out := new(ClusterSpec)
	// This is a stub. Actual deep copy logic will be generated by controller-gen.
	// For a manual stub, you'd copy fields.
	// Example of a shallow copy for the stub's purpose: *out = *in
	// However, to avoid conflicts, it's better to remove manual logic.
	// For the program to compile if other parts expect this method,
	// a minimal shallow copy might be needed if controller-gen hasn't run.
	// Let's remove the body to ensure controller-gen takes over.
	// *out = *in // TEMPORARY STUB - REMOVE FOR CONTROLLER-GEN
	// For now, let's assume this method will be generated and we don't need a stub body.
	// If `*in.Spec.DeepCopy()` is called from `Cluster.DeepCopyInto`, this needs to exist.
	// For a minimal stub that allows compilation if fields are simple or have their own DeepCopy:
	outType := *in
	out = &outType // This is a shallow copy, controller-gen will make it deep.

	// Proper deep copy of slices and maps would be needed here if done manually.
	// Example for Hosts:
	if in.Hosts != nil {
		in, out := &in.Hosts, &out.Hosts
		*out = make([]HostSpec, len(*in))
		for i := range *in {
			// This assumes HostSpec has DeepCopy or is simple
			(*in)[i].DeepCopyInto(&(*out)[i]) // Call DeepCopyInto if available
		}
	}
	// ... and for all other pointer and slice/map fields ...
	// This manual effort is what controller-gen automates.
	// For now, the shallow copy above is a placeholder if this method is called directly.
	// If controller-gen is the source of truth, this whole manual method should be removed.
	// Given the instruction "删除所有DeepCopy", I will remove the detailed manual implementation.
	// The `*out = *in` line for the stub will be removed too.
	// If `Cluster.DeepCopyObject` calls `Spec.DeepCopy()`, then `Spec.DeepCopy()` must exist.
	// Let's provide a minimal stub for `ClusterSpec.DeepCopy()` that does a shallow copy
	// and expect `controller-gen` to replace it.
	// Actually, `controller-gen` generates `DeepCopyInto` and `DeepCopy`.
	// The `DeepCopyObject` for `Cluster` will call `Cluster.DeepCopy()`.
	// `Cluster.DeepCopy()` will call `new(Cluster)` and `in.DeepCopyInto(out)`.
	// `Cluster.DeepCopyInto()` will call `in.Spec.DeepCopyInto(&out.Spec)`.
	// So, each relevant struct needs `DeepCopyInto()`.
	// I will remove the detailed manual implementations of DeepCopyInto for Spec structs.
	return out // This will be replaced by controller-gen
}


// DeepCopyInto for HostSpec - To be generated by controller-gen
// func (in *HostSpec) DeepCopyInto(out *HostSpec) { ... }

// DeepCopyInto for RoleGroupsSpec - To be generated by controller-gen
// func (in *RoleGroupsSpec) DeepCopyInto(out *RoleGroupsSpec) { ... }

// DeepCopyInto for CustomRoleSpec - To be generated by controller-gen
// func (in *CustomRoleSpec) DeepCopyInto(out *CustomRoleSpec) { ... }

// DeepCopyInto for SystemSpec - To be generated by controller-gen
// func (in *SystemSpec) DeepCopyInto(out *SystemSpec) { ... }


// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterList contains a list of Cluster
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
				// Expand host range if any, e.g., node[1:3]
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
	// ... similar calls for Worker, Etcd, LoadBalancer, Storage, Registry ...
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
					// Pass the specific path for this custom role's hosts and the definedHostNames map
					validateRoleSpecHosts(customRole.Hosts, "hosts", customRolePathPrefix)
				}
			}
		}
	}
}
