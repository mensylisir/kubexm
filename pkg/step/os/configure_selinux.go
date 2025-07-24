package os

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type ConfigureSelinuxStep struct {
	step.Base
	// Add any necessary fields here
}

type ConfigureSelinuxStepBuilder struct {
	step.Builder[ConfigureSelinuxStepBuilder, *ConfigureSelinuxStep]
}

func NewConfigureSelinuxStepBuilder(ctx runtime.Context, instanceName string) *ConfigureSelinuxStepBuilder {
	s := &ConfigureSelinuxStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure SELinux", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(ConfigureSelinuxStepBuilder).Init(s)
	return b
}

func (s *ConfigureSelinuxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureSelinuxStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *ConfigureSelinuxStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *ConfigureSelinuxStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
