package step

import (
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
)

type Step interface {
	Meta() *spec.StepMeta
	Precheck(ctx runtime.ExecutionContext) (isDone bool, err error)
	Run(ctx runtime.ExecutionContext) error
	Rollback(ctx runtime.ExecutionContext) error
}
