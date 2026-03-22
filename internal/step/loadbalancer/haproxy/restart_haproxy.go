package haproxy

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type RestartHAProxyStep struct {
	step.Base
}

type RestartHAProxyStepBuilder struct {
	step.Builder[RestartHAProxyStepBuilder, *RestartHAProxyStep]
}

func NewRestartHAProxyStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RestartHAProxyStepBuilder {
	s := &RestartHAProxyStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Restart HAProxy service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(RestartHAProxyStepBuilder).Init(s)
	return b
}

func (s *RestartHAProxyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartHAProxyStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *RestartHAProxyStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		result.MarkFailed(err, "failed to gather facts to restart service")
		return result, err
	}

	logger.Infof("Restarting HAProxy service...")
	if err := runner.RestartService(ctx.GoContext(), conn, facts, haproxyServiceName); err != nil {
		result.MarkFailed(err, "failed to restart HAProxy service")
		return result, err
	}

	logger.Info("HAProxy service restarted successfully.")
	result.MarkCompleted("HAProxy service restarted successfully")
	return result, nil
}

func (s *RestartHAProxyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name)
	logger.Info("Rollback for a restart step is not applicable. No action taken.")
	return nil
}

var _ step.Step = (*RestartHAProxyStep)(nil)
