package nginx

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DisableNginxStep struct {
	step.Base
}

func NewDisableNginxStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *DisableNginxStep] {
	s := &DisableNginxStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Disable nginx service"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *DisableNginxStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Disabling nginx service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl disable nginx"); err != nil {
		return fmt.Errorf("failed to disable nginx service: %w", err)
	}

	logger.Info("nginx service disabled successfully.")
	return nil
}

func (s *DisableNginxStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warn("Rolling back by enabling nginx service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl enable nginx"); err != nil {
		logger.Errorf("Failed to enable nginx during rollback: %v", err)
	}
	return nil
}

func (s *DisableNginxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
