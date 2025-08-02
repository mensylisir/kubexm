package haproxy

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type EnableHAProxyStep struct {
	step.Base
}

func NewEnableHAProxyStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *EnableHAProxyStep] {
	s := &EnableHAProxyStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Enable haproxy service"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *EnableHAProxyStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Enabling haproxy service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl enable haproxy"); err != nil {
		return fmt.Errorf("failed to enable haproxy service: %w", err)
	}

	logger.Info("haproxy service enabled successfully.")
	return nil
}

func (s *EnableHAProxyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warn("Rolling back by disabling haproxy service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl disable haproxy"); err != nil {
		logger.Errorf("Failed to disable haproxy during rollback: %v", err)
	}
	return nil
}

func (s *EnableHAProxyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
