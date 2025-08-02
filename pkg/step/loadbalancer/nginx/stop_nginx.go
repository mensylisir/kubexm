package nginx

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StopNginxStep struct {
	step.Base
}

func NewStopNginxStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *StopNginxStep] {
	s := &StopNginxStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Stop nginx service"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *StopNginxStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Stopping nginx service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl stop nginx"); err != nil {
		return fmt.Errorf("failed to stop nginx service: %w", err)
	}

	logger.Info("nginx service stopped successfully.")
	return nil
}

func (s *StopNginxStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warn("Rolling back stop action by starting nginx service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl start nginx"); err != nil {
		logger.Errorf("Failed to start nginx during rollback: %v", err)
	}
	return nil
}

func (s *StopNginxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
