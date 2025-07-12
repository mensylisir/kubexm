package v1alpha1

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/mensylisir/kubexm/pkg/util" // Added import
)

// Local helpers removed, using global ones from zz_helpers.go (e.g. util.IntPtr, util.StrPtr, util.BoolPtr)

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
		VRID:      util.IntPtr(51),
		Priority:  util.IntPtr(101),
		Interface: util.StrPtr("eth0"),
		AuthType:  util.StrPtr("PASS"),
		AuthPass:  util.StrPtr("secret"),
	}
	SetDefaults_KeepalivedConfig(&validCfg)

	verrs := &ValidationErrors{}
	Validate_KeepalivedConfig(&validCfg, verrs, "keepalived")
	assert.False(t, verrs.HasErrors(), "Validation failed for valid config: %v", verrs.Error())

	skipInstallCfg := KeepalivedConfig{SkipInstall: util.BoolPtr(true)}
	SetDefaults_KeepalivedConfig(&skipInstallCfg)
	verrsSkip := &ValidationErrors{}
	Validate_KeepalivedConfig(&skipInstallCfg, verrsSkip, "keepalived")
	assert.False(t, verrsSkip.HasErrors(), "Validation should pass (mostly skipped) if SkipInstall is true: %v", verrsSkip.Error())


	tests := []struct {
		name       string
		cfg        KeepalivedConfig
		wantErrMsg string
	}{
		{"nil_vrid", KeepalivedConfig{Priority: util.IntPtr(100), Interface: util.StrPtr("eth0")}, ".vrid: virtual router ID must be specified"},
		{"bad_vrid_low", KeepalivedConfig{VRID: util.IntPtr(0)}, ".vrid: must be between 1 and 255"},
		{"bad_vrid_high", KeepalivedConfig{VRID: util.IntPtr(256)}, ".vrid: must be between 1 and 255"},
		{"nil_priority", KeepalivedConfig{VRID: util.IntPtr(50), Interface: util.StrPtr("eth0")}, ".priority: must be specified"},
		{"bad_priority_low", KeepalivedConfig{Priority: util.IntPtr(0)}, ".priority: must be between 1 and 254"},
		{"bad_priority_high", KeepalivedConfig{Priority: util.IntPtr(255)}, ".priority: must be between 1 and 254"},
		{"nil_interface", KeepalivedConfig{VRID: util.IntPtr(50), Priority: util.IntPtr(100)}, ".interface: network interface must be specified"},
		{"empty_interface", KeepalivedConfig{Interface: util.StrPtr(" ")}, ".interface: network interface must be specified"},
		{"invalid_auth_type", KeepalivedConfig{AuthType: util.StrPtr("NONE")}, "invalid value 'NONE'"},
		{"pass_auth_no_pass", KeepalivedConfig{AuthType: util.StrPtr("PASS"), AuthPass: util.StrPtr(" ")}, ".authPass: must be specified if authType is 'PASS'"},
		{"pass_auth_long_pass", KeepalivedConfig{AuthType: util.StrPtr("PASS"), AuthPass: util.StrPtr("longpassword")}, ".authPass: password too long"},
		{"ah_auth_with_pass", KeepalivedConfig{AuthType: util.StrPtr("AH"), AuthPass: util.StrPtr("secret")}, ".authPass: should not be specified if authType is 'AH'"},
		{"empty_extra_config_line", KeepalivedConfig{ExtraConfig: []string{" "}}, ".extraConfig[0]: extra config line cannot be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cfg.VRID == nil && !strings.Contains(tt.name, "vrid") { tt.cfg.VRID = util.IntPtr(1) }
			if tt.cfg.Priority == nil && !strings.Contains(tt.name, "priority") { tt.cfg.Priority = util.IntPtr(100) }
			if tt.cfg.Interface == nil && !strings.Contains(tt.name, "interface") { tt.cfg.Interface = util.StrPtr("net1") }

			if tt.cfg.AuthType == nil || *tt.cfg.AuthType == "PASS" {
				if tt.cfg.AuthPass == nil && !strings.Contains(tt.name, "auth_no_pass") && !strings.Contains(tt.name, "auth_long_pass") {
					tt.cfg.AuthPass = util.StrPtr("default")
				}
			}
			SetDefaults_KeepalivedConfig(&tt.cfg)

			verrs := &ValidationErrors{}
			Validate_KeepalivedConfig(&tt.cfg, verrs, "keepalived")
			assert.True(t, verrs.HasErrors(), "Validate_KeepalivedConfig expected error for %s, got none", tt.name)
			if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
				t.Errorf("Error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
			}
		})
	}
}
