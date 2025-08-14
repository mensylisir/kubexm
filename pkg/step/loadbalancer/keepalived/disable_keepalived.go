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

type DisableKeepalivedStepBuilder struct {
	step.Builder[DisableKeepalivedStepBuilder, *DisableKeepalivedStep]
}

func NewDisableKeepalivedStepBuilder(ctx runtime.Context, instanceName string) *DisableKeepalivedStepBuilder {
	s := &DisableKeepalivedStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable Keepalived service from starting on boot", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	b := new(DisableKeepalivedStepBuilder).Init(s)
	return b
}

func (s *DisableKeepalivedStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableKeepalivedStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return false, fmt.Errorf("failed to gather facts to check service status: %w", err)
	}

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, keepalivedServiceName)
	if err != nil {
		logger.Warnf("Failed to check if Keepalived service is enabled, assuming it needs to be disabled. Error: %v", err)
		return false, nil
	}

	if !enabled {
		logger.Infof("Keepalived service is already disabled. Step is done.")
		return true, nil
	}

	logger.Info("Keepalived service is enabled. Step needs to run to disable it.")
	return false, nil
}

func (s *DisableKeepalivedStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return fmt.Errorf("failed to gather facts to disable service: %w", err)
	}

	logger.Infof("Disabling Keepalived service...")
	if err := runner.DisableService(ctx.GoContext(), conn, facts, keepalivedServiceName); err != nil {
		return fmt.Errorf("failed to disable Keepalived service: %w", err)
	}

	logger.Info("Keepalived service disabled successfully.")
	return nil
}

func (s *DisableKeepalivedStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name)
	logger.Info("Rollback is not applicable for a disable step.")
	return nil
}

var _ step.Step = (*DisableKeepalivedStep)(nil)
