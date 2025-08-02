package nginx

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type EnableNginxStep struct {
	step.Base
}

func NewEnableNginxStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *EnableNginxStep] {
	s := &EnableNginxStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Enable nginx service"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *EnableNginxStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Enabling nginx service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl enable nginx"); err != nil {
		return fmt.Errorf("failed to enable nginx service: %w", err)
	}

	logger.Info("nginx service enabled successfully.")
	return nil
}

func (s *EnableNginxStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warn("Rolling back by disabling nginx service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl disable nginx"); err != nil {
		logger.Errorf("Failed to disable nginx during rollback: %v", err)
	}
	return nil
}

func (s *EnableNginxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
