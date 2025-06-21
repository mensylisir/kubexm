package v1alpha1

import (
	"strings"
	"testing"
	// "net" // Removed as unused
)

// Helper for pointer to int
func pintHA(i int) *int { return &i }

// Helper for pointer to bool
func boolPtr(b bool) *bool { return &b }

func TestSetDefaults_HighAvailabilityConfig(t *testing.T) {
	cfg := &HighAvailabilityConfig{}
	SetDefaults_HighAvailabilityConfig(cfg)

	// cfg.Type is removed. HA type is now in cfg.External.Type or cfg.Internal.Type
	// if cfg.Type != "" {
	// 	t.Errorf("Default Type = %s, want empty or specific default", cfg.Type)
	// }

	// Check ControlPlaneEndpoint defaults
	if cfg.ControlPlaneEndpoint == nil {
		t.Fatal("ControlPlaneEndpoint should be initialized by defaults")
	}
	if cfg.ControlPlaneEndpoint.Port == nil || *cfg.ControlPlaneEndpoint.Port != 6443 {
		t.Errorf("Default ControlPlaneEndpoint.Port = %v, want 6443", cfg.ControlPlaneEndpoint.Port)
	}

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

}

func TestValidate_HighAvailabilityConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *HighAvailabilityConfig
		wantErrMsg string
		expectErr  bool
	}{
		{
			name: "valid external ManagedKeepalivedHAProxy with VIP and endpoint IP",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true), VIP: "192.168.1.100",
				ControlPlaneEndpoint: &ControlPlaneEndpointConfig{Address: "192.168.1.100"},
				External: &ExternalLoadBalancerConfig{Type: "ManagedKeepalivedHAProxy", Enabled: boolPtr(true), Keepalived: &KeepalivedConfig{}, HAProxy: &HAProxyConfig{}}},
			expectErr: false,
		},
		{
			name: "valid external UserProvided with endpoint domain",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				ControlPlaneEndpoint: &ControlPlaneEndpointConfig{Domain: "lb.example.com"},
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
			wantErrMsg: ".external.type: invalid external LB type 'unknownExternalLB'",
			expectErr:  true,
		},
		{
			name: "ManagedKeepalived without Keepalived section",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				External: &ExternalLoadBalancerConfig{Type: "ManagedKeepalivedHAProxy", Enabled: boolPtr(true), Keepalived: nil}},
			wantErrMsg: ".external.keepalived: section must be present if type includes 'Keepalived'",
			expectErr:  true,
		},
		{
			name: "invalid VIP format",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true), VIP: "invalid-ip",
				External: &ExternalLoadBalancerConfig{Type: "ManagedKeepalivedHAProxy", Enabled: boolPtr(true), Keepalived: &KeepalivedConfig{}}},
			wantErrMsg: ".vip: invalid IP address format 'invalid-ip'",
			expectErr:  true,
		},
		{
			name: "invalid ControlPlaneEndpoint.Address format",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				ControlPlaneEndpoint: &ControlPlaneEndpointConfig{Address: "invalid-ip-too"}},
			wantErrMsg: ".controlPlaneEndpoint.address: invalid IP address format 'invalid-ip-too'",
			expectErr:  true,
		},
		{
			name: "invalid ControlPlaneEndpoint.Port low",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				ControlPlaneEndpoint: &ControlPlaneEndpointConfig{Port: pintHA(0)}},
			wantErrMsg: ".controlPlaneEndpoint.port: invalid port 0",
			expectErr:  true,
		},
		{
			name: "invalid ControlPlaneEndpoint.Port high",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				ControlPlaneEndpoint: &ControlPlaneEndpointConfig{Port: pintHA(70000)}},
			wantErrMsg: ".controlPlaneEndpoint.port: invalid port 70000",
			expectErr:  true,
		},
		{
			name: "keepalived_config_present_external_type_mismatch",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				External: &ExternalLoadBalancerConfig{Type: "UserProvided", Enabled: boolPtr(true), Keepalived: &KeepalivedConfig{VRID: pintHA(1)}}},
			// This validation is in Validate_ExternalLoadBalancerConfig, path will be .external.type or .external.keepalived
			wantErrMsg: ".external.keepalived: should not be set for UserProvided external LB type", // Or type mismatch
			expectErr:  true,
		},
		{
			name: "UserProvided external LB missing endpoint details",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				ControlPlaneEndpoint: &ControlPlaneEndpointConfig{Address: "", Domain: ""}, // Explicitly empty
				External: &ExternalLoadBalancerConfig{Type: "UserProvided", Enabled: boolPtr(true)}},
			wantErrMsg: ".external: if type is UserProvided, parent controlPlaneEndpoint address or domain must be set",
			expectErr:  true,
		},
		// Add more tests for Internal Load Balancer types and their validations
		{
			name: "valid internal KubeVIP",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				Internal: &InternalLoadBalancerConfig{Type: "KubeVIP", Enabled: boolPtr(true), KubeVIP: &KubeVIPConfig{}}},
			expectErr: false,
		},
		{
			name: "invalid internal LB type",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				Internal: &InternalLoadBalancerConfig{Type: "unknownInternalLB", Enabled: boolPtr(true)}},
			wantErrMsg: ".internal.type: invalid internal LB type 'unknownInternalLB'",
			expectErr:  true,
		},
		{
			name: "KubeVIP internal LB missing KubeVIP section",
			cfg: &HighAvailabilityConfig{Enabled: boolPtr(true),
				Internal: &InternalLoadBalancerConfig{Type: "KubeVIP", Enabled: boolPtr(true), KubeVIP: nil}},
			wantErrMsg: ".internal.kubevip: section must be present if type is 'KubeVIP'",
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
				if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
					t.Errorf("Validate_HighAvailabilityConfig error for %s = %v, want to contain %q", tt.name, verrs, tt.wantErrMsg)
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
