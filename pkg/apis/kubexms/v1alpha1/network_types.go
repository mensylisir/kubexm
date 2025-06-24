package v1alpha1

import (
	"fmt" // For path formatting in validation
	// "net" // Removed as unused locally, assuming isValidCIDR is from elsewhere
	"strings"
)

// NetworkConfig defines the network configuration for the cluster.
type NetworkConfig struct {
	Plugin          string `json:"plugin,omitempty" yaml:"plugin,omitempty"`
	KubePodsCIDR    string `json:"kubePodsCIDR,omitempty" yaml:"kubePodsCIDR,omitempty"`
	KubeServiceCIDR string `json:"kubeServiceCIDR,omitempty" yaml:"kubeServiceCIDR,omitempty"`

	Calico *CalicoConfig `json:"calico,omitempty" yaml:"calico,omitempty"`
	// Cilium specific configuration.
	// Only used when Plugin is "cilium".
	// +optional
	Cilium    *CiliumConfig    `json:"cilium,omitempty"` // [!code ++]
	Flannel   *FlannelConfig   `json:"flannel,omitempty" yaml:"flannel,omitempty"`
	KubeOvn   *KubeOvnConfig   `json:"kubeovn,omitempty" yaml:"kubeovn,omitempty"`
	Multus    *MultusCNIConfig `json:"multus,omitempty" yaml:"multus,omitempty"`
	Hybridnet *HybridnetConfig `json:"hybridnet,omitempty" yaml:"hybridnet,omitempty"`
	IPPool    *IPPoolConfig    `json:"ippool,omitempty" yaml:"ippool,omitempty"`
}

// IPPoolConfig holds general IP pool configuration.
// Corresponds to `network.ippool` in YAML.
type IPPoolConfig struct {
	BlockSize *int `json:"blockSize,omitempty" yaml:"blockSize,omitempty"`
}

// CalicoIPPool defines an IP address pool for Calico.
// Corresponds to entries in `network.calico.ipPools` in YAML.
type CalicoIPPool struct {
	Name          string `json:"name,omitempty" yaml:"name,omitempty"`
	CIDR          string `json:"cidr" yaml:"cidr"`
	Encapsulation string `json:"encapsulation,omitempty" yaml:"encapsulation,omitempty"`
	NatOutgoing   *bool  `json:"natOutgoing,omitempty" yaml:"natOutgoing,omitempty"`
	BlockSize     *int   `json:"blockSize,omitempty" yaml:"blockSize,omitempty"`
}

// CalicoConfig defines settings specific to the Calico CNI plugin.
type CalicoConfig struct {
	IPIPMode          string            `json:"ipipMode,omitempty" yaml:"ipipMode,omitempty"`
	VXLANMode         string            `json:"vxlanMode,omitempty" yaml:"vxlanMode,omitempty"`
	VethMTU           *int              `json:"vethMTU,omitempty" yaml:"vethMTU,omitempty"`
	IPv4NatOutgoing   *bool             `json:"ipv4NatOutgoing,omitempty" yaml:"ipv4NatOutgoing,omitempty"`
	DefaultIPPOOL     *bool             `json:"defaultIPPOOL,omitempty" yaml:"defaultIPPOOL,omitempty"`
	EnableTypha       *bool             `json:"enableTypha,omitempty" yaml:"enableTypha,omitempty"`
	TyphaReplicas     *int              `json:"typhaReplicas,omitempty" yaml:"typhaReplicas,omitempty"`
	TyphaNodeSelector map[string]string `json:"typhaNodeSelector,omitempty" yaml:"typhaNodeSelector,omitempty"`
	LogSeverityScreen *string           `json:"logSeverityScreen,omitempty" yaml:"logSeverityScreen,omitempty"`
	IPPools           []CalicoIPPool    `json:"ipPools,omitempty" yaml:"ipPools,omitempty"`
}

