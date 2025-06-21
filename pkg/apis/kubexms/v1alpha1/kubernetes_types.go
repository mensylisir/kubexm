package v1alpha1

import (
	"strings"
	"net" // Added for isValidCIDR
	"k8s.io/apimachinery/pkg/runtime" // Added for RawExtension
	versionutil "k8s.io/apimachinery/pkg/util/version"
)

// KubernetesConfig defines the configuration for Kubernetes components.
type KubernetesConfig struct {
	Version                string            `json:"version" yaml:"version"`
	ClusterName            string            `json:"clusterName,omitempty" yaml:"clusterName,omitempty"`
	DNSDomain              string            `json:"dnsDomain,omitempty" yaml:"dnsDomain,omitempty"`
	DisableKubeProxy       *bool             `json:"disableKubeProxy,omitempty" yaml:"disableKubeProxy,omitempty"`
	MasqueradeAll          *bool             `json:"masqueradeAll,omitempty" yaml:"masqueradeAll,omitempty"`
	MaxPods                *int32            `json:"maxPods,omitempty" yaml:"maxPods,omitempty"`
	NodeCidrMaskSize       *int32            `json:"nodeCidrMaskSize,omitempty" yaml:"nodeCidrMaskSize,omitempty"` // YAML might be nodeCIDRMaskSize
	ApiserverCertExtraSans []string          `json:"apiserverCertExtraSans,omitempty" yaml:"apiserverCertExtraSans,omitempty"` // YAML might be apiServerCertExtraSANs
	ProxyMode              string            `json:"proxyMode,omitempty" yaml:"proxyMode,omitempty"`
	AutoRenewCerts         *bool             `json:"autoRenewCerts,omitempty" yaml:"autoRenewCerts,omitempty"`
	ContainerManager       string            `json:"containerManager,omitempty" yaml:"containerManager,omitempty"`
	PodSubnet              string            `json:"podSubnet,omitempty" yaml:"podSubnet,omitempty"` // Often kubePodsCIDR in network section
	ServiceSubnet          string            `json:"serviceSubnet,omitempty" yaml:"serviceSubnet,omitempty"` // Often kubeServiceCIDR in network section
	FeatureGates           map[string]bool   `json:"featureGates,omitempty" yaml:"featureGates,omitempty"`

	APIServer            *APIServerConfig            `json:"apiServer,omitempty" yaml:"apiServer,omitempty"`
	ControllerManager    *ControllerManagerConfig    `json:"controllerManager,omitempty" yaml:"controllerManager,omitempty"`
	Scheduler            *SchedulerConfig            `json:"scheduler,omitempty" yaml:"scheduler,omitempty"`
	Kubelet              *KubeletConfig              `json:"kubelet,omitempty" yaml:"kubelet,omitempty"`
	KubeProxy            *KubeProxyConfig            `json:"kubeProxy,omitempty" yaml:"kubeProxy,omitempty"`
	KubeletConfiguration *runtime.RawExtension       `json:"kubeletConfiguration,omitempty" yaml:"kubeletConfiguration,omitempty"`
	KubeProxyConfiguration *runtime.RawExtension     `json:"kubeProxyConfiguration,omitempty" yaml:"kubeProxyConfiguration,omitempty"`
	Nodelocaldns         *NodelocaldnsConfig         `json:"nodelocaldns,omitempty" yaml:"nodelocaldns,omitempty"`
	Audit                *AuditConfig                `json:"audit,omitempty" yaml:"audit,omitempty"`
	Kata                 *KataConfig                 `json:"kata,omitempty" yaml:"kata,omitempty"`
	NodeFeatureDiscovery *NodeFeatureDiscoveryConfig `json:"nodeFeatureDiscovery,omitempty" yaml:"nodeFeatureDiscovery,omitempty"`
}

// APIServerConfig holds configuration for the Kubernetes API Server.
type APIServerConfig struct {
	ExtraArgs            []string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	EtcdServers          []string `json:"etcdServers,omitempty" yaml:"etcdServers,omitempty"`
	EtcdCAFile           string   `json:"etcdCAFile,omitempty" yaml:"etcdCAFile,omitempty"`
	EtcdCertFile         string   `json:"etcdCertFile,omitempty" yaml:"etcdCertFile,omitempty"`
	EtcdKeyFile          string   `json:"etcdKeyFile,omitempty" yaml:"etcdKeyFile,omitempty"`
	AdmissionPlugins     []string `json:"admissionPlugins,omitempty" yaml:"admissionPlugins,omitempty"`
	ServiceNodePortRange string   `json:"serviceNodePortRange,omitempty" yaml:"serviceNodePortRange,omitempty"`
}

