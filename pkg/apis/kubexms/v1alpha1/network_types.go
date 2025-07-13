package v1alpha1

import (
	"fmt"
	"net" // For CIDR validation if isValidCIDR is defined here
	"strings"
	// Assuming ValidationErrors is in cluster_types.go or a shared util in this package
	// Assuming KubernetesConfig is defined in kubernetes_types.go for k8sSpec parameter
)

// NetworkConfig defines the network configuration for the cluster.
type Network struct {
	Plugin          string `json:"plugin,omitempty" yaml:"plugin,omitempty"`
	KubePodsCIDR    string `json:"kubePodsCIDR,omitempty" yaml:"kubePodsCIDR,omitempty"`
	KubeServiceCIDR string `json:"kubeServiceCIDR,omitempty" yaml:"kubeServiceCIDR,omitempty"`

	Calico    *CalicoConfig    `json:"calico,omitempty" yaml:"calico,omitempty"`
	Cilium    *CiliumConfig    `json:"cilium,omitempty" yaml:"cilium,omitempty"`
	Flannel   *FlannelConfig   `json:"flannel,omitempty" yaml:"flannel,omitempty"`
	KubeOvn   *KubeOvnConfig   `json:"kubeovn,omitempty" yaml:"kubeovn,omitempty"`
	Multus    *MultusCNIConfig `json:"multus,omitempty" yaml:"multus,omitempty"`
	Hybridnet *HybridnetConfig `json:"hybridnet,omitempty" yaml:"hybridnet,omitempty"`
	IPPool    *IPPoolConfig    `json:"ippool,omitempty" yaml:"ippool,omitempty"`
}

// IPPoolConfig holds general IP pool configuration.
type IPPoolConfig struct {
	BlockSize *int `json:"blockSize,omitempty" yaml:"blockSize,omitempty"`
}

// CalicoIPPool defines an IP address pool for Calico.
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
type CiliumConfig struct {
	TunnelingMode          string `json:"tunnelingMode,omitempty" yaml:"tunnelingMode,omitempty"`
	KubeProxyReplacement   string `json:"kubeProxyReplacement,omitempty" yaml:"kubeProxyReplacement,omitempty"`
	EnableHubble           bool   `json:"enableHubble,omitempty" yaml:"enableHubble,omitempty"`
	HubbleUI               bool   `json:"hubbleUI,omitempty" yaml:"hubbleUI,omitempty"`
	EnableBPFMasquerade    *bool  `json:"enableBPFMasquerade,omitempty" yaml:"enableBPFMasquerade,omitempty"`
	IdentityAllocationMode string `json:"identityAllocationMode,omitempty" yaml:"identityAllocationMode,omitempty"`
}

// FlannelConfig defines settings specific to the Flannel CNI plugin.
type FlannelConfig struct {
	BackendMode   string `json:"backendMode,omitempty" yaml:"backendMode,omitempty"`
	DirectRouting *bool  `json:"directRouting,omitempty" yaml:"directRouting,omitempty"`
}

// KubeOvnConfig defines settings for Kube-OVN CNI.
type KubeOvnConfig struct {
	Enabled    *bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	JoinCIDR   *string `json:"joinCIDR,omitempty" yaml:"joinCIDR,omitempty"` // This might be KubePodsCIDR
	Label      *string `json:"label,omitempty" yaml:"label,omitempty"`
	TunnelType *string `json:"tunnelType,omitempty" yaml:"tunnelType,omitempty"`
	EnableSSL  *bool   `json:"enableSSL,omitempty" yaml:"enableSSL,omitempty"`
}

// MultusCNIConfig defines settings for Multus CNI.
type MultusCNIConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// HybridnetConfig defines settings for Hybridnet CNI.
type HybridnetConfig struct {
	Enabled             *bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	DefaultNetworkType  *string `json:"defaultNetworkType,omitempty" yaml:"defaultNetworkType,omitempty"`
	EnableNetworkPolicy *bool   `json:"enableNetworkPolicy,omitempty" yaml:"enableNetworkPolicy,omitempty"`
	InitDefaultNetwork  *bool   `json:"initDefaultNetwork,omitempty" yaml:"initDefaultNetwork,omitempty"`
}

