package haproxy

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const haproxyServiceName = "haproxy.service"

type StartHAProxyStep struct {
	step.Base
}

type StartHAProxyStepBuilder struct {
	step.Builder[StartHAProxyStepBuilder, *StartHAProxyStep]
}

func NewStartHAProxyStepBuilder(ctx runtime.Context, instanceName string) *StartHAProxyStepBuilder {
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

func (s *StartHAProxyStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return fmt.Errorf("failed to gather facts to start service: %w", err)
	}

	logger.Infof("Starting HAProxy service...")
	if err := runner.StartService(ctx.GoContext(), conn, facts, haproxyServiceName); err != nil {
		logsCmd := fmt.Sprintf("journalctl -u %s --no-pager -n 50", haproxyServiceName)
		out, _, _ := runner.OriginRun(ctx.GoContext(), conn, logsCmd, s.Sudo)
		logger.Errorf("Failed to start HAProxy service. Recent logs:\n%s", out)
		return fmt.Errorf("failed to start HAProxy service: %w", err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, haproxyServiceName)
	if err != nil {
		return fmt.Errorf("failed to verify HAProxy service status after starting: %w", err)
	}
	if !active {
		return fmt.Errorf("HAProxy service did not become active after start command")
	}

	logger.Info("HAProxy service started successfully.")
	return nil
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