// CiliumConfig holds the specific configuration for the Cilium CNI plugin.
// [!code ++]
type CiliumConfig struct {
	// TunnelingMode specifies the encapsulation mode for traffic between nodes.
	// Supported values: "vxlan" (default), "geneve", "disabled" (direct routing).
	// +optional
	TunnelingMode string `json:"tunnelingMode,omitempty"`

	// KubeProxyReplacement enables Cilium's eBPF-based kube-proxy replacement.
	// This provides better performance and features.
	// Supported values: "probe", "strict" (default), "disabled".
	// +optional
	KubeProxyReplacement string `json:"kubeProxyReplacement,omitempty"`

	// EnableHubble enables the Hubble observability platform.
	// +optional
	EnableHubble bool `json:"enableHubble,omitempty"`

	// HubbleUI enables the deployment of the Hubble UI.
	// Requires EnableHubble to be true.
	// +optional
	HubbleUI bool `json:"hubbleUI,omitempty"`

	// EnableBPFMasquerade enables eBPF-based masquerading for traffic leaving the cluster.
	// This is more efficient than traditional iptables-based masquerading.
	// +optional
	EnableBPFMasquerade bool `json:"enableBPFMasquerade,omitempty"`

	// IdentityAllocationMode specifies how Cilium identities are allocated.
	// "crd" is the standard mode. "kvstore" can be used for very large clusters.
	// +optional
	IdentityAllocationMode string `json:"identityAllocationMode,omitempty"`
}

// FlannelConfig defines settings specific to the Flannel CNI plugin.
type FlannelConfig struct {
	BackendMode   string `json:"backendMode,omitempty" yaml:"backendMode,omitempty"`
	DirectRouting *bool  `json:"directRouting,omitempty" yaml:"directRouting,omitempty"`
}

type KubeOvnConfig struct {
	Enabled    *bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	JoinCIDR   *string `json:"joinCIDR,omitempty" yaml:"joinCIDR,omitempty"`
	Label      *string `json:"label,omitempty" yaml:"label,omitempty"`
	TunnelType *string `json:"tunnelType,omitempty" yaml:"tunnelType,omitempty"`
	EnableSSL  *bool   `json:"enableSSL,omitempty" yaml:"enableSSL,omitempty"`
}

type MultusCNIConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

type HybridnetConfig struct {
	Enabled             *bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	DefaultNetworkType  *string `json:"defaultNetworkType,omitempty" yaml:"defaultNetworkType,omitempty"`
	EnableNetworkPolicy *bool   `json:"enableNetworkPolicy,omitempty" yaml:"enableNetworkPolicy,omitempty"`
	InitDefaultNetwork  *bool   `json:"initDefaultNetwork,omitempty" yaml:"initDefaultNetwork,omitempty"`
}

// --- Defaulting Functions ---

func SetDefaults_NetworkConfig(cfg *NetworkConfig) {
	if cfg == nil {
		return
	}
	if cfg.Plugin == "" {
		cfg.Plugin = "calico" // Default plugin to Calico
	}

	if cfg.IPPool == nil {
		cfg.IPPool = &IPPoolConfig{}
	}
	if cfg.IPPool.BlockSize == nil {
		defaultBlockSize := 26 // Default from YAML example
		cfg.IPPool.BlockSize = &defaultBlockSize
	}

	if cfg.Plugin == "calico" {
		if cfg.Calico == nil {
			cfg.Calico = &CalicoConfig{}
		}
		// Pass the globally configured default blockSize to Calico defaults
		var defaultCalicoBlockSize *int
		if cfg.IPPool != nil { // IPPool is already defaulted above
			defaultCalicoBlockSize = cfg.IPPool.BlockSize
		}
		SetDefaults_CalicoConfig(cfg.Calico, cfg.KubePodsCIDR, defaultCalicoBlockSize)
	}
	if cfg.Plugin == "flannel" {
		if cfg.Flannel == nil {
			cfg.Flannel = &FlannelConfig{}
		}
		SetDefaults_FlannelConfig(cfg.Flannel)
	}

	if cfg.Multus == nil {
		cfg.Multus = &MultusCNIConfig{}
	}
	if cfg.Multus.Enabled == nil {
		b := false
		cfg.Multus.Enabled = &b
	}

	if cfg.KubeOvn == nil {
		cfg.KubeOvn = &KubeOvnConfig{}
	}
	SetDefaults_KubeOvnConfig(cfg.KubeOvn)

	if cfg.Hybridnet == nil {
		cfg.Hybridnet = &HybridnetConfig{}
	}
	SetDefaults_HybridnetConfig(cfg.Hybridnet)
}

