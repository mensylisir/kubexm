package os

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type UpdateEtcHostsStep struct {
	step.Base
	// Add any necessary fields here
}

type UpdateEtcHostsStepBuilder struct {
	step.Builder[UpdateEtcHostsStepBuilder, *UpdateEtcHostsStep]
}

func NewUpdateEtcHostsStepBuilder(ctx runtime.Context, instanceName string) *UpdateEtcHostsStepBuilder {
	s := &UpdateEtcHostsStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Update /etc/hosts", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(UpdateEtcHostsStepBuilder).Init(s)
	return b
}

func (s *UpdateEtcHostsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *UpdateEtcHostsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *UpdateEtcHostsStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *UpdateEtcHostsStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
