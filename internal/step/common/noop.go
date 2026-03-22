package common

import (
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
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

func (s *NoOpStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	result.MarkCompleted("NoOp step completed successfully")
	return result, nil
}

func (s *NoOpStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*NoOpStep)(nil)