// ControllerManagerConfig holds configuration for the Kubernetes Controller Manager.
type ControllerManagerConfig struct {
	ExtraArgs                    []string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	ServiceAccountPrivateKeyFile string   `json:"serviceAccountPrivateKeyFile,omitempty" yaml:"serviceAccountPrivateKeyFile,omitempty"`
}

// SchedulerConfig holds configuration for the Kubernetes Scheduler.
type SchedulerConfig struct {
	ExtraArgs        []string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	PolicyConfigFile string   `json:"policyConfigFile,omitempty" yaml:"policyConfigFile,omitempty"`
}

// KubeletConfig holds configuration for the Kubelet.
type KubeletConfig struct {
	ExtraArgs        []string            `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	CgroupDriver     *string             `json:"cgroupDriver,omitempty" yaml:"cgroupDriver,omitempty"`
	EvictionHard     map[string]string   `json:"evictionHard,omitempty" yaml:"evictionHard,omitempty"`
	HairpinMode      *string             `json:"hairpinMode,omitempty" yaml:"hairpinMode,omitempty"`
}

// KubeProxyIPTablesConfig defines specific configuration for KubeProxy in IPTables mode.
type KubeProxyIPTablesConfig struct {
   MasqueradeAll *bool  `json:"masqueradeAll,omitempty" yaml:"masqueradeAll,omitempty"`
   MasqueradeBit *int32 `json:"masqueradeBit,omitempty" yaml:"masqueradeBit,omitempty"`
   SyncPeriod    string `json:"syncPeriod,omitempty" yaml:"syncPeriod,omitempty"`
   MinSyncPeriod string `json:"minSyncPeriod,omitempty" yaml:"minSyncPeriod,omitempty"`
}

// KubeProxyIPVSConfig defines specific configuration for KubeProxy in IPVS mode.
type KubeProxyIPVSConfig struct {
   Scheduler     string   `json:"scheduler,omitempty" yaml:"scheduler,omitempty"`
   SyncPeriod    string   `json:"syncPeriod,omitempty" yaml:"syncPeriod,omitempty"`
   MinSyncPeriod string   `json:"minSyncPeriod,omitempty" yaml:"minSyncPeriod,omitempty"`
   ExcludeCIDRs  []string `json:"excludeCIDRs,omitempty" yaml:"excludeCIDRs,omitempty"` // Matches kubeProxyConfiguration.ipvs.excludeCIDRs from prompt
}

// KubeProxyConfig holds configuration for KubeProxy.
type KubeProxyConfig struct {
	ExtraArgs    []string                 `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	IPTables     *KubeProxyIPTablesConfig `json:"ipTables,omitempty" yaml:"ipTables,omitempty"`
	IPVS         *KubeProxyIPVSConfig     `json:"ipvs,omitempty" yaml:"ipvs,omitempty"`
}

// NodelocaldnsConfig holds configuration for nodelocaldns.
type NodelocaldnsConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// AuditConfig holds configuration for Kubernetes API server audit logging.
type AuditConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// KataConfig holds configuration for deploying Kata Containers runtime.
type KataConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// NodeFeatureDiscoveryConfig holds configuration for node-feature-discovery.
type NodeFeatureDiscoveryConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// --- Defaulting Functions ---

