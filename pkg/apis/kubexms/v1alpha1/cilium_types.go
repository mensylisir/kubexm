package v1alpha1

// Valid values for Cilium configuration
var (
	validCiliumTunnelModes = []string{"vxlan", "geneve", "disabled", ""}
	validCiliumKPRModes    = []string{"probe", "strict", "disabled", ""}
	validCiliumIdentModes  = []string{"crd", "kvstore", ""}
)

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
	// EnableBPFMasquerade defaults to true if not explicitly set
	if cfg.EnableBPFMasquerade == nil {
		trueVal := true
		cfg.EnableBPFMasquerade = &trueVal
	}
}

// Validate_CiliumConfig validates CiliumConfig.
// The CiliumConfig struct itself is defined in network_types.go.
func Validate_CiliumConfig(cfg *CiliumConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	if cfg.TunnelingMode != "" {
		if !containsString(validCiliumTunnelModes, cfg.TunnelingMode) {
			verrs.Add(pathPrefix+".tunnelingMode: invalid mode '"+cfg.TunnelingMode+"', must be one of vxlan, geneve, disabled, or empty")
		}
	}

	if cfg.KubeProxyReplacement != "" {
		if !containsString(validCiliumKPRModes, cfg.KubeProxyReplacement) {
			verrs.Add(pathPrefix+".kubeProxyReplacement: invalid mode '"+cfg.KubeProxyReplacement+"', must be one of probe, strict, disabled, or empty")
		}
	}

	if cfg.IdentityAllocationMode != "" {
		if !containsString(validCiliumIdentModes, cfg.IdentityAllocationMode) {
			verrs.Add(pathPrefix+".identityAllocationMode: invalid mode '"+cfg.IdentityAllocationMode+"', must be one of crd, kvstore, or empty")
		}
	}

	// Check for logical inconsistencies
	if cfg.HubbleUI && !cfg.EnableHubble {
		verrs.Add(pathPrefix+".hubbleUI", "cannot be true if enableHubble is false")
	}
}
