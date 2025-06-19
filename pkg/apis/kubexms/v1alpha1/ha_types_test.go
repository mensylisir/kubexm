package v1alpha1

import (
	"net"
	"strings"
	"testing"
)

// Helper for pointer to int
func pintHA(i int) *int { return &i }

func TestSetDefaults_HighAvailabilityConfig(t *testing.T) {
	cfg := &HighAvailabilityConfig{}
	SetDefaults_HighAvailabilityConfig(cfg)

	if cfg.Type != "" { // No default type is set currently
		t.Errorf("Default Type = %s, want empty or specific default", cfg.Type)
	}
	if cfg.ControlPlaneEndpointPort == nil || *cfg.ControlPlaneEndpointPort != 6443 {
		t.Errorf("Default ControlPlaneEndpointPort = %v, want 6443", cfg.ControlPlaneEndpointPort)
	}

	t.Run("default with keepalived type", func(t *testing.T) {
		cfgKeepalived := &HighAvailabilityConfig{Type: "keepalived"}
		SetDefaults_HighAvailabilityConfig(cfgKeepalived)
		if cfgKeepalived.Keepalived == nil {
			t.Fatal("Keepalived config should be initialized when HA Type is 'keepalived'")
		}
		if cfgKeepalived.Keepalived.AuthType == nil || *cfgKeepalived.Keepalived.AuthType != "PASS" {
			t.Errorf("Keepalived.AuthType default failed, got %v", cfgKeepalived.Keepalived.AuthType)
		}
	})

	t.Run("default with haproxy type", func(t *testing.T) {
		cfgHAProxy := &HighAvailabilityConfig{Type: "haproxy"}
		SetDefaults_HighAvailabilityConfig(cfgHAProxy)
		if cfgHAProxy.HAProxy == nil {
			t.Fatal("HAProxy config should be initialized when HA Type is 'haproxy'")
		}
		if cfgHAProxy.HAProxy.Mode == nil || *cfgHAProxy.HAProxy.Mode != "tcp" {
			t.Errorf("HAProxy.Mode default failed, got %v", cfgHAProxy.HAProxy.Mode)
		}
	})

	t.Run("default with nginx_lb type", func(t *testing.T) {
		cfgNginxLB := &HighAvailabilityConfig{Type: "nginx_lb"}
		SetDefaults_HighAvailabilityConfig(cfgNginxLB)
		if cfgNginxLB.NginxLB == nil {
			t.Fatal("NginxLB config should be initialized when HA Type is 'nginx_lb'")
		}
		if cfgNginxLB.NginxLB.Mode == nil || *cfgNginxLB.NginxLB.Mode != "tcp" {
			t.Errorf("NginxLB.Mode default failed, got %v", cfgNginxLB.NginxLB.Mode)
		}
	})

	t.Run("default with combined type keepalived+haproxy", func(t *testing.T) {
		cfgCombined := &HighAvailabilityConfig{Type: "keepalived+haproxy"}
		SetDefaults_HighAvailabilityConfig(cfgCombined)
		if cfgCombined.Keepalived == nil {
			t.Error("Keepalived config should be initialized for 'keepalived+haproxy'")
		}
		if cfgCombined.HAProxy == nil {
			t.Error("HAProxy config should be initialized for 'keepalived+haproxy'")
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
			name:       "valid keepalived with VIP and endpoint IP",
			cfg:        &HighAvailabilityConfig{Type: "keepalived", VIP: "192.168.1.100", ControlPlaneEndpointAddress: "192.168.1.100"},
			expectErr:  false,
		},
		{
			name:       "valid external",
			cfg:        &HighAvailabilityConfig{Type: "external", ControlPlaneEndpointDomain: "lb.example.com"},
			expectErr:  false,
		},
		{
			name:       "valid none type",
			cfg:        &HighAvailabilityConfig{Type: "none"},
			expectErr:  false,
		},
		{
			name:       "valid empty type (defaults to unspecified/none)",
			cfg:        &HighAvailabilityConfig{},
			expectErr:  false,
		},
		{
			name:       "invalid type",
			cfg:        &HighAvailabilityConfig{Type: "unknownha"},
			wantErrMsg: ".type: invalid HA type 'unknownha'",
			expectErr:  true,
		},
		{
			name:       "keepalived missing VIP",
			cfg:        &HighAvailabilityConfig{Type: "keepalived", VIP: ""},
			wantErrMsg: ".vip: must be set if HA type is 'keepalived'",
			expectErr:  true,
		},
		{
			name:       "invalid VIP format",
			cfg:        &HighAvailabilityConfig{Type: "keepalived", VIP: "invalid-ip"},
			wantErrMsg: ".vip: invalid IP address format 'invalid-ip'",
			expectErr:  true,
		},
		{
			name:       "invalid ControlPlaneEndpointAddress format",
			cfg:        &HighAvailabilityConfig{ControlPlaneEndpointAddress: "invalid-ip-too"},
			wantErrMsg: ".controlPlaneEndpointAddress: invalid IP address format 'invalid-ip-too'",
			expectErr:  true,
		},
		{
			name:       "invalid ControlPlaneEndpointPort low",
			cfg:        &HighAvailabilityConfig{ControlPlaneEndpointPort: pintHA(0)},
			wantErrMsg: ".controlPlaneEndpointPort: invalid port 0",
			expectErr:  true,
		},
		{
			name:       "invalid ControlPlaneEndpointPort high",
			cfg:        &HighAvailabilityConfig{ControlPlaneEndpointPort: pintHA(70000)},
			wantErrMsg: ".controlPlaneEndpointPort: invalid port 70000",
			expectErr:  true,
		},
		{
			name:       "type_keepalived_missing_keepalived_config",
			cfg:        &HighAvailabilityConfig{Type: "keepalived", VIP: "1.1.1.1", Keepalived: nil /* missing */},
			wantErrMsg: ".keepalived: configuration section must be present if type includes 'keepalived'",
			expectErr:  true,
		},
		{
			name:       "keepalived_config_present_type_mismatch",
			cfg:        &HighAvailabilityConfig{Type: "external", Keepalived: &KeepalivedConfig{VRID: pintHA(1)}},
			wantErrMsg: ".type: must include 'keepalived' if keepalived configuration section is present",
			expectErr:  true,
		},
		{
			name:       "type_haproxy_missing_haproxy_config",
			cfg:        &HighAvailabilityConfig{Type: "haproxy", HAProxy: nil /* missing */}, // Removed VIP as it's not strictly for haproxy type
			wantErrMsg: ".haproxy: configuration section must be present if type includes 'haproxy'",
			expectErr:  true,
		},
		{
			name:       "haproxy_config_present_type_mismatch",
			cfg:        &HighAvailabilityConfig{Type: "none", HAProxy: &HAProxyConfig{FrontendPort: pintHA(80)}},
			wantErrMsg: ".type: must include 'haproxy' if haproxy configuration section is present",
			expectErr:  true,
		},
		{
			name:       "type_nginx_lb_missing_nginxLB_config",
			cfg:        &HighAvailabilityConfig{Type: "nginx_lb", NginxLB: nil /* missing */},
			wantErrMsg: ".nginxLB: configuration section must be present if type includes 'nginx_lb'",
			expectErr:  true,
		},
		{
			name:       "nginxLB_config_present_type_mismatch",
			cfg:        &HighAvailabilityConfig{Type: "keepalived", NginxLB: &NginxLBConfig{ListenPort: pintHA(80)}},
			wantErrMsg: ".type: must include 'nginx_lb' if nginxLB configuration section is present",
			expectErr:  true,
		},
		{
			name:       "external_lb_with_keepalived_config",
			cfg:        &HighAvailabilityConfig{Type: "external_lb", ControlPlaneEndpointDomain:"d", Keepalived: &KeepalivedConfig{VRID: pintHA(1)}},
			wantErrMsg: ".keepalived: should not be set for external_lb type",
			expectErr:  true,
		},
		{
			name:       "external_lb_missing_endpoint_details",
			cfg:        &HighAvailabilityConfig{Type: "external_lb", ControlPlaneEndpointAddress: "", ControlPlaneEndpointDomain: ""},
			wantErrMsg: "either controlPlaneEndpointAddress or controlPlaneEndpointDomain must be set for external_lb type",
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
