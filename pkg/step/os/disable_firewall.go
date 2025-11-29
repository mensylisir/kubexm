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
}

type DisableFirewallStepBuilder struct {
	step.Builder[DisableFirewallStepBuilder, *DisableFirewallStep]
}

func NewDisableFirewallStepBuilder(ctx runtime.ExecutionContext, instanceName string) *DisableFirewallStepBuilder {
	s := &DisableFirewallStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s] >> Disable Firewall", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(DisableFirewallStepBuilder).Init(s)
	return b
}

func (s *DisableFirewallStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableFirewallStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	// Check if firewalld is active
	output, err := runner.Run(ctx.GoContext(), conn, "systemctl is-active firewalld", s.Sudo)
	if err != nil {
		// If command fails, it might mean firewalld is not installed or service not found, which is fine.
		// Or it returns non-zero exit code if not active.
		logger.Debugf("systemctl is-active firewalld returned error (likely not active): %v", err)
		return true, nil
	}

	if output == "active" {
		logger.Info("Firewalld is active, needs to be disabled.")
		return false, nil
	}

	logger.Info("Firewalld is not active.")
	return true, nil
}

func (s *DisableFirewallStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	cmds := []string{
		"systemctl stop firewalld",
		"systemctl disable firewalld",
	}

	for _, cmd := range cmds {
		logger.Infof("Executing: %s", cmd)
		if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
			// Ignore error if service doesn't exist
			logger.Warnf("Failed to execute %s: %v (ignoring)", cmd, err)
		}
	}

	logger.Info("Firewall disabled successfully.")
	return nil
}

func (s *DisableFirewallStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Rolling back: Enabling firewalld...")
	cmds := []string{
		"systemctl enable firewalld",
		"systemctl start firewalld",
	}

	for _, cmd := range cmds {
		if _, err := runner.Run(ctx.GoContext(), conn, cmd, s.Sudo); err != nil {
			logger.Warnf("Failed to execute %s during rollback: %v", cmd, err)
		}
	}

	return nil
}

var _ step.Step = (*DisableFirewallStep)(nil)
