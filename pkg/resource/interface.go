package resource

import (
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/task"
)

type Handle interface {
	ID() string
	Path(ctx runtime.TaskContext) (string, error)
	EnsurePlan(ctx runtime.TaskContext) (*task.ExecutionFragment, error)
}
