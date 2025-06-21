package module

import (
	// Adjust these import paths based on your actual project structure
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
)

type Module interface {
	Name() string
	Tasks() []task.Task // Returns a slice of task.Task interfaces
	Plan(ctx runtime.ModuleContext) (*plan.ExecutionPlan, error)
}
