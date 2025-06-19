package v1alpha1

import (
	"strings"
	"testing"
)

func TestSetDefaults_HighAvailabilityConfig(t *testing.T) {
	cfg := &HighAvailabilityConfig{}
	SetDefaults_HighAvailabilityConfig(cfg)
	// Current SetDefaults_HighAvailabilityConfig is minimal, mostly a placeholder.
	// Test that it runs without panic. If Type gets a default, test it.
	if cfg.Type != "" { // Assuming no default type is set for now
		t.Errorf("Default Type = %s, want empty or specific default", cfg.Type)
	}
}

func TestValidate_HighAvailabilityConfig(t *testing.T) {
	validCfg := &HighAvailabilityConfig{Type: "keepalived", VIP: "192.168.1.100"}
	verrsValid := &ValidationErrors{}
	Validate_HighAvailabilityConfig(validCfg, verrsValid, "spec.highAvailability")
	if !verrsValid.IsEmpty() {
		t.Errorf("Validate_HighAvailabilityConfig for valid config failed: %v", verrsValid)
	}

	invalidCfg := &HighAvailabilityConfig{Type: "keepalived", VIP: ""} // Missing VIP
	verrsInvalid := &ValidationErrors{}
	Validate_HighAvailabilityConfig(invalidCfg, verrsInvalid, "spec.highAvailability")
	if verrsInvalid.IsEmpty() || !strings.Contains(verrsInvalid.Errors[0], ".vip: must be set if HA type is 'keepalived'") {
		t.Errorf("Validate_HighAvailabilityConfig for missing VIP failed or wrong message: %v", verrsInvalid)
	}
}
