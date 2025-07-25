package kube_controller_manager

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type EnableKubeControllerManagerStep struct {
	step.Base
	ServiceName string
}

type EnableKubeControllerManagerStepBuilder struct {
	step.Builder[EnableKubeControllerManagerStepBuilder, *EnableKubeControllerManagerStep]
}

func NewEnableKubeControllerManagerStepBuilder(ctx runtime.Context, instanceName string) *EnableKubeControllerManagerStepBuilder {
	s := &EnableKubeControllerManagerStep{
		ServiceName: "kube-controller-manager.service",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Enable kube-controller-manager service to start on boot", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(EnableKubeControllerManagerStepBuilder).Init(s)
	return b
}

func (s *EnableKubeControllerManagerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EnableKubeControllerManagerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return false, fmt.Errorf("failed to gather facts on host %s: %w", ctx.GetHost().GetName(), err)
	}

	enabled, err := runner.IsServiceEnabled(ctx.GoContext(), conn, facts, s.ServiceName)
	if err != nil {
		logger.Warnf("Failed to check if service '%s' is enabled, assuming it is not. Error: %v", s.ServiceName, err)
		return false, nil
	}

	if enabled {
		logger.Infof("Service '%s' is already enabled. Step is done.", s.ServiceName)
		return true, nil
	}

	logger.Infof("Service '%s' is not enabled. Step needs to run.", s.ServiceName)
	return false, nil
}

func (s *EnableKubeControllerManagerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return fmt.Errorf("failed to gather facts on host %s before enabling service: %w", ctx.GetHost().GetName(), err)
	}

	logger.Infof("Enabling service: %s", s.ServiceName)
	if err := runner.EnableService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		return fmt.Errorf("failed to enable service '%s' on host %s: %w", s.ServiceName, ctx.GetHost().GetName(), err)
	}

	logger.Infof("Service '%s' enabled successfully.", s.ServiceName)
	return nil
}

func (s *EnableKubeControllerManagerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Warnf("Failed to gather facts during rollback, cannot disable service gracefully. Error: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by disabling service: %s", s.ServiceName)
	if err := runner.DisableService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logger.Errorf("Failed to disable service '%s' during rollback: %v", s.ServiceName, err)
	}

	return nil
}

var _ step.Step = (*EnableKubeControllerManagerStep)(nil)
