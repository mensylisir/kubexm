package module

import (
	"github.com/mensylisir/kubexm/pkg/task"
)

// Module defines the methods that all concrete module types must implement.
// Modules are responsible for planning a larger ExecutionFragment by orchestrating
// and linking multiple Task ExecutionFragments.
type Module interface {
	// Name returns the designated name of the module.
	Name() string

	// Description returns a brief description of the module.
	Description() string

	// GetTasks returns a list of tasks that belong to this module.
	GetTasks(ctx ModuleContext) ([]task.Task, error)

	// Plan aggregates ExecutionFragments from its tasks into a larger ExecutionFragment.
	Plan(ctx ModuleContext) (*task.ExecutionFragment, error)
}
