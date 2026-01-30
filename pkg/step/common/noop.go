package common

import (
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type NoOpStep struct {
	step.Base
}

type NoOpStepBuilder struct {
	step.Builder[NoOpStepBuilder, *NoOpStep]
}

func NewNoOpStepBuilder(instanceName, description string) *NoOpStepBuilder {
	s := &NoOpStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = description
	s.Base.Timeout = 1 * time.Minute

	b := new(NoOpStepBuilder).Init(s)
	return b
}

func (s *NoOpStep) Meta() *spec.StepMeta { return &s.Base.Meta }

func (s *NoOpStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return true, nil
}

func (s *NoOpStep) Run(ctx runtime.ExecutionContext) error {
	return nil
}

func (s *NoOpStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*NoOpStep)(nil)
