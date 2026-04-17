package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// ConfirmStep waits for user confirmation to proceed.
type ConfirmStep struct {
	step.Base
	Message string
}

type ConfirmStepBuilder struct {
	step.Builder[ConfirmStepBuilder, *ConfirmStep]
}

func NewConfirmStepBuilder(ctx runtime.ExecutionContext, instanceName, message string) *ConfirmStepBuilder {
	s := &ConfirmStep{
		Message: message,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Confirm: %s", instanceName, message)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 24 * time.Hour // Wait indefinitely for user input
	return new(ConfirmStepBuilder).Init(s)
}

func (s *ConfirmStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfirmStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *ConfirmStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	logger.Info("Waiting for user confirmation...")
	result.MarkCompleted("Confirmed")
	return result, nil
}

func (s *ConfirmStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*ConfirmStep)(nil)
