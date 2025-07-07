package v1alpha1

import (
	"fmt" // Import fmt
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
		cfg.EnableHubble = true
	}
	if !cfg.EnableBPFMasquerade { // Default to true if not set (zero value is false)
		// This logic is slightly off if we want to default to true only if the field is nil.
		// However, bools don't have a nil state. If the user explicitly sets it to false, this would override.
		// A common pattern for defaulting a bool to true if not specified is:
		// if cfg.EnableBPFMasquerade == nil { // This would require EnableBPFMasquerade to be *bool
		//    b := true; cfg.EnableBPFMasquerade = &b
		// }
		// Given EnableBPFMasquerade is bool, if we want its effective default to be true
		// when the user doesn't specify it, we'd have to check if it's the zero value (false)
		// and then set it. But this means a user cannot explicitly set it to false if this logic is here.
		// The current struct has `EnableBPFMasquerade bool`.
		// If we want to make its *effective* default true if unspecified:
		// This is typically done by the controller interpreting the zero value.
		// Or, if the API intends true as default when user omits, it should be *bool.
		// Assuming the intent is to default to true if user hasn't set it (which means it's currently false):
		// This interpretation is tricky for non-pointer booleans.
		// Let's assume for now the existing struct field `EnableBPFMasquerade bool` means:
		// - if user provides `enableBPFMasquerade: true`, it's true.
		// - if user provides `enableBPFMasquerade: false`, it's false.
		// - if user omits `enableBPFMasquerade`, it's `false` (Go's zero value).
		// To change the *effective default upon omission* to true, the controller/operator would typically handle this.
		// If we want to change the struct's default initialization value when the field is omitted in YAML,
		// and the field is a plain bool, we can't distinguish "omitted" from "set to false".
		// The plan item says "consider setting to true". This implies if not specified, it becomes true.
		// Given the current type is `bool`, not `*bool`, we cannot do this in `SetDefaults`
		// without overriding an explicit `false` from the user.
		//
		// Let's assume the most straightforward interpretation: if the field *could* be a pointer and isn't,
		// we set it if it's currently the zero value. But this is not ideal.
		// The best way is to make it *bool if a tri-state (set true, set false, not set->default) is desired.
		// Sticking to the current struct type `bool`:
		// If it's false (either omitted or explicitly set to false), and we want the default to be true if omitted,
		// this default function cannot distinguish.
		//
		// Re-evaluating: The current code `EnableBPFMasquerade bool` means it defaults to `false`.
		// If the plan is to change this default to `true` *if the user doesn't specify it*,
		// the controller would need to interpret the `false` (zero value) as "not specified, so use true".
		// This is not something `SetDefaults` can typically do for a non-pointer bool if you want to allow users to set `false`.
		//
		// Let's proceed by NOT changing it here, as the current struct definition doesn't lend itself well
		// to a `SetDefaults` function making `false` mean `true` by default if omitted.
		// The controller/operator would be a better place to interpret the zero value if needed.
		// Or the field should be `*bool`.
		// For now, no change to EnableBPFMasquerade default logic here.
	}
	// EnableHubble defaults to false unless HubbleUI is true
}

// Validate_CiliumConfig validates CiliumConfig.
// The CiliumConfig struct itself is defined in network_types.go.
func Validate_CiliumConfig(cfg *CiliumConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	if cfg.TunnelingMode != "" {
		validTunnelModes := []string{"vxlan", "geneve", "disabled"}
		if !containsString(validTunnelModes, cfg.TunnelingMode) {
			verrs.Add(pathPrefix+".tunnelingMode", fmt.Sprintf("invalid mode '%s', must be one of %v", cfg.TunnelingMode, validTunnelModes))
		}
	}

	if cfg.KubeProxyReplacement != "" {
		validKPRModes := []string{"probe", "strict", "disabled"}
		if !containsString(validKPRModes, cfg.KubeProxyReplacement) {
			verrs.Add(pathPrefix+".kubeProxyReplacement", fmt.Sprintf("invalid mode '%s', must be one of %v", cfg.KubeProxyReplacement, validKPRModes))
		}
	}

	if cfg.IdentityAllocationMode != "" {
		validIdentModes := []string{"crd", "kvstore"}
		if !containsString(validIdentModes, cfg.IdentityAllocationMode) {
			verrs.Add(pathPrefix+".identityAllocationMode", fmt.Sprintf("invalid mode '%s', must be one of %v", cfg.IdentityAllocationMode, validIdentModes))
		}
	}

	// The case where cfg.HubbleUI is true and cfg.EnableHubble is false
	// is handled by SetDefaults_CiliumConfig, which will set cfg.EnableHubble to true.
	// Therefore, this specific inconsistent state check is not strictly needed here
	// if defaults are always applied before validation.
}
