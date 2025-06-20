package task

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

// Interface defines the methods that all concrete task types must implement.
// Tasks are responsible for planning a series of actions (steps on hosts)
// to achieve a specific part of a module's goal.
type Interface interface {
	// Name returns the designated name of the task.
	Name() string

	// Description provides a brief summary of what the task does.
	Description() string

	// IsRequired determines if the task needs to generate a plan.
	// This can be based on the current system state (via TaskContext)
	// or configuration.
	IsRequired(ctx runtime.TaskContext) (bool, error)

	// Plan generates an ExecutionPlan for this task.
	// It uses the TaskContext to access cluster configuration, host information,
	// facts, and to instantiate necessary Step objects.
	Plan(ctx runtime.TaskContext) (*plan.ExecutionPlan, error)
}
