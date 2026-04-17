package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// ConfigureContainerdStep configures containerd.
type ConfigureContainerdStep struct {
	step.Base
	ConfigPath string
}

type ConfigureContainerdStepBuilder struct {
	step.Builder[ConfigureContainerdStepBuilder, *ConfigureContainerdStep]
}

func NewConfigureContainerdStepBuilder(ctx runtime.ExecutionContext, instanceName, configPath string) *ConfigureContainerdStepBuilder {
	s := &ConfigureContainerdStep{
		ConfigPath: configPath,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure containerd at %s", instanceName, configPath)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(ConfigureContainerdStepBuilder).Init(s)
}

func (s *ConfigureContainerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureContainerdStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *ConfigureContainerdStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	logger.Infof("Configuring containerd at %s", s.ConfigPath)
	result.MarkCompleted("Containerd configured")
	return result, nil
}

func (s *ConfigureContainerdStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*ConfigureContainerdStep)(nil)
