package v1alpha1

import (
	"fmt" // For path formatting in validation
	"net" // For CIDR parsing and overlap checks
	"strings"
	"github.com/mensylisir/kubexm/pkg/util" // Import the util package
)

var (
	// validCalicoEncModes lists the supported encapsulation modes for Calico IPIP and VXLAN.
	validCalicoEncModes = []string{"Always", "CrossSubnet", "Never", ""}
	// validCalicoLogSeverities lists the supported log severities for Calico.
	validCalicoLogSeverities = []string{"Info", "Debug", "Warning", "Error", "Critical", "None", ""}
	// validCalicoPoolEncapsulations lists the supported encapsulation types for Calico IP pools.
	validCalicoPoolEncapsulations = []string{"IPIP", "VXLAN", "None", ""} // "IPIP" is IPIP, "VXLAN" is VXLAN

	// validFlannelBackendModes lists the supported backend modes for Flannel.
	validFlannelBackendModes = []string{"vxlan", "host-gw", "udp", ""}

	// validKubeOvnTunnelTypes lists the supported tunnel types for KubeOvn.
	validKubeOvnTunnelTypes = []string{"geneve", "vxlan", "stt"}

	// validHybridnetNetworkTypes lists the supported default network types for Hybridnet.
	validHybridnetNetworkTypes = []string{"Underlay", "Overlay"}
	// validCiliumTunnelModes lists the supported tunnel modes for Cilium.
	validCiliumTunnelModes = []string{"vxlan", "geneve", "disabled", ""}
	// validCiliumKPRModes lists the supported KubeProxyReplacement modes for Cilium.
	validCiliumKPRModes = []string{"probe", "strict", "disabled", ""}
	// validCiliumIdentModes lists the supported IdentityAllocation modes for Cilium.
	validCiliumIdentModes = []string{"crd", "kvstore", ""}
)

// NetworkConfig defines the overall network configuration for the Kubernetes cluster.
// It specifies the CNI plugin to be used and its specific settings,
// as well as general network parameters like Pod and Service CIDRs.
type NetworkConfig struct {
	// Plugin specifies the CNI (Container Network Interface) plugin to use for networking.
	// Supported values include "calico", "flannel", "cilium", etc.
	// Defaults to "calico".
	Plugin string `json:"plugin,omitempty" yaml:"plugin,omitempty"`
	// KubePodsCIDR is the IP address range from which Pod IPs are allocated.
	// This field is mandatory. Example: "10.244.0.0/16".
	KubePodsCIDR string `json:"kubePodsCIDR,omitempty" yaml:"kubePodsCIDR,omitempty"`
	// KubeServiceCIDR is the IP address range from which Service ClusterIPs are allocated.
	// This field is optional but highly recommended. Example: "10.96.0.0/12".
	KubeServiceCIDR string `json:"kubeServiceCIDR,omitempty" yaml:"kubeServiceCIDR,omitempty"`

	// Calico contains Calico specific CNI plugin configuration.
	// Used only if Plugin is "calico".
	// +optional
	Calico *CalicoConfig `json:"calico,omitempty" yaml:"calico,omitempty"`
	// Cilium specific configuration.
	// Used only if Plugin is "cilium".
	// +optional
	Cilium *CiliumConfig `json:"cilium,omitempty"`
	// Flannel contains Flannel specific CNI plugin configuration.
	// Used only if Plugin is "flannel".
	// +optional
	Flannel *FlannelConfig `json:"flannel,omitempty" yaml:"flannel,omitempty"`
	// KubeOvn contains KubeOvn specific CNI plugin configuration.
	// Used only if Plugin is "kubeovn" or Multus is enabled with KubeOvn as a delegate.
	// +optional
	KubeOvn *KubeOvnConfig `json:"kubeovn,omitempty" yaml:"kubeovn,omitempty"`
	// Multus contains configuration for the Multus CNI meta-plugin.
	// +optional
	Multus *MultusCNIConfig `json:"multus,omitempty" yaml:"multus,omitempty"`
	// Hybridnet contains Hybridnet specific CNI plugin configuration.
	// Used only if Plugin is "hybridnet".
	// +optional
	Hybridnet *HybridnetConfig `json:"hybridnet,omitempty" yaml:"hybridnet,omitempty"`
	// IPPool contains general IP pool configurations, potentially usable by multiple CNIs.
	// For example, it can define a default IP block size.
	// +optional
	IPPool *IPPoolConfig `json:"ippool,omitempty" yaml:"ippool,omitempty"`
}

