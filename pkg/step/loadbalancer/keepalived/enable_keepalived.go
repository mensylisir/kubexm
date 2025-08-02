package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type EnableKeepalivedStep struct {
	step.Base
}

func NewEnableKeepalivedStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *EnableKeepalivedStep] {
	s := &EnableKeepalivedStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Enable keepalived service"
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

func (s *EnableKeepalivedStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Enabling keepalived service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl enable keepalived"); err != nil {
		return fmt.Errorf("failed to enable keepalived service: %w", err)
	}

	logger.Info("keepalived service enabled successfully.")
	return nil
}

func (s *EnableKeepalivedStep) Rollback(ctx runtime.ExecutionContext) error {
	// Rollback for "enable" is "disable".
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warn("Rolling back by disabling keepalived service...")
	if _, err := runner.SudoExec(ctx.GoContext(), conn, "systemctl disable keepalived"); err != nil {
		logger.Errorf("Failed to disable keepalived during rollback: %v", err)
	}
	return nil
}

func (s *EnableKeepalivedStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
