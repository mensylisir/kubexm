package chrony

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RestartChronyStep struct {
	step.Base
	ServiceName    string
	postWaitSettle time.Duration
}

type RestartChronyStepBuilder struct {
	step.Builder[RestartChronyStepBuilder, *RestartChronyStep]
}

func NewRestartChronyStepBuilder(ctx runtime.Context, instanceName string) *RestartChronyStepBuilder {
	s := &RestartChronyStep{
		ServiceName:    "chronyd.service",
		postWaitSettle: 15 * time.Second,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Restart the chronyd service to apply new configuration"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(RestartChronyStepBuilder).Init(s)
	return b
}

func (s *RestartChronyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartChronyStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for chronyd service restart...")

	logger.Info("Precheck passed: Service restart will always be attempted.")
	return false, nil
}

func (s *RestartChronyStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Info("Reloading systemd daemon before restarting service...")
	if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
		logger.Warnf("Failed to run daemon-reload, continuing with restart anyway. Error: %v", err)
	}

	logger.Infof("Restarting service: %s", s.ServiceName)
	if err := runner.RestartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logsCmd := fmt.Sprintf("journalctl -u %s --no-pager -n 20", s.ServiceName)
		out, _, _ := runner.OriginRun(ctx.GoContext(), conn, logsCmd, s.Sudo)
		logger.Errorf("Failed to restart %s. Recent logs:\n%s", s.ServiceName, out)
		return fmt.Errorf("failed to restart service '%s': %w", s.ServiceName, err)
	}

	logger.Infof("Service '%s' restart signal sent. Waiting for %v to settle...", s.ServiceName, s.postWaitSettle)
	time.Sleep(s.postWaitSettle)

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, s.ServiceName)
	if err != nil {
		return fmt.Errorf("failed to verify service status for %s after restarting: %w", s.ServiceName, err)
	}
	if !active {
		return fmt.Errorf("service %s did not become active after restart command", s.ServiceName)
	}

	logger.Infof("Service '%s' has been restarted successfully.", s.ServiceName)
	return nil
}

func (s *RestartChronyStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warnf("Rolling back by attempting to restart service again: %s", s.ServiceName)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Warnf("Failed to gather facts during rollback, cannot restart service. Error: %v", err)
		return nil
	}

	if err := runner.RestartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logger.Errorf("Failed to restart service '%s' during rollback. The node may need manual intervention. Error: %v", s.ServiceName, err)
	}

	logger.Info("Rollback: Restart signal sent to service.")
	return nil
}

var _ step.Step = (*RestartChronyStep)(nil)
