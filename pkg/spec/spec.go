package spec

import "github.com/mensylisir/kubexm/pkg/runtime"

// StepSpec is a marker interface for all concrete step specifications.
// Concrete step specifications are data structures that define the parameters
// for a particular type of step (e.g., CommandStepSpec, CheckCPUSpec).
//
// The primary purpose of a StepSpec is to hold the declarative configuration for a step.
// The actual execution logic for a StepSpec is handled by a corresponding StepExecutor
// (defined in the pkg/step package).
//
// Example of a concrete StepSpec (would be defined in its respective step package, e.g., pkg/step/command):
//   type CommandStepSpec struct {
//       Name string
//       Cmd  string
//       Sudo bool
//       // ... other command specific parameters
//   }
//   func (s *CommandStepSpec) GetName() string { return s.Name }
//
// StepSpec instances are collected within TaskSpec objects.
type StepSpec interface {
	// GetName returns the configured or generated name of the step.
	// This is useful for logging and identification within the executor.
	GetName() string
}

// TaskSpec defines the declarative specification for a task.
// A task is a collection of steps aimed at achieving a small, independent functional goal.
// TaskSpecs are collected within ModuleSpec objects.
// The actual execution of a TaskSpec is handled by the Executor.
type TaskSpec struct {
	// Name is a descriptive name for the task. Used for logging and identification.
	Name string

	// Steps is an ordered slice of StepSpec interfaces. These define the individual
	// operations to be performed by this task, in sequence.
	Steps []StepSpec

	// Description is a human-readable description of what the task does.
	Description string

	// RunOnRoles specifies which host roles this task should target.
	// The Executor will use this, along with the Filter, to determine the
	// actual list of hosts on which to run the steps of this task.
	RunOnRoles []string

	// Filter is a placeholder for a filter identifier or DSL string.
	// This string will be processed by the Executor to dynamically determine
	// the target hosts, potentially in conjunction with RunOnRoles.
	Filter string

	// IgnoreError, if true, indicates that an error from this task's execution
	// (i.e., a failure of one of its critical steps) should not cause the parent
	// ModuleSpec's execution to halt. The error will still be logged.
	IgnoreError bool

	// Concurrency specifies the maximum number of hosts on which the steps of this task
	// will be executed concurrently by the Executor. If zero or negative, the Executor
	// might use a sensible default (e.g., 10).
	Concurrency int
}

// ModuleSpec defines the declarative specification for a module.
// A module groups related tasks to manage the lifecycle or a significant
// functional aspect of a software component (e.g., Etcd, Containerd).
// ModuleSpecs are collected within a PipelineSpec.
// The actual execution of a ModuleSpec is handled by the Executor.
type ModuleSpec struct {
	// Name is a descriptive name for the module. Used for logging and identification.
	Name string

	// Description is a human-readable summary of what the module does.
	Description string

	// Tasks is an ordered slice of *TaskSpec pointers. These define the tasks
	// to be performed by this module, in sequence.
	Tasks []*TaskSpec

	// IsEnabled is a condition string (e.g., a CEL expression or a custom DSL)
	// that the Executor will evaluate to determine if this module should be executed.
	// Example: "cfg.Spec.ContainerRuntime.Type == 'containerd'"
	IsEnabled string

	// PreRunHook is an identifier (e.g., a string key) for a PreRun hook function or task
	// that should be executed by the Executor before the main Tasks of this module.
	// The Executor will need a way to map this identifier to an actual executable hook.
	// Example: "common_network_check_hook"
	PreRunHook string

	// PostRunHook is an identifier for a PostRun hook function or task, similar to PreRunHook,
	// to be executed by the Executor after the main Tasks of this module have completed (or failed).
	// Example: "cleanup_temp_files_hook"
	PostRunHook string
}


// PipelineSpec defines the declarative specification for an entire pipeline.
// A pipeline orchestrates a sequence of modules to achieve a major operational goal,
// such as creating a new cluster, upgrading a cluster, or adding nodes.
// The actual execution of a PipelineSpec is handled by the Executor.
type PipelineSpec struct {
	// Name is a descriptive name for the pipeline (e.g., "CreateCluster", "UpgradeCluster").
	Name string

	// Description is a human-readable summary of what the pipeline does.
	Description string

	// Modules is an ordered slice of *ModuleSpec pointers. These define the modules
	// to be executed by this pipeline, in sequence. The order is critical as it
	// often represents dependencies between modules.
	Modules []*ModuleSpec

	// PreRunHook is an identifier (e.g., a string key) for a PreRun hook function or task
	// that should be executed by the Executor before any Modules in this pipeline are run.
	// The Executor will need a way to map this identifier to an actual executable hook.
	// Example: "pipeline_init_logging_hook"
	PreRunHook string

	// PostRunHook is an identifier for a PostRun hook function or task, similar to PreRunHook,
	// to be executed by the Executor after all Modules in this pipeline have completed (or failed).
	// Example: "pipeline_final_report_hook"
	PostRunHook string
}

// Hooks are now TaskSpecs. // This comment seems outdated in context of PipelineSpec.

// Ensure necessary imports are present, especially for runtime.ClusterRuntime and runtime.Host
// The file already contains "github.com/kubexms/kubexms/pkg/runtime" based on TaskSpec.Filter
// No import for "github.com/kubexms/kubexms/pkg/config" should be needed here anymore.