// SetDefaults_KubernetesConfig sets default values for KubernetesConfig.
// clusterMetaName is the Name from the parent Cluster's ObjectMeta, used for defaulting KubernetesConfig.ClusterName.
func SetDefaults_KubernetesConfig(cfg *KubernetesConfig, clusterMetaName string) {
	if cfg == nil {
		return
	}
	if cfg.ClusterName == "" && clusterMetaName != "" {
		cfg.ClusterName = clusterMetaName
	}
	if cfg.DNSDomain == "" {
		cfg.DNSDomain = "cluster.local"
	}
	if cfg.ProxyMode == "" {
		cfg.ProxyMode = "iptables"
	}
	if cfg.AutoRenewCerts == nil { b := false; cfg.AutoRenewCerts = &b }
	if cfg.DisableKubeProxy == nil { b := false; cfg.DisableKubeProxy = &b }
	if cfg.MasqueradeAll == nil { b := false; cfg.MasqueradeAll = &b } // YAML: false
	if cfg.MaxPods == nil { mp := int32(110); cfg.MaxPods = &mp } // YAML: 110
	if cfg.NodeCidrMaskSize == nil { ncms := int32(24); cfg.NodeCidrMaskSize = &ncms } // YAML: 24
	if cfg.ContainerManager == "" { cfg.ContainerManager = "systemd" } // Changed default to systemd

	if cfg.Nodelocaldns == nil { cfg.Nodelocaldns = &NodelocaldnsConfig{} }
	if cfg.Nodelocaldns.Enabled == nil { b := true; cfg.Nodelocaldns.Enabled = &b }

	if cfg.Audit == nil { cfg.Audit = &AuditConfig{} }
	if cfg.Audit.Enabled == nil { b := false; cfg.Audit.Enabled = &b }

	if cfg.Kata == nil { cfg.Kata = &KataConfig{} }
	if cfg.Kata.Enabled == nil { b := false; cfg.Kata.Enabled = &b }

	if cfg.NodeFeatureDiscovery == nil { cfg.NodeFeatureDiscovery = &NodeFeatureDiscoveryConfig{} }
	if cfg.NodeFeatureDiscovery.Enabled == nil { b := false; cfg.NodeFeatureDiscovery.Enabled = &b }

	if cfg.FeatureGates == nil { cfg.FeatureGates = make(map[string]bool) }

	if cfg.APIServer == nil { cfg.APIServer = &APIServerConfig{} }
	if cfg.APIServer.ExtraArgs == nil { cfg.APIServer.ExtraArgs = []string{} }
	if cfg.APIServer.AdmissionPlugins == nil { cfg.APIServer.AdmissionPlugins = []string{} }

	if cfg.ControllerManager == nil { cfg.ControllerManager = &ControllerManagerConfig{} }
	if cfg.ControllerManager.ExtraArgs == nil { cfg.ControllerManager.ExtraArgs = []string{} }

	if cfg.Scheduler == nil { cfg.Scheduler = &SchedulerConfig{} }
	if cfg.Scheduler.ExtraArgs == nil { cfg.Scheduler.ExtraArgs = []string{} }

	if cfg.Kubelet == nil { cfg.Kubelet = &KubeletConfig{} }
	if cfg.Kubelet.ExtraArgs == nil { cfg.Kubelet.ExtraArgs = []string{} }
	if cfg.Kubelet.EvictionHard == nil { cfg.Kubelet.EvictionHard = make(map[string]string) }
	// Default Kubelet CgroupDriver based on ContainerManager if set
	if cfg.Kubelet.CgroupDriver == nil && cfg.ContainerManager != "" {
		cfg.Kubelet.CgroupDriver = &cfg.ContainerManager
	} else if cfg.Kubelet.CgroupDriver == nil {
		defDriver := "systemd"; cfg.Kubelet.CgroupDriver = &defDriver // Fallback default
	}

	if cfg.KubeProxy == nil { cfg.KubeProxy = &KubeProxyConfig{} }
	if cfg.KubeProxy.ExtraArgs == nil { cfg.KubeProxy.ExtraArgs = []string{} }
	if cfg.ProxyMode == "iptables" && cfg.KubeProxy.IPTables == nil { // Default IPTables config if mode is iptables
		 cfg.KubeProxy.IPTables = &KubeProxyIPTablesConfig{}
	}
	if cfg.KubeProxy.IPTables != nil {
		 if cfg.KubeProxy.IPTables.MasqueradeAll == nil { b := true; cfg.KubeProxy.IPTables.MasqueradeAll = &b } // KubeProxy default
		 if cfg.KubeProxy.IPTables.MasqueradeBit == nil { mb := int32(14); cfg.KubeProxy.IPTables.MasqueradeBit = &mb } // KubeProxy default
	}
	if cfg.ProxyMode == "ipvs" && cfg.KubeProxy.IPVS == nil { // Default IPVS config if mode is ipvs
		 cfg.KubeProxy.IPVS = &KubeProxyIPVSConfig{}
	}
	if cfg.KubeProxy.IPVS != nil {
		 if cfg.KubeProxy.IPVS.Scheduler == "" { sched := "rr"; cfg.KubeProxy.IPVS.Scheduler = sched }
		 if cfg.KubeProxy.IPVS.ExcludeCIDRs == nil { cfg.KubeProxy.IPVS.ExcludeCIDRs = []string{} }
	}
}

