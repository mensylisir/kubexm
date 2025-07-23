package os

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DisableSwapStep struct {
	step.Base
	// Add any necessary fields here
}

type DisableSwapStepBuilder struct {
	step.Builder[DisableSwapStepBuilder, *DisableSwapStep]
}

func NewDisableSwapStepBuilder(ctx runtime.Context, instanceName string) *DisableSwapStepBuilder {
	s := &DisableSwapStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable swap", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(DisableSwapStepBuilder).Init(s)
	return b
}

func (s *DisableSwapStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableSwapStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *DisableSwapStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *DisableSwapStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
