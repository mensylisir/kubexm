package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// RenderConfigStep renders a configuration file from template and stores in context.
type RenderConfigStep struct {
	step.Base
	ConfigKey string // Context key to store rendered config
}

type RenderConfigStepBuilder struct {
	step.Builder[RenderConfigStepBuilder, *RenderConfigStep]
}

func NewRenderConfigStepBuilder(ctx runtime.ExecutionContext, instanceName, configKey string) *RenderConfigStepBuilder {
	s := &RenderConfigStep{
		ConfigKey: configKey,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Render config and store with key %s", instanceName, configKey)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(RenderConfigStepBuilder).Init(s)
}

func (s *RenderConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RenderConfigStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

// RenderContent renders the configuration content.
// Subclasses should override this method.
func (s *RenderConfigStep) RenderContent(ctx runtime.ExecutionContext) (string, error) {
	return "", fmt.Errorf("RenderContent not implemented for %s", s.Base.Meta.Name)
}

func (s *RenderConfigStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	content, err := s.RenderContent(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to render config content")
		return result, err
	}

	ctx.Export("task", s.ConfigKey, content)
	logger.Infof("Config rendered and stored with key %s", s.ConfigKey)
	result.MarkCompleted("Config rendered")
	return result, nil
}

func (s *RenderConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*RenderConfigStep)(nil)
