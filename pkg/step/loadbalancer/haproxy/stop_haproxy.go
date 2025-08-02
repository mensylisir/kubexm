package haproxy

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StopHAProxyStep struct {
	step.Base
}

func NewStopHAProxyStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *StopHAProxyStep] {
	s := &StopHAProxyStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Stop haproxy service"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *StopHAProxyStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Stopping haproxy service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl stop haproxy"); err != nil {
		return fmt.Errorf("failed to stop haproxy service: %w", err)
	}

	logger.Info("haproxy service stopped successfully.")
	return nil
}

func (s *StopHAProxyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warn("Rolling back stop action by starting haproxy service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl start haproxy"); err != nil {
		logger.Errorf("Failed to start haproxy during rollback: %v", err)
	}
	return nil
}

func (s *StopHAProxyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
