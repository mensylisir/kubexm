package kube_apiserver

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type RestartKubeApiServerStep struct {
	step.Base
	ServiceName string
}

type RestartKubeApiServerStepBuilder struct {
	step.Builder[RestartKubeApiServerStepBuilder, *RestartKubeApiServerStep]
}

func NewRestartKubeApiServerStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RestartKubeApiServerStepBuilder {
	s := &RestartKubeApiServerStep{
		ServiceName: "kube-apiserver.service",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Restart kube-apiserver service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(RestartKubeApiServerStepBuilder).Init(s)
	return b
}

func (s *RestartKubeApiServerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartKubeApiServerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *RestartKubeApiServerStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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

	logger.Infof("Restarting kube-apiserver service...")
	if err := runner.RestartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logsCmd := fmt.Sprintf("journalctl -u %s --no-pager -n 50", s.ServiceName)
		out, _, _ := runner.OriginRun(ctx.GoContext(), conn, logsCmd, s.Sudo)
		logger.Errorf("Failed to restart kube-apiserver service. Recent logs:\n%s", out)
		err = fmt.Errorf("failed to restart kube-apiserver service: %w", err)
		result.MarkFailed(err, "failed to restart service")
		return result, err
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, s.ServiceName)
	if err != nil {
		err = fmt.Errorf("failed to verify kube-apiserver service status after restarting: %w", err)
		result.MarkFailed(err, "failed to verify service status")
		return result, err
	}
	if !active {
		err = fmt.Errorf("kube-apiserver service did not become active after restart command")
		result.MarkFailed(err, "service not active after restart")
		return result, err
	}

	logger.Info("KubeApiServer service restarted successfully.")
	result.MarkCompleted("service restarted successfully")
	return result, nil
}

func (s *RestartKubeApiServerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Restart step has no specific rollback action.")
	return nil
}