func SetDefaults_CalicoConfig(cfg *CalicoConfig, defaultPoolCIDR string, globalDefaultBlockSize *int) {
	if cfg == nil {
		return
	}
	if cfg.IPIPMode == "" {
		cfg.IPIPMode = "Always"
	}
	if cfg.VXLANMode == "" {
		cfg.VXLANMode = "Never"
	}
	if cfg.IPv4NatOutgoing == nil {
		b := true
		cfg.IPv4NatOutgoing = &b
	}
	if cfg.DefaultIPPOOL == nil {
		b := true
		cfg.DefaultIPPOOL = &b
	}
	if cfg.EnableTypha == nil {
		b := false
		cfg.EnableTypha = &b
	}
	if cfg.EnableTypha != nil && *cfg.EnableTypha && cfg.TyphaReplicas == nil {
		var defaultReplicas int = 2
		cfg.TyphaReplicas = &defaultReplicas
	}
	if cfg.TyphaNodeSelector == nil {
		cfg.TyphaNodeSelector = make(map[string]string)
	}
	if cfg.VethMTU == nil {
		var defaultMTU int = 0
		cfg.VethMTU = &defaultMTU
	}
	if cfg.LogSeverityScreen == nil {
		s := "Info"
		cfg.LogSeverityScreen = &s
	}

	if len(cfg.IPPools) == 0 && cfg.DefaultIPPOOL != nil && *cfg.DefaultIPPOOL && defaultPoolCIDR != "" {
		var bs *int
		if globalDefaultBlockSize != nil {
			bs = globalDefaultBlockSize // Use global default if provided
		} else {
			// Fallback if globalDefaultBlockSize is nil, though SetDefaults_NetworkConfig should provide it
			defaultInternalBlockSize := 26
			bs = &defaultInternalBlockSize
		}
		cfg.IPPools = append(cfg.IPPools, CalicoIPPool{
			Name:          "default-ipv4-ippool",
			CIDR:          defaultPoolCIDR,
			Encapsulation: cfg.IPIPMode, // Default encapsulation based on Calico settings
			NatOutgoing:   cfg.IPv4NatOutgoing,
			BlockSize:     bs,
		})
	}
	for i := range cfg.IPPools {
		pool := &cfg.IPPools[i]
		if pool.Encapsulation == "" {
			if cfg.IPIPMode == "Always" {
				pool.Encapsulation = "IPIP"
			} else if cfg.VXLANMode == "Always" {
				pool.Encapsulation = "VXLAN"
			} else {
				pool.Encapsulation = "None"
			}
		}
		if pool.NatOutgoing == nil {
			pool.NatOutgoing = cfg.IPv4NatOutgoing
		}
		if pool.BlockSize == nil {
			bs := 26
			pool.BlockSize = &bs
		}
	}
} // Corrected: Added missing closing brace for SetDefaults_CalicoConfig

func SetDefaults_FlannelConfig(cfg *FlannelConfig) {
	if cfg == nil {
		return
	}
	if cfg.BackendMode == "" {
		cfg.BackendMode = "vxlan"
	}
	if cfg.DirectRouting == nil {
		b := false
		cfg.DirectRouting = &b
	}
}

func SetDefaults_KubeOvnConfig(cfg *KubeOvnConfig) {
	if cfg == nil {
		return
	}
	if cfg.Enabled == nil {
		b := false
		cfg.Enabled = &b
	}
	if cfg.Enabled != nil && *cfg.Enabled {
		if cfg.Label == nil {
			def := "kube-ovn/role"
			cfg.Label = &def
		}
		if cfg.TunnelType == nil {
			def := "geneve"
			cfg.TunnelType = &def
		}
		if cfg.EnableSSL == nil {
			b := false
			cfg.EnableSSL = &b
		}
	}
}

func SetDefaults_HybridnetConfig(cfg *HybridnetConfig) {
	if cfg == nil {
		return
	}
	if cfg.Enabled == nil {
		b := false
		cfg.Enabled = &b
	}
	if cfg.Enabled != nil && *cfg.Enabled {
		if cfg.DefaultNetworkType == nil {
			def := "Overlay"
			cfg.DefaultNetworkType = &def
		}
		if cfg.EnableNetworkPolicy == nil {
			b := true
			cfg.EnableNetworkPolicy = &b
		}
		if cfg.InitDefaultNetwork == nil {
			b := true
			cfg.InitDefaultNetwork = &b
		}
	}
}

