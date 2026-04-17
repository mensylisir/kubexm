package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// RenderHelmValuesStep renders helm values file.
type RenderHelmValuesStep struct {
	step.Base
	ValuesKey string // Context key to store rendered values
}

type RenderHelmValuesStepBuilder struct {
	step.Builder[RenderHelmValuesStepBuilder, *RenderHelmValuesStep]
}

func NewRenderHelmValuesStepBuilder(ctx runtime.ExecutionContext, instanceName, valuesKey string) *RenderHelmValuesStepBuilder {
	s := &RenderHelmValuesStep{
		ValuesKey: valuesKey,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Render helm values with key %s", instanceName, valuesKey)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(RenderHelmValuesStepBuilder).Init(s)
}

func (s *RenderHelmValuesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RenderHelmValuesStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

// RenderValues renders helm values content.
// Subclasses should override this method.
func (s *RenderHelmValuesStep) RenderValues(ctx runtime.ExecutionContext) (string, error) {
	return "", fmt.Errorf("RenderValues not implemented for %s", s.Base.Meta.Name)
}

func (s *RenderHelmValuesStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	content, err := s.RenderValues(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to render helm values")
		return result, err
	}

	ctx.Export("task", s.ValuesKey, content)
	logger.Infof("Helm values rendered and stored with key %s", s.ValuesKey)
	result.MarkCompleted("Helm values rendered")
	return result, nil
}

func (s *RenderHelmValuesStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*RenderHelmValuesStep)(nil)
