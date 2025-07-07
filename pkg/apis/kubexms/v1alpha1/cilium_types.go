package v1alpha1

import (
	"fmt" // Import fmt
	"github.com/mensylisir/kubexm/pkg/util" // Added import for util
	"github.com/mensylisir/kubexm/pkg/util/validation"
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
		cfg.EnableHubble = true // EnableHubble is bool, so this is fine.
	}

	// EnableBPFMasquerade is now *bool in CiliumConfig (network_types.go)
	if cfg.EnableBPFMasquerade == nil {
		cfg.EnableBPFMasquerade = util.BoolPtr(true) // Default to true if not specified
	}
	// EnableHubble (bool type) defaults to false unless HubbleUI is true (already handled above)
}

// Validate_CiliumConfig validates CiliumConfig.
// The CiliumConfig struct itself is defined in network_types.go.
func Validate_CiliumConfig(cfg *CiliumConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	if cfg.TunnelingMode != "" {
		// Use package-level variable from network_types.go
		if !util.ContainsString(validCiliumTunnelModes, cfg.TunnelingMode) {
			verrs.Add(pathPrefix+".tunnelingMode", fmt.Sprintf("invalid mode '%s', must be one of %v", cfg.TunnelingMode, validCiliumTunnelModes))
		}
	}

	if cfg.KubeProxyReplacement != "" {
		// Use package-level variable from network_types.go
		if !util.ContainsString(validCiliumKPRModes, cfg.KubeProxyReplacement) {
			verrs.Add(pathPrefix+".kubeProxyReplacement", fmt.Sprintf("invalid mode '%s', must be one of %v", cfg.KubeProxyReplacement, validCiliumKPRModes))
		}
	}

	if cfg.IdentityAllocationMode != "" {
		// Use package-level variable from network_types.go
		if !util.ContainsString(validCiliumIdentModes, cfg.IdentityAllocationMode) {
			verrs.Add(pathPrefix+".identityAllocationMode", fmt.Sprintf("invalid mode '%s', must be one of %v", cfg.IdentityAllocationMode, validCiliumIdentModes))
		}
	}

	// The case where cfg.HubbleUI is true and cfg.EnableHubble is false
	// is handled by SetDefaults_CiliumConfig, which will set cfg.EnableHubble to true.
	// Therefore, this specific inconsistent state check is not strictly needed here
	// if defaults are always applied before validation.
}
