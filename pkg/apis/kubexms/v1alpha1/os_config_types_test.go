package v1alpha1

import (
	"strings"
	"testing"
)

// Helpers for OSConfig tests
func pboolOsTest(b bool) *bool { return &b }
func pstrOsTest(s string) *string { return &s }

func TestSetDefaults_OSConfig(t *testing.T) {
	cfg := &OSConfig{}
	SetDefaults_OSConfig(cfg)

	if cfg.NtpServers == nil || cap(cfg.NtpServers) != 0 {
		t.Error("NtpServers should be initialized to empty slice")
	}
	if cfg.Rpms == nil || cap(cfg.Rpms) != 0 {
		t.Error("Rpms should be initialized to empty slice")
	}
	if cfg.Debs == nil || cap(cfg.Debs) != 0 {
		t.Error("Debs should be initialized to empty slice")
	}
	if cfg.SkipConfigureOS == nil || *cfg.SkipConfigureOS != false {
		t.Errorf("Default SkipConfigureOS = %v, want false", cfg.SkipConfigureOS)
	}
	if cfg.Timezone != nil { // Timezone is not defaulted
		t.Errorf("Timezone should be nil by default, got %v", *cfg.Timezone)
	}
}

func TestValidate_OSConfig(t *testing.T) {
	validCfg := &OSConfig{
		NtpServers: []string{"0.pool.ntp.org", "1.pool.ntp.org"},
		Timezone:   pstrOsTest("Etc/UTC"),
		Rpms:       []string{"some-package"},
		Debs:       []string{"another-package"},
	}
	SetDefaults_OSConfig(validCfg) // Apply defaults
	verrsValid := &ValidationErrors{}
	Validate_OSConfig(validCfg, verrsValid, "spec.os")
	if !verrsValid.IsEmpty() {
		t.Errorf("Validate_OSConfig for valid config failed: %v", verrsValid)
	}

	tests := []struct {
		name       string
		cfg        *OSConfig
		wantErrMsg string
	}{
		{"empty_ntp_server", &OSConfig{NtpServers: []string{" "}}, ".ntpServers[0]: NTP server address cannot be empty"},
		{"empty_timezone_if_set", &OSConfig{Timezone: pstrOsTest(" ")}, ".timezone: cannot be empty if specified"},
		{"empty_rpm_package", &OSConfig{Rpms: []string{" "}}, ".rpms[0]: RPM package name cannot be empty"},
		{"empty_deb_package", &OSConfig{Debs: []string{" "}}, ".debs[0]: DEB package name cannot be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_OSConfig(tt.cfg) // Apply defaults
			verrs := &ValidationErrors{}
			Validate_OSConfig(tt.cfg, verrs, "spec.os")
			if verrs.IsEmpty() {
				t.Fatalf("Validate_OSConfig expected error for %s, got none", tt.name)
			}
			if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
				t.Errorf("Validate_OSConfig error for %s = %v, want to contain %q", tt.name, verrs, tt.wantErrMsg)
			}
		})
	}
}
