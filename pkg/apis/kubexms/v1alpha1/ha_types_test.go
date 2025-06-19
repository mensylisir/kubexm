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