// --- Validation Functions ---

// Validate_KubernetesConfig validates KubernetesConfig.
func Validate_KubernetesConfig(cfg *KubernetesConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		verrs.Add("%s: kubernetes configuration section cannot be nil", pathPrefix)
		return
	}
	if strings.TrimSpace(cfg.Version) == "" {
		verrs.Add("%s.version: cannot be empty", pathPrefix)
	} else if !strings.HasPrefix(cfg.Version, "v") {
		verrs.Add("%s.version: must start with 'v' (e.g., v1.23.4), got '%s'", pathPrefix, cfg.Version)
	}
	if strings.TrimSpace(cfg.DNSDomain) == "" {
		verrs.Add("%s.dnsDomain: cannot be empty", pathPrefix)
	}

	validProxyModes := []string{"iptables", "ipvs", ""}
	isValidMode := false
	for _, m := range validProxyModes { if cfg.ProxyMode == m { isValidMode = true; break } }
	if !isValidMode {
		verrs.Add("%s.proxyMode: invalid mode '%s', must be one of %v or empty for default", pathPrefix, cfg.ProxyMode, validProxyModes)
	}

	if cfg.PodSubnet != "" && !isValidCIDR(cfg.PodSubnet) {
	   verrs.Add("%s.podSubnet: invalid CIDR format '%s'", pathPrefix, cfg.PodSubnet)
	}
	if cfg.ServiceSubnet != "" && !isValidCIDR(cfg.ServiceSubnet) {
	   verrs.Add("%s.serviceSubnet: invalid CIDR format '%s'", pathPrefix, cfg.ServiceSubnet)
	}

	if cfg.APIServer != nil { Validate_APIServerConfig(cfg.APIServer, verrs, pathPrefix+".apiServer") }
	if cfg.ControllerManager != nil { Validate_ControllerManagerConfig(cfg.ControllerManager, verrs, pathPrefix+".controllerManager") }
	if cfg.Scheduler != nil { Validate_SchedulerConfig(cfg.Scheduler, verrs, pathPrefix+".scheduler") }
	if cfg.Kubelet != nil { Validate_KubeletConfig(cfg.Kubelet, verrs, pathPrefix+".kubelet") }
	if cfg.KubeProxy != nil { Validate_KubeProxyConfig(cfg.KubeProxy, verrs, pathPrefix+".kubeProxy", cfg.ProxyMode) }

	if cfg.ContainerManager != "" && cfg.ContainerManager != "cgroupfs" && cfg.ContainerManager != "systemd" {
		verrs.Add("%s.containerManager: must be 'cgroupfs' or 'systemd', got '%s'", pathPrefix, cfg.ContainerManager)
	}
	if cfg.KubeletConfiguration != nil && len(cfg.KubeletConfiguration.Raw) == 0 {
		verrs.Add("%s.kubeletConfiguration: raw data cannot be empty if section is present", pathPrefix)
	}
	if cfg.KubeProxyConfiguration != nil && len(cfg.KubeProxyConfiguration.Raw) == 0 {
		verrs.Add("%s.kubeProxyConfiguration: raw data cannot be empty if section is present", pathPrefix)
	}
}

func Validate_APIServerConfig(cfg *APIServerConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	// Validate EtcdServers (e.g. valid URLs)
	// Validate AdmissionPlugins against known list or format
	if cfg.ServiceNodePortRange != "" {
	   parts := strings.Split(cfg.ServiceNodePortRange, "-")
	   if len(parts) != 2 { // Basic check
		   verrs.Add("%s.serviceNodePortRange: invalid format '%s', expected 'min-max'", pathPrefix, cfg.ServiceNodePortRange)
	   } // Further checks for numbers and min < max could be added
	}
}
func Validate_ControllerManagerConfig(cfg *ControllerManagerConfig, verrs *ValidationErrors, pathPrefix string) { if cfg == nil {return} /* TODO */ }
func Validate_SchedulerConfig(cfg *SchedulerConfig, verrs *ValidationErrors, pathPrefix string) { if cfg == nil {return} /* TODO */ }

