package step

import (
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
)

// Step defines an atomic, reusable execution unit.
type Step interface {
	Name() string
	Description() string
	Precheck(ctx runtime.StepContext, host connector.Host) (bool, error)
	Run(ctx runtime.StepContext, host connector.Host) error
	Rollback(ctx runtime.StepContext, host connector.Host) error
}
