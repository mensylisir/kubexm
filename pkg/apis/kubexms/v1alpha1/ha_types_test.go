package v1alpha1

import (
	"strings"
	"testing"
	// "net" // Removed as unused
	"github.com/stretchr/testify/assert" // Added import for testify/assert
)

func TestSetDefaults_HighAvailabilityConfig(t *testing.T) {
	cfg := &HighAvailabilityConfig{}
	SetDefaults_HighAvailabilityConfig(cfg)

	// cfg.Type is removed. HA type is now in cfg.External.Type or cfg.Internal.Type
	// if cfg.Type != "" {
	// 	t.Errorf("Default Type = %s, want empty or specific default", cfg.Type)
	// }

	// ControlPlaneEndpoint is no longer part of HighAvailabilityConfig, so these checks are removed.
	// if cfg.ControlPlaneEndpoint == nil {
	// 	t.Fatal("ControlPlaneEndpoint should be initialized by defaults")
	// }
	// if cfg.ControlPlaneEndpoint.Port == nil || *cfg.ControlPlaneEndpoint.Port != 6443 {
	// 	t.Errorf("Default ControlPlaneEndpoint.Port = %v, want 6443", cfg.ControlPlaneEndpoint.Port)
	// }

	t.Run("default with external ManagedKeepalivedHAProxy type", func(t *testing.T) {
		haEnabled := true
		cfgExt := &HighAvailabilityConfig{
			Enabled: &haEnabled,
			External: &ExternalLoadBalancerConfig{
				Type: "ManagedKeepalivedHAProxy",
				// Enabled field is removed
			},
		}
		SetDefaults_HighAvailabilityConfig(cfgExt)
		assert.NotNil(t, cfgExt.External, "External config should be initialized")
		assert.NotNil(t, cfgExt.External.Keepalived, "External.Keepalived config should be initialized")
		assert.NotNil(t, cfgExt.External.Keepalived.AuthType, "External.Keepalived.AuthType should have a default")
		assert.Equal(t, "PASS", *cfgExt.External.Keepalived.AuthType, "External.Keepalived.AuthType default mismatch")
		assert.NotNil(t, cfgExt.External.HAProxy, "External.HAProxy config should be initialized")
		assert.NotNil(t, cfgExt.External.HAProxy.Mode, "External.HAProxy.Mode should have a default")
		assert.Equal(t, "tcp", *cfgExt.External.HAProxy.Mode, "External.HAProxy.Mode default mismatch")
	})

	t.Run("default with external ManagedKeepalivedNginxLB type", func(t *testing.T) {
		haEnabled := true
		cfgExt := &HighAvailabilityConfig{
			Enabled: &haEnabled,
			External: &ExternalLoadBalancerConfig{
				Type: "ManagedKeepalivedNginxLB",
				// Enabled field is removed
			},
		}
		SetDefaults_HighAvailabilityConfig(cfgExt)
		assert.NotNil(t, cfgExt.External, "External config should be initialized")
		assert.NotNil(t, cfgExt.External.Keepalived, "External.Keepalived config should be initialized")
		assert.NotNil(t, cfgExt.External.NginxLB, "External.NginxLB config should be initialized")
		assert.NotNil(t, cfgExt.External.NginxLB.Mode, "External.NginxLB.Mode should have a default")
		assert.Equal(t, "tcp", *cfgExt.External.NginxLB.Mode, "External.NginxLB.Mode default mismatch")
	})

	t.Run("default with internal KubeVIP type", func(t *testing.T) {
		haEnabled := true
		cfgInt := &HighAvailabilityConfig{
			Enabled: &haEnabled,
			Internal: &InternalLoadBalancerConfig{
				Type: "KubeVIP",
				// Enabled field is removed. Activation is based on HAConfig.Enabled and Internal block presence.
			},
		}
		SetDefaults_HighAvailabilityConfig(cfgInt)
		assert.NotNil(t, cfgInt.Internal, "Internal config should be initialized")
		assert.NotNil(t, cfgInt.Internal.KubeVIP, "Internal.KubeVIP config should be initialized for Type KubeVIP")
		// Check a KubeVIP default, e.g., Mode if it's defaulted in SetDefaults_KubeVIPConfig
		assert.NotNil(t, cfgInt.Internal.KubeVIP.Mode, "KubeVIP.Mode should have a default")
		assert.Equal(t, KubeVIPModeARP, *cfgInt.Internal.KubeVIP.Mode, "KubeVIP.Mode default mismatch")
	})
}

