package crio

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StopCrioStep struct {
	step.Base
}

type StopCrioStepBuilder struct {
	step.Builder[StopCrioStepBuilder, *StopCrioStep]
}

func NewStopCrioStepBuilder(ctx runtime.Context, instanceName string) *StopCrioStepBuilder {
	s := &StopCrioStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop CRI-O systemd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(StopCrioStepBuilder).Init(s)
	return b
}

func (s *StopCrioStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StopCrioStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, crioServiceName)
	if err != nil {
		logger.Infof("Failed to check if CRI-O service is active, assuming it's already stopped. Error: %v", err)
		return true, nil
	}

	if !active {
		logger.Infof("CRI-O service is already stopped. Step is done.")
		return true, nil
	}

	logger.Info("CRI-O service is active. Stop is required.")
	return false, nil
}

func (s *StopCrioStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return fmt.Errorf("failed to gather facts to stop service: %w", err)
	}

	logger.Infof("Stopping CRI-O service...")
	if err := runner.StopService(ctx.GoContext(), conn, facts, crioServiceName); err != nil {
		return fmt.Errorf("failed to stop CRI-O service: %w", err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, crioServiceName)
	if err != nil {
		return fmt.Errorf("failed to verify cri-o service status after stopping: %w", err)
	}
	if active {
		return fmt.Errorf("CRI-O service did not become inactive after stop command")
	}

	logger.Info("CRI-O service stopped successfully.")
	return nil
}

func (s *StopCrioStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by starting CRI-O service")
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for rollback, cannot start service: %v", err)
		return nil
	}

	if err := runner.StartService(ctx.GoContext(), conn, facts, crioServiceName); err != nil {
		logger.Errorf("Failed to start CRI-O service during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*StopCrioStep)(nil)
