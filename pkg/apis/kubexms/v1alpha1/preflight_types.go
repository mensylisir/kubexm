package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common" // Added common import
	"github.com/mensylisir/kubexm/pkg/util" // Added import for util
	"github.com/mensylisir/kubexm/pkg/util/validation"
)

// PreflightConfig holds configuration for preflight checks.
// +k8s:deepcopy-gen=true
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
		// Use common.DefaultMinCPUCores
		defaultCPU := int32(common.DefaultMinCPUCores)
		cfg.MinCPUCores = &defaultCPU
	}
	if cfg.MinMemoryMB == nil {
		// Use common.DefaultMinMemoryMB
		defaultMem := uint64(common.DefaultMinMemoryMB) // Ensure type compatibility if necessary
		cfg.MinMemoryMB = &defaultMem
	}
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
