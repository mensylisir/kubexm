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

	if cfg != nil { // cfg itself might be nil in some test scenarios or if not loaded
		// PreflightConfig is a struct, not a pointer, so it always exists if cfg.Spec does.
		// Check if user provided values that are greater than 0 (for numbers)
		if cfg.Spec.PreflightConfig.MinCPUCores > 0 {
			minCores = cfg.Spec.PreflightConfig.MinCPUCores
		}
		if cfg.Spec.PreflightConfig.MinMemoryMB > 0 {
			minMemoryMB = cfg.Spec.PreflightConfig.MinMemoryMB
		}
		// For DisableSwap: if the field `disableSwap` is present in YAML and set to `false`,
		// then cfg.Spec.PreflightConfig.DisableSwap will be `false`.
		// If `disableSwap` is `true` in YAML, it will be `true`.
		// If `disableSwap` is missing in YAML, it will be `false` (Go default for bool).
		// So, we run DisableSwapStep only if cfg.Spec.PreflightConfig.DisableSwap is true.
		// This makes disabling swap an opt-in feature if specified via config.
		// If we want "disable swap by default unless explicitly told not to", the logic would be:
		// runDisableSwapStep = !cfg.Spec.PreflightConfig.ExplicitlyKeepSwapEnabled (new bool field)
		// Or, if DisableSwap means "ensure swap is disabled":
		// if cfg.Spec.PreflightConfig.IsSet("DisableSwap") && !cfg.Spec.PreflightConfig.DisableSwap { runDisableSwapStep = false }
		// Given the current simple bool `DisableSwap`, the most straightforward is:
		// only run the step if the config explicitly says `disableSwap: true`.
		runDisableSwapStep = cfg.Spec.PreflightConfig.DisableSwap
	}

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
