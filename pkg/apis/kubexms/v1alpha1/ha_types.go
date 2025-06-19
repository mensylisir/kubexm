package v1alpha1

// HighAvailabilityConfig defines settings for cluster high availability.
type HighAvailabilityConfig struct {
	// Type of HA. e.g., "keepalived+haproxy", "external_lb", "none"
	// Specific fields for each type might be added later.
	Type string `json:"type,omitempty"`
	// VIP is the virtual IP address for HA configurations like keepalived.
	VIP string `json:"vip,omitempty"`
	// TODO: Add more fields based on KubeKey or other requirements,
	// e.g., for load balancer addresses, specific keepalived/haproxy settings.
}

// SetDefaults_HighAvailabilityConfig sets default values for HighAvailabilityConfig.
func SetDefaults_HighAvailabilityConfig(cfg *HighAvailabilityConfig) {
	if cfg == nil {
		return
	}
	if cfg.Type == "" {
		// Default HA type could be "none" or intelligent based on node counts.
		// For now, leave empty; validation can enforce if needed.
	}
}

// Validate_HighAvailabilityConfig validates HighAvailabilityConfig.
func Validate_HighAvailabilityConfig(cfg *HighAvailabilityConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	// Example validation:
	if cfg.Type == "keepalived" && cfg.VIP == "" { // Assuming "keepalived" is a valid type
		verrs.Add("%s.vip: must be set if HA type is 'keepalived'", pathPrefix)
	}
	// Add more validation for VIP format, other types, etc.
}
