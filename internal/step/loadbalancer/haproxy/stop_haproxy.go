package haproxy

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type StopHAProxyStep struct {
	step.Base
}

type StopHAProxyStepBuilder struct {
	step.Builder[StopHAProxyStepBuilder, *StopHAProxyStep]
}

func NewStopHAProxyStepBuilder(ctx runtime.ExecutionContext, instanceName string) *StopHAProxyStepBuilder {
	s := &StopHAProxyStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop HAProxy service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(StopHAProxyStepBuilder).Init(s)
	return b
}

func (s *StopHAProxyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StopHAProxyStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return false, fmt.Errorf("failed to gather facts to check service status: %w", err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, haproxyServiceName)
	if err != nil {
		logger.Warnf("Failed to check if HAProxy service is active, assuming it needs to be stopped. Error: %v", err)
		return false, nil
	}

	if !active {
		logger.Infof("HAProxy service is already inactive. Step is done.")
		return true, nil
	}

	logger.Info("HAProxy service is active. Step needs to run to stop it.")
	return false, nil
}

func (s *StopHAProxyStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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
		result.MarkFailed(err, "failed to gather facts to stop service")
		return result, err
	}

	logger.Infof("Stopping HAProxy service...")
	if err := runner.StopService(ctx.GoContext(), conn, facts, haproxyServiceName); err != nil {
		result.MarkFailed(err, "failed to stop HAProxy service")
		return result, err
	}

	logger.Info("HAProxy service stopped successfully.")
	result.MarkCompleted("HAProxy service stopped successfully")
	return result, nil
}

func (s *StopHAProxyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name)
	logger.Info("Rollback is not applicable for a stop step.")
	return nil
}

var _ step.Step = (*StopHAProxyStep)(nil)
