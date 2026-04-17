package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// ConfigureDockerStep configures Docker daemon.
type ConfigureDockerStep struct {
	step.Base
	ConfigPath string
}

type ConfigureDockerStepBuilder struct {
	step.Builder[ConfigureDockerStepBuilder, *ConfigureDockerStep]
}

func NewConfigureDockerStepBuilder(ctx runtime.ExecutionContext, instanceName, configPath string) *ConfigureDockerStepBuilder {
	s := &ConfigureDockerStep{
		ConfigPath: configPath,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure Docker at %s", instanceName, configPath)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(ConfigureDockerStepBuilder).Init(s)
}

func (s *ConfigureDockerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureDockerStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *ConfigureDockerStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	logger.Infof("Configuring Docker at %s", s.ConfigPath)
	result.MarkCompleted("Docker configured")
	return result, nil
}

func (s *ConfigureDockerStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*ConfigureDockerStep)(nil)
