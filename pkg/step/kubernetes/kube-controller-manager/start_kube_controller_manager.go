package kube_controller_manager

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type StartKubeControllerManagerStep struct {
	step.Base
	ServiceName string
}

type StartKubeControllerManagerStepBuilder struct {
	step.Builder[StartKubeControllerManagerStepBuilder, *StartKubeControllerManagerStep]
}

func NewStartKubeControllerManagerStepBuilder(ctx runtime.Context, instanceName string) *StartKubeControllerManagerStepBuilder {
	s := &StartKubeControllerManagerStep{
		ServiceName: "kube-controller-manager.service",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Start kube-controller-manager service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(StartKubeControllerManagerStepBuilder).Init(s)
	return b
}

func (s *StartKubeControllerManagerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StartKubeControllerManagerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

	if active {
		logger.Infof("Service '%s' is already active. Step is done.", s.ServiceName)
		return true, nil
	}

	logger.Infof("Service '%s' is not active. Step needs to run.", s.ServiceName)
	return false, nil
}

func (s *StartKubeControllerManagerStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Starting service: %s", s.ServiceName)
	if err := runner.StartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		return fmt.Errorf("failed to start service '%s' on host %s: %w", s.ServiceName, ctx.GetHost().GetName(), err)
	}

	logger.Infof("Service '%s' started successfully.", s.ServiceName)
	return nil
}

func (s *StartKubeControllerManagerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Warnf("Failed to gather facts during rollback, cannot stop service gracefully. Error: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by stopping service: %s", s.ServiceName)
	if err := runner.StopService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logger.Errorf("Failed to stop service '%s' during rollback: %v", s.ServiceName, err)
	}

	return nil
}

var _ step.Step = (*StartKubeControllerManagerStep)(nil)
