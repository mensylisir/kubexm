package chrony

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DisableChronyStep struct {
	step.Base
	ServiceName string
}

type DisableChronyStepBuilder struct {
	step.Builder[DisableChronyStepBuilder, *DisableChronyStep]
}

func NewDisableChronyStepBuilder(ctx runtime.Context, instanceName string) *DisableChronyStepBuilder {
	s := &DisableChronyStep{
		ServiceName: "chronyd.service",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Disable the chronyd service from starting on boot"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(DisableChronyStepBuilder).Init(s)
	return b
}

func (s *DisableChronyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableChronyStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for chronyd service disable...")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return false, fmt.Errorf("failed to gather host facts for precheck: %w", err)
	}

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, s.ServiceName)
	if err != nil {
		logger.Warnf("Failed to check if service '%s' is enabled, assuming it is. Error: %v", s.ServiceName, err)
		return false, nil
	}

	if !enabled {
		logger.Info("Precheck: Service is already disabled. Step is done.")
		return true, nil
	}

	logger.Info("Precheck passed: Service needs to be disabled.")
	return false, nil
}

func (s *DisableChronyStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return err
	}

	logger.Infof("Disabling service: %s", s.ServiceName)
	if err := runner.DisableService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		return fmt.Errorf("failed to disable service '%s': %w", s.ServiceName, err)
	}

	logger.Infof("Service '%s' disabled successfully.", s.ServiceName)
	return nil
}

func (s *DisableChronyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warnf("Rolling back by enabling service: %s", s.ServiceName)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Warnf("Failed to gather facts during rollback, cannot enable service. Error: %v", err)
		return nil
	}

	if err := runner.EnableService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logger.Errorf("Failed to enable service '%s' during rollback: %v", s.ServiceName, err)
	}

	logger.Info("Rollback: Enable signal sent to service.")
	return nil
}

var _ step.Step = (*DisableChronyStep)(nil)
