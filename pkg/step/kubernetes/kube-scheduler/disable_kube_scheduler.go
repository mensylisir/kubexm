package kube_scheduler

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DisableKubeSchedulerStep struct {
	step.Base
	ServiceName string
}

type DisableKubeSchedulerStepBuilder struct {
	step.Builder[DisableKubeSchedulerStepBuilder, *DisableKubeSchedulerStep]
}

func NewDisableKubeSchedulerStepBuilder(ctx runtime.Context, instanceName string) *DisableKubeSchedulerStepBuilder {
	s := &DisableKubeSchedulerStep{
		ServiceName: "kube-scheduler.service",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable kube-scheduler service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(DisableKubeSchedulerStepBuilder).Init(s)
	return b
}

func (s *DisableKubeSchedulerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableKubeSchedulerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, s.ServiceName)
	if err != nil {
		logger.Warnf("Failed to check if service '%s' is enabled, assuming it is. Error: %v", s.ServiceName, err)
		return false, nil
	}

	if !enabled {
		logger.Infof("Service '%s' is already disabled. Step is done.", s.ServiceName)
		return true, nil
	}

	logger.Infof("Service '%s' is enabled. Step needs to run.", s.ServiceName)
	return false, nil
}

func (s *DisableKubeSchedulerStep) Run(ctx runtime.ExecutionContext) error {
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
		return fmt.Errorf("failed to disable service '%s' on host %s: %w", s.ServiceName, ctx.GetHost().GetName(), err)
	}

	logger.Infof("Service '%s' disabled successfully.", s.ServiceName)
	return nil
}

func (s *DisableKubeSchedulerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Warnf("Failed to gather facts during rollback, cannot enable service gracefully. Error: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by enabling service: %s", s.ServiceName)
	if err := runner.EnableService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logger.Errorf("Failed to enable service '%s' during rollback: %v", s.ServiceName, err)
	}

	return nil
}

var _ step.Step = (*DisableKubeSchedulerStep)(nil)
