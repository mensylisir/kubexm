package task

import (
	"fmt"
	// "context" // No longer directly needed by BaseTask if Run is gone

	"github.com/mensylisir/kubexm/pkg/connector" // For connector.Host in Filter
	"github.com/mensylisir/kubexm/pkg/plan"      // For plan.ExecutionPlan
	"github.com/mensylisir/kubexm/pkg/runner"    // For runner.Facts in Filter
	"github.com/mensylisir/kubexm/pkg/runtime"   // For runtime.TaskContext
	// Unused imports like sync, time, errgroup, spec, step, logger will be removed
)

// BaseTask provides a common structure for tasks.
// Concrete task types should embed BaseTask and implement the Plan method,
// and optionally override other methods from the Interface.
type BaseTask struct {
	TaskName   string // Renamed from Name to avoid conflict with Name() method if embedding
	TaskDesc   string // Optional description
	RunOnRoles []string
	// HostFilter allows for more granular host selection based on host properties and facts.
	// This filter is typically applied by the concrete Task's Plan method.
	HostFilter func(host connector.Host, facts *runner.Facts) bool
	// IgnoreError indicates if an error from this task's Plan method, or subsequently
	// from its execution by the Engine, should halt the parent Module's planning/execution.
	IgnoreError bool
}

// NewBaseTask creates a basic task structure.
// Concrete tasks will call this and then set their specific step generation logic.
func NewBaseTask(name string, desc string, roles []string, filter func(connector.Host, *runner.Facts) bool, ignoreError bool) BaseTask {
	return BaseTask{
		TaskName:    name,
		TaskDesc:    desc,
		RunOnRoles:  roles,
		HostFilter:  filter,
		IgnoreError: ignoreError,
	}
}

// Name returns the name of the task.
func (bt *BaseTask) Name() string {
	return bt.TaskName
}

// Description returns a brief summary of the task.
func (bt *BaseTask) Description() string {
	if bt.TaskDesc != "" {
		return bt.TaskDesc
	}
	return fmt.Sprintf("Task: %s", bt.TaskName)
}

// IsRequired determines if the task's plan should be generated.
// Default implementation returns true. Concrete tasks can override this.
func (bt *BaseTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

// Plan is intended to be implemented by concrete task types.
// BaseTask.Plan returns an error indicating it's not implemented.
func (bt *BaseTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionPlan, error) {
	return nil, fmt.Errorf("Plan() not implemented for task: %s. Concrete task types must implement this method", bt.Name())
}

// Ensure BaseTask itself doesn't fully satisfy the interface if Plan is meant to be abstract,
// but methods it does implement are helpful.
// A concrete task struct will embed BaseTask and add its own Plan method.
// var _ Interface = (*SomeConcreteTask)(nil) // This would be in the concrete task's file
