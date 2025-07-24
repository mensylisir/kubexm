package network

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type InstallCniPluginsStep struct {
	step.Base
	// Add any necessary fields here
}

type InstallCniPluginsStepBuilder struct {
	step.Builder[InstallCniPluginsStepBuilder, *InstallCniPluginsStep]
}

func NewInstallCniPluginsStepBuilder(ctx runtime.Context, instanceName string) *InstallCniPluginsStepBuilder {
	s := &InstallCniPluginsStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install CNI plugins", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallCniPluginsStepBuilder).Init(s)
	return b
}

func (s *InstallCniPluginsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallCniPluginsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *InstallCniPluginsStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *InstallCniPluginsStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
