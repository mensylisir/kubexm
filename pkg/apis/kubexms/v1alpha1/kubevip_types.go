package v1alpha1

import (
	"strings"
	// "net" // For isValidIP if needed directly here, but VIP is validated in ha_types or endpoint_types
)

const (
	KubeVIPModeARP = "ARP"
	KubeVIPModeBGP = "BGP"
	// KubeVIPModeFoo = "FOO" // Example if other modes existed
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
	// Note: If KubeVIP is used for ControlPlane LB, this VIP should generally match
	// the HighAvailabilityConfig.ControlPlaneEndpoint.Address or HighAvailabilityConfig.VIP.
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

	// TODO: Add other Kube-VIP specific fields as they become relevant,
	// e.g., LeaderElection, LeaseDuration, etc.
}

// --- Defaulting Functions ---

// SetDefaults_KubeVIPConfig sets default values for KubeVIPConfig.
func SetDefaults_KubeVIPConfig(cfg *KubeVIPConfig) {
	if cfg == nil {
		return
	}
	if cfg.Mode == nil {
		cfg.Mode = stringPtr(KubeVIPModeARP)
	}
	if cfg.EnableControlPlaneLB == nil {
		cfg.EnableControlPlaneLB = boolPtr(true) // Typically, if KubeVIP is used, it's for CP LB.
	}
	if cfg.EnableServicesLB == nil {
		cfg.EnableServicesLB = boolPtr(false) // Service LB is often an optional add-on.
	}
	if cfg.ExtraArgs == nil {
		cfg.ExtraArgs = []string{}
	}
	if cfg.Mode != nil && *cfg.Mode == KubeVIPModeBGP && cfg.BGPConfig == nil {
	   cfg.BGPConfig = &KubeVIPBGPConfig{}
	}
	// VIP and Interface are highly specific and generally required; no universal defaults.
	// Image: let the deployment logic choose a default compatible image if not specified.
}

// --- Validation Functions ---

// Validate_KubeVIPConfig validates KubeVIPConfig.
// Note: ValidationErrors type is expected to be defined in cluster_types.go or a common errors file.
func Validate_KubeVIPConfig(cfg *KubeVIPConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return // Nothing to validate if the config section is absent
	}

	if cfg.Mode != nil && *cfg.Mode != "" {
	   validModes := []string{KubeVIPModeARP, KubeVIPModeBGP}
	   if !containsString(validModes, *cfg.Mode) {
		   verrs.Add("%s.mode: invalid mode '%s', must be one of %v", pathPrefix, *cfg.Mode, validModes)
	   }
	} else { // Mode is nil, but it's defaulted to ARP. This path should ideally not be hit if defaults run first.
	   // If it were possible for Mode to be nil here (e.g. direct validation call without defaults),
	   // it implies a required field is missing. However, SetDefaults_KubeVIPConfig ensures Mode is non-nil.
	   // For robustness, if Mode somehow ends up nil post-defaulting (shouldn't happen), it's an issue.
	   // But the current default logic makes this check for nil redundant for typical validation flow.
	   // The primary check is for valid *values* of Mode.
	   // Let's assume defaults have run, so Mode is never nil. If it was empty string, it's handled by containsString.
	}


	if cfg.VIP == nil || strings.TrimSpace(*cfg.VIP) == "" {
		verrs.Add("%s.vip: virtual IP address must be specified", pathPrefix)
	} else if !isValidIP(*cfg.VIP) { // isValidIP assumed from endpoint_types.go or similar
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
		   // Basic BGP config validation
		   bp := pathPrefix + ".bgpConfig"
		   if strings.TrimSpace(cfg.BGPConfig.RouterID) == "" {
			   verrs.Add("%s.routerID: router ID must be specified for BGP mode", bp)
			   // Kube-VIP might allow non-IP router ID, so primary check is for non-emptiness.
			   // Further validation if it must be an IP could be added if strictness is required:
			   // else if !isValidIP(cfg.BGPConfig.RouterID) {
			   //    verrs.Add("%s.routerID: if specified, must be a valid IP address (or KubeVIP specific format)", bp)
			   // }
		   }
		   if cfg.BGPConfig.ASN == 0 { verrs.Add("%s.asn: local ASN must be specified for BGP mode", bp)}
		   if cfg.BGPConfig.PeerASN == 0 { verrs.Add("%s.peerASN: peer ASN must be specified for BGP mode", bp)}
		   if strings.TrimSpace(cfg.BGPConfig.PeerAddress) == "" { verrs.Add("%s.peerAddress: peer address must be specified for BGP mode", bp)
		   } else if !isValidIP(cfg.BGPConfig.PeerAddress) { verrs.Add("%s.peerAddress: invalid IP '%s'", bp, cfg.BGPConfig.PeerAddress)}

	   }
	}
}
