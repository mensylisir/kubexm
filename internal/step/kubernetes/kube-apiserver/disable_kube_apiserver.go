package kube_apiserver

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type DisableKubeApiServerStep struct {
	step.Base
	ServiceName string
}

type DisableKubeApiServerStepBuilder struct {
	step.Builder[DisableKubeApiServerStepBuilder, *DisableKubeApiServerStep]
}

func NewDisableKubeApiServerStepBuilder(ctx runtime.ExecutionContext, instanceName string) *DisableKubeApiServerStepBuilder {
	s := &DisableKubeApiServerStep{
		ServiceName: "kube-apiserver.service",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Disable kube-apiserver service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(DisableKubeApiServerStepBuilder).Init(s)
	return b
}

func (s *DisableKubeApiServerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DisableKubeApiServerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		logger.Warnf("Failed to check if service is enabled, assuming it's disabled. Error: %v", err)
		return true, nil
	}

	if !enabled {
		logger.Infof("KubeApiServer service is already disabled. Step is done.")
		return true, nil
	}

	logger.Info("KubeApiServer service is currently enabled. Step needs to run.")
	return false, nil
}

func (s *DisableKubeApiServerStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		result.MarkFailed(err, "failed to gather facts")
		return result, err
	}

	logger.Infof("Disabling kube-apiserver service...")
	if err := runner.DisableService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		err = fmt.Errorf("failed to disable kube-apiserver service: %w", err)
		result.MarkFailed(err, "failed to disable service")
		return result, err
	}

	logger.Info("KubeApiServer service disabled successfully.")
	result.MarkCompleted("service disabled successfully")
	return result, nil
}

func (s *DisableKubeApiServerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for rollback, cannot enable service: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by enabling kube-apiserver service...")
	if err := runner.EnableService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logger.Errorf("Failed to enable kube-apiserver service during rollback: %v", err)
	}

	return nil
}
