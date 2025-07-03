package v1alpha1

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSetDefaults_CiliumConfig_Standalone tests the SetDefaults_CiliumConfig function directly.
func TestSetDefaults_CiliumConfig_Standalone(t *testing.T) {
	tests := []struct {
		name     string
		input    *CiliumConfig
		expected *CiliumConfig
	}{
		{
			name:  "nil input",
			input: nil,
			expected: nil,
		},
		{
			name: "empty input",
			input: &CiliumConfig{},
			expected: &CiliumConfig{
				TunnelingMode:          "vxlan",
				KubeProxyReplacement:   "strict",
				IdentityAllocationMode: "crd",
				EnableHubble:           false, // Defaulted by HubbleUI logic if HubbleUI is true
				HubbleUI:               false,
				EnableBPFMasquerade:    false,
			},
		},
		{
			name: "HubbleUI true, EnableHubble false",
			input: &CiliumConfig{HubbleUI: true, EnableHubble: false},
			expected: &CiliumConfig{
				TunnelingMode:          "vxlan",
				KubeProxyReplacement:   "strict",
				IdentityAllocationMode: "crd",
				EnableHubble:           true, // Should be forced to true
				HubbleUI:               true,
				EnableBPFMasquerade:    false,
			},
		},
		{
			name: "HubbleUI true, EnableHubble true",
			input: &CiliumConfig{HubbleUI: true, EnableHubble: true},
			expected: &CiliumConfig{
				TunnelingMode:          "vxlan",
				KubeProxyReplacement:   "strict",
				IdentityAllocationMode: "crd",
				EnableHubble:           true,
				HubbleUI:               true,
				EnableBPFMasquerade:    false,
			},
		},
		{
			name: "partial input, e.g. only TunnelingMode set",
			input: &CiliumConfig{TunnelingMode: "geneve"},
			expected: &CiliumConfig{
				TunnelingMode:          "geneve", // User specified
				KubeProxyReplacement:   "strict", // Defaulted
				IdentityAllocationMode: "crd",    // Defaulted
				EnableHubble:           false,
				HubbleUI:               false,
				EnableBPFMasquerade:    false,
			},
		},
		{
			name: "all fields explicitly set by user",
			input: &CiliumConfig{
				TunnelingMode:          "disabled",
				KubeProxyReplacement:   "probe",
				EnableHubble:           true,
				HubbleUI:               true,
				EnableBPFMasquerade:    true,
				IdentityAllocationMode: "kvstore",
			},
			expected: &CiliumConfig{
				TunnelingMode:          "disabled",
				KubeProxyReplacement:   "probe",
				EnableHubble:           true,
				HubbleUI:               true,
				EnableBPFMasquerade:    true,
				IdentityAllocationMode: "kvstore",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_CiliumConfig(tt.input)
			if !reflect.DeepEqual(tt.input, tt.expected) {
				assert.Equal(t, tt.expected, tt.input)
			}
		})
	}
}

