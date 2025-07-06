package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/mensylisir/kubexm/pkg/util/validation"
)

func TestSetDefaults_PreflightConfig(t *testing.T) {
	cfg := &PreflightConfig{}
	SetDefaults_PreflightConfig(cfg)
	assert.NotNil(t, cfg.DisableSwap)
	assert.True(t, *cfg.DisableSwap, "Default DisableSwap should be true")

	// Test case where DisableSwap is already set
	falseVal := false
	cfgCustom := &PreflightConfig{DisableSwap: &falseVal}
	SetDefaults_PreflightConfig(cfgCustom)
	assert.NotNil(t, cfgCustom.DisableSwap)
	assert.False(t, *cfgCustom.DisableSwap, "DisableSwap should not be overridden if already set")

	// MinCPUCores and MinMemoryMB are not defaulted by SetDefaults_PreflightConfig currently
	cfgNilCPU := &PreflightConfig{}
	SetDefaults_PreflightConfig(cfgNilCPU)
	assert.Nil(t, cfgNilCPU.MinCPUCores)
	assert.Nil(t, cfgNilCPU.MinMemoryMB)
}

func TestValidate_PreflightConfig(t *testing.T) {
	validCfg := &PreflightConfig{MinCPUCores: int32Ptr(2), MinMemoryMB: uint64Ptr(2048), DisableSwap: boolPtr(true)}
	SetDefaults_PreflightConfig(validCfg)
	verrsValid := &validation.ValidationErrors{}
	Validate_PreflightConfig(validCfg, verrsValid, "spec.preflight")
	assert.False(t, verrsValid.HasErrors(), "Validate_PreflightConfig for valid config failed: %v", verrsValid.Error())

	tests := []struct {
		name        string
		cfg         *PreflightConfig
		wantErrMsg  string
		expectErr   bool
	}{
		{"valid_nil_values", &PreflightConfig{DisableSwap: boolPtr(true)}, "", false}, // MinCPU/Mem not set, should be valid
		{"negative_cpu", &PreflightConfig{MinCPUCores: int32Ptr(-1)}, ".minCPUCores: must be positive", true},
		{"zero_cpu", &PreflightConfig{MinCPUCores: int32Ptr(0)}, ".minCPUCores: must be positive", true},
		{"zero_mem", &PreflightConfig{MinMemoryMB: uint64Ptr(0)}, ".minMemoryMB: must be positive", true},
		// {"negative_mem", &PreflightConfig{MinMemoryMB: uint64Ptr(^uint64(0))}, ".minMemoryMB: must be positive", true}, // This test was flawed: MaxUint64 is not <= 0.
		{"valid_min_values_only", &PreflightConfig{MinCPUCores: int32Ptr(1), MinMemoryMB: uint64Ptr(1)}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply defaults. For these tests, DisableSwap is the only thing defaulted if not set.
			if tt.cfg.DisableSwap == nil { // Explicitly ensure DisableSwap has a value for consistent testing if needed.
				SetDefaults_PreflightConfig(tt.cfg)
			}

			verrs := &validation.ValidationErrors{}
			Validate_PreflightConfig(tt.cfg, verrs, "spec.preflight")

			if tt.expectErr {
				assert.True(t, verrs.HasErrors(), "Expected error for %s, got none", tt.name)
				assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
			} else {
				assert.False(t, verrs.HasErrors(), "Expected no error for %s, got: %v", tt.name, verrs.Error())
			}
		})
	}
}
