package v1alpha1

import (
	"strings"
	"testing"
)

// Helper for Keepalived tests
func pintKeepalived(i int) *int { v := i; return &v }
func pstrKeepalived(s string) *string { v := s; return &v }
func pboolKeepalived(b bool) *bool { v := b; return &v }

func TestSetDefaults_KeepalivedConfig(t *testing.T) {
	cfg := &KeepalivedConfig{}
	SetDefaults_KeepalivedConfig(cfg)
	if cfg.AuthType == nil || *cfg.AuthType != "PASS" {
		t.Errorf("Default AuthType failed, got %v", cfg.AuthType)
	}
	if cfg.SkipInstall == nil || *cfg.SkipInstall != false {
		t.Errorf("Default SkipInstall failed, got %v", cfg.SkipInstall)
	}
	if cfg.ExtraConfig == nil || cap(cfg.ExtraConfig) != 0 {
		t.Errorf("Default ExtraConfig failed, got %v", cfg.ExtraConfig)
	}
}

func TestValidate_KeepalivedConfig(t *testing.T) {
	validCfg := KeepalivedConfig{
		VRID:      pintKeepalived(51),
		Priority:  pintKeepalived(101),
		Interface: pstrKeepalived("eth0"),
		AuthType:  pstrKeepalived("PASS"),
		AuthPass:  pstrKeepalived("secret"),
	}
	// Apply defaults to fill in SkipInstall, etc.
	SetDefaults_KeepalivedConfig(&validCfg)

	verrs := &ValidationErrors{}
	Validate_KeepalivedConfig(&validCfg, verrs, "keepalived")
	if !verrs.IsEmpty() {
		t.Errorf("Validation failed for valid config: %v", verrs)
	}

	// Test SkipInstall
	skipInstallCfg := KeepalivedConfig{SkipInstall: pboolKeepalived(true)}
	SetDefaults_KeepalivedConfig(&skipInstallCfg)
	verrsSkip := &ValidationErrors{}
	Validate_KeepalivedConfig(&skipInstallCfg, verrsSkip, "keepalived")
	if !verrsSkip.IsEmpty() {
		t.Errorf("Validation should pass (mostly skipped) if SkipInstall is true: %v", verrsSkip)
	}


	tests := []struct {
		name       string
		cfg        KeepalivedConfig
		wantErrMsg string
	}{
		{"nil_vrid", KeepalivedConfig{Priority: pintKeepalived(100), Interface: pstrKeepalived("eth0")}, ".vrid: virtual router ID must be specified"},
		{"bad_vrid_low", KeepalivedConfig{VRID: pintKeepalived(-1)}, ".vrid: must be between 0 and 255"},
		{"bad_vrid_high", KeepalivedConfig{VRID: pintKeepalived(256)}, ".vrid: must be between 0 and 255"},
		{"nil_priority", KeepalivedConfig{VRID: pintKeepalived(50), Interface: pstrKeepalived("eth0")}, ".priority: must be specified"},
		{"bad_priority_low", KeepalivedConfig{Priority: pintKeepalived(0)}, ".priority: must be between 1 and 254"},
		{"bad_priority_high", KeepalivedConfig{Priority: pintKeepalived(255)}, ".priority: must be between 1 and 254"},
		{"nil_interface", KeepalivedConfig{VRID: pintKeepalived(50), Priority: pintKeepalived(100)}, ".interface: network interface must be specified"},
		{"empty_interface", KeepalivedConfig{Interface: pstrKeepalived(" ")}, ".interface: network interface must be specified"},
		{"invalid_auth_type", KeepalivedConfig{AuthType: pstrKeepalived("NONE")}, ".authType: invalid or missing"},
		{"pass_auth_no_pass", KeepalivedConfig{AuthType: pstrKeepalived("PASS"), AuthPass: pstrKeepalived(" ")}, ".authPass: must be specified if authType is 'PASS'"},
		{"pass_auth_long_pass", KeepalivedConfig{AuthType: pstrKeepalived("PASS"), AuthPass: pstrKeepalived("longpassword")}, ".authPass: password too long"},
		{"ah_auth_with_pass", KeepalivedConfig{AuthType: pstrKeepalived("AH"), AuthPass: pstrKeepalived("secret")}, ".authPass: should not be specified if authType is 'AH'"},
		{"empty_extra_config_line", KeepalivedConfig{ExtraConfig: []string{" "}}, ".extraConfig[0]: extra config line cannot be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure mandatory fields for other checks are set to valid values to isolate the test
			if tt.cfg.VRID == nil && !strings.Contains(tt.name, "vrid") { tt.cfg.VRID = pintKeepalived(1) }
			if tt.cfg.Priority == nil && !strings.Contains(tt.name, "priority") { tt.cfg.Priority = pintKeepalived(100) }
			if tt.cfg.Interface == nil && !strings.Contains(tt.name, "interface") { tt.cfg.Interface = pstrKeepalived("net1") }

			// Apply defaults AFTER potentially setting fields for a specific test case
			// This ensures that default AuthType doesn't mask an AuthType test, for example.
			// However, for some tests (like nil_auth_type), we want to see the default applied first.
			// The current Validate_KeepalivedConfig handles AuthType defaulting correctly.
			SetDefaults_KeepalivedConfig(&tt.cfg)

			verrs := &ValidationErrors{}
			Validate_KeepalivedConfig(&tt.cfg, verrs, "keepalived")
			if verrs.IsEmpty() {
				t.Fatalf("Validate_KeepalivedConfig expected error for %s, got none", tt.name)
			}
			if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
				t.Errorf("Error for %s = %v, want to contain %q", tt.name, verrs, tt.wantErrMsg)
			}
		})
	}
}
