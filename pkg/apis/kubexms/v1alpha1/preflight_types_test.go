package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/mensylisir/kubexm/pkg/util/validation"
	"github.com/mensylisir/kubexm/pkg/util" // Added import
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

	// MinCPUCores and MinMemoryMB are now defaulted
	cfgNilCPU := &PreflightConfig{}
	SetDefaults_PreflightConfig(cfgNilCPU)
	assert.NotNil(t, cfgNilCPU.MinCPUCores, "MinCPUCores should be defaulted")
	if cfgNilCPU.MinCPUCores != nil {
		assert.Equal(t, int32(2), *cfgNilCPU.MinCPUCores, "Default MinCPUCores mismatch")
	}
	assert.NotNil(t, cfgNilCPU.MinMemoryMB, "MinMemoryMB should be defaulted")
	if cfgNilCPU.MinMemoryMB != nil {
		assert.Equal(t, uint64(2048), *cfgNilCPU.MinMemoryMB, "Default MinMemoryMB mismatch")
	}
}

func TestValidate_PreflightConfig(t *testing.T) {
	// Test case where all values are valid (using defaults or explicit valid settings)
	validCfg := &PreflightConfig{} // Rely on defaults
	SetDefaults_PreflightConfig(validCfg)
	verrsValid := &validation.ValidationErrors{}
	Validate_PreflightConfig(validCfg, verrsValid, "spec.preflight")
	assert.False(t, verrsValid.HasErrors(), "Validate_PreflightConfig for default valid config failed: %v", verrsValid.Error())

	// Test with explicit valid values that are different from defaults (if applicable)
	validExplicitCfg := &PreflightConfig{MinCPUCores: util.Int32Ptr(4), MinMemoryMB: util.Uint64Ptr(4096), DisableSwap: util.BoolPtr(false)}
	SetDefaults_PreflightConfig(validExplicitCfg) // Defaults won't override these
	verrsValidExplicit := &validation.ValidationErrors{}
	Validate_PreflightConfig(validExplicitCfg, verrsValidExplicit, "spec.preflight")
	assert.False(t, verrsValidExplicit.HasErrors(), "Validate_PreflightConfig for explicit valid config failed: %v", verrsValidExplicit.Error())


	tests := []struct {
		name        string
		cfg         *PreflightConfig
		wantErrMsg  string
		expectErr   bool
	}{
		// Now that MinCPU/Mem are defaulted, a config with only DisableSwap set will get defaults for others.
		{"valid_disable_swap_only", &PreflightConfig{DisableSwap: util.BoolPtr(true)}, "", false},
		{"negative_cpu", &PreflightConfig{MinCPUCores: util.Int32Ptr(-1)}, ".minCPUCores: must be positive", true},
		{"zero_cpu", &PreflightConfig{MinCPUCores: util.Int32Ptr(0)}, ".minCPUCores: must be positive", true},
		{"zero_mem", &PreflightConfig{MinMemoryMB: util.Uint64Ptr(0)}, ".minMemoryMB: must be positive", true},
		{"valid_min_values_override_defaults", &PreflightConfig{MinCPUCores: util.Int32Ptr(1), MinMemoryMB: util.Uint64Ptr(1)}, "", false},
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
