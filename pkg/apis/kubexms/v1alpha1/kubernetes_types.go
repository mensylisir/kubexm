package v1alpha1

import (
	"strings"
	"net" // Added for isValidCIDR
	// Import "k8s.io/apimachinery/pkg/runtime" if using RawExtension fields
	versionutil "k8s.io/apimachinery/pkg/util/version"
)

// KubernetesConfig defines the configuration for Kubernetes components.
type KubernetesConfig struct {
	Version                string            `json:"version"`
	ClusterName            string            `json:"clusterName,omitempty"`
	DNSDomain              string            `json:"dnsDomain,omitempty"`
	DisableKubeProxy       *bool             `json:"disableKubeProxy,omitempty"` // Default: false (proxy enabled)
	MasqueradeAll          *bool             `json:"masqueradeAll,omitempty"`    // Default: false
	MaxPods                *int32            `json:"maxPods,omitempty"`
	NodeCidrMaskSize       *int32            `json:"nodeCidrMaskSize,omitempty"`
	ApiserverCertExtraSans []string          `json:"apiserverCertExtraSans,omitempty"`
	ProxyMode              string            `json:"proxyMode,omitempty"` // e.g., "iptables", "ipvs"
	AutoRenewCerts         *bool             `json:"autoRenewCerts,omitempty"` // Default: false
	ContainerManager       string            `json:"containerManager,omitempty"` // e.g., "cgroupfs", "systemd" (for Kubelet)
	PodSubnet              string            `json:"podSubnet,omitempty"`
	ServiceSubnet          string            `json:"serviceSubnet,omitempty"`
	FeatureGates           map[string]bool   `json:"featureGates,omitempty"`

	APIServer            *APIServerConfig            `json:"apiServer,omitempty"`
	ControllerManager    *ControllerManagerConfig    `json:"controllerManager,omitempty"`
	Scheduler            *SchedulerConfig            `json:"scheduler,omitempty"`
	Kubelet              *KubeletConfig              `json:"kubelet,omitempty"`
	KubeProxy            *KubeProxyConfig            `json:"kubeProxy,omitempty"`
	Nodelocaldns         *NodelocaldnsConfig         `json:"nodelocaldns,omitempty"`
	Audit                *AuditConfig                `json:"audit,omitempty"`
	Kata                 *KataConfig                 `json:"kata,omitempty"`
	NodeFeatureDiscovery *NodeFeatureDiscoveryConfig `json:"nodeFeatureDiscovery,omitempty"`
}

// APIServerConfig holds configuration for the Kubernetes API Server.
type APIServerConfig struct {
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
	// TODO: Add more specific fields from KubeKey's APIServer type
}

// ControllerManagerConfig holds configuration for the Kubernetes Controller Manager.
type ControllerManagerConfig struct {
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
	// TODO: Add specific fields
}

// SchedulerConfig holds configuration for the Kubernetes Scheduler.
type SchedulerConfig struct {
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
	// TODO: Add specific fields
}

// KubeletConfig holds configuration for the Kubelet.
type KubeletConfig struct {
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
	// KubeletConfiguration *runtime.RawExtension `json:"kubeletConfiguration,omitempty"`
	// TODO: Add specific fields
}

// KubeProxyConfig holds configuration for KubeProxy.
type KubeProxyConfig struct {
	ExtraArgs map[string]string `json:"extraArgs,omitempty"`
	// KubeProxyConfiguration *runtime.RawExtension `json:"kubeProxyConfiguration,omitempty"`
	// TODO: Add specific fields
}

// NodelocaldnsConfig holds configuration for nodelocaldns.
type NodelocaldnsConfig struct {
	Enabled *bool `json:"enabled,omitempty"` // Default: true
}

// AuditConfig holds configuration for Kubernetes API server audit logging.
type AuditConfig struct {
	Enabled *bool `json:"enabled,omitempty"` // Default: false
	// TODO: Add fields like PolicyFile, LogPath, MaxAge, MaxBackups, MaxSize from KubeKey
}

// KataConfig holds configuration for deploying Kata Containers runtime.
type KataConfig struct {
	Enabled *bool `json:"enabled,omitempty"` // Default: false
}

// NodeFeatureDiscoveryConfig holds configuration for node-feature-discovery.
type NodeFeatureDiscoveryConfig struct {
	Enabled *bool `json:"enabled,omitempty"` // Default: false
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
	if cfg.MasqueradeAll == nil { b := false; cfg.MasqueradeAll = &b }

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
	if cfg.APIServer.ExtraArgs == nil { cfg.APIServer.ExtraArgs = make(map[string]string) }

	if cfg.ControllerManager == nil { cfg.ControllerManager = &ControllerManagerConfig{} }
	if cfg.ControllerManager.ExtraArgs == nil { cfg.ControllerManager.ExtraArgs = make(map[string]string) }

	if cfg.Scheduler == nil { cfg.Scheduler = &SchedulerConfig{} }
	if cfg.Scheduler.ExtraArgs == nil { cfg.Scheduler.ExtraArgs = make(map[string]string) }

	if cfg.Kubelet == nil { cfg.Kubelet = &KubeletConfig{} }
	if cfg.Kubelet.ExtraArgs == nil { cfg.Kubelet.ExtraArgs = make(map[string]string) }

	if cfg.KubeProxy == nil { cfg.KubeProxy = &KubeProxyConfig{} }
	if cfg.KubeProxy.ExtraArgs == nil { cfg.KubeProxy.ExtraArgs = make(map[string]string) }
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
	// TODO: Add validation for APIServerConfig, ControllerManagerConfig, etc.
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
