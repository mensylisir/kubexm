package v1alpha1

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/mensylisir/kubexm/pkg/util/validation" // Import validation
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
				EnableHubble:           false,
				HubbleUI:               false,
				EnableBPFMasquerade:    boolPtr(true),
			},
		},
		{
			name: "HubbleUI true, EnableHubble false",
			input: &CiliumConfig{HubbleUI: true, EnableHubble: false},
			expected: &CiliumConfig{
				TunnelingMode:          "vxlan",
				KubeProxyReplacement:   "strict",
				IdentityAllocationMode: "crd",
				EnableHubble:           true,
				HubbleUI:               true,
				EnableBPFMasquerade:    boolPtr(true),
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
				EnableBPFMasquerade:    boolPtr(true),
			},
		},
		{
			name: "partial input, e.g. only TunnelingMode set",
			input: &CiliumConfig{TunnelingMode: "geneve"},
			expected: &CiliumConfig{
				TunnelingMode:          "geneve",
				KubeProxyReplacement:   "strict",
				IdentityAllocationMode: "crd",
				EnableHubble:           false,
				HubbleUI:               false,
				EnableBPFMasquerade:    boolPtr(true),
			},
		},
		{
			name: "all fields explicitly set by user, EnableBPFMasquerade set to true by user",
			input: &CiliumConfig{
				TunnelingMode:          "disabled",
				KubeProxyReplacement:   "probe",
				EnableHubble:           true,
				HubbleUI:               true,
				EnableBPFMasquerade:    boolPtr(true),
				IdentityAllocationMode: "kvstore",
			},
			expected: &CiliumConfig{
				TunnelingMode:          "disabled",
				KubeProxyReplacement:   "probe",
				EnableHubble:           true,
				HubbleUI:               true,
				EnableBPFMasquerade:    boolPtr(true),
				IdentityAllocationMode: "kvstore",
			},
		},
		{
			name: "all fields explicitly set by user, EnableBPFMasquerade set to false by user",
			input: &CiliumConfig{
				TunnelingMode:          "disabled",
				KubeProxyReplacement:   "probe",
				EnableHubble:           true,
				HubbleUI:               true,
				EnableBPFMasquerade:    boolPtr(false),
				IdentityAllocationMode: "kvstore",
			},
			expected: &CiliumConfig{
				TunnelingMode:          "disabled",
				KubeProxyReplacement:   "probe",
				EnableHubble:           true,
				HubbleUI:               true,
				EnableBPFMasquerade:    boolPtr(false),
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
	validBaseConfig := func() *CiliumConfig {
		cfg := &CiliumConfig{}
		SetDefaults_CiliumConfig(cfg)
		return cfg
	}

	tests := []struct {
		name        string
		setup       func() *CiliumConfig
		expectErr   bool
		errContains []string
	}{
		{
			name: "nil input",
			setup: func() *CiliumConfig {
				return nil
			},
			expectErr: false,
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
			name: "HubbleUI true, EnableHubble false (after defaults this is fixed)",
			setup: func() *CiliumConfig {
				cfg := &CiliumConfig{
					HubbleUI:               true,
					EnableHubble:           false, // This will be true after SetDefaults
				}
				SetDefaults_CiliumConfig(cfg) // Apply defaults to simulate real scenario
				return cfg
			},
			expectErr:   false,
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
			name: "all fields valid, BPFMasquerade true",
			setup: func() *CiliumConfig {
				cfg := validBaseConfig()
				cfg.TunnelingMode = "geneve"
				cfg.KubeProxyReplacement = "probe"
				cfg.EnableHubble = true
				cfg.HubbleUI = true
				cfg.EnableBPFMasquerade = boolPtr(true)
				cfg.IdentityAllocationMode = "kvstore"
				return cfg
			},
			expectErr: false,
		},
		{
			name: "all fields valid, BPFMasquerade false",
			setup: func() *CiliumConfig {
				cfg := validBaseConfig()
				cfg.TunnelingMode = "geneve"
				cfg.KubeProxyReplacement = "probe"
				cfg.EnableHubble = true
				cfg.HubbleUI = true
				cfg.EnableBPFMasquerade = boolPtr(false)
				cfg.IdentityAllocationMode = "kvstore"
				return cfg
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputConfig := tt.setup()
			verrs := &validation.ValidationErrors{}
			Validate_CiliumConfig(inputConfig, verrs, "spec.network.cilium")

			if tt.expectErr {
				assert.True(t, verrs.HasErrors(), "Expected validation errors for test: %s, but got none", tt.name)
				if len(tt.errContains) > 0 {
					combinedErrors := verrs.Error()
					for _, errStr := range tt.errContains {
						assert.Contains(t, combinedErrors, errStr, "Error message for test '%s' does not contain expected substring '%s'. Full error: %s", tt.name, errStr, combinedErrors)
					}
				}
			} else {
				assert.False(t, verrs.HasErrors(), "Expected no validation errors for test: %s, but got: %s", tt.name, verrs.Error())
			}
		})
	}
}

// TestValidate_NetworkConfig_Calls_Validate_CiliumConfig_Standalone ensures NetworkConfig validation calls Cilium validation.
func TestValidate_NetworkConfig_Calls_Validate_CiliumConfig_Standalone(t *testing.T) {
	netCfg := &NetworkConfig{
		Plugin: "cilium",
		Cilium: &CiliumConfig{
			TunnelingMode: "invalid-mode",
		},
		KubePodsCIDR:    "10.244.0.0/16",
		KubeServiceCIDR: "10.96.0.0/12",
	}
	k8sConfig := &KubernetesConfig{Version: "v1.25.0"}
	SetDefaults_KubernetesConfig(k8sConfig, "test-cluster")
	SetDefaults_NetworkConfig(netCfg)

	verrs := &validation.ValidationErrors{}
	Validate_NetworkConfig(netCfg, verrs, "spec.network", k8sConfig)

	assert.True(t, verrs.HasErrors(), "Expected errors from CiliumConfig validation")
	assert.Contains(t, verrs.Error(), "cilium.tunnelingMode: invalid mode 'invalid-mode'", "Expected Cilium validation error")
}