func Validate_KubeletConfig(cfg *KubeletConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	if cfg.CgroupDriver != nil && *cfg.CgroupDriver != "cgroupfs" && *cfg.CgroupDriver != "systemd" {
	   verrs.Add("%s.cgroupDriver: must be 'cgroupfs' or 'systemd' if specified, got '%s'", pathPrefix, *cfg.CgroupDriver)
	}
	validHairpinModes := []string{"promiscuous-bridge", "hairpin-veth", "none", ""}
	if cfg.HairpinMode != nil && !contains(validHairpinModes, *cfg.HairpinMode) { // contains() from network_types.go
	   verrs.Add("%s.hairpinMode: invalid mode '%s'", pathPrefix, *cfg.HairpinMode)
	}
	// Validate EvictionHard map keys/values if needed
}
func Validate_KubeProxyConfig(cfg *KubeProxyConfig, verrs *ValidationErrors, pathPrefix string, parentProxyMode string) {
	if cfg == nil { return }
	if parentProxyMode == "iptables" && cfg.IPTables == nil {
		// verrs.Add("%s.ipTables: cannot be nil if kubernetes.proxyMode is 'iptables'", pathPrefix) // Defaulting handles this
	}
	if parentProxyMode == "ipvs" && cfg.IPVS == nil {
		// verrs.Add("%s.ipvs: cannot be nil if kubernetes.proxyMode is 'ipvs'", pathPrefix) // Defaulting handles this
	}
	if cfg.IPTables != nil && cfg.IPTables.MasqueradeBit != nil && (*cfg.IPTables.MasqueradeBit < 0 || *cfg.IPTables.MasqueradeBit > 31) {
	   verrs.Add("%s.ipTables.masqueradeBit: must be between 0 and 31, got %d", pathPrefix, *cfg.IPTables.MasqueradeBit)
	}
	// Add more validation for IPVS scheduler, sync periods (time format)
}


func isValidCIDR(cidr string) bool { _, _, err := net.ParseCIDR(cidr); return err == nil }

// --- Helper Methods ---
func (k *KubernetesConfig) IsKubeProxyDisabled() bool {
	if k != nil && k.DisableKubeProxy != nil { return *k.DisableKubeProxy }
	return false
}
func (k *KubernetesConfig) IsNodelocaldnsEnabled() bool {
	if k != nil && k.Nodelocaldns != nil && k.Nodelocaldns.Enabled != nil { return *k.Nodelocaldns.Enabled }
	return true
}
func (k *KubernetesConfig) IsAuditEnabled() bool {
	if k != nil && k.Audit != nil && k.Audit.Enabled != nil { return *k.Audit.Enabled }
	return false
}
func (k *KubernetesConfig) IsKataEnabled() bool {
	if k != nil && k.Kata != nil && k.Kata.Enabled != nil { return *k.Kata.Enabled }
	return false
}
func (k *KubernetesConfig) IsNodeFeatureDiscoveryEnabled() bool {
	if k != nil && k.NodeFeatureDiscovery != nil && k.NodeFeatureDiscovery.Enabled != nil { return *k.NodeFeatureDiscovery.Enabled }
	return false
}
func (k *KubernetesConfig) IsAutoRenewCertsEnabled() bool {
   if k != nil && k.AutoRenewCerts != nil { return *k.AutoRenewCerts }
   return false
}
func (k *KubernetesConfig) GetMaxPods() int32 {
   if k != nil && k.MaxPods != nil { return *k.MaxPods }
   // Kubelet's default is 110. If MaxPods is nil, this helper could return that default.
   return 110
}
// IsAtLeastVersion compares the KubernetesConfig's Version field against a given semantic version string.
// Example: IsAtLeastVersion("v1.24.0")
func (k *KubernetesConfig) IsAtLeastVersion(versionStr string) bool {
	if k == nil || k.Version == "" { return false }
	parsedVersion, err := versionutil.ParseGeneric(k.Version)
	if err != nil { return false } // Or handle error, e.g., log it

	compareVersion, err := versionutil.ParseGeneric(versionStr) // Use ParseGeneric for flexibility (e.g. "v1.24")
	if err != nil { return false } // Or handle error

	return parsedVersion.AtLeast(compareVersion)
}
