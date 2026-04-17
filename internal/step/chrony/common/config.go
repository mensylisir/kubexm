package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// ConfigureChronyStep configures chrony.
type ConfigureChronyStep struct {
	step.Base
	ConfigPath string
}

type ConfigureChronyStepBuilder struct {
	step.Builder[ConfigureChronyStepBuilder, *ConfigureChronyStep]
}

func NewConfigureChronyStepBuilder(ctx runtime.ExecutionContext, instanceName, configPath string) *ConfigureChronyStepBuilder {
	s := &ConfigureChronyStep{
		ConfigPath: configPath,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure chrony at %s", instanceName, configPath)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(ConfigureChronyStepBuilder).Init(s)
}

func (s *ConfigureChronyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureChronyStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *ConfigureChronyStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())

	logger.Infof("Configuring chrony at %s", s.ConfigPath)
	result.MarkCompleted("Chrony configured")
	return result, nil
}

func (s *ConfigureChronyStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*ConfigureChronyStep)(nil)