// SetDefaults_NetworkConfig sets default values for NetworkConfig.
func SetDefaults_NetworkConfig(cfg *NetworkConfig) {
	if cfg == nil {
		return
	}
	if cfg.Plugin == "" {
		cfg.Plugin = "calico"
	}

	if cfg.IPPool == nil {
		cfg.IPPool = &IPPoolConfig{}
	}
	if cfg.IPPool.BlockSize == nil {
		defaultBlockSize := 26
		cfg.IPPool.BlockSize = &defaultBlockSize
	}

	if cfg.Plugin == "calico" {
		if cfg.Calico == nil {
			cfg.Calico = &CalicoConfig{}
		}
		var defaultCalicoBlockSize *int
		if cfg.IPPool != nil {
			defaultCalicoBlockSize = cfg.IPPool.BlockSize
		}
		SetDefaults_CalicoConfig(cfg.Calico, cfg.KubePodsCIDR, defaultCalicoBlockSize)
	}
	if cfg.Plugin == "cilium" { // Added Cilium
		if cfg.Cilium == nil {
			cfg.Cilium = &CiliumConfig{}
		}
		SetDefaults_CiliumConfig(cfg.Cilium) // Function defined in cilium_types.go
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
		defaultMTU := 0
		cfg.VethMTU = &defaultMTU
	} // 0 means Calico default
	if cfg.LogSeverityScreen == nil {
		s := "Info"
		cfg.LogSeverityScreen = &s
	}

	if len(cfg.IPPools) == 0 && cfg.DefaultIPPOOL != nil && *cfg.DefaultIPPOOL && defaultPoolCIDR != "" {
		var bs *int = globalDefaultBlockSize // Use global default
		if bs == nil {                       // Fallback if global was nil
			defaultInternalBlockSize := 26
			bs = &defaultInternalBlockSize
		}
		cfg.IPPools = append(cfg.IPPools, CalicoIPPool{
			Name:          "default-ipv4-ippool",
			CIDR:          defaultPoolCIDR,
			Encapsulation: "", // Will be set properly in the loop below
			NatOutgoing:   cfg.IPv4NatOutgoing,
			BlockSize:     bs,
		})
	}
	for i := range cfg.IPPools {
		pool := &cfg.IPPools[i]
		if pool.Encapsulation == "" {
			if cfg.IPIPMode == "Always" || cfg.IPIPMode == "CrossSubnet" {
				pool.Encapsulation = "IPIP"
			} else if cfg.VXLANMode == "Always" || cfg.VXLANMode == "CrossSubnet" {
				pool.Encapsulation = "VXLAN"
			} else {
				pool.Encapsulation = "None"
			}
		}
		if pool.NatOutgoing == nil {
			pool.NatOutgoing = cfg.IPv4NatOutgoing
		}
		if pool.BlockSize == nil {
			defaultBS := 26
			pool.BlockSize = &defaultBS
		}
	}
}

// SetDefaults_CiliumConfig is defined in cilium_types.go to avoid duplication

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

