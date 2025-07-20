package task

import (
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

type Task interface {
	Name() string
	Description() string
	IsRequired(ctx runtime.TaskContext) (bool, error)
	Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error)
	GetBase() *Base
}
