package haproxy

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DisableHAProxyStep struct {
	step.Base
}

func NewDisableHAProxyStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *DisableHAProxyStep] {
	s := &DisableHAProxyStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Disable haproxy service"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *DisableHAProxyStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Disabling haproxy service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl disable haproxy"); err != nil {
		return fmt.Errorf("failed to disable haproxy service: %w", err)
	}

	logger.Info("haproxy service disabled successfully.")
	return nil
}

func (s *DisableHAProxyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warn("Rolling back by enabling haproxy service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl enable haproxy"); err != nil {
		logger.Errorf("Failed to enable haproxy during rollback: %v", err)
	}
	return nil
}

func (s *DisableHAProxyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
