package v1alpha1

import (
	"strings"
	"github.com/mensylisir/kubexm/pkg/util"
	"github.com/mensylisir/kubexm/pkg/common"
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
	RouterID      string `json:"routerID,omitempty" yaml:"routerID,omitempty"`             // BGP Router ID
	PeerAddress   string `json:"peerAddress,omitempty" yaml:"peerAddress,omitempty"`       // BGP Peer IP Address
	PeerASN       uint32 `json:"peerASN,omitempty" yaml:"peerASN,omitempty"`                // BGP Peer ASN
	ASN           uint32 `json:"asn,omitempty" yaml:"asn,omitempty"`                        // Local BGP ASN
	SourceAddress string `json:"sourceAddress,omitempty" yaml:"sourceAddress,omitempty"` // Optional: Specific source IP for BGP peering
}

// KubeVIPConfig defines settings for using Kube-VIP.
type KubeVIPConfig struct {
	// Mode for Kube-VIP operation. Supported: "ARP", "BGP".
	// Defaults to "ARP".
	Mode *string `json:"mode,omitempty" yaml:"mode,omitempty"`

	// VIP is the Virtual IP address that Kube-VIP will manage.
	// This field is often required.
	VIP *string `json:"vip,omitempty" yaml:"vip,omitempty"`

	// Interface is the network interface Kube-VIP should use.
	// Crucial for ARP mode. Example: "eth0".
	Interface *string `json:"interface,omitempty" yaml:"interface,omitempty"`

	// EnableControlPlaneLB indicates if Kube-VIP should manage the VIP for the Kubernetes control plane.
	// Defaults to true if KubeVIP is configured.
	EnableControlPlaneLB *bool `json:"enableControlPlaneLB,omitempty" yaml:"enableControlPlaneLB,omitempty"`

	// EnableServicesLB indicates if Kube-VIP should provide LoadBalancer services for Kubernetes Services.
	// Defaults to false.
	EnableServicesLB *bool `json:"enableServicesLB,omitempty" yaml:"enableServicesLB,omitempty"`

	// Image allows overriding the default Kube-VIP container image.
	// Example: "ghcr.io/kube-vip/kube-vip:v0.5.7"
	Image *string `json:"image,omitempty" yaml:"image,omitempty"`

	// ExtraArgs allows passing additional command-line arguments to the Kube-VIP process.
	ExtraArgs []string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`

	// BGPConfig holds BGP specific settings, used if Mode is "BGP".
	BGPConfig *KubeVIPBGPConfig `json:"bgpConfig,omitempty" yaml:"bgpConfig,omitempty"`
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
	if cfg.Image == nil || *cfg.Image == "" { // Set default image if not specified or empty
		cfg.Image = util.StrPtr(common.DefaultKubeVIPImage)
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
	   if !util.ContainsString(validKubeVIPModes, *cfg.Mode) {
		   verrs.Add(pathPrefix+".mode: invalid mode '"+*cfg.Mode+"', must be one of ARP, BGP")
	   }
	}
	if cfg.VIP == nil || strings.TrimSpace(*cfg.VIP) == "" {
		verrs.Add(pathPrefix+".vip", "virtual IP address must be specified")
	} else if !util.IsValidIP(*cfg.VIP) {
		verrs.Add(pathPrefix+".vip: invalid IP address format '"+*cfg.VIP+"'")
	}
	if cfg.Mode != nil && *cfg.Mode == KubeVIPModeARP {
		if cfg.Interface == nil || strings.TrimSpace(*cfg.Interface) == "" {
			verrs.Add(pathPrefix+".interface", "network interface must be specified for ARP mode")
		}
	}
	if cfg.Image != nil && strings.TrimSpace(*cfg.Image) == "" {
	   verrs.Add(pathPrefix+".image", "cannot be empty if specified")
	}
	if cfg.Mode != nil && *cfg.Mode == KubeVIPModeBGP {
	   if cfg.BGPConfig == nil {
		   verrs.Add(pathPrefix+".bgpConfig", "BGP configuration must be provided for BGP mode")
	   } else {
		   bp := pathPrefix + ".bgpConfig"
		   if strings.TrimSpace(cfg.BGPConfig.RouterID) == "" {
			   verrs.Add(bp+".routerID: router ID must be specified for BGP mode")
		   }
		   if cfg.BGPConfig.ASN == 0 { verrs.Add(bp+".asn: local ASN must be specified for BGP mode")}
		   if cfg.BGPConfig.PeerASN == 0 { verrs.Add(bp+".peerASN: peer ASN must be specified for BGP mode")}
		   if strings.TrimSpace(cfg.BGPConfig.PeerAddress) == "" { verrs.Add(bp+".peerAddress: peer address must be specified for BGP mode")
		   } else if !util.IsValidIP(cfg.BGPConfig.PeerAddress) {
			   verrs.Add(bp+".peerAddress: invalid IP '"+cfg.BGPConfig.PeerAddress+"'")
		   }
			if cfg.BGPConfig.SourceAddress != "" && !util.IsValidIP(cfg.BGPConfig.SourceAddress) {
				verrs.Add(bp+".sourceAddress: invalid IP address format '"+cfg.BGPConfig.SourceAddress+"'")
			}
	   }
	}
}
