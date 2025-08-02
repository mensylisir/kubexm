package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DisableKeepalivedStep struct {
	step.Base
}

func NewDisableKeepalivedStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *DisableKeepalivedStep] {
	s := &DisableKeepalivedStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Disable keepalived service"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *DisableKeepalivedStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Disabling keepalived service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl disable keepalived"); err != nil {
		return fmt.Errorf("failed to disable keepalived service: %w", err)
	}

	logger.Info("keepalived service disabled successfully.")
	return nil
}

func (s *DisableKeepalivedStep) Rollback(ctx runtime.ExecutionContext) error {
	// Rollback for "disable" is "enable".
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warn("Rolling back by enabling keepalived service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl enable keepalived"); err != nil {
		logger.Errorf("Failed to enable keepalived during rollback: %v", err)
	}
	return nil
}

func (s *DisableKeepalivedStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
