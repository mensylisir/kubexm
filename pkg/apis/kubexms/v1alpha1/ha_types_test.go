package v1alpha1

import (
	"strings"
	"testing"
	// "net" // Removed as unused
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
		enabled := true
		cfgExt := &HighAvailabilityConfig{
			Enabled: &enabled, // Enable HA for sub-configs to be defaulted
			External: &ExternalLoadBalancerConfig{
				Type: "ManagedKeepalivedHAProxy",
			},
		}
		SetDefaults_HighAvailabilityConfig(cfgExt)
		if cfgExt.External == nil {
			t.Fatal("External config should be initialized")
		}
		if cfgExt.External.Keepalived == nil {
			t.Fatal("External.Keepalived config should be initialized for Type ManagedKeepalivedHAProxy")
		}
		if cfgExt.External.Keepalived.AuthType == nil || *cfgExt.External.Keepalived.AuthType != "PASS" {
			t.Errorf("External.Keepalived.AuthType default failed, got %v", cfgExt.External.Keepalived.AuthType)
		}
		if cfgExt.External.HAProxy == nil {
			t.Fatal("External.HAProxy config should be initialized for Type ManagedKeepalivedHAProxy")
		}
		if cfgExt.External.HAProxy.Mode == nil || *cfgExt.External.HAProxy.Mode != "tcp" {
			t.Errorf("External.HAProxy.Mode default failed, got %v", cfgExt.External.HAProxy.Mode)
		}
	})

	t.Run("default with external ManagedKeepalivedNginxLB type", func(t *testing.T) {
		enabled := true
		cfgExt := &HighAvailabilityConfig{
			Enabled: &enabled,
			External: &ExternalLoadBalancerConfig{
				Type: "ManagedKeepalivedNginxLB",
			},
		}
		SetDefaults_HighAvailabilityConfig(cfgExt)
		if cfgExt.External == nil {
			t.Fatal("External config should be initialized")
		}
		if cfgExt.External.Keepalived == nil {
			t.Fatal("External.Keepalived config should be initialized for Type ManagedKeepalivedNginxLB")
		}
		if cfgExt.External.NginxLB == nil {
			t.Fatal("External.NginxLB config should be initialized for Type ManagedKeepalivedNginxLB")
		}
		if cfgExt.External.NginxLB.Mode == nil || *cfgExt.External.NginxLB.Mode != "tcp" {
			t.Errorf("External.NginxLB.Mode default failed, got %v", cfgExt.External.NginxLB.Mode)
		}
	})

	t.Run("default with internal KubeVIP type", func(t *testing.T) {
		enabled := true
		cfgInt := &HighAvailabilityConfig{
			Enabled: &enabled,
			Internal: &InternalLoadBalancerConfig{
				Type: "KubeVIP",
				Enabled: boolPtr(true), // Explicitly enable internal LB for this test case
			},
		}
		SetDefaults_HighAvailabilityConfig(cfgInt)
		if cfgInt.Internal == nil {
			t.Fatal("Internal config should be initialized")
		}
		if cfgInt.Internal.KubeVIP == nil {
			t.Fatal("Internal.KubeVIP config should be initialized for Type KubeVIP")
		}
		// Add assertions for KubeVIP defaults if any, e.g.
		// if cfgInt.Internal.KubeVIP.Image == "" { t.Error("KubeVIP image default failed") }
	})

	t.Run("external LB enabled default with any type", func(t *testing.T) {
		haEnabled := true
		cfg := &HighAvailabilityConfig{
			Enabled: &haEnabled,
			External: &ExternalLoadBalancerConfig{
				Type: "SomeCustomType", // Not "Managed*" or "UserProvided"
				// Enabled is nil
			},
		}
		SetDefaults_HighAvailabilityConfig(cfg)
		if cfg.External == nil || cfg.External.Enabled == nil || !*cfg.External.Enabled {
			t.Errorf("External.Enabled should default to true if External.Type is set, got %v", cfg.External.Enabled)
		}
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
				Type:    "ManagedKeepalivedHAProxy",
				Enabled: boolPtr(true),
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
				External: &ExternalLoadBalancerConfig{Type: "UserProvided", Enabled: boolPtr(true)}},
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
				External: &ExternalLoadBalancerConfig{Type: "unknownExternalLB", Enabled: boolPtr(true)}},
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
				External: &ExternalLoadBalancerConfig{Type: "UserProvided", Enabled: boolPtr(true), Keepalived: &KeepalivedConfig{VRID: intPtr(1)}}},
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
				Internal: &InternalLoadBalancerConfig{Type: "KubeVIP", Enabled: boolPtr(true), KubeVIP: &KubeVIPConfig{
					VIP:       stringPtr("192.168.1.100"),
					Interface: stringPtr("eth0"),
				}}},
			expectErr: false,
		},
		{
			name: "invalid internal LB type",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				Internal: &InternalLoadBalancerConfig{Type: "unknownInternalLB", Enabled: boolPtr(true)}},
			wantErrMsg: "spec.highAvailability.internal.type: unknown internal LB type 'unknownInternalLB'", // Exact full message
			expectErr:  true,
		},
		{
			name: "KubeVIP internal LB missing KubeVIP section",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				Internal: &InternalLoadBalancerConfig{Type: "KubeVIP", Enabled: boolPtr(true), KubeVIP: nil}}, // KubeVIP will be defaulted to {}
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
