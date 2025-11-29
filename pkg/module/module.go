package module

import (
	"time"

	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/task"
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
