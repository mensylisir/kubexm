package spec

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

	// RunOnRoles specifies which host roles this task should target.
	// The Executor will use this, along with the Filter, to determine the
	// actual list of hosts on which to run the steps of this task.
	RunOnRoles []string

	// Filter provides a more granular, dynamic way to select target hosts.
	// If non-nil, the Executor will apply this function to hosts that match RunOnRoles.
	// Note: Including a function type here means TaskSpec structs created in Go
	// are not directly serializable to formats like JSON/YAML without special handling
	// (e.g., by omitting this field during serialization or using a string-based rule).
	// For specs defined and consumed within Go code, this is acceptable.
	Filter func(host *runtime.Host) bool

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

	// Tasks is an ordered slice of *TaskSpec pointers. These define the tasks
	// to be performed by this module, in sequence.
	Tasks []*TaskSpec

	// IsEnabled is a function that determines if this module should be executed,
	// typically based on the provided cluster configuration. If nil, the Executor
	// will consider the module always enabled.
	// Note: Similar to TaskSpec.Filter, a function type here impacts direct
	// serializability if specs are to be defined in formats like JSON/YAML.
	IsEnabled func(clusterRt *runtime.ClusterRuntime) bool // Changed to use ClusterRuntime

	// PreRun is a StepSpec that defines a step to be executed once before any
	// tasks in this module are run. Can be nil if no pre-run step is needed.
	// If this step fails, the module's tasks and PostRun step are typically skipped.
	PreRun StepSpec

	// PostRun is a StepSpec that defines a step to be executed once after all
	// tasks in this module have attempted to run (or after a PreRun/critical task failure).
	// Can be nil if no post-run step is needed.
	// Errors from this step are typically logged but might not override a primary
	// error from the module's main task execution.
	PostRun StepSpec
}


// PipelineSpec defines the declarative specification for an entire pipeline.
// A pipeline orchestrates a sequence of modules to achieve a major operational goal,
// such as creating a new cluster, upgrading a cluster, or adding nodes.
// The actual execution of a PipelineSpec is handled by the Executor.
type PipelineSpec struct {
	// Name is a descriptive name for the pipeline (e.g., "CreateCluster", "UpgradeCluster").
	Name string

	// Modules is an ordered slice of *ModuleSpec pointers. These define the modules
	// to be executed by this pipeline, in sequence. The order is critical as it
	// often represents dependencies between modules.
	Modules []*ModuleSpec

	// PreRun is a StepSpec that defines a step to be executed once before any
	// modules in this pipeline are run. Can be nil. If this step fails,
	// the pipeline's modules and PostRun step are typically skipped.
	PreRun StepSpec

	// PostRun is a StepSpec that defines a step to be executed once after all
	// modules in this pipeline have attempted to run (or after a PreRun or
	// critical module failure). Can be nil. Errors from this step are
	// typically logged but might not override a primary error from the pipeline's
	// main execution.
	PostRun StepSpec
}

// No separate HookSpec is needed as PreRun/PostRun are directly StepSpec.

// Ensure necessary imports are present, especially for runtime.ClusterRuntime and runtime.Host
// The file already contains "github.com/kubexms/kubexms/pkg/runtime" based on TaskSpec.Filter
// No import for "github.com/kubexms/kubexms/pkg/config" should be needed here anymore.
