package os

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type ConfigureSysctlStep struct {
	step.Base
	// Add any necessary fields here
}

type ConfigureSysctlStepBuilder struct {
	step.Builder[ConfigureSysctlStepBuilder, *ConfigureSysctlStep]
}

func NewConfigureSysctlStepBuilder(ctx runtime.Context, instanceName string) *ConfigureSysctlStepBuilder {
	s := &ConfigureSysctlStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure sysctl", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(ConfigureSysctlStepBuilder).Init(s)
	return b
}

func (s *ConfigureSysctlStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureSysctlStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *ConfigureSysctlStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *ConfigureSysctlStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
