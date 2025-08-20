package chrony

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type EnableChronyStep struct {
	step.Base
	ServiceName string
}

type EnableChronyStepBuilder struct {
	step.Builder[EnableChronyStepBuilder, *EnableChronyStep]
}

func NewEnableChronyStepBuilder(ctx runtime.Context, instanceName string) *EnableChronyStepBuilder {
	s := &EnableChronyStep{
		ServiceName: "chronyd.service",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Enable the chronyd service to start on boot"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(EnableChronyStepBuilder).Init(s)
	return b
}

func (s *EnableChronyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EnableChronyStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for chronyd service enable...")

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
		logger.Warnf("Failed to check if service '%s' is enabled, assuming it is not. Error: %v", s.ServiceName, err)
		return false, nil
	}

	if enabled {
		logger.Info("Precheck: Service is already enabled. Step is done.")
		return true, nil
	}

	logger.Info("Precheck passed: Service needs to be enabled.")
	return false, nil
}

func (s *EnableChronyStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Enabling service: %s", s.ServiceName)
	if err := runner.EnableService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		return fmt.Errorf("failed to enable service '%s': %w", s.ServiceName, err)
	}

	logger.Infof("Service '%s' enabled successfully.", s.ServiceName)
	return nil
}

func (s *EnableChronyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warnf("Rolling back by disabling service: %s", s.ServiceName)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Warnf("Failed to gather facts during rollback, cannot disable service. Error: %v", err)
		return nil
	}

	if err := runner.DisableService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logger.Errorf("Failed to disable service '%s' during rollback: %v", s.ServiceName, err)
	}

	logger.Info("Rollback: Disable signal sent to service.")
	return nil
}

var _ step.Step = (*EnableChronyStep)(nil)
