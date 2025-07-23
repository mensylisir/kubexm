package os

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DisableFirewallStep struct {
	step.Base
	// Add any necessary fields here
}

type DisableFirewallStepBuilder struct {
	step.Builder[DisableFirewallStepBuilder, *DisableFirewallStep]
}

func NewDisableFirewallStepBuilder(ctx runtime.Context, instanceName string) *DisableFirewallStepBuilder {
	s := &DisableFirewallStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable firewall", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(DisableFirewallStepBuilder).Init(s)
	return b
}

func (s *DisableFirewallStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableFirewallStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// Add precheck logic here
	return false, nil
}

func (s *DisableFirewallStep) Run(ctx runtime.ExecutionContext) error {
	// Add run logic here
	return nil
}

func (s *DisableFirewallStep) Rollback(ctx runtime.ExecutionContext) error {
	// Add rollback logic here
	return nil
}
