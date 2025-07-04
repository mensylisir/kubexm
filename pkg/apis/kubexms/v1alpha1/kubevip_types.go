package v1alpha1

import (
	"strings"
	"github.com/mensylisir/kubexm/pkg/util" // Ensured util is imported
)

const (
	KubeVIPModeARP = "ARP"
	KubeVIPModeBGP = "BGP"
)

var (
	// validKubeVIPModes lists the supported KubeVIP operation modes.
	validKubeVIPModes = []string{KubeVIPModeARP, KubeVIPModeBGP}
)

// KubeVIPBGPConfig holds BGP specific settings for KubeVIP.
type KubeVIPBGPConfig struct {
	RouterID      string `json:"routerID,omitempty"`      // BGP Router ID
	PeerAddress   string `json:"peerAddress,omitempty"`   // BGP Peer IP Address
	PeerASN       uint32 `json:"peerASN,omitempty"`       // BGP Peer ASN
	ASN           uint32 `json:"asn,omitempty"`           // Local BGP ASN
	SourceAddress string `json:"sourceAddress,omitempty"` // Optional: Specific source IP for BGP peering
}

// KubeVIPConfig defines settings for using Kube-VIP.
type KubeVIPConfig struct {
	// Mode for Kube-VIP operation. Supported: "ARP", "BGP".
	// Defaults to "ARP".
	Mode *string `json:"mode,omitempty"`

	// VIP is the Virtual IP address that Kube-VIP will manage.
	// This field is often required.
	VIP *string `json:"vip,omitempty"`

	// Interface is the network interface Kube-VIP should use.
	// Crucial for ARP mode. Example: "eth0".
	Interface *string `json:"interface,omitempty"`

	// EnableControlPlaneLB indicates if Kube-VIP should manage the VIP for the Kubernetes control plane.
	// Defaults to true if KubeVIP is configured.
	EnableControlPlaneLB *bool `json:"enableControlPlaneLB,omitempty"`

	// EnableServicesLB indicates if Kube-VIP should provide LoadBalancer services for Kubernetes Services.
	// Defaults to false.
	EnableServicesLB *bool `json:"enableServicesLB,omitempty"`

	// Image allows overriding the default Kube-VIP container image.
	// Example: "ghcr.io/kube-vip/kube-vip:v0.5.7"
	Image *string `json:"image,omitempty"`

	// ExtraArgs allows passing additional command-line arguments to the Kube-VIP process.
	ExtraArgs []string `json:"extraArgs,omitempty"`

	// BGPConfig holds BGP specific settings, used if Mode is "BGP".
	BGPConfig *KubeVIPBGPConfig `json:"bgpConfig,omitempty"`
}

// SetDefaults_KubeVIPConfig sets default values for KubeVIPConfig.
func SetDefaults_KubeVIPConfig(cfg *KubeVIPConfig) {
	if cfg == nil {
		return
	}
	if cfg.Mode == nil {
		cfg.Mode = util.StrPtr(KubeVIPModeARP) // Use util.StrPtr
	}
	if cfg.EnableControlPlaneLB == nil {
		cfg.EnableControlPlaneLB = util.BoolPtr(true)
	}
	if cfg.EnableServicesLB == nil {
		cfg.EnableServicesLB = util.BoolPtr(false)
	}
	if cfg.ExtraArgs == nil {
		cfg.ExtraArgs = []string{}
	}
	if cfg.Mode != nil && *cfg.Mode == KubeVIPModeBGP && cfg.BGPConfig == nil {
	   cfg.BGPConfig = &KubeVIPBGPConfig{}
	}
}

// Validate_KubeVIPConfig validates KubeVIPConfig.
func Validate_KubeVIPConfig(cfg *KubeVIPConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.Mode != nil && *cfg.Mode != "" {
	   if !util.ContainsString(validKubeVIPModes, *cfg.Mode) { // Use package-level var
		   verrs.Add("%s.mode: invalid mode '%s', must be one of %v", pathPrefix, *cfg.Mode, validKubeVIPModes)
	   }
	}
	if cfg.VIP == nil || strings.TrimSpace(*cfg.VIP) == "" {
		verrs.Add("%s.vip: virtual IP address must be specified", pathPrefix)
	} else if !util.IsValidIP(*cfg.VIP) { // Use util.IsValidIP
		verrs.Add("%s.vip: invalid IP address format '%s'", pathPrefix, *cfg.VIP)
	}
	if cfg.Mode != nil && *cfg.Mode == KubeVIPModeARP {
		if cfg.Interface == nil || strings.TrimSpace(*cfg.Interface) == "" {
			verrs.Add("%s.interface: network interface must be specified for ARP mode", pathPrefix)
		}
	}
	if cfg.Image != nil && strings.TrimSpace(*cfg.Image) == "" {
	   verrs.Add("%s.image: cannot be empty if specified", pathPrefix)
	}
	if cfg.Mode != nil && *cfg.Mode == KubeVIPModeBGP {
	   if cfg.BGPConfig == nil {
		   verrs.Add("%s.bgpConfig: BGP configuration must be provided for BGP mode", pathPrefix)
	   } else {
		   bp := pathPrefix + ".bgpConfig"
		   if strings.TrimSpace(cfg.BGPConfig.RouterID) == "" {
			   verrs.Add("%s.routerID: router ID must be specified for BGP mode", bp)
		   }
		   if cfg.BGPConfig.ASN == 0 { verrs.Add("%s.asn: local ASN must be specified for BGP mode", bp)}
		   if cfg.BGPConfig.PeerASN == 0 { verrs.Add("%s.peerASN: peer ASN must be specified for BGP mode", bp)}
		   if strings.TrimSpace(cfg.BGPConfig.PeerAddress) == "" { verrs.Add("%s.peerAddress: peer address must be specified for BGP mode", bp)
		   } else if !util.IsValidIP(cfg.BGPConfig.PeerAddress) { // Use util.IsValidIP
			   verrs.Add("%s.peerAddress: invalid IP '%s'", bp, cfg.BGPConfig.PeerAddress)
		   }
			if cfg.BGPConfig.SourceAddress != "" && !util.IsValidIP(cfg.BGPConfig.SourceAddress) { // Use util.IsValidIP
				verrs.Add("%s.sourceAddress: invalid IP address format '%s'", bp, cfg.BGPConfig.SourceAddress)
			}
	   }
	}
}
