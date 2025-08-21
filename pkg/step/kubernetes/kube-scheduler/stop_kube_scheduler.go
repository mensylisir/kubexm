package kube_scheduler

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StopKubeSchedulerStep struct {
	step.Base
	ServiceName string
}

type StopKubeSchedulerStepBuilder struct {
	step.Builder[StopKubeSchedulerStepBuilder, *StopKubeSchedulerStep]
}

func NewStopKubeSchedulerStepBuilder(ctx runtime.Context, instanceName string) *StopKubeSchedulerStepBuilder {
	s := &StopKubeSchedulerStep{
		ServiceName: "kube-scheduler.service",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop kube-scheduler service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(StopKubeSchedulerStepBuilder).Init(s)
	return b
}

func (s *StopKubeSchedulerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StopKubeSchedulerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return false, err
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, s.ServiceName)
	if err != nil {
		logger.Warnf("Failed to check if service '%s' is active, assuming it is not. Error: %v", s.ServiceName, err)
		return false, nil
	}

	if !active {
		logger.Infof("Service '%s' is already inactive. Step is done.", s.ServiceName)
		return true, nil
	}

	logger.Infof("Service '%s' is active. Step needs to run.", s.ServiceName)
	return false, nil
}

func (s *StopKubeSchedulerStep) Run(ctx runtime.ExecutionContext) error {
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
		return fmt.Errorf("failed to stop service '%s' on host %s: %w", s.ServiceName, ctx.GetHost().GetName(), err)
	}

	logger.Infof("Service '%s' stopped successfully.", s.ServiceName)
	return nil
}

func (s *StopKubeSchedulerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Warnf("Failed to gather facts during rollback, cannot start service gracefully. Error: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by starting service: %s", s.ServiceName)
	if err := runner.StartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logger.Errorf("Failed to start service '%s' during rollback: %v", s.ServiceName, err)
	}

	return nil
}

var _ step.Step = (*StopKubeSchedulerStep)(nil)
