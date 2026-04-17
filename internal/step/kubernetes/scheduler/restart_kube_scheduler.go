package kube_scheduler

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type RestartKubeSchedulerStep struct {
	step.Base
	ServiceName string
}

type RestartKubeSchedulerStepBuilder struct {
	step.Builder[RestartKubeSchedulerStepBuilder, *RestartKubeSchedulerStep]
}

func NewRestartKubeSchedulerStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RestartKubeSchedulerStepBuilder {
	s := &RestartKubeSchedulerStep{
		ServiceName: "kube-scheduler.service",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Restart kube-scheduler service"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(RestartKubeSchedulerStepBuilder).Init(s)
	return b
}

func (s *RestartKubeSchedulerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartKubeSchedulerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		return false, fmt.Errorf("failed to determine kube-scheduler service status: %w", err)
	}

	if !active {
		logger.Infof("Kube-scheduler service is not active. Nothing to restart, skipping.")
		return true, nil
	}

	logger.Info("Kube-scheduler service is active. Step needs to run to restart it.")
	return false, nil
}

func (s *RestartKubeSchedulerStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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
		err = fmt.Errorf("failed to gather facts to restart service: %w", err)
		result.MarkFailed(err, "failed to gather facts")
		return result, err
	}

	time.Sleep(5 * time.Second)

	logger.Infof("Restarting %s...", s.ServiceName)
	if err := runner.RestartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logsCmd := fmt.Sprintf("journalctl -u %s --no-pager -n 50", s.ServiceName)
		out, _, _ := runner.OriginRun(ctx.GoContext(), conn, logsCmd, s.Sudo)
		logger.Errorf("Failed to restart %s. Recent logs:\n%s", s.ServiceName, out)
		err = fmt.Errorf("failed to restart %s: %w", s.ServiceName, err)
		result.MarkFailed(err, "failed to restart service")
		return result, err
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, s.ServiceName)
	if err != nil {
		err = fmt.Errorf("failed to verify service status for %s after restarting: %w", s.ServiceName, err)
		result.MarkFailed(err, "failed to verify service status")
		return result, err
	}
	if !active {
		err = fmt.Errorf("service %s did not become active after restart command", s.ServiceName)
		result.MarkFailed(err, "service not active")
		return result, err
	}

	logger.Infof("Service %s restarted successfully.", s.ServiceName)
	result.MarkCompleted("service restarted successfully")
	return result, nil
}

func (s *RestartKubeSchedulerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Restart step has no specific rollback action.")
	return nil
}

var _ step.Step = (*RestartKubeSchedulerStep)(nil)