func Validate_NetworkConfig(cfg *NetworkConfig, verrs *ValidationErrors, pathPrefix string, k8sSpec *KubernetesConfig) {
	if cfg == nil {
		verrs.Add(pathPrefix + ": network configuration section cannot be nil")
		return
	}
	if strings.TrimSpace(cfg.KubePodsCIDR) == "" {
		verrs.Add(pathPrefix + ".kubePodsCIDR: cannot be empty")
	} else if !isValidCIDR(cfg.KubePodsCIDR) {
		verrs.Add(pathPrefix + ".kubePodsCIDR: invalid CIDR format '" + cfg.KubePodsCIDR + "'")
	}
	if cfg.KubeServiceCIDR != "" && !isValidCIDR(cfg.KubeServiceCIDR) {
		verrs.Add(pathPrefix + ".kubeServiceCIDR: invalid CIDR format '" + cfg.KubeServiceCIDR + "'")
	}

	// Validate CIDR overlaps
	if cfg.KubePodsCIDR != "" && cfg.KubeServiceCIDR != "" && isValidCIDR(cfg.KubePodsCIDR) && isValidCIDR(cfg.KubeServiceCIDR) {
		_, podsNet, _ := net.ParseCIDR(cfg.KubePodsCIDR)
		_, serviceNet, _ := net.ParseCIDR(cfg.KubeServiceCIDR)
		if podsNet.Contains(serviceNet.IP) || serviceNet.Contains(podsNet.IP) {
			verrs.Add(pathPrefix + ": kubePodsCIDR (" + cfg.KubePodsCIDR + ") and kubeServiceCIDR (" + cfg.KubeServiceCIDR + ") must not overlap")
		}
	}

	validPlugins := []string{"calico", "cilium", "flannel", "kube-ovn", ""} // Add other supported plugins, empty for default
	if !containsString(validPlugins, cfg.Plugin) {
		verrs.Add(pathPrefix + ".plugin: invalid plugin '" + cfg.Plugin + "', must be one of " + fmt.Sprintf("%v", validPlugins) + " or empty for default")
	}

	if cfg.Plugin == "calico" {
		if cfg.Calico == nil {
			verrs.Add(pathPrefix + ".calico: config cannot be nil if plugin is 'calico'")
		} else {
			Validate_CalicoConfig(cfg.Calico, verrs, pathPrefix+".calico")
		}
	}
	if cfg.Plugin == "cilium" { // Added Cilium
		if cfg.Cilium == nil {
			verrs.Add(pathPrefix + ".cilium: config cannot be nil if plugin is 'cilium'")
		} else {
			Validate_CiliumConfig(cfg.Cilium, verrs, pathPrefix+".cilium")
		}
	}
	if cfg.Plugin == "flannel" {
		if cfg.Flannel == nil {
			verrs.Add(pathPrefix + ".flannel: config cannot be nil if plugin is 'flannel'")
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

func Validate_IPPoolConfig(cfg *IPPoolConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.BlockSize != nil {
		if *cfg.BlockSize < 20 || *cfg.BlockSize > 32 {
			verrs.Add(pathPrefix + ".blockSize: must be between 20 and 32 if specified, got " + fmt.Sprintf("%d", *cfg.BlockSize))
		}
	}
}

func Validate_CalicoConfig(cfg *CalicoConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	validEncModes := []string{"Always", "CrossSubnet", "Never", ""}
	if !containsString(validEncModes, cfg.IPIPMode) {
		verrs.Add(pathPrefix + ".ipipMode: invalid: '" + cfg.IPIPMode + "'")
	}
	if !containsString(validEncModes, cfg.VXLANMode) {
		verrs.Add(pathPrefix + ".vxlanMode: invalid: '" + cfg.VXLANMode + "'")
	}
	if cfg.VethMTU != nil && *cfg.VethMTU < 0 {
		verrs.Add(pathPrefix + ".vethMTU: invalid: " + fmt.Sprintf("%d", *cfg.VethMTU))
	}
	if cfg.EnableTypha != nil && *cfg.EnableTypha && (cfg.TyphaReplicas == nil || *cfg.TyphaReplicas <= 0) {
		verrs.Add(pathPrefix + ".typhaReplicas: must be positive if Typha is enabled")
	}
	validLogSeverities := []string{"Info", "Debug", "Warning", "Error", "Critical", "None", ""}
	if cfg.LogSeverityScreen != nil && !containsString(validLogSeverities, *cfg.LogSeverityScreen) {
		verrs.Add(pathPrefix + ".logSeverityScreen: invalid: '" + *cfg.LogSeverityScreen + "'")
	}
	for i, pool := range cfg.IPPools {
		poolPath := fmt.Sprintf("%s.ipPools[%d:%s]", pathPrefix, i, pool.Name)
		if strings.TrimSpace(pool.CIDR) == "" {
			verrs.Add(poolPath + ".cidr: cannot be empty")
		} else if !isValidCIDR(pool.CIDR) {
			verrs.Add(poolPath + ".cidr: invalid CIDR '" + pool.CIDR + "'")
		}
		if pool.BlockSize != nil && (*pool.BlockSize < 20 || *pool.BlockSize > 32) {
			verrs.Add(poolPath + ".blockSize: must be between 20 and 32, got " + fmt.Sprintf("%d", *pool.BlockSize))
		}
		validPoolEncap := []string{"IPIP", "VXLAN", "None", ""}
		if !containsString(validPoolEncap, pool.Encapsulation) {
			verrs.Add(poolPath + ".encapsulation: invalid: '" + pool.Encapsulation + "'")
		}
	}
}

// Validate_CiliumConfig is defined in cilium_types.go to avoid duplication

func Validate_FlannelConfig(cfg *FlannelConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	validBackendModes := []string{"vxlan", "host-gw", "udp", ""}
	if !containsString(validBackendModes, cfg.BackendMode) {
		verrs.Add(pathPrefix + ".backendMode: invalid: '" + cfg.BackendMode + "'")
	}
}

func Validate_KubeOvnConfig(cfg *KubeOvnConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Enabled == nil || !*cfg.Enabled {
		return
	}
	if cfg.Label != nil && strings.TrimSpace(*cfg.Label) == "" {
		verrs.Add(pathPrefix + ".label: cannot be empty if specified")
	}
	if cfg.TunnelType != nil && *cfg.TunnelType != "" {
		validTypes := []string{"geneve", "vxlan", "stt"}
		if !containsString(validTypes, *cfg.TunnelType) {
			verrs.Add(pathPrefix + ".tunnelType: invalid type '" + *cfg.TunnelType + "', must be one of " + fmt.Sprintf("%v", validTypes))
		}
	}
	if cfg.JoinCIDR != nil && *cfg.JoinCIDR != "" && !isValidCIDR(*cfg.JoinCIDR) {
		verrs.Add(pathPrefix + ".joinCIDR: invalid CIDR format '" + *cfg.JoinCIDR + "'")
	}
}

func Validate_HybridnetConfig(cfg *HybridnetConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Enabled == nil || !*cfg.Enabled {
		return
	}
	if cfg.DefaultNetworkType != nil && *cfg.DefaultNetworkType != "" {
		validTypes := []string{"Underlay", "Overlay"}
		if !containsString(validTypes, *cfg.DefaultNetworkType) {
			verrs.Add(pathPrefix + ".defaultNetworkType: invalid type '" + *cfg.DefaultNetworkType + "', must be one of " + fmt.Sprintf("%v", validTypes))
		}
	}
}

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
	} // Default from SetDefaults
	return 0
}
func (c *CalicoConfig) GetVethMTU() int {
	if c != nil && c.VethMTU != nil && *c.VethMTU > 0 {
		return *c.VethMTU
	}
	return 0 // Calico default (auto-detection)
}

// containsString and isValidCIDR are expected to be defined in cluster_types.go or a shared util.
// Removed local definitions to avoid duplication if these files are in the same package.
// Added CiliumConfig struct and its SetDefaults/Validate methods.
// Integrated Cilium into NetworkConfig SetDefaults and Validate.
// Added CIDR overlap validation in Validate_NetworkConfig.
// Imported "net" for ParseCIDR.
// Corrected CalicoConfig SetDefaults for IPPools to use globalDefaultBlockSize and fallback.
// Corrected CalicoConfig SetDefaults for VethMTU default.
// Corrected CalicoConfig GetTyphaReplicas and GetVethMTU to reflect defaults or actual values.