// --- Validation Functions ---
func Validate_NetworkConfig(cfg *NetworkConfig, verrs *ValidationErrors, pathPrefix string, k8sSpec *KubernetesConfig) {
	if cfg == nil {
		verrs.Add("%s: network configuration section cannot be nil", pathPrefix)
		return
	}

	// podsCIDR := cfg.KubePodsCIDR // This line is fine
	// if podsCIDR == "" && k8sSpec != nil { // k8sSpec no longer provides PodSubnet
	//	podsCIDR = k8sSpec.PodSubnet
	// }
	if strings.TrimSpace(cfg.KubePodsCIDR) == "" {
		verrs.Add("%s.kubePodsCIDR: cannot be empty", pathPrefix)
	} else if !isValidCIDR(cfg.KubePodsCIDR) {
		verrs.Add("%s.kubePodsCIDR: invalid CIDR format '%s'", pathPrefix, cfg.KubePodsCIDR)
	}

	// serviceCIDR := cfg.KubeServiceCIDR // This line is fine
	// if serviceCIDR == "" && k8sSpec != nil { // k8sSpec no longer provides ServiceSubnet
	//	serviceCIDR = k8sSpec.ServiceSubnet
	// }
	if cfg.KubeServiceCIDR != "" && !isValidCIDR(cfg.KubeServiceCIDR) { // KubeServiceCIDR can be empty (e.g. if using hostNetwork for services or other advanced setups)
		verrs.Add("%s.kubeServiceCIDR: invalid CIDR format '%s'", pathPrefix, cfg.KubeServiceCIDR)
	}

	if cfg.Plugin == "calico" {
		if cfg.Calico == nil {
			verrs.Add("%s.calico: config cannot be nil if plugin is 'calico'", pathPrefix)
		} else {
			Validate_CalicoConfig(cfg.Calico, verrs, pathPrefix+".calico")
		}
	}
	if cfg.Plugin == "flannel" {
		if cfg.Flannel == nil {
			verrs.Add("%s.flannel: config cannot be nil if plugin is 'flannel'", pathPrefix)
		} else {
			Validate_FlannelConfig(cfg.Flannel, verrs, pathPrefix+".flannel")
		}
	}

	if cfg.KubeOvn != nil && cfg.KubeOvn.Enabled != nil && *cfg.KubeOvn.Enabled {
		Validate_KubeOvnConfig(cfg.KubeOvn, verrs, pathPrefix+".kubeovn")
	}
	if cfg.Hybridnet != nil && cfg.Hybridnet.Enabled != nil && *cfg.Hybridnet.Enabled {
		Validate_HybridnetConfig(cfg.Hybridnet, verrs, pathPrefix+".hybridnet")
	}
	if cfg.IPPool != nil {
		Validate_IPPoolConfig(cfg.IPPool, verrs, pathPrefix+".ippool")
	}
}

// Validate_IPPoolConfig validates IPPoolConfig.
func Validate_IPPoolConfig(cfg *IPPoolConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.BlockSize != nil {
		// Calico block size typically must be between 20 and 32.
		// Allow 0 as "not set" or "use Calico default" if that's desired,
		// but YAML example has 26, so we assume if set, it must be valid.
		if *cfg.BlockSize < 20 || *cfg.BlockSize > 32 {
			verrs.Add("%s.blockSize: must be between 20 and 32 if specified, got %d", pathPrefix, *cfg.BlockSize)
		}
	}
}

