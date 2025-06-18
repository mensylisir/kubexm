package preflight

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec" // Use spec.TaskSpec and spec.StepSpec
	stepPreflight "github.com/kubexms/kubexms/pkg/step/preflight" // Import the actual preflight step specs
	// "github.com/kubexms/kubexms/pkg/task" // No longer needed for defining the TaskSpec
)

// NewSystemChecksTask creates a new task specification for common system preflight checks.
// The cfg parameter is included for future use if checks need to be data-driven from config,
// e.g., specific OS versions to support, or configurable min CPU/memory.
func NewSystemChecksTask(cfg *config.Cluster) *spec.TaskSpec {
	minCores := 2
	minMemoryMB := uint64(2048) // 2GB

	// Example of how values could be read from config if defined:
	// if cfg != nil && cfg.Spec.PreflightChecks != nil { // Assuming a PreflightChecksSpec in config
	//    if cfg.Spec.PreflightChecks.MinCPUCores > 0 {
	//        minCores = cfg.Spec.PreflightChecks.MinCPUCores
	//    }
	//    if cfg.Spec.PreflightChecks.MinMemoryMB > 0 {
	//        minMemoryMB = cfg.Spec.PreflightChecks.MinMemoryMB
	//    }
	// }

	return &spec.TaskSpec{
		Name: "Run System Preflight Checks",
		// RunOnRoles being empty implies it should run on all hosts selected by the module/pipeline
		// or by a higher-level orchestrator that calls this task factory.
		RunOnRoles: []string{},
		Steps: []spec.StepSpec{ // Populate with instances of concrete StepSpec implementers
			&stepPreflight.CheckCPUStepSpec{MinCores: minCores},
			&stepPreflight.CheckMemoryStepSpec{MinMemoryMB: minMemoryMB},
			&stepPreflight.DisableSwapStepSpec{},
			// Example: Add CheckOSVersionStepSpec if it were defined
			// &stepPreflight.CheckOSVersionStepSpec{SupportedVersions: map[string][]string{"ubuntu": {"20.04", "22.04"}}},
		},
		Concurrency: 10,
		IgnoreError: false, // Typically, preflight check failures are critical for cluster setup.
	}
}
