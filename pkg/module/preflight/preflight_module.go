package preflight

import (
	"github.com/kubexms/kubexms/pkg/config"
	// "github.com/kubexms/kubexms/pkg/runtime" // No longer needed for PreRun/PostRun func signatures
	"github.com/kubexms/kubexms/pkg/spec"   // Use spec.ModuleSpec and spec.TaskSpec
	taskPreflight "github.com/kubexms/kubexms/pkg/task/preflight"
	// commandStepSpec "github.com/kubexms/kubexms/pkg/step/command" // Example if hooks were CommandStepSpec
	// "github.com/kubexms/kubexms/pkg/module" // No longer needed for defining ModuleSpec
)

// NewPreflightModule creates a new module specification for preflight checks and setup.
func NewPreflightModule(cfg *config.Cluster) *spec.ModuleSpec {
	return &spec.ModuleSpec{
		Name: "Preflight Checks and Setup",
		// IsEnabled is typically always true for preflight, unless explicitly configured otherwise.
		IsEnabled: func(clusterCfg *config.Cluster) bool {
			// Example: Could be disabled via a global config flag if needed
			// if cfg != nil && cfg.Spec.Global != nil && cfg.Spec.Global.SkipPreflight { return false }
			return true
		},
		Tasks: []*spec.TaskSpec{ // Populate with *spec.TaskSpec from refactored task factories
			taskPreflight.NewSystemChecksTask(cfg),
			taskPreflight.NewSetupKernelTask(cfg),
			// Add other preflight tasks here as they are defined
			// e.g., taskPreflight.NewSetupEtcHostsTaskSpec(cfg), // Assuming TaskSpec naming
		},
		// Example PreRun/PostRun hooks using a simple command step spec
		// These would need actual StepSpec definitions (e.g., from pkg/step/command/spec.go if that existed)
		// PreRun: &commandStepSpec.CommandStepSpec{SpecName: "Preflight Module PreRun Echo", Cmd: "echo 'Starting preflight module'"},
		// PostRun: &commandStepSpec.CommandStepSpec{SpecName: "Preflight Module PostRun Echo", Cmd: "echo 'Finished preflight module'"},
		PreRun: nil,  // No specific pre-run step defined for this module yet
		PostRun: nil, // No specific post-run step defined for this module yet
	}
}