func Validate_CalicoConfig(cfg *CalicoConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	validEncModes := []string{"Always", "CrossSubnet", "Never", ""}
	if !contains(validEncModes, cfg.IPIPMode) {
		verrs.Add("%s.ipipMode: invalid: '%s'", pathPrefix, cfg.IPIPMode)
	}
	if !contains(validEncModes, cfg.VXLANMode) {
		verrs.Add("%s.vxlanMode: invalid: '%s'", pathPrefix, cfg.VXLANMode)
	}
	if cfg.VethMTU != nil && *cfg.VethMTU < 0 {
		verrs.Add("%s.vethMTU: invalid: %d", pathPrefix, *cfg.VethMTU)
	}
	if cfg.EnableTypha != nil && *cfg.EnableTypha && (cfg.TyphaReplicas == nil || *cfg.TyphaReplicas <= 0) {
		verrs.Add("%s.typhaReplicas: must be positive if Typha is enabled", pathPrefix)
	}
	validLogSeverities := []string{"Info", "Debug", "Warning", "Error", "Critical", "None", ""}
	if cfg.LogSeverityScreen != nil && !contains(validLogSeverities, *cfg.LogSeverityScreen) {
		verrs.Add("%s.logSeverityScreen: invalid: '%s'", pathPrefix, *cfg.LogSeverityScreen)
	}
	for i, pool := range cfg.IPPools {
		poolPath := fmt.Sprintf("%s.ipPools[%d:%s]", pathPrefix, i, pool.Name)
		if strings.TrimSpace(pool.CIDR) == "" {
			verrs.Add("%s.cidr: cannot be empty", poolPath)
		} else if !isValidCIDR(pool.CIDR) {
			verrs.Add("%s.cidr: invalid CIDR '%s'", poolPath, pool.CIDR)
		}

		if pool.BlockSize != nil && (*pool.BlockSize < 20 || *pool.BlockSize > 32) {
			verrs.Add("%s.blockSize: must be between 20 and 32, got %d", poolPath, *pool.BlockSize)
		}
		validPoolEncap := []string{"IPIP", "VXLAN", "None", ""}
		if !contains(validPoolEncap, pool.Encapsulation) {
			verrs.Add("%s.encapsulation: invalid: '%s'", poolPath, pool.Encapsulation)
		}
	}
} // Corrected: Added missing closing brace for Validate_CalicoConfig

func Validate_FlannelConfig(cfg *FlannelConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	validBackendModes := []string{"vxlan", "host-gw", "udp", ""}
	if !contains(validBackendModes, cfg.BackendMode) {
		verrs.Add("%s.backendMode: invalid: '%s'", pathPrefix, cfg.BackendMode)
	}
}

func Validate_KubeOvnConfig(cfg *KubeOvnConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Enabled == nil || !*cfg.Enabled {
		return
	}
	if cfg.Label != nil && strings.TrimSpace(*cfg.Label) == "" {
		verrs.Add("%s.label: cannot be empty if specified", pathPrefix)
	}
	if cfg.TunnelType != nil && *cfg.TunnelType != "" {
		validTypes := []string{"geneve", "vxlan", "stt"}
		if !contains(validTypes, *cfg.TunnelType) {
			verrs.Add("%s.tunnelType: invalid type '%s', must be one of %v", pathPrefix, *cfg.TunnelType, validTypes)
		}
	}
	if cfg.JoinCIDR != nil && *cfg.JoinCIDR != "" && !isValidCIDR(*cfg.JoinCIDR) {
		verrs.Add("%s.joinCIDR: invalid CIDR format '%s'", pathPrefix, *cfg.JoinCIDR)
	}
}

func Validate_HybridnetConfig(cfg *HybridnetConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Enabled == nil || !*cfg.Enabled {
		return
	}
	if cfg.DefaultNetworkType != nil && *cfg.DefaultNetworkType != "" {
		validTypes := []string{"Underlay", "Overlay"}
		if !contains(validTypes, *cfg.DefaultNetworkType) {
			verrs.Add("%s.defaultNetworkType: invalid type '%s', must be one of %v", pathPrefix, *cfg.DefaultNetworkType, validTypes)
		}
	}
}

// --- Helper Methods & Functions ---
func (n *NetworkConfig) EnableMultusCNI() bool {
	if n != nil && n.Multus != nil && n.Multus.Enabled != nil {
		return *n.Multus.Enabled
	}
	return false
}
func (c *CalicoConfig) IsTyphaEnabled() bool {
	if c != nil && c.EnableTypha != nil {
		return *c.EnableTypha
	}
	return false
}
func (c *CalicoConfig) GetTyphaReplicas() int {
	if c != nil && c.TyphaReplicas != nil {
		return *c.TyphaReplicas
	}
	if c.IsTyphaEnabled() {
		return 2
	}
	return 0
}
func (c *CalicoConfig) GetVethMTU() int {
	if c != nil && c.VethMTU != nil && *c.VethMTU > 0 {
		return *c.VethMTU
	}
	return 0
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// isValidCIDR is expected to be available from kubernetes_types.go or a shared util.
// For self-containment if processed independently:
// func isValidCIDR(cidr string) bool { _, _, err := net.ParseCIDR(cidr); return err == nil }
