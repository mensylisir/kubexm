package helm

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type InstallHelmStep struct {
	step.Base
	// Add any necessary fields here
}

type InstallHelmStepBuilder struct {
	step.Builder[InstallHelmStepBuilder, *InstallHelmStep]
}

func NewInstallHelmStepBuilder(ctx runtime.Context, instanceName string) *InstallHelmStepBuilder {
	s := &InstallHelmStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install Helm", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallHelmStepBuilder).Init(s)
	return b
}

func (s *InstallHelmStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallHelmStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *InstallHelmStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *InstallHelmStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
