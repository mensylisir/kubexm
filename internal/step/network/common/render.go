package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// RenderNetworkConfigStep renders network plugin configuration.
type RenderNetworkConfigStep struct {
	step.Base
	ConfigKey string // Context key to store rendered config
}

type RenderNetworkConfigStepBuilder struct {
	step.Builder[RenderNetworkConfigStepBuilder, *RenderNetworkConfigStep]
}

func NewRenderNetworkConfigStepBuilder(ctx runtime.ExecutionContext, instanceName, configKey string) *RenderNetworkConfigStepBuilder {
	s := &RenderNetworkConfigStep{
		ConfigKey: configKey,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Render network config and store with key %s", instanceName, configKey)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(RenderNetworkConfigStepBuilder).Init(s)
}

func (s *RenderNetworkConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RenderNetworkConfigStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

// RenderContent renders the configuration content.
// Subclasses should override this method.
func (s *RenderNetworkConfigStep) RenderContent(ctx runtime.ExecutionContext) (string, error) {
	return "", fmt.Errorf("RenderContent not implemented for %s", s.Base.Meta.Name)
}

func (s *RenderNetworkConfigStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	content, err := s.RenderContent(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to render config content")
		return result, err
	}

	ctx.Export("task", s.ConfigKey, content)
	logger.Infof("Network config rendered and stored with key %s", s.ConfigKey)
	result.MarkCompleted("Network config rendered")
	return result, nil
}

func (s *RenderNetworkConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*RenderNetworkConfigStep)(nil)
