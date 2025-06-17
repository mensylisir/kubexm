package preflight

import (
	// Assuming config.Cluster will be defined in this path.
	// For now, this task doesn't directly use cfg but it's good practice for factories.
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/preflight" // Import the actual preflight steps
	"github.com/kubexms/kubexms/pkg/task"
)

// NewSystemChecksTask creates a new task that performs common system preflight checks.
// The cfg parameter is included for future use if checks need to be data-driven from config,
// e.g., specific OS versions to support, or configurable min CPU/memory.
func NewSystemChecksTask(cfg *config.Cluster) *task.Task {
	// Min CPU and Memory values can be hardcoded for now or come from cfg if available.
	// Example: minCores := cfg.Spec.Preflight.MinCPUCores (if such a field existed in config.Cluster)
	minCores := 2
	minMemoryMB := uint64(2048) // 2GB

	// TODO: Implement preflight.CheckOSVersionStep if it's defined.
	// For now, it's omitted as it wasn't part of the specific steps implemented earlier.
	// If it were: &preflight.CheckOSVersionStep{SupportedOS: ...},

	return &task.Task{
		Name: "Run System Preflight Checks",
		// RunOnRoles being empty implies it should run on all hosts selected by the module/pipeline.
		// Or, specific roles like ["all"], ["master"], ["worker"] could be used by orchestrator.
		RunOnRoles: []string{},
		Steps: []step.Step{
			&preflight.CheckCPUStep{MinCores: minCores},
			&preflight.CheckMemoryStep{MinMemoryMB: minMemoryMB},
			// &preflight.CheckFirewallStep{}, // Example: Assuming not yet implemented
			&preflight.DisableSwapStep{},
		},
		Concurrency: 10, // Default concurrency for this task
		IgnoreError: false, // By default, preflight check failures should halt execution.
	}
}
