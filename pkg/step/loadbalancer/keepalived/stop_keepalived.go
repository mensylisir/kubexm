package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StopKeepalivedStep struct {
	step.Base
}

func NewStopKeepalivedStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *StopKeepalivedStep] {
	s := &StopKeepalivedStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Stop keepalived service"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *StopKeepalivedStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Stopping keepalived service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl stop keepalived"); err != nil {
		return fmt.Errorf("failed to stop keepalived service: %w", err)
	}

	logger.Info("keepalived service stopped successfully.")
	return nil
}

func (s *StopKeepalivedStep) Rollback(ctx runtime.ExecutionContext) error {
	// If we are rolling back a "stop" action, we should try to start the service again.
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warn("Rolling back stop action by starting keepalived service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl start keepalived"); err != nil {
		logger.Errorf("Failed to start keepalived during rollback: %v", err)
	}
	return nil
}

func (s *StopKeepalivedStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