func TestValidate_HighAvailabilityConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *HighAvailabilityConfig
		wantErrMsg string
		expectErr  bool
	}{
		{
		name: "valid external ManagedKeepalivedHAProxy with endpoint IP (VIP removed, CPE moved)",
		cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
			External: &ExternalLoadBalancerConfig{
				Type: "ManagedKeepalivedHAProxy",
				// Enabled field removed
				Keepalived: &KeepalivedConfig{
					VRID:      intPtr(1),
					Priority:  intPtr(101), // Typical master priority
					Interface: stringPtr("eth0"),
					// AuthType defaults to PASS
					AuthPass:  stringPtr("secret"),
				},
				HAProxy: &HAProxyConfig{
					FrontendPort:   intPtr(6443), // Explicitly set, though defaults
					BackendServers: []HAProxyBackendServer{{Name: "cp1", Address: "192.168.0.10", Port: 6443}},
				},
			}},
		expectErr: false,
		},
		{
		name: "valid external UserProvided (CPE validation is at Cluster level)",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				External: &ExternalLoadBalancerConfig{Type: "UserProvided"}},
			expectErr: false,
		},
		{
			name:      "valid HA disabled",
			cfg:       &HighAvailabilityConfig{Enabled: boolPtr(false)},
			expectErr: false,
		},
		{
			name:      "valid empty config (HA disabled by default)",
			cfg:       &HighAvailabilityConfig{},
			expectErr: false,
		},
		{
			name: "invalid external LB type",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				External: &ExternalLoadBalancerConfig{Type: "unknownExternalLB"}},
			wantErrMsg: "spec.highAvailability.external.type: unknown external LB type 'unknownExternalLB'", // Exact full message
			expectErr:  true,
		},
		// { // This test case is invalid because defaults will initialize Keepalived if nil.
		//	name: "ManagedKeepalived without Keepalived section",
		//	cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
		//		External: &ExternalLoadBalancerConfig{
		//			Type:       "ManagedKeepalivedHAProxy",
		//			Enabled:    boolPtr(true),
		//			Keepalived: nil, // This is being tested
		//			HAProxy: &HAProxyConfig{ // Make HAProxy part valid
		//				FrontendPort:   intPtr(6443),
		//				BackendServers: []HAProxyBackendServer{{Name: "cp1", Address: "192.168.0.10", Port: 6443}},
		//			},
		//		}},
		//	wantErrMsg: "spec.highAvailability.external.keepalived: section must be present if type includes 'Keepalived'", // Exact error
		//	expectErr:  true,
		// },
		// { // VIP validation removed as VIP field is removed
		// 	name: "invalid VIP format",
		// 	cfg: &HighAvailabilityConfig{Enabled: boolPtr(true), VIP: "invalid-ip",
		// 		External: &ExternalLoadBalancerConfig{Type: "ManagedKeepalivedHAProxy", Enabled: boolPtr(true), Keepalived: &KeepalivedConfig{}}},
		// 	wantErrMsg: ".vip: invalid IP address format 'invalid-ip'",
		// 	expectErr:  true,
		// },
		// ControlPlaneEndpoint validation is now done at the ClusterSpec level, not within HAConfig validation directly.
		// {
		// 	name: "invalid ControlPlaneEndpoint.Address format",
		// 	cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
		// 		/* ControlPlaneEndpoint moved */},
		// 	wantErrMsg: ".controlPlaneEndpoint.address: invalid IP address format 'invalid-ip-too'",
		// 	expectErr:  true,
		// },
		// {
		// 	name: "invalid ControlPlaneEndpoint.Port low",
		// 	cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
		// 		/* ControlPlaneEndpoint moved */},
		// 	wantErrMsg: ".controlPlaneEndpoint.port: invalid port 0",
		// 	expectErr:  true,
		// },
		// {
		// 	name: "invalid ControlPlaneEndpoint.Port high",
		// 	cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
		// 		/* ControlPlaneEndpoint moved */},
		// 	wantErrMsg: ".controlPlaneEndpoint.port: invalid port 70000",
		// 	expectErr:  true,
		// },
		{
			name: "keepalived_config_present_external_type_mismatch",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				External: &ExternalLoadBalancerConfig{Type: "UserProvided", Keepalived: &KeepalivedConfig{VRID: intPtr(1)}}},
			wantErrMsg: ".external.keepalived: should not be set for UserProvided external LB type",
			expectErr:  true,
		},
		// { // This validation now depends on ClusterSpec.ControlPlaneEndpoint, tested at higher level.
		// 	name: "UserProvided external LB missing endpoint details",
		// 	cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
		// 		External: &ExternalLoadBalancerConfig{Type: "UserProvided", Enabled: boolPtr(true)}},
		// 	wantErrMsg: "if type is UserProvided, a corresponding ControlPlaneEndpoint", // Message might change
		// 	expectErr:  true,
		// },
		// Add more tests for Internal Load Balancer types and their validations
		{
			name: "valid internal KubeVIP",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				Internal: &InternalLoadBalancerConfig{Type: "KubeVIP", KubeVIP: &KubeVIPConfig{ // Enabled field removed
					VIP:       stringPtr("192.168.1.100"),
					Interface: stringPtr("eth0"),
				}}},
			expectErr: false,
		},
		{
			name: "invalid internal LB type",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				Internal: &InternalLoadBalancerConfig{Type: "unknownInternalLB"}}, // Enabled field removed
			wantErrMsg: "spec.highAvailability.internal.type: unknown internal LB type 'unknownInternalLB'", // Exact full message
			expectErr:  true,
		},
		{
			name: "KubeVIP internal LB missing KubeVIP section",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				Internal: &InternalLoadBalancerConfig{Type: "KubeVIP", KubeVIP: nil}}, // Enabled field removed, KubeVIP will be defaulted to {}
			wantErrMsg: ".internal.kubevip.vip: virtual IP address must be specified", // Error comes from Validate_KubeVIPConfig
			expectErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_HighAvailabilityConfig(tt.cfg) // Apply defaults first
			verrs := &ValidationErrors{}
			Validate_HighAvailabilityConfig(tt.cfg, verrs, "spec.highAvailability")

			if tt.expectErr {
				if verrs.IsEmpty() {
					t.Fatalf("Validate_HighAvailabilityConfig expected error for %s, got none", tt.name)
				}
				// Use exact match for specific known single error messages, otherwise substring
				if verrs.IsEmpty() {
					t.Fatalf("Validate_HighAvailabilityConfig expected error for %s, got none", tt.name)
				}

				found := false
			// Exact match for specific single-error cases
			if (tt.name == "invalid_external_LB_type" ||
				 tt.name == "invalid_internal_LB_type" ||
				 tt.name == "ManagedKeepalived_without_Keepalived_section") && len(verrs.Errors) == 1 {
					if verrs.Errors[0] == tt.wantErrMsg {
						found = true
					}
				} else {
					// Fallback to strings.Contains for other error messages or multiple errors
					if strings.Contains(verrs.Error(), tt.wantErrMsg) {
						found = true
					}
				}

				if !found {
					t.Errorf("Validate_HighAvailabilityConfig error for %s. Expected to find '%s', got errors: %v", tt.name, tt.wantErrMsg, verrs.Errors)
				}
			} else {
				if !verrs.IsEmpty() {
					t.Errorf("Validate_HighAvailabilityConfig for valid case %s failed: %v", tt.name, verrs)
				}
			}
		})
	}
}

// isValidIP is already defined in ha_types.go, no need to redefine here
// func isValidIP(ip string) bool { return net.ParseIP(ip) != nil }