// IPPoolConfig holds general IP pool configuration, currently primarily used to
// define a default block size for CNI plugins like Calico that support IP pool block sizes.
// It can be expanded in the future for more generic IPAM configurations.
// Corresponds to `network.ippool` in YAML.
type IPPoolConfig struct {
	// BlockSize is the size of the IP address blocks that can be allocated.
	// For Calico, this translates to the `blockSize` in its IPPool configuration.
	// For example, a blockSize of 26 means /26 blocks.
	// If not specified, a CNI-specific default might apply, or a global default from this config.
	BlockSize *int `json:"blockSize,omitempty" yaml:"blockSize,omitempty"`
}

// CalicoIPPool defines an IP address pool for Calico.
// Corresponds to entries in `network.calico.ipPools` in YAML.
type CalicoIPPool struct {
	// Name is a descriptive name for the IP pool.
	// +optional
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
	// CIDR is the IP address range for this pool in CIDR notation.
	// This field is mandatory for each pool.
	CIDR string `json:"cidr" yaml:"cidr"`
	// Encapsulation specifies the encapsulation method for this pool.
	// Supported values: "IPIP", "VXLAN", "None".
	// Defaults based on global Calico IPIPMode and VXLANMode.
	// +optional
	Encapsulation string `json:"encapsulation,omitempty" yaml:"encapsulation,omitempty"`
	// NatOutgoing enables or disables NAT for outgoing traffic from pods in this pool.
	// Defaults to the global Calico IPv4NatOutgoing setting.
	// +optional
	NatOutgoing *bool `json:"natOutgoing,omitempty" yaml:"natOutgoing,omitempty"`
	// BlockSize specifies the prefix length of IP address blocks to allocate to nodes from this pool.
	// Defaults to 26.
	// +optional
	BlockSize *int `json:"blockSize,omitempty" yaml:"blockSize,omitempty"`
}

// CalicoConfig defines settings specific to the Calico CNI plugin.
type CalicoConfig struct {
	// IPIPMode specifies the mode for IPIP encapsulation.
	// Supported values: "Always", "CrossSubnet", "Never".
	// Defaults to "Always".
	// +optional
	IPIPMode string `json:"ipipMode,omitempty" yaml:"ipipMode,omitempty"`
	// VXLANMode specifies the mode for VXLAN encapsulation.
	// Supported values: "Always", "CrossSubnet", "Never".
	// Defaults to "Never".
	// +optional
	VXLANMode string `json:"vxlanMode,omitempty" yaml:"vxlanMode,omitempty"`
	// VethMTU specifies the MTU (Maximum Transmission Unit) for veth interfaces created by Calico.
	// Set to 0 for auto-detection. Defaults to 0.
	// +optional
	VethMTU *int `json:"vethMTU,omitempty" yaml:"vethMTU,omitempty"`
	// IPv4NatOutgoing enables or disables NAT for outgoing IPv4 traffic from pods.
	// Defaults to true.
	// +optional
	IPv4NatOutgoing *bool `json:"ipv4NatOutgoing,omitempty" yaml:"ipv4NatOutgoing,omitempty"`
	// DefaultIPPOOL enables or disables the creation of a default IP pool using KubePodsCIDR.
	// Defaults to true.
	// +optional
	DefaultIPPOOL *bool `json:"defaultIPPOOL,omitempty" yaml:"defaultIPPOOL,omitempty"`
	// EnableTypha enables the Typha component for scaling Calico.
	// Defaults to false.
	// +optional
	EnableTypha *bool `json:"enableTypha,omitempty" yaml:"enableTypha,omitempty"`
	// TyphaReplicas specifies the number of Typha replicas.
	// Used only if EnableTypha is true. Defaults to 2.
	// +optional
	TyphaReplicas *int `json:"typhaReplicas,omitempty" yaml:"typhaReplicas,omitempty"`
	// TyphaNodeSelector is a map of key-value pairs used to select nodes for Typha deployment.
	// Used only if EnableTypha is true.
	// +optional
	TyphaNodeSelector map[string]string `json:"typhaNodeSelector,omitempty" yaml:"typhaNodeSelector,omitempty"`
	// LogSeverityScreen specifies the log severity level for Calico components.
	// Supported values: "Info", "Debug", "Warning", "Error", "Critical", "None".
	// Defaults to "Info".
	// +optional
	LogSeverityScreen *string `json:"logSeverityScreen,omitempty" yaml:"logSeverityScreen,omitempty"`
	// IPPools is a list of custom Calico IP pools.
	// +optional
	IPPools []CalicoIPPool `json:"ipPools,omitempty" yaml:"ipPools,omitempty"`
}

