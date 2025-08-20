package chrony

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StopChronyStep struct {
	step.Base
	ServiceName string
}

type StopChronyStepBuilder struct {
	step.Builder[StopChronyStepBuilder, *StopChronyStep]
}

func NewStopChronyStepBuilder(ctx runtime.Context, instanceName string) *StopChronyStepBuilder {
	s := &StopChronyStep{
		ServiceName: "chronyd.service",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Stop the chronyd service"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(StopChronyStepBuilder).Init(s)
	return b
}

func (s *StopChronyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StopChronyStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for chronyd service stop...")

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return false, fmt.Errorf("failed to gather host facts for precheck: %w", err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, s.ServiceName)
	if err != nil {
		logger.Warnf("Failed to check if service '%s' is active, assuming it is. Error: %v", s.ServiceName, err)
		return false, nil
	}

	if !active {
		logger.Info("Precheck: Service is already inactive. Step is done.")
		return true, nil
	}

	logger.Info("Precheck passed: Service is active and needs to be stopped.")
	return false, nil
}

func (s *StopChronyStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Stopping service: %s", s.ServiceName)
	if err := runner.StopService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		return fmt.Errorf("failed to stop service '%s': %w", s.ServiceName, err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, s.ServiceName)
	if err != nil {
		return fmt.Errorf("failed to verify service status for %s after stopping: %w", s.ServiceName, err)
	}
	if active {
		return fmt.Errorf("service %s did not become inactive after stop command", s.ServiceName)
	}

	logger.Infof("Service '%s' stopped successfully.", s.ServiceName)
	return nil
}

func (s *StopChronyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warnf("Rolling back by starting service: %s", s.ServiceName)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Warnf("Failed to gather facts during rollback, cannot start service. Error: %v", err)
		return nil
	}

	if err := runner.StartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logger.Errorf("Failed to start service '%s' during rollback: %v", s.ServiceName, err)
	}

	logger.Info("Rollback: Start signal sent to service.")
	return nil
}

var _ step.Step = (*StopChronyStep)(nil)
