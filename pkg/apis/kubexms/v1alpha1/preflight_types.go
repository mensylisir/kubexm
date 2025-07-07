package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/util" // Added import for util
	"github.com/mensylisir/kubexm/pkg/util/validation"
)

// PreflightConfig holds configuration for preflight checks.
type PreflightConfig struct {
	MinCPUCores   *int32  `json:"minCPUCores,omitempty" yaml:"minCPUCores,omitempty"`     // Pointer for optionality
	MinMemoryMB   *uint64 `json:"minMemoryMB,omitempty" yaml:"minMemoryMB,omitempty"`     // Pointer for optionality
	DisableSwap   *bool   `json:"disableSwap,omitempty" yaml:"disableSwap,omitempty"`     // Pointer for three-state (true, false, not set)
	// TODO: Add more preflight checks like disk space, specific kernel modules required, etc.
}

// SetDefaults_PreflightConfig sets default values for PreflightConfig.
func SetDefaults_PreflightConfig(cfg *PreflightConfig) {
	if cfg == nil {
		return
	}
	if cfg.DisableSwap == nil {
		cfg.DisableSwap = util.BoolPtr(true) // Default to disabling swap
	}
	if cfg.MinCPUCores == nil {
		defaultCPU := int32(2)
		cfg.MinCPUCores = &defaultCPU
	}
	if cfg.MinMemoryMB == nil {
		defaultMem := uint64(2048) // 2GB
		cfg.MinMemoryMB = &defaultMem
	}
}

// DeepCopyInto creates a deep copy of the PreflightConfig.
func (in *PreflightConfig) DeepCopyInto(out *PreflightConfig) {
	*out = *in
	if in.MinCPUCores != nil {
		val := *in.MinCPUCores
		out.MinCPUCores = &val
	}
	if in.MinMemoryMB != nil {
		val := *in.MinMemoryMB
		out.MinMemoryMB = &val
	}
	if in.DisableSwap != nil {
		val := *in.DisableSwap
		out.DisableSwap = &val
	}
}

// DeepCopy creates a new PreflightConfig.
func (in *PreflightConfig) DeepCopy() *PreflightConfig {
	if in == nil {
		return nil
	}
	out := new(PreflightConfig)
	in.DeepCopyInto(out)
	return out
}


// Validate_PreflightConfig validates PreflightConfig.
func Validate_PreflightConfig(cfg *PreflightConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.MinCPUCores != nil && *cfg.MinCPUCores <= 0 {
		verrs.Add(pathPrefix+".minCPUCores", fmt.Sprintf("must be positive if specified, got %d", *cfg.MinCPUCores))
	}
	if cfg.MinMemoryMB != nil && *cfg.MinMemoryMB <= 0 { // Memory should be positive
		verrs.Add(pathPrefix+".minMemoryMB", fmt.Sprintf("must be positive if specified, got %d", *cfg.MinMemoryMB))
	}
}
