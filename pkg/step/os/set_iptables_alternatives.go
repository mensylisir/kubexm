package os

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type SetIptablesAlternativesStep struct {
	step.Base
	// Add any necessary fields here
}

type SetIptablesAlternativesStepBuilder struct {
	step.Builder[SetIptablesAlternativesStepBuilder, *SetIptablesAlternativesStep]
}

func NewSetIptablesAlternativesStepBuilder(ctx runtime.Context, instanceName string) *SetIptablesAlternativesStepBuilder {
	s := &SetIptablesAlternativesStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Set iptables alternatives", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(SetIptablesAlternativesStepBuilder).Init(s)
	return b
}

func (s *SetIptablesAlternativesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *SetIptablesAlternativesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *SetIptablesAlternativesStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *SetIptablesAlternativesStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