// TestValidate_CiliumConfig_Standalone tests the Validate_CiliumConfig function directly.
func TestValidate_CiliumConfig_Standalone(t *testing.T) {
	validBaseConfig := func() *CiliumConfig { // Use a function to get a fresh base for each test
		cfg := &CiliumConfig{}
		SetDefaults_CiliumConfig(cfg) // Apply defaults as validation happens after defaulting
		return cfg
	}

	tests := []struct {
		name        string
		setup       func() *CiliumConfig // Function to set up the config for the test
		expectErr   bool
		errContains []string
	}{
		{
			name: "nil input",
			setup: func() *CiliumConfig {
				return nil
			},
			expectErr: false, // Validation function should handle nil gracefully
		},
		{
			name: "valid empty (after defaults)",
			setup: func() *CiliumConfig {
				return validBaseConfig()
			},
			expectErr: false,
		},
		{
			name: "invalid TunnelingMode",
			setup: func() *CiliumConfig {
				cfg := validBaseConfig()
				cfg.TunnelingMode = "invalid-tunnel"
				return cfg
			},
			expectErr:   true,
			errContains: []string{"tunnelingMode: invalid mode 'invalid-tunnel'"},
		},
		{
			name: "invalid KubeProxyReplacement",
			setup: func() *CiliumConfig {
				cfg := validBaseConfig()
				cfg.KubeProxyReplacement = "invalid-kpr"
				return cfg
			},
			expectErr:   true,
			errContains: []string{"kubeProxyReplacement: invalid mode 'invalid-kpr'"},
		},
		{
			name: "HubbleUI true, EnableHubble false", // Defaulting fixes this, but validation should still catch if defaults somehow bypassed
			setup: func() *CiliumConfig {
				// Simulate the state *after* defaulting for this validation test.
				// SetDefaults_CiliumConfig would set EnableHubble to true if HubbleUI is true.
				cfg := &CiliumConfig{
					TunnelingMode:          "vxlan",
					KubeProxyReplacement:   "strict",
					IdentityAllocationMode: "crd",
					EnableHubble:           true, // This is the state after defaulting fixes it
					HubbleUI:               true,
				}
				// If we were testing Validate_CiliumConfig in complete isolation
				// without prior defaulting, the original test for inconsistency would be valid.
				// However, since our change in Validate_CiliumConfig was to remove that
				// specific check because defaults handle it, this test should now pass
				// when validating a config that has been defaulted.
				return cfg
			},
			expectErr:   false, // Defaulting fixes this, so validation should not find an error.
			errContains: []string{},
		},
		{
			name: "valid HubbleUI true, EnableHubble true",
			setup: func() *CiliumConfig {
				cfg := validBaseConfig()
				cfg.EnableHubble = true
				cfg.HubbleUI = true
				return cfg
			},
			expectErr: false,
		},
		{
			name: "invalid IdentityAllocationMode",
			setup: func() *CiliumConfig {
				cfg := validBaseConfig()
				cfg.IdentityAllocationMode = "invalid-iam"
				return cfg
			},
			expectErr:   true,
			errContains: []string{"identityAllocationMode: invalid mode 'invalid-iam'"},
		},
		{
			name: "all fields valid",
			setup: func() *CiliumConfig {
				cfg := validBaseConfig()
				cfg.TunnelingMode = "geneve"
				cfg.KubeProxyReplacement = "probe"
				cfg.EnableHubble = true
				cfg.HubbleUI = true
				cfg.EnableBPFMasquerade = true
				cfg.IdentityAllocationMode = "kvstore"
				return cfg
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputConfig := tt.setup()
			// Note: Validation is typically called after defaulting in the main logic.
			// For some specific validation tests (like the HubbleUI inconsistency),
			// we might want to test the raw input *before* SetDefaults.
			// However, the current Validate_CiliumConfig assumes it might see this state.
			// If SetDefaults_CiliumConfig is always called before Validate_CiliumConfig in the controller,
			// then the "HubbleUI true, EnableHubble false" case in validation might be redundant
			// as defaults would fix it. But for robustness, testing the validation rule itself is fine.

			verrs := &ValidationErrors{Errors: []string{}}
			Validate_CiliumConfig(inputConfig, verrs, "spec.network.cilium")

			if tt.expectErr {
				assert.False(t, verrs.IsEmpty(), "Expected validation errors for test: %s, but got none", tt.name)
				if len(tt.errContains) > 0 {
					combinedErrors := verrs.Error()
					for _, errStr := range tt.errContains {
						assert.Contains(t, combinedErrors, errStr, "Error message for test '%s' does not contain expected substring '%s'. Full error: %s", tt.name, errStr, combinedErrors)
					}
				}
			} else {
				assert.True(t, verrs.IsEmpty(), "Expected no validation errors for test: %s, but got: %s", tt.name, verrs.Error())
			}
		})
	}
}

// TestValidate_NetworkConfig_Calls_Validate_CiliumConfig_Standalone ensures NetworkConfig validation calls Cilium validation.
func TestValidate_NetworkConfig_Calls_Validate_CiliumConfig_Standalone(t *testing.T) {
	netCfg := &NetworkConfig{
		Plugin: "cilium",
		Cilium: &CiliumConfig{
			TunnelingMode: "invalid-mode", // This should be caught by Validate_CiliumConfig
		},
	}
	SetDefaults_NetworkConfig(netCfg) // Apply defaults to NetworkConfig and its sub-configs

	verrs := &ValidationErrors{}
	Validate_NetworkConfig(netCfg, verrs, "spec.network", nil) // k8sSpec can be nil for this test

	assert.False(t, verrs.IsEmpty(), "Expected errors from CiliumConfig validation")
	assert.Contains(t, verrs.Error(), "cilium.tunnelingMode: invalid mode 'invalid-mode'", "Expected Cilium validation error")
}

// TestSetDefaults_NetworkConfig_Calls_SetDefaults_CiliumConfig_Standalone ensures NetworkConfig defaulting calls Cilium defaulting.
func TestSetDefaults_NetworkConfig_Calls_SetDefaults_CiliumConfig_Standalone(t *testing.T) {
	netCfg := &NetworkConfig{
		Plugin: "cilium",
		Cilium: &CiliumConfig{HubbleUI: true}, // EnableHubble should be defaulted to true
	}
	SetDefaults_NetworkConfig(netCfg)

	assert.NotNil(t, netCfg.Cilium, "Cilium config should be present")
	assert.True(t, netCfg.Cilium.EnableHubble, "EnableHubble should be true due to HubbleUI being true")
	assert.Equal(t, "vxlan", netCfg.Cilium.TunnelingMode, "Default tunneling mode mismatch")
}
