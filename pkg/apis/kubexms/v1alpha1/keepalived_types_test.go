package v1alpha1

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Local helpers removed, using global ones from zz_helpers.go (e.g. intPtr, stringPtr, boolPtr)

func TestSetDefaults_KeepalivedConfig(t *testing.T) {
	cfg := &KeepalivedConfig{}
	SetDefaults_KeepalivedConfig(cfg)

	assert.NotNil(t, cfg.AuthType, "AuthType should be defaulted")
	assert.Equal(t, "PASS", *cfg.AuthType, "Default AuthType failed")

	assert.NotNil(t, cfg.SkipInstall, "SkipInstall should be defaulted")
	assert.False(t, *cfg.SkipInstall, "Default SkipInstall failed")

	assert.NotNil(t, cfg.ExtraConfig, "ExtraConfig should be initialized")
	assert.Len(t, cfg.ExtraConfig, 0, "ExtraConfig should be empty by default")
}

func TestValidate_KeepalivedConfig(t *testing.T) {
	validCfg := KeepalivedConfig{
		VRID:      intPtr(51),
		Priority:  intPtr(101),
		Interface: stringPtr("eth0"),
		AuthType:  stringPtr("PASS"), // Will be defaulted if nil, but explicit for clarity
		AuthPass:  stringPtr("secret"),
	}
	// Apply defaults to fill in SkipInstall, etc.
	SetDefaults_KeepalivedConfig(&validCfg)

	verrs := &ValidationErrors{}
	Validate_KeepalivedConfig(&validCfg, verrs, "keepalived")
	assert.True(t, verrs.IsEmpty(), "Validation failed for valid config: %v", verrs.Error())

	// Test SkipInstall
	skipInstallCfg := KeepalivedConfig{SkipInstall: boolPtr(true)}
	SetDefaults_KeepalivedConfig(&skipInstallCfg) // Ensure other defaults like AuthType are set
	verrsSkip := &ValidationErrors{}
	Validate_KeepalivedConfig(&skipInstallCfg, verrsSkip, "keepalived")
	assert.True(t, verrsSkip.IsEmpty(), "Validation should pass (mostly skipped) if SkipInstall is true: %v", verrsSkip.Error())


	tests := []struct {
		name       string
		cfg        KeepalivedConfig
		wantErrMsg string
	}{
		{"nil_vrid", KeepalivedConfig{Priority: intPtr(100), Interface: stringPtr("eth0")}, ".vrid: virtual router ID must be specified"},
		{"bad_vrid_low", KeepalivedConfig{VRID: intPtr(0)}, ".vrid: must be between 1 and 255"},    // Changed from -1 and 0-255
		{"bad_vrid_high", KeepalivedConfig{VRID: intPtr(256)}, ".vrid: must be between 1 and 255"}, // Changed error message range
		{"nil_priority", KeepalivedConfig{VRID: intPtr(50), Interface: stringPtr("eth0")}, ".priority: must be specified"},
		{"bad_priority_low", KeepalivedConfig{Priority: intPtr(0)}, ".priority: must be between 1 and 254"},
		{"bad_priority_high", KeepalivedConfig{Priority: intPtr(255)}, ".priority: must be between 1 and 254"},
		{"nil_interface", KeepalivedConfig{VRID: intPtr(50), Priority: intPtr(100)}, ".interface: network interface must be specified"},
		{"empty_interface", KeepalivedConfig{Interface: stringPtr(" ")}, ".interface: network interface must be specified"},
		{"invalid_auth_type", KeepalivedConfig{AuthType: stringPtr("NONE")}, "invalid value 'NONE'"}, // AuthType will be defaulted to PASS if nil, so this must be explicitly wrong
		{"pass_auth_no_pass", KeepalivedConfig{AuthType: stringPtr("PASS"), AuthPass: stringPtr(" ")}, ".authPass: must be specified if authType is 'PASS'"},
		{"pass_auth_long_pass", KeepalivedConfig{AuthType: stringPtr("PASS"), AuthPass: stringPtr("longpassword")}, ".authPass: password too long"},
		{"ah_auth_with_pass", KeepalivedConfig{AuthType: stringPtr("AH"), AuthPass: stringPtr("secret")}, ".authPass: should not be specified if authType is 'AH'"},
		{"empty_extra_config_line", KeepalivedConfig{ExtraConfig: []string{" "}}, ".extraConfig[0]: extra config line cannot be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure mandatory fields for other checks are set to valid values to isolate the test
			if tt.cfg.VRID == nil && !strings.Contains(tt.name, "vrid") { tt.cfg.VRID = intPtr(1) }
			if tt.cfg.Priority == nil && !strings.Contains(tt.name, "priority") { tt.cfg.Priority = intPtr(100) }
			if tt.cfg.Interface == nil && !strings.Contains(tt.name, "interface") { tt.cfg.Interface = stringPtr("net1") }
			// AuthType is defaulted so it will always be present. If testing AuthType, it should be explicitly set in `cfg`.
			// If AuthType is "PASS" (default or explicit), AuthPass is needed.
			if tt.cfg.AuthType == nil || *tt.cfg.AuthType == "PASS" { // Handle default case for AuthPass
				if tt.cfg.AuthPass == nil && !strings.Contains(tt.name, "auth_no_pass") && !strings.Contains(tt.name, "auth_long_pass") {
					tt.cfg.AuthPass = stringPtr("default")
				}
			}


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
