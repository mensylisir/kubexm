package v1alpha1

import (
	"strings"
	"net" // For CIDR validation helper
	"fmt" // For path formatting in validation
)

// NetworkConfig defines the network configuration for the cluster.
type NetworkConfig struct {
	Plugin          string `json:"plugin,omitempty" yaml:"plugin,omitempty"`
	KubePodsCIDR    string `json:"kubePodsCIDR,omitempty" yaml:"kubePodsCIDR,omitempty"`
	KubeServiceCIDR string `json:"kubeServiceCIDR,omitempty" yaml:"kubeServiceCIDR,omitempty"`

	Calico    *CalicoConfig    `json:"calico,omitempty" yaml:"calico,omitempty"`
	Flannel   *FlannelConfig   `json:"flannel,omitempty" yaml:"flannel,omitempty"`
	KubeOvn   *KubeOvnConfig   `json:"kubeovn,omitempty" yaml:"kubeovn,omitempty"`
	Multus    *MultusCNIConfig `json:"multus,omitempty" yaml:"multus,omitempty"`
	Hybridnet *HybridnetConfig `json:"hybridnet,omitempty" yaml:"hybridnet,omitempty"`
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
	IPIPMode        string `json:"ipipMode,omitempty" yaml:"ipipMode,omitempty"`
	VXLANMode       string `json:"vxlanMode,omitempty" yaml:"vxlanMode,omitempty"`
	VethMTU         *int   `json:"vethMTU,omitempty" yaml:"vethMTU,omitempty"`
	IPv4NatOutgoing *bool  `json:"ipv4NatOutgoing,omitempty" yaml:"ipv4NatOutgoing,omitempty"`
	DefaultIPPOOL   *bool  `json:"defaultIPPOOL,omitempty" yaml:"defaultIPPOOL,omitempty"`
	EnableTypha     *bool  `json:"enableTypha,omitempty" yaml:"enableTypha,omitempty"`
	TyphaReplicas   *int   `json:"typhaReplicas,omitempty" yaml:"typhaReplicas,omitempty"`
	TyphaNodeSelector map[string]string `json:"typhaNodeSelector,omitempty" yaml:"typhaNodeSelector,omitempty"`
	LogSeverityScreen *string `json:"logSeverityScreen,omitempty" yaml:"logSeverityScreen,omitempty"`

	IPPools []CalicoIPPool `json:"ipPools,omitempty" yaml:"ipPools,omitempty"`
}

// FlannelConfig defines settings specific to the Flannel CNI plugin.
type FlannelConfig struct {
	BackendMode   string `json:"backendMode,omitempty" yaml:"backendMode,omitempty"`
	DirectRouting *bool  `json:"directRouting,omitempty" yaml:"directRouting,omitempty"`
}

type KubeOvnConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	JoinCIDR *string `json:"joinCIDR,omitempty" yaml:"joinCIDR,omitempty"`
	Label *string `json:"label,omitempty" yaml:"label,omitempty"`
	TunnelType *string `json:"tunnelType,omitempty" yaml:"tunnelType,omitempty"`
	EnableSSL *bool `json:"enableSSL,omitempty" yaml:"enableSSL,omitempty"`
}

type MultusCNIConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

type HybridnetConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	DefaultNetworkType *string `json:"defaultNetworkType,omitempty" yaml:"defaultNetworkType,omitempty"`
	EnableNetworkPolicy *bool `json:"enableNetworkPolicy,omitempty" yaml:"enableNetworkPolicy,omitempty"`
	InitDefaultNetwork *bool `json:"initDefaultNetwork,omitempty" yaml:"initDefaultNetwork,omitempty"`
}

// --- Defaulting Functions ---

