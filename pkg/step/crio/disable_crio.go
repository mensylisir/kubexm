package crio

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DisableCrioStep struct {
	step.Base
}

type DisableCrioStepBuilder struct {
	step.Builder[DisableCrioStepBuilder, *DisableCrioStep]
}

func NewDisableCrioStepBuilder(ctx runtime.Context, instanceName string) *DisableCrioStepBuilder {
	s := &DisableCrioStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable CRI-O systemd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(DisableCrioStepBuilder).Init(s)
	return b
}

func (s *DisableCrioStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableCrioStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return false, fmt.Errorf("failed to gather facts to check service status: %w", err)
	}

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, crioServiceName)
	if err != nil {
		logger.Infof("Failed to check if CRI-O service is enabled, assuming it's already disabled. Error: %v", err)
		return true, nil
	}

	if !enabled {
		logger.Infof("CRI-O service is already disabled. Step is done.")
		return true, nil
	}

	logger.Info("CRI-O service is enabled. Disable is required.")
	return false, nil
}

func (s *DisableCrioStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Disabling CRI-O service...")
	if err := runner.DisableService(ctx.GoContext(), conn, facts, crioServiceName); err != nil {
		return fmt.Errorf("failed to disable CRI-O service: %w", err)
	}

	logger.Info("CRI-O service disabled successfully.")
	return nil
}

func (s *DisableCrioStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by enabling CRI-O service")
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for rollback, cannot enable service: %v", err)
		return nil
	}

	if err := runner.EnableService(ctx.GoContext(), conn, facts, crioServiceName); err != nil {
		logger.Errorf("Failed to enable CRI-O service during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*DisableCrioStep)(nil)
