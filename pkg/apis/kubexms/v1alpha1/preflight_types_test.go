package v1alpha1

import (
	"strings"
	"testing"
)

func TestSetDefaults_PreflightConfig(t *testing.T) {
	cfg := &PreflightConfig{}
	SetDefaults_PreflightConfig(cfg)
	if cfg.DisableSwap == nil || !*cfg.DisableSwap {
		t.Errorf("Default DisableSwap = %v, want true", cfg.DisableSwap)
	}
	// Add tests for MinCPUCores/MinMemoryMB defaults if they are implemented
}

func TestValidate_PreflightConfig(t *testing.T) {
	validCfg := &PreflightConfig{MinCPUCores: int32Ptr(2), MinMemoryMB: uint64Ptr(2048), DisableSwap: boolPtr(true)}
	verrsValid := &ValidationErrors{}
	Validate_PreflightConfig(validCfg, verrsValid, "spec.preflight")
	if !verrsValid.IsEmpty() {
		t.Errorf("Validate_PreflightConfig for valid config failed: %v", verrsValid)
	}

	tests := []struct{
		name string
		cfg *PreflightConfig
		wantErrMsg string
	}{
		{"negative_cpu", &PreflightConfig{MinCPUCores: int32Ptr(-1)}, ".minCPUCores: must be positive"},
		{"zero_cpu", &PreflightConfig{MinCPUCores: int32Ptr(0)}, ".minCPUCores: must be positive"},
		// Assuming MinMemoryMB must also be positive, though 0 could mean "don't check".
		// Current Validate_PreflightConfig checks for "<= 0", so 0 is an error.
		{"zero_mem", &PreflightConfig{MinMemoryMB: uint64Ptr(0)}, ".minMemoryMB: must be positive"},
	}
	for _, tt := range tests {
	   t.Run(tt.name, func(t *testing.T){
		   SetDefaults_PreflightConfig(tt.cfg)
		   verrs := &ValidationErrors{}
		   Validate_PreflightConfig(tt.cfg, verrs, "spec.preflight")
		   if verrs.IsEmpty() {
			   t.Fatalf("Expected error for %s, got none", tt.name)
		   }
		   if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
			   t.Errorf("Error for %s = %v, want to contain %q", tt.name, verrs, tt.wantErrMsg)
		   }
	   })
	}
}
