package v1alpha1

import (
	"net" // Add to imports of ha_types.go
	"strings"
)

// HighAvailabilityConfig defines settings for cluster high availability.
type HighAvailabilityConfig struct {
	// Type of HA. e.g., "keepalived", "external" (for external load balancers).
	// "none" or empty implies no specific HA mechanism managed by this config.
	Type string `json:"type,omitempty"`

	// VIP is the virtual IP address for HA configurations like keepalived.
	// Required if Type is "keepalived".
	VIP string `json:"vip,omitempty"`

	// ControlPlaneEndpointDomain is the domain name for the control plane endpoint.
	// Used for generating kubeconfig and API server access. e.g., "lb.kubexms.internal"
	ControlPlaneEndpointDomain string `json:"controlPlaneEndpointDomain,omitempty"`

	// ControlPlaneEndpointAddress is the IP address of the load balancer or VIP if different from VIP field.
	// If using an external LB, this would be its IP.
	ControlPlaneEndpointAddress string `json:"controlPlaneEndpointAddress,omitempty"`

	// ControlPlaneEndpointPort is the port for the control plane endpoint.
	// Defaults to 6443.
	ControlPlaneEndpointPort *int `json:"controlPlaneEndpointPort,omitempty"`

	// TODO: Add specific configs for different HA types if needed, e.g. KeepalivedConfig, HAProxyConfig.
	// For instance, KubeKey's ControlPlaneEndpoint has KubeVip settings.
}

// SetDefaults_HighAvailabilityConfig sets default values for HighAvailabilityConfig.
func SetDefaults_HighAvailabilityConfig(cfg *HighAvailabilityConfig) {
	if cfg == nil {
		return
	}
	// if cfg.Type == "" { // No default HA type, must be explicit or handled by higher logic
	//    cfg.Type = "none"
	// }
	if cfg.ControlPlaneEndpointPort == nil {
		defaultPort := 6443
		cfg.ControlPlaneEndpointPort = &defaultPort
	}
}

// Validate_HighAvailabilityConfig validates HighAvailabilityConfig.
func Validate_HighAvailabilityConfig(cfg *HighAvailabilityConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	validTypes := []string{"keepalived", "external", "none", ""} // "" treated as "none" or unspecified
	isTypeValid := false
	for _, vt := range validTypes { if cfg.Type == vt { isTypeValid = true; break } }
	if !isTypeValid {
		verrs.Add("%s.type: invalid HA type '%s', must be one of %v", pathPrefix, cfg.Type, validTypes)
	}

	if cfg.Type == "keepalived" && strings.TrimSpace(cfg.VIP) == "" {
		verrs.Add("%s.vip: must be set if HA type is 'keepalived'", pathPrefix)
	}
	if cfg.VIP != "" && !isValidIP(cfg.VIP) { // isValidIP would be a helper like isValidCIDR
		verrs.Add("%s.vip: invalid IP address format '%s'", pathPrefix, cfg.VIP)
	}
	if cfg.ControlPlaneEndpointAddress != "" && !isValidIP(cfg.ControlPlaneEndpointAddress) {
		verrs.Add("%s.controlPlaneEndpointAddress: invalid IP address format '%s'", pathPrefix, cfg.ControlPlaneEndpointAddress)
	}
	if cfg.ControlPlaneEndpointPort != nil && (*cfg.ControlPlaneEndpointPort <= 0 || *cfg.ControlPlaneEndpointPort > 65535) {
		verrs.Add("%s.controlPlaneEndpointPort: invalid port %d", pathPrefix, *cfg.ControlPlaneEndpointPort)
	}
	// ControlPlaneEndpointDomain could be validated for hostname format.
}

func isValidIP(ip string) bool { return net.ParseIP(ip) != nil }
