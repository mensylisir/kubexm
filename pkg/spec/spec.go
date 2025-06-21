package spec

// StepMeta contains common metadata for a step.
// This structure can be embedded in concrete Step specifications.
type StepMeta struct {
	// Name is a unique identifier for the step type or a user-defined name for an instance of a step.
	// For example, a CommandStep might have its Name set to "Install Essential Packages".
	// The plan.ExecutionNode.StepName will typically be derived from this.
	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	// Description provides a human-readable summary of what the step does.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Hidden specifies whether the step's execution details (like commands)
	// should be hidden in logs. Useful for sensitive operations.
	Hidden bool `json:"hidden,omitempty" yaml:"hidden,omitempty"`

	// AllowFailure, if true, means that a failure of this step will not cause the
	// entire node (and thus potentially the graph) to fail. The failure will be
	// recorded, but execution of dependent nodes (if any, though typically a failing
	// node stops its branch) might proceed if the graph logic allows.
	// Note: The current DAG engine propagates failure. This field might require
	// more sophisticated engine logic if it's to allow continuation past failure.
	// For now, it's more informational or for a future enhancement.
	AllowFailure bool `json:"allowFailure,omitempty" yaml:"allowFailure,omitempty"`
}

// The rest of the file (StepSpec, TaskSpec, ModuleSpec, PipelineSpec) represents
// a declarative model. While the current refactoring focuses on an active object model
// (Task, Module, Pipeline objects with Plan() methods), these Spec structures might still
// be useful for defining the configuration that these active objects consume, or for
// a future declarative execution pathway.
//
// For the immediate refactoring, StepMeta is the most relevant piece to be used by
// step.Step implementations. The DAG engine consumes step.Step instances directly.

// Removing the old Spec interfaces and structs for now to avoid confusion with the
// active planning model being implemented. If a declarative config loading mechanism
// is built on top of the active planners, these can be reintroduced or redesigned.
/*
import "github.com/mensylisir/kubexm/pkg/runtime"

// StepSpec is a marker interface for all concrete step specifications.
type StepSpec interface {
	GetName() string
}

type TaskSpec struct {
	Name string
	Steps []StepSpec
	RunOnRoles []string
	Filter func(host *runtime.Host) bool
	IgnoreError bool
	Concurrency int
}

type ModuleSpec struct {
	Name string
	Tasks []*TaskSpec
	IsEnabled func(clusterRt *runtime.ClusterRuntime) bool
	PreRun *TaskSpec
	PostRun *TaskSpec
}

type PipelineSpec struct {
	Name string
	Modules []*ModuleSpec
	PreRun *TaskSpec
	PostRun *TaskSpec
}
*/
