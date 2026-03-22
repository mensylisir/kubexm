package haproxy

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

const haproxyServiceName = "haproxy.service"

type StartHAProxyStep struct {
	step.Base
}

type StartHAProxyStepBuilder struct {
	step.Builder[StartHAProxyStepBuilder, *StartHAProxyStep]
}

func NewStartHAProxyStepBuilder(ctx runtime.ExecutionContext, instanceName string) *StartHAProxyStepBuilder {
	s := &StartHAProxyStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Start HAProxy service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(StartHAProxyStepBuilder).Init(s)
	return b
}

func (s *StartHAProxyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StartHAProxyStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		logger.Warnf("Failed to check if HAProxy service is active, assuming it needs to be started. Error: %v", err)
		return false, nil
	}

	if active {
		logger.Infof("HAProxy service is already active. Step is done.")
		return true, nil
	}

	logger.Info("HAProxy service is not active. Step needs to run.")
	return false, nil
}

func (s *StartHAProxyStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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
		result.MarkFailed(err, "failed to gather facts to start service")
		return result, err
	}

	logger.Infof("Starting HAProxy service...")
	if err := runner.StartService(ctx.GoContext(), conn, facts, haproxyServiceName); err != nil {
		logsCmd := fmt.Sprintf("journalctl -u %s --no-pager -n 50", haproxyServiceName)
		out, _, _ := runner.OriginRun(ctx.GoContext(), conn, logsCmd, s.Sudo)
		logger.Errorf("Failed to start HAProxy service. Recent logs:\n%s", out)
		result.MarkFailed(err, "failed to start HAProxy service")
		return result, err
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, haproxyServiceName)
	if err != nil {
		result.MarkFailed(err, "failed to verify HAProxy service status after starting")
		return result, err
	}
	if !active {
		result.MarkFailed(err, "HAProxy service did not become active after start command")
		return result, err
	}

	logger.Info("HAProxy service started successfully.")
	result.MarkCompleted("HAProxy service started successfully")
	return result, nil
}

func (s *StartHAProxyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for rollback, cannot stop service: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by stopping HAProxy service...")
	if err := runner.StopService(ctx.GoContext(), conn, facts, haproxyServiceName); err != nil {
		logger.Errorf("Failed to stop HAProxy service during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*StartHAProxyStep)(nil)
