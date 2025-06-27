package v1alpha1

// SetDefaults_CiliumConfig provides default values for CiliumConfig.
// The CiliumConfig struct itself is defined in network_types.go.
func SetDefaults_CiliumConfig(cfg *CiliumConfig) {
	if cfg == nil {
		return
	}
	if cfg.TunnelingMode == "" {
		cfg.TunnelingMode = "vxlan"
	}
	if cfg.KubeProxyReplacement == "" {
		cfg.KubeProxyReplacement = "strict"
	}
	if cfg.IdentityAllocationMode == "" {
		cfg.IdentityAllocationMode = "crd"
	}
	// If HubbleUI is true, EnableHubble should also be true.
	if cfg.HubbleUI && !cfg.EnableHubble {
		cfg.EnableHubble = true
	}
	// EnableBPFMasquerade defaults to false (zero value)
	// EnableHubble defaults to false unless HubbleUI is true
}

// Validate_CiliumConfig validates CiliumConfig.
// The CiliumConfig struct itself is defined in network_types.go.
func Validate_CiliumConfig(cfg *CiliumConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	if cfg.TunnelingMode != "" {
		validTunnelModes := []string{"vxlan", "geneve", "disabled"}
		if !containsString(validTunnelModes, cfg.TunnelingMode) {
			verrs.Add("%s.tunnelingMode: invalid mode '%s', must be one of %v", pathPrefix, cfg.TunnelingMode, validTunnelModes)
		}
	}

	if cfg.KubeProxyReplacement != "" {
		validKPRModes := []string{"probe", "strict", "disabled"}
		if !containsString(validKPRModes, cfg.KubeProxyReplacement) {
			verrs.Add("%s.kubeProxyReplacement: invalid mode '%s', must be one of %v", pathPrefix, cfg.KubeProxyReplacement, validKPRModes)
		}
	}

	if cfg.IdentityAllocationMode != "" {
		validIdentModes := []string{"crd", "kvstore"}
		if !containsString(validIdentModes, cfg.IdentityAllocationMode) {
			verrs.Add("%s.identityAllocationMode: invalid mode '%s', must be one of %v", pathPrefix, cfg.IdentityAllocationMode, validIdentModes)
		}
	}

	if cfg.HubbleUI && !cfg.EnableHubble {
		// This case should ideally be handled by defaulting, but validation can also catch it
		// if defaulting logic changes or isn't applied prior to validation in some scenarios.
		// However, SetDefaults_CiliumConfig now handles this.
		// For strictness, we can keep it:
		verrs.Add("%s.hubbleUI: inconsistent state: hubbleUI is true but enableHubble is false. Defaulting should ensure enableHubble is true when hubbleUI is true.", pathPrefix)
	}
}
