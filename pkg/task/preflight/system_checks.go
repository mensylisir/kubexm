package preflight

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	stepPreflight "github.com/kubexms/kubexms/pkg/step/preflight"
)

// NewSystemChecksTask creates a new task specification for common system preflight checks.
func NewSystemChecksTask(cfg *config.Cluster) *spec.TaskSpec {
	// Default values for checks
	minCores := 2
	minMemoryMB := uint64(2048) // 2GB
	runDisableSwapStep := true // By default, try to disable swap

	if cfg != nil && cfg.Spec.Preflight != nil { // cfg is *v1alpha1.Cluster, Spec.Preflight is *v1alpha1.PreflightConfig
		// Check if user provided values that are greater than 0 (for numbers)
		if cfg.Spec.Preflight.MinCPUCores > 0 {
			minCores = cfg.Spec.Preflight.MinCPUCores
		}
		if cfg.Spec.Preflight.MinMemoryMB > 0 {
			minMemoryMB = cfg.Spec.Preflight.MinMemoryMB
		}
		// v1alpha1.PreflightConfig.DisableSwap is a bool. If the Preflight section is present,
		// and DisableSwap is true in YAML, this will be true.
		// If DisableSwap is false or omitted in YAML (and Preflight section is present), this will be false.
		// The initial `runDisableSwapStep := true` acts as the default if the entire Preflight section is missing
		// or if cfg is nil.
		// If Preflight section is present, we use its DisableSwap value.
		runDisableSwapStep = cfg.Spec.Preflight.DisableSwap
	}
	// If cfg is nil, or cfg.Spec.Preflight is nil, the hardcoded defaults for minCores, minMemoryMB,
	// and runDisableSwapStep (true) will be used.

	steps := []spec.StepSpec{
		&stepPreflight.CheckCPUStepSpec{MinCores: minCores},
		&stepPreflight.CheckMemoryStepSpec{MinMemoryMB: minMemoryMB},
	}

	if runDisableSwapStep {
		steps = append(steps, &stepPreflight.DisableSwapStepSpec{})
	}

	return &spec.TaskSpec{
		Name: "Run System Preflight Checks",
		RunOnRoles: []string{},
		Steps: steps,
		Concurrency: 10,
		IgnoreError: false,
	}
}