// CiliumConfig holds the specific configuration for the Cilium CNI plugin.
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
	// Defaults to false.
	// +optional
	EnableHubble bool `json:"enableHubble,omitempty"`

	// HubbleUI enables the deployment of the Hubble UI.
	// Requires EnableHubble to be true. Defaults to false.
	// +optional
	HubbleUI bool `json:"hubbleUI,omitempty"`

	// EnableBPFMasquerade enables eBPF-based masquerading for traffic leaving the cluster.
	// This is more efficient than traditional iptables-based masquerading.
	// Defaults to false.
	// +optional
	EnableBPFMasquerade bool `json:"enableBPFMasquerade,omitempty"`

	// IdentityAllocationMode specifies how Cilium identities are allocated.
	// "crd" is the standard mode. "kvstore" can be used for very large clusters.
	// Defaults to "crd".
	// +optional
	IdentityAllocationMode string `json:"identityAllocationMode,omitempty"`
}

// FlannelConfig defines settings specific to the Flannel CNI plugin.
type FlannelConfig struct {
	// BackendMode specifies the backend to use for Flannel (e.g., "vxlan", "host-gw", "udp").
	// Defaults to "vxlan".
	// +optional
	BackendMode string `json:"backendMode,omitempty" yaml:"backendMode,omitempty"`
	// DirectRouting enables or disables direct routing when possible (e.g., in host-gw mode).
	// Defaults to false.
	// +optional
	DirectRouting *bool `json:"directRouting,omitempty" yaml:"directRouting,omitempty"`
}

// KubeOvnConfig defines settings specific to the KubeOvn CNI plugin.
type KubeOvnConfig struct {
	// Enabled determines if KubeOvn is active.
	// Defaults to false.
	// +optional
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// JoinCIDR is the CIDR for the KubeOvn join subnet.
	// Example: "100.64.0.0/16".
	// +optional
	JoinCIDR *string `json:"joinCIDR,omitempty" yaml:"joinCIDR,omitempty"`
	// Label is the node label to identify KubeOvn managed nodes.
	// Defaults to "kube-ovn/role".
	// +optional
	Label *string `json:"label,omitempty" yaml:"label,omitempty"`
	// TunnelType specifies the tunnel encapsulation type (e.g., "geneve", "vxlan", "stt").
	// Defaults to "geneve".
	// +optional
	TunnelType *string `json:"tunnelType,omitempty" yaml:"tunnelType,omitempty"`
	// EnableSSL enables SSL/TLS for KubeOvn components communication.
	// Defaults to false.
	// +optional
	EnableSSL *bool `json:"enableSSL,omitempty" yaml:"enableSSL,omitempty"`
}