func SetDefaults_NetworkConfig(cfg *NetworkConfig) {
	if cfg == nil { return }
	if cfg.Plugin == "" {
		cfg.Plugin = "calico" // Default plugin to Calico
	}
	// KubePodsCIDR and KubeServiceCIDR defaults are better handled at KubernetesConfig or by user.

	if cfg.Plugin == "calico" {
		if cfg.Calico == nil { cfg.Calico = &CalicoConfig{} }
		SetDefaults_CalicoConfig(cfg.Calico, cfg.KubePodsCIDR) // Pass PodsCIDR for default IPPool
	}
	if cfg.Plugin == "flannel" {
		if cfg.Flannel == nil { cfg.Flannel = &FlannelConfig{} }
		SetDefaults_FlannelConfig(cfg.Flannel)
	}

	if cfg.Multus == nil { cfg.Multus = &MultusCNIConfig{} }
	if cfg.Multus.Enabled == nil { b := false; cfg.Multus.Enabled = &b }

	if cfg.KubeOvn == nil { cfg.KubeOvn = &KubeOvnConfig{} }
	// if cfg.KubeOvn.Enabled == nil { b := false; cfg.KubeOvn.Enabled = &b } // This will be handled by SetDefaults_KubeOvnConfig now
	SetDefaults_KubeOvnConfig(cfg.KubeOvn)


	if cfg.Hybridnet == nil { cfg.Hybridnet = &HybridnetConfig{} }
	// if cfg.Hybridnet.Enabled == nil { b := false; cfg.Hybridnet.Enabled = &b } // Handled by SetDefaults_HybridnetConfig
	SetDefaults_HybridnetConfig(cfg.Hybridnet)
}

func SetDefaults_CalicoConfig(cfg *CalicoConfig, defaultPoolCIDR string) {
	if cfg == nil { return }
	if cfg.IPIPMode == "" { // Default IPIPMode to Always if empty
	   cfg.IPIPMode = "Always"
	}
	if cfg.VXLANMode == "" { // Default VXLANMode to Never if empty
	   cfg.VXLANMode = "Never"
	}
	// If IPIPMode is "Always", VXLANMode should ideally be "Never" and vice-versa,
	// but current logic allows both to be e.g. "CrossSubnet".
	// The prompt's direct defaults are simpler.

	if cfg.IPv4NatOutgoing == nil { b := true; cfg.IPv4NatOutgoing = &b }
	if cfg.DefaultIPPOOL == nil { b := true; cfg.DefaultIPPOOL = &b } // Default to creating a default ippool

	if cfg.EnableTypha == nil { b := false; cfg.EnableTypha = &b }
	if cfg.EnableTypha != nil && *cfg.EnableTypha && cfg.TyphaReplicas == nil {
		var defaultReplicas int = 2; cfg.TyphaReplicas = &defaultReplicas
	}
    if cfg.TyphaNodeSelector == nil { cfg.TyphaNodeSelector = make(map[string]string) }
	if cfg.VethMTU == nil {
	   var defaultMTU int = 0 // Default VethMTU to 0 (Calico default)
	   cfg.VethMTU = &defaultMTU
	}
	if cfg.LogSeverityScreen == nil { s := "Info"; cfg.LogSeverityScreen = &s }

	if len(cfg.IPPools) == 0 && cfg.DefaultIPPOOL != nil && *cfg.DefaultIPPOOL && defaultPoolCIDR != "" {
		// Create a default IPPool using the KubePodsCIDR if no pools are defined
		// and DefaultIPPOOL is true.
		defaultBlockSize := 26
		cfg.IPPools = append(cfg.IPPools, CalicoIPPool{
			Name: "default-ipv4-ippool", // Calico's typical default name
			CIDR: defaultPoolCIDR,
			Encapsulation: cfg.IPIPMode, // Default encapsulation to global IPIP mode
			NatOutgoing: cfg.IPv4NatOutgoing, // Default NAT to global setting
			BlockSize: &defaultBlockSize,
		})
	}
	for i := range cfg.IPPools {
	   pool := &cfg.IPPools[i]
	   if pool.Encapsulation == "" {
		   if cfg.IPIPMode == "Always" { pool.Encapsulation = "IPIP" } // Simplified logic
		   else if cfg.VXLANMode == "Always" { pool.Encapsulation = "VXLAN" }
		   else { pool.Encapsulation = "None" }
	   }
	   if pool.NatOutgoing == nil { pool.NatOutgoing = cfg.IPv4NatOutgoing }
	   if pool.BlockSize == nil { bs := 26; pool.BlockSize = &bs }
	}
}

func SetDefaults_FlannelConfig(cfg *FlannelConfig) {
	if cfg == nil { return }
	if cfg.BackendMode == "" { cfg.BackendMode = "vxlan" }
	if cfg.DirectRouting == nil { b := false; cfg.DirectRouting = &b }
}

func SetDefaults_KubeOvnConfig(cfg *KubeOvnConfig) {
	if cfg == nil { return }
	if cfg.Enabled == nil { b := false; cfg.Enabled = &b }
	if cfg.Enabled != nil && *cfg.Enabled { // Only default these if KubeOvn is actually enabled
		if cfg.Label == nil { def := "kube-ovn/role"; cfg.Label = &def }
		if cfg.TunnelType == nil { def := "geneve"; cfg.TunnelType = &def }
		if cfg.EnableSSL == nil { b := false; cfg.EnableSSL = &b }
		// JoinCIDR has no obvious universal default, should be user-provided if joining.
	}
}

