package module

import (
	"time"

	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/task"
)

type BaseModule struct {
	Meta        spec.ModuleMeta
	Timeout     time.Duration
	IgnoreError bool
	ModuleTasks []task.Task
}

func NewBaseModule(name string, tasks []task.Task) BaseModule {
	return BaseModule{
		Meta: spec.ModuleMeta{
			Name: name,
		},
		ModuleTasks: tasks,
	}
}

func (b *BaseModule) Name() string {
	return b.Meta.Name
}

func (b *BaseModule) Description() string {
	return b.Meta.Description
}

func (b *BaseModule) Tasks() []task.Task {
	return b.ModuleTasks
}

func (b *BaseModule) GetBase() *BaseModule {
	return b
}

func (b *BaseModule) Plan(ctx runtime.ModuleContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(b.Name())
	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
