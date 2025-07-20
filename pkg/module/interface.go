package module

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
)

type Module interface {
	Name() string
	Description() string
	GetTasks(ctx runtime.ModuleContext) ([]task.Task, error)
	Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error)
	GetBase() *Base
}