// MultusCNIConfig defines settings for the Multus CNI meta-plugin.
type MultusCNIConfig struct {
	// Enabled determines if Multus is active, allowing multiple CNI plugins.
	// Defaults to false.
	// +optional
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// HybridnetConfig defines settings specific to the Hybridnet CNI plugin.
type HybridnetConfig struct {
	// Enabled determines if Hybridnet is active.
	// Defaults to false.
	// +optional
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// DefaultNetworkType specifies the default network type for Hybridnet ("Underlay" or "Overlay").
	// Defaults to "Overlay".
	// +optional
	DefaultNetworkType *string `json:"defaultNetworkType,omitempty" yaml:"defaultNetworkType,omitempty"`
	// EnableNetworkPolicy enables or disables network policy enforcement by Hybridnet.
	// Defaults to true.
	// +optional
	EnableNetworkPolicy *bool `json:"enableNetworkPolicy,omitempty" yaml:"enableNetworkPolicy,omitempty"`
	// InitDefaultNetwork determines if Hybridnet should initialize a default network.
	// Defaults to true.
	// +optional
	InitDefaultNetwork *bool `json:"initDefaultNetwork,omitempty" yaml:"initDefaultNetwork,omitempty"`
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
func Validate_NetworkConfig(cfg *NetworkConfig, verrs *ValidationErrors, pathPrefix string) {
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
	} else if !util.IsValidCIDR(cfg.KubePodsCIDR) {
		verrs.Add("%s.kubePodsCIDR: invalid CIDR format '%s'", pathPrefix, cfg.KubePodsCIDR)
	}

	// serviceCIDR := cfg.KubeServiceCIDR // This line is fine
	// if serviceCIDR == "" && k8sSpec != nil { // k8sSpec no longer provides ServiceSubnet
	//	serviceCIDR = k8sSpec.ServiceSubnet
	// }
	if cfg.KubeServiceCIDR != "" && !util.IsValidCIDR(cfg.KubeServiceCIDR) { // KubeServiceCIDR can be empty
		verrs.Add("%s.kubeServiceCIDR: invalid CIDR format '%s'", pathPrefix, cfg.KubeServiceCIDR)
	}

	// Check for CIDR overlap if both are validly formatted
	if util.IsValidCIDR(cfg.KubePodsCIDR) && cfg.KubeServiceCIDR != "" && util.IsValidCIDR(cfg.KubeServiceCIDR) {
		_, podsNet, errPods := net.ParseCIDR(cfg.KubePodsCIDR)
		_, serviceNet, errServices := net.ParseCIDR(cfg.KubeServiceCIDR)

		// Ensure no error occurred during parsing before using the nets
		if errPods == nil && errServices == nil {
			if util.NetworksOverlap(podsNet, serviceNet) {
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
	if !util.ContainsString(validCalicoEncModes, cfg.IPIPMode) {
		verrs.Add("%s.ipipMode: invalid: '%s', must be one of %v", pathPrefix, cfg.IPIPMode, validCalicoEncModes)
	}
	if !util.ContainsString(validCalicoEncModes, cfg.VXLANMode) {
		verrs.Add("%s.vxlanMode: invalid: '%s', must be one of %v", pathPrefix, cfg.VXLANMode, validCalicoEncModes)
	}
	if cfg.VethMTU != nil && *cfg.VethMTU < 0 {
		verrs.Add("%s.vethMTU: invalid: %d", pathPrefix, *cfg.VethMTU)
	}
	if cfg.EnableTypha != nil && *cfg.EnableTypha && (cfg.TyphaReplicas == nil || *cfg.TyphaReplicas <= 0) {
		verrs.Add("%s.typhaReplicas: must be positive if Typha is enabled", pathPrefix)
	}
	if cfg.LogSeverityScreen != nil && !util.ContainsString(validCalicoLogSeverities, *cfg.LogSeverityScreen) {
		verrs.Add("%s.logSeverityScreen: invalid: '%s',  must be one of %v", pathPrefix, *cfg.LogSeverityScreen, validCalicoLogSeverities)
	}
	for i, pool := range cfg.IPPools {
		poolPath := fmt.Sprintf("%s.ipPools[%d:%s]", pathPrefix, i, pool.Name)
		if strings.TrimSpace(pool.CIDR) == "" {
			verrs.Add("%s.cidr: cannot be empty", poolPath)
		} else if !util.IsValidCIDR(pool.CIDR) {
			verrs.Add("%s.cidr: invalid CIDR '%s'", poolPath, pool.CIDR)
		}

		if pool.BlockSize != nil && (*pool.BlockSize < 20 || *pool.BlockSize > 32) {
			verrs.Add("%s.blockSize: must be between 20 and 32, got %d", poolPath, *pool.BlockSize)
		}
		if !util.ContainsString(validCalicoPoolEncapsulations, pool.Encapsulation) {
			verrs.Add("%s.encapsulation: invalid: '%s', must be one of %v", poolPath, pool.Encapsulation, validCalicoPoolEncapsulations)
		}
	}
}

func Validate_FlannelConfig(cfg *FlannelConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if !util.ContainsString(validFlannelBackendModes, cfg.BackendMode) {
		verrs.Add("%s.backendMode: invalid: '%s', must be one of %v", pathPrefix, cfg.BackendMode, validFlannelBackendModes)
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
		if !util.ContainsString(validKubeOvnTunnelTypes, *cfg.TunnelType) {
			verrs.Add("%s.tunnelType: invalid type '%s', must be one of %v", pathPrefix, *cfg.TunnelType, validKubeOvnTunnelTypes)
		}
	}
	if cfg.JoinCIDR != nil && *cfg.JoinCIDR != "" && !util.IsValidCIDR(*cfg.JoinCIDR) {
		verrs.Add("%s.joinCIDR: invalid CIDR format '%s'", pathPrefix, *cfg.JoinCIDR)
	}
}

func Validate_HybridnetConfig(cfg *HybridnetConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Enabled == nil || !*cfg.Enabled {
		return
	}
	if cfg.DefaultNetworkType != nil && *cfg.DefaultNetworkType != "" {
		if !util.ContainsString(validHybridnetNetworkTypes, *cfg.DefaultNetworkType) {
			verrs.Add("%s.defaultNetworkType: invalid type '%s', must be one of %v", pathPrefix, *cfg.DefaultNetworkType, validHybridnetNetworkTypes)
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
