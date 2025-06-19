package v1alpha1

import (
	"strings"
	"net" // For CIDR validation helper
	"fmt" // For path formatting in validation
)

// NetworkConfig defines the network configuration for the cluster.
type NetworkConfig struct {
	Plugin          string `json:"plugin,omitempty"` // e.g., "calico", "flannel", "cilium", "kubeovn"
	KubePodsCIDR    string `json:"kubePodsCIDR,omitempty"`
	KubeServiceCIDR string `json:"kubeServiceCIDR,omitempty"`

	Calico    *CalicoConfig    `json:"calico,omitempty"`
	Flannel   *FlannelConfig   `json:"flannel,omitempty"`
	KubeOvn   *KubeOvnConfig   `json:"kubeovn,omitempty"`
	Multus    *MultusCNIConfig `json:"multus,omitempty"`
	Hybridnet *HybridnetConfig `json:"hybridnet,omitempty"`
}

// CalicoIPPool defines an IP address pool for Calico.
type CalicoIPPool struct {
	Name          string `json:"name,omitempty"` // Name for the IPPool resource
	CIDR          string `json:"cidr"`           // The CIDR for this pool
	Encapsulation string `json:"encapsulation,omitempty"` // e.g., "IPIP", "VXLAN", "None". Defaults based on CalicoConfig.
	NatOutgoing   *bool  `json:"natOutgoing,omitempty"`   // Enable NAT outgoing for this pool. Defaults based on CalicoConfig.
	BlockSize     *int   `json:"blockSize,omitempty"`     // Calico IPAM block size. Default: 26
	// Add other IPPool fields as needed e.g. NodeSelector, AllowedUses etc.
}

// CalicoConfig defines settings specific to the Calico CNI plugin.
type CalicoConfig struct {
	IPIPMode        string `json:"ipipMode,omitempty"` // e.g., "Always", "CrossSubnet", "Never"
	VXLANMode       string `json:"vxlanMode,omitempty"` // e.g., "Always", "CrossSubnet", "Never"
	VethMTU         *int   `json:"vethMTU,omitempty"`  // Changed to *int
	IPv4NatOutgoing *bool  `json:"ipv4NatOutgoing,omitempty"`
	DefaultIPPOOL   *bool  `json:"defaultIPPOOL,omitempty"` // Whether to create a default IPPool if IPPools list is empty
	EnableTypha     *bool  `json:"enableTypha,omitempty"`
	TyphaReplicas   *int   `json:"typhaReplicas,omitempty"` // Changed to *int
	TyphaNodeSelector map[string]string `json:"typhaNodeSelector,omitempty"`
	LogSeverityScreen *string `json:"logSeverityScreen,omitempty"` // e.g., "Info", "Debug", "Warning"

	// IPPools is a list of Calico IPPools to configure.
	// If empty and DefaultIPPOOL is true, Calico typically creates a default pool based on KubePodsCIDR.
	IPPools []CalicoIPPool `json:"ipPools,omitempty"`
}

// FlannelConfig defines settings specific to the Flannel CNI plugin.
type FlannelConfig struct {
	BackendMode   string `json:"backendMode,omitempty"` // e.g., "vxlan", "host-gw", "udp"
	DirectRouting *bool  `json:"directRouting,omitempty"`
}

type KubeOvnConfig struct {
	Enabled *bool `json:"enabled,omitempty"`
	// TODO: Further enrich based on KubeKey's KubeovnCfg
}

type MultusCNIConfig struct {
	Enabled *bool `json:"enabled,omitempty"`
}

type HybridnetConfig struct {
	Enabled *bool `json:"enabled,omitempty"`
	// TODO: Further enrich based on KubeKey's HybridnetCfg
}

// --- Defaulting Functions ---

func SetDefaults_NetworkConfig(cfg *NetworkConfig) {
	if cfg == nil { return }
	// No default plugin, user must specify or have higher-level logic.
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
	if cfg.KubeOvn.Enabled == nil { b := false; cfg.KubeOvn.Enabled = &b }

	if cfg.Hybridnet == nil { cfg.Hybridnet = &HybridnetConfig{} }
	if cfg.Hybridnet.Enabled == nil { b := false; cfg.Hybridnet.Enabled = &b }
}

func SetDefaults_CalicoConfig(cfg *CalicoConfig, defaultPoolCIDR string) {
	if cfg == nil { return }
	if cfg.IPIPMode == "" && cfg.VXLANMode == "" { // If neither is set, default IPIP
	   cfg.IPIPMode = "Always"
	}
	if cfg.IPIPMode == "" { cfg.IPIPMode = "Never" } // Default if only VXLANMode is set
	if cfg.VXLANMode == "" { cfg.VXLANMode = "Never" } // Default if only IPIPMode is set

	if cfg.IPv4NatOutgoing == nil { b := true; cfg.IPv4NatOutgoing = &b }
	if cfg.DefaultIPPOOL == nil { b := true; cfg.DefaultIPPOOL = &b } // Default to creating a default ippool

	if cfg.EnableTypha == nil { b := false; cfg.EnableTypha = &b }
	if cfg.EnableTypha != nil && *cfg.EnableTypha && cfg.TyphaReplicas == nil {
		var defaultReplicas int = 2; cfg.TyphaReplicas = &defaultReplicas
	}
    if cfg.TyphaNodeSelector == nil { cfg.TyphaNodeSelector = make(map[string]string) }
	if cfg.VethMTU == nil {
	   var defaultMTU int = 0 // Let Calico decide default MTU, or set a common one like 1440/1420
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
	// TODO: Validate KubeOvn, Multus, Hybridnet if enabled
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
