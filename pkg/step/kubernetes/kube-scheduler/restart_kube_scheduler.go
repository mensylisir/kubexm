package kube_scheduler

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RestartKubeSchedulerStep struct {
	step.Base
	ServiceName string
}

type RestartKubeSchedulerStepBuilder struct {
	step.Builder[RestartKubeSchedulerStepBuilder, *RestartKubeSchedulerStep]
}

func NewRestartKubeSchedulerStepBuilder(ctx runtime.Context, instanceName string) *RestartKubeSchedulerStepBuilder {
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
	logger.Info("Starting precheck for kube-scheduler restart...")

	logger.Info("Precheck passed: Service restart will always be attempted.")
	return false, nil
}

func (s *RestartKubeSchedulerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return fmt.Errorf("failed to gather facts to restart service: %w", err)
	}

	time.Sleep(5 * time.Second)

	logger.Infof("Restarting %s...", s.ServiceName)
	if err := runner.RestartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		logsCmd := fmt.Sprintf("journalctl -u %s --no-pager -n 50", s.ServiceName)
		out, _, _ := runner.OriginRun(ctx.GoContext(), conn, logsCmd, s.Sudo)
		logger.Errorf("Failed to restart %s. Recent logs:\n%s", s.ServiceName, out)
		return fmt.Errorf("failed to restart %s: %w", s.ServiceName, err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, s.ServiceName)
	if err != nil {
		return fmt.Errorf("failed to verify service status for %s after restarting: %w", s.ServiceName, err)
	}
	if !active {
		return fmt.Errorf("service %s did not become active after restart command", s.ServiceName)
	}

	logger.Infof("Service %s restarted successfully.", s.ServiceName)
	return nil
}

func (s *RestartKubeSchedulerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Restart step has no specific rollback action.")
	return nil
}

var _ step.Step = (*RestartKubeSchedulerStep)(nil)
