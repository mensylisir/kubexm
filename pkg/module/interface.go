package module

import (
	"github.com/mensylisir/kubexm/pkg/plan"    // Updated import path
	"github.com/mensylisir/kubexm/pkg/runtime" // Updated import path
	"github.com/mensylisir/kubexm/pkg/task"    // Updated import path
)

type Module interface {
	Name() string
	Tasks() []task.Task // Returns a list of tasks that belong to this module
	Plan(ctx runtime.ModuleContext) (*plan.ExecutionPlan, error)
}