func SetDefaults_HybridnetConfig(cfg *HybridnetConfig) {
	if cfg == nil { return }
	if cfg.Enabled == nil { b := false; cfg.Enabled = &b }
	if cfg.Enabled != nil && *cfg.Enabled {
		if cfg.DefaultNetworkType == nil { def := "Overlay"; cfg.DefaultNetworkType = &def }
		if cfg.EnableNetworkPolicy == nil { b := true; cfg.EnableNetworkPolicy = &b }
		if cfg.InitDefaultNetwork == nil { b := true; cfg.InitDefaultNetwork = &b }
	}
}


// --- Validation Functions ---
func Validate_NetworkConfig(cfg *NetworkConfig, verrs *ValidationErrors, pathPrefix string, k8sSpec *KubernetesConfig) {
	if cfg == nil { verrs.Add("%s: network configuration section cannot be nil", pathPrefix); return }
	// Plugin can be empty if a default is applied by higher level logic or auto-detected.
	// if strings.TrimSpace(cfg.Plugin) == "" { verrs.Add("%s.plugin: CNI plugin name cannot be empty", pathPrefix) }

	podsCIDR := cfg.KubePodsCIDR
	if podsCIDR == "" && k8sSpec != nil { podsCIDR = k8sSpec.PodSubnet } // Use from KubernetesConfig if empty here
	if strings.TrimSpace(podsCIDR) == "" {
		verrs.Add("%s.kubePodsCIDR: (or kubernetes.podSubnet) cannot be empty", pathPrefix)
	} else if !isValidCIDR(podsCIDR) { // isValidCIDR from kubernetes_types.go
		verrs.Add("%s.kubePodsCIDR: invalid CIDR format '%s'", pathPrefix, podsCIDR)
	}

	serviceCIDR := cfg.KubeServiceCIDR
	if serviceCIDR == "" && k8sSpec != nil { serviceCIDR = k8sSpec.ServiceSubnet }
	if serviceCIDR != "" && !isValidCIDR(serviceCIDR) { // serviceCIDR can sometimes be empty
	   verrs.Add("%s.kubeServiceCIDR: invalid CIDR format '%s'", pathPrefix, serviceCIDR)
	}

	if cfg.Plugin == "calico" {
		if cfg.Calico == nil { verrs.Add("%s.calico: config cannot be nil if plugin is 'calico'", pathPrefix)
		} else { Validate_CalicoConfig(cfg.Calico, verrs, pathPrefix+".calico") }
	}
	if cfg.Plugin == "flannel" {
		if cfg.Flannel == nil { verrs.Add("%s.flannel: config cannot be nil if plugin is 'flannel'", pathPrefix)
		} else { Validate_FlannelConfig(cfg.Flannel, verrs, pathPrefix+".flannel") }
	}

	if cfg.KubeOvn != nil && cfg.KubeOvn.Enabled != nil && *cfg.KubeOvn.Enabled {
		Validate_KubeOvnConfig(cfg.KubeOvn, verrs, pathPrefix+".kubeovn")
	}
	if cfg.Hybridnet != nil && cfg.Hybridnet.Enabled != nil && *cfg.Hybridnet.Enabled {
		Validate_HybridnetConfig(cfg.Hybridnet, verrs, pathPrefix+".hybridnet")
	}
}

