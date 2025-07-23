package os

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type ApplySecurityLimitsStep struct {
	step.Base
	// Add any necessary fields here
}

type ApplySecurityLimitsStepBuilder struct {
	step.Builder[ApplySecurityLimitsStepBuilder, *ApplySecurityLimitsStep]
}

func NewApplySecurityLimitsStepBuilder(ctx runtime.Context, instanceName string) *ApplySecurityLimitsStepBuilder {
	s := &ApplySecurityLimitsStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Apply security limits", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(ApplySecurityLimitsStepBuilder).Init(s)
	return b
}

func (s *ApplySecurityLimitsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ApplySecurityLimitsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *ApplySecurityLimitsStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *ApplySecurityLimitsStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
