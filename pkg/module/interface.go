package module

import (
	// "github.com/mensylisir/kubexm/pkg/plan" // No longer directly returns ExecutionPlan
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task" // Uses task.Task and task.ExecutionFragment
)

// Module defines the methods that all concrete module types must implement.
// Modules are responsible for planning a larger ExecutionFragment by orchestrating
// and linking multiple Task ExecutionFragments.
type Module interface {
	// Name returns the designated name of the module.
	Name() string

	// Tasks returns a list of tasks that belong to this module.
	// This might still be useful for introspection or if the module's Plan method
	// dynamically decides which tasks to include based on some logic.
	// Alternatively, tasks could be hardcoded within the module's Plan method.
	// Keeping it for now as per original design.
	Tasks() []task.Task

	// Plan now aggregates ExecutionFragments from its tasks into a larger ExecutionFragment.
	// It is responsible for linking the exit nodes of one task's fragment
	// to the entry nodes of the next task's fragment, creating dependencies.
	Plan(ctx runtime.ModuleContext) (*task.ExecutionFragment, error)
}
