package v1alpha1

import (
	"fmt" // For path formatting in validation
	"net" // For CIDR parsing and overlap checks
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
		cfg.IPPool.BlockSize = intPtr(26) // Default from YAML example
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
	if cfg.Plugin == "cilium" {
		if cfg.Cilium == nil {
			cfg.Cilium = &CiliumConfig{}
		}
		SetDefaults_CiliumConfig(cfg.Cilium)
	}

	if cfg.Multus == nil {
		cfg.Multus = &MultusCNIConfig{}
	}
	if cfg.Multus.Enabled == nil {
		cfg.Multus.Enabled = boolPtr(false)
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
		cfg.IPv4NatOutgoing = boolPtr(true)
	}
	if cfg.DefaultIPPOOL == nil {
		cfg.DefaultIPPOOL = boolPtr(true)
	}
	if cfg.EnableTypha == nil {
		cfg.EnableTypha = boolPtr(false)
	}
	if cfg.EnableTypha != nil && *cfg.EnableTypha && cfg.TyphaReplicas == nil {
		cfg.TyphaReplicas = intPtr(2)
	}
	if cfg.TyphaNodeSelector == nil {
		cfg.TyphaNodeSelector = make(map[string]string)
	}
	if cfg.VethMTU == nil {
		cfg.VethMTU = intPtr(0)
	}
	if cfg.LogSeverityScreen == nil {
		cfg.LogSeverityScreen = stringPtr("Info")
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

		var defaultPoolEncap string
		// Prioritize IPIP if it's enabled, then VXLAN, then None.
		if cfg.IPIPMode == "Always" || cfg.IPIPMode == "CrossSubnet" {
			defaultPoolEncap = "IPIP"
		} else if cfg.VXLANMode == "Always" || cfg.VXLANMode == "CrossSubnet" {
			defaultPoolEncap = "VXLAN"
		} else {
			defaultPoolEncap = "None" // If neither IPIP nor VXLAN is explicitly enabled for inter-node traffic.
		}

		cfg.IPPools = append(cfg.IPPools, CalicoIPPool{
			Name:          "default-ipv4-ippool",
			CIDR:          defaultPoolCIDR,
			Encapsulation: defaultPoolEncap, // Use the derived valid pool encapsulation
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
			pool.NatOutgoing = cfg.IPv4NatOutgoing // This correctly copies the pointer if already set, or the bool value
		}
		if pool.BlockSize == nil {
			pool.BlockSize = intPtr(26)
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
		cfg.DirectRouting = boolPtr(false)
	}
}

func SetDefaults_KubeOvnConfig(cfg *KubeOvnConfig) {
	if cfg == nil {
		return
	}
	if cfg.Enabled == nil {
		cfg.Enabled = boolPtr(false)
	}
	if cfg.Enabled != nil && *cfg.Enabled {
		if cfg.Label == nil {
			cfg.Label = stringPtr("kube-ovn/role")
		}
		if cfg.TunnelType == nil {
			cfg.TunnelType = stringPtr("geneve")
		}
		if cfg.EnableSSL == nil {
			cfg.EnableSSL = boolPtr(false)
		}
	}
}

func SetDefaults_HybridnetConfig(cfg *HybridnetConfig) {
	if cfg == nil {
		return
	}
	if cfg.Enabled == nil {
		cfg.Enabled = boolPtr(false)
	}
	if cfg.Enabled != nil && *cfg.Enabled {
		if cfg.DefaultNetworkType == nil {
			cfg.DefaultNetworkType = stringPtr("Overlay")
		}
		if cfg.EnableNetworkPolicy == nil {
			cfg.EnableNetworkPolicy = boolPtr(true)
		}
		if cfg.InitDefaultNetwork == nil {
			cfg.InitDefaultNetwork = boolPtr(true)
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
	if cfg.KubeServiceCIDR != "" && !isValidCIDR(cfg.KubeServiceCIDR) { // KubeServiceCIDR can be empty
		verrs.Add("%s.kubeServiceCIDR: invalid CIDR format '%s'", pathPrefix, cfg.KubeServiceCIDR)
	}

	// Check for CIDR overlap if both are validly formatted
	if isValidCIDR(cfg.KubePodsCIDR) && cfg.KubeServiceCIDR != "" && isValidCIDR(cfg.KubeServiceCIDR) {
		_, podsNet, _ := net.ParseCIDR(cfg.KubePodsCIDR)
		_, serviceNet, _ := net.ParseCIDR(cfg.KubeServiceCIDR)

		if podsNet.Contains(serviceNet.IP) || serviceNet.Contains(podsNet.IP) {
			// This check is simplified; a more robust check would compare the networks themselves, not just one IP.
			// For example, serviceNet.Contains(podsNet.IP) && serviceNet.Mask != nil && podsNet.Mask != nil &&
			// (serviceNet.Contains(podsNet.IP) || podsNet.Contains(serviceNet.IP))
			// A common way to check for overlap: (net1.Contains(net2.IP) || net2.Contains(net1.IP))
			// More accurate: check if the start or end of one range falls within the other.
			// However, the simple IP containment check catches most common overlaps.
			if networksOverlap(podsNet, serviceNet) {
				verrs.Add("%s: kubePodsCIDR (%s) and kubeServiceCIDR (%s) overlap", pathPrefix, cfg.KubePodsCIDR, cfg.KubeServiceCIDR)
			}
		}
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
	if cfg.Plugin == "cilium" {
		if cfg.Cilium == nil {
			verrs.Add("%s.cilium: config cannot be nil if plugin is 'cilium'", pathPrefix)
		} else {
			Validate_CiliumConfig(cfg.Cilium, verrs, pathPrefix+".cilium")
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
	if !containsString(validEncModes, cfg.IPIPMode) {
		verrs.Add("%s.ipipMode: invalid: '%s'", pathPrefix, cfg.IPIPMode)
	}
	if !containsString(validEncModes, cfg.VXLANMode) {
		verrs.Add("%s.vxlanMode: invalid: '%s'", pathPrefix, cfg.VXLANMode)
	}
	if cfg.VethMTU != nil && *cfg.VethMTU < 0 {
		verrs.Add("%s.vethMTU: invalid: %d", pathPrefix, *cfg.VethMTU)
	}
	if cfg.EnableTypha != nil && *cfg.EnableTypha && (cfg.TyphaReplicas == nil || *cfg.TyphaReplicas <= 0) {
		verrs.Add("%s.typhaReplicas: must be positive if Typha is enabled", pathPrefix)
	}
	validLogSeverities := []string{"Info", "Debug", "Warning", "Error", "Critical", "None", ""}
	if cfg.LogSeverityScreen != nil && !containsString(validLogSeverities, *cfg.LogSeverityScreen) {
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
		if !containsString(validPoolEncap, pool.Encapsulation) {
			verrs.Add("%s.encapsulation: invalid: '%s'", poolPath, pool.Encapsulation)
		}
	}
} // Corrected: Added missing closing brace for Validate_CalicoConfig

func Validate_FlannelConfig(cfg *FlannelConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	validBackendModes := []string{"vxlan", "host-gw", "udp", ""}
	if !containsString(validBackendModes, cfg.BackendMode) {
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
		if !containsString(validTypes, *cfg.TunnelType) {
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
		if !containsString(validTypes, *cfg.DefaultNetworkType) {
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

// isValidCIDR is expected to be available from kubernetes_types.go or a shared util.
// func isValidCIDR(cidr string) bool { _, _, err := net.ParseCIDR(cidr); return err == nil } // Assumed to be present

// networksOverlap checks if two IP networks overlap.
func networksOverlap(n1, n2 *net.IPNet) bool {
	// Check if one network contains the other's network address or broadcast address
	// This is a common way to check for overlap.
	// A more precise method might involve comparing start and end IPs of each range.
	return n1.Contains(n2.IP) || n2.Contains(n1.IP)
}
