package preflight

import (
	"github.com/mensylisir/kubexm/pkg/config" // For config.Cluster
	"github.com/mensylisir/kubexm/pkg/spec"
	taskPreflightFactory "github.com/mensylisir/kubexm/pkg/task/preflight" // Alias for task spec factories
)

// NewPreflightModuleSpec creates a new module specification for preflight checks and setup.
func NewPreflightModuleSpec(cfg *config.Cluster) *spec.ModuleSpec {
	if cfg == nil {
		return &spec.ModuleSpec{
			Name:        "Preflight Checks and Setup",
			Description: "Performs system preflight checks and applies necessary kernel/system configurations (Error: Missing Configuration)",
			IsEnabled:   "false",
			Tasks:       []*spec.TaskSpec{},
		}
	}

	// Define the condition string for IsEnabled.
	// This reflects the logic: enabled if Global is nil, or Global.SkipPreflight is false.
	// The Executor will evaluate this against the 'cfg' object.
	isEnabledCondition := "(cfg.Spec.Global == nil) || (cfg.Spec.Global.SkipPreflight == false)"

	return &spec.ModuleSpec{
		Name:        "Preflight Checks and Setup",
		Description: "Performs system preflight checks (CPU, memory) and applies necessary kernel/system configurations.",
		IsEnabled:   isEnabledCondition,
		Tasks: []*spec.TaskSpec{
			// These factories already return *spec.TaskSpec
			taskPreflightFactory.NewSystemChecksTask(cfg),
			taskPreflightFactory.NewSetupKernelTask(cfg),
			// Consider adding NewSetupKubernetesPrerequisitesTask(cfg) here if it's standard for preflight
		},
		PreRunHook:  "", // Example: "preflight_start_logging_hook"
		PostRunHook: "", // Example: "preflight_report_results_hook"
	}
}