func Validate_CalicoConfig(cfg *CalicoConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	validEncModes := []string{"Always", "CrossSubnet", "Never", ""}
	if !contains(validEncModes, cfg.IPIPMode) { verrs.Add("%s.ipipMode: invalid: '%s'", pathPrefix, cfg.IPIPMode) }
	if !contains(validEncModes, cfg.VXLANMode) { verrs.Add("%s.vxlanMode: invalid: '%s'", pathPrefix, cfg.VXLANMode) }
	if cfg.VethMTU != nil && *cfg.VethMTU < 0 { verrs.Add("%s.vethMTU: invalid: %d", pathPrefix, *cfg.VethMTU) } // MTU should be non-negative, 0 can mean auto
	if cfg.EnableTypha != nil && *cfg.EnableTypha && (cfg.TyphaReplicas == nil || *cfg.TyphaReplicas <=0) {
	   verrs.Add("%s.typhaReplicas: must be positive if Typha is enabled", pathPrefix)
	}
	validLogSeverities := []string{"Info", "Debug", "Warning", "Error", "Critical", "None", ""}
	if cfg.LogSeverityScreen != nil && !contains(validLogSeverities, *cfg.LogSeverityScreen) {
	   verrs.Add("%s.logSeverityScreen: invalid: '%s'", pathPrefix, *cfg.LogSeverityScreen)
	}
	for i, pool := range cfg.IPPools {
	   poolPath := fmt.Sprintf("%s.ipPools[%d:%s]", pathPrefix, i, pool.Name)
	   if strings.TrimSpace(pool.CIDR) == "" { verrs.Add("%s.cidr: cannot be empty", poolPath) }
	   else if !isValidCIDR(pool.CIDR) { verrs.Add("%s.cidr: invalid CIDR '%s'", poolPath, pool.CIDR) }
	   if pool.BlockSize != nil && (*pool.BlockSize < 20 || *pool.BlockSize > 32) { // Calico typical range
		   verrs.Add("%s.blockSize: must be between 20 and 32, got %d", poolPath, *pool.BlockSize)
	   }
	   validPoolEncap := []string{"IPIP", "VXLAN", "None", ""} // Simplified from Calico's full list
	   if !contains(validPoolEncap, pool.Encapsulation) { verrs.Add("%s.encapsulation: invalid: '%s'", poolPath, pool.Encapsulation)}
	}
}

func Validate_FlannelConfig(cfg *FlannelConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	validBackendModes := []string{"vxlan", "host-gw", "udp", ""}
	if !contains(validBackendModes, cfg.BackendMode) {
		verrs.Add("%s.backendMode: invalid: '%s'", pathPrefix, cfg.BackendMode)
	}
}

func Validate_KubeOvnConfig(cfg *KubeOvnConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Enabled == nil || !*cfg.Enabled { return } // Only validate if enabled

	if cfg.Label != nil && strings.TrimSpace(*cfg.Label) == "" {
		verrs.Add("%s.label: cannot be empty if specified", pathPrefix)
	}
	if cfg.TunnelType != nil && *cfg.TunnelType != "" {
		validTypes := []string{"geneve", "vxlan", "stt"}
		if !contains(validTypes, *cfg.TunnelType) { // contains() from network_types.go
			verrs.Add("%s.tunnelType: invalid type '%s', must be one of %v", pathPrefix, *cfg.TunnelType, validTypes)
		}
	}
	if cfg.JoinCIDR != nil && *cfg.JoinCIDR != "" && !isValidCIDR(*cfg.JoinCIDR) { // isValidCIDR from kubernetes_types.go
		 verrs.Add("%s.joinCIDR: invalid CIDR format '%s'", pathPrefix, *cfg.JoinCIDR)
	}
}

func Validate_HybridnetConfig(cfg *HybridnetConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Enabled == nil || !*cfg.Enabled { return } // Only validate if enabled
	if cfg.DefaultNetworkType != nil && *cfg.DefaultNetworkType != "" {
		validTypes := []string{"Underlay", "Overlay"}
		if !contains(validTypes, *cfg.DefaultNetworkType) {
			verrs.Add("%s.defaultNetworkType: invalid type '%s', must be one of %v", pathPrefix, *cfg.DefaultNetworkType, validTypes)
		}
	}
}


// --- Helper Methods & Functions ---
func (n *NetworkConfig) EnableMultusCNI() bool {
	if n != nil && n.Multus != nil && n.Multus.Enabled != nil { return *n.Multus.Enabled }
	return false
}
func (c *CalicoConfig) IsTyphaEnabled() bool {
   if c != nil && c.EnableTypha != nil { return *c.EnableTypha }
   return false
}
func (c *CalicoConfig) GetTyphaReplicas() int { // Return int for simplicity
   if c != nil && c.TyphaReplicas != nil { return *c.TyphaReplicas }
   if c.IsTyphaEnabled() { return 2 }
   return 0
}
func (c *CalicoConfig) GetVethMTU() int {
   if c != nil && c.VethMTU != nil && *c.VethMTU > 0 { return *c.VethMTU } // Allow 0 to mean auto/default
   return 0 // 0 means Calico auto-detects or uses its internal default
}

func contains(slice []string, item string) bool {
	for _, s := range slice { if s == item { return true } }
	return false
}
// isValidCIDR is expected to be available from kubernetes_types.go or a shared util.
// For self-containment if processed independently:
// func isValidCIDR(cidr string) bool { _, _, err := net.ParseCIDR(cidr); return err == nil }
