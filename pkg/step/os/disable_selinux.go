package os

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DisableSelinuxStep struct {
	step.Base
	// Add any necessary fields here
}

type DisableSelinuxStepBuilder struct {
	step.Builder[DisableSelinuxStepBuilder, *DisableSelinuxStep]
}

func NewDisableSelinuxStepBuilder(ctx runtime.Context, instanceName string) *DisableSelinuxStepBuilder {
	s := &DisableSelinuxStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable SELinux", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(DisableSelinuxStepBuilder).Init(s)
	return b
}

func (s *DisableSelinuxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableSelinuxStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *DisableSelinuxStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *DisableSelinuxStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
