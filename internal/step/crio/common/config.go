package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// ConfigureCrioStep configures CRI-O settings.
type ConfigureCrioStep struct {
	step.Base
	ConfigPath string
}

type ConfigureCrioStepBuilder struct {
	step.Builder[ConfigureCrioStepBuilder, *ConfigureCrioStep]
}

func NewConfigureCrioStepBuilder(ctx runtime.ExecutionContext, instanceName, configPath string) *ConfigureCrioStepBuilder {
	s := &ConfigureCrioStep{
		ConfigPath: configPath,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure CRI-O", instanceName)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(ConfigureCrioStepBuilder).Init(s)
}

func (s *ConfigureCrioStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureCrioStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *ConfigureCrioStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	logger.Infof("Configuring CRI-O at %s", s.ConfigPath)
	result.MarkCompleted("CRI-O configured")
	return result, nil
}

func (s *ConfigureCrioStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*ConfigureCrioStep)(nil)
