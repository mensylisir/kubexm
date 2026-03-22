package module

import (
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/task"
)

type Module interface {
	Name() string
	Description() string
	Tasks() []task.Task
	Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error)
	GetBase() *BaseModule
}
