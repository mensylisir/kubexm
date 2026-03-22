package kube_apiserver

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type StopKubeApiServerStep struct {
	step.Base
	ServiceName string
}

type StopKubeApiServerStepBuilder struct {
	step.Builder[StopKubeApiServerStepBuilder, *StopKubeApiServerStep]
}

func NewStopKubeApiServerStepBuilder(ctx runtime.ExecutionContext, instanceName string) *StopKubeApiServerStepBuilder {
	s := &StopKubeApiServerStep{
		ServiceName: "kube-apiserver.service",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop kube-apiserver service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(StopKubeApiServerStepBuilder).Init(s)
	return b
}

func (s *StopKubeApiServerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StopKubeApiServerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		logger.Warnf("Failed to check service status, assuming it's not active. Error: %v", err)
		return true, nil
	}

	if !active {
		logger.Infof("KubeApiServer service is already inactive. Step is done.")
		return true, nil
	}

	logger.Info("KubeApiServer service is currently active. Step needs to run.")
	return false, nil
}

func (s *StopKubeApiServerStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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

	logger.Infof("Stopping kube-apiserver service...")
	if err := runner.StopService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		err = fmt.Errorf("failed to stop kube-apiserver service: %w", err)
		result.MarkFailed(err, "failed to stop service")
		return result, err
	}

	logger.Info("KubeApiServer service stopped successfully.")
	result.MarkCompleted("service stopped successfully")
	return result, nil
}

func (s *StopKubeApiServerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for rollback, cannot start service: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by starting kube-apiserver service...")
	if err := runner.StartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logger.Errorf("Failed to start kube-apiserver service during rollback: %v", err)
	}

	return nil
}
