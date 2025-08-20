package kube_controller_manager

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RestartKubeControllerManagerStep struct {
	step.Base
	ServiceName string
}

type RestartKubeControllerManagerStepBuilder struct {
	step.Builder[RestartKubeControllerManagerStepBuilder, *RestartKubeControllerManagerStep]
}

func NewRestartKubeControllerManagerStepBuilder(ctx runtime.Context, instanceName string) *RestartKubeControllerManagerStepBuilder {
	s := &RestartKubeControllerManagerStep{
		ServiceName: "kube-controller-manager.service",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Restart kube-controller-manager service"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(RestartKubeControllerManagerStepBuilder).Init(s)
	return b
}

func (s *RestartKubeControllerManagerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartKubeControllerManagerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for kube-controller-manager restart...")
	logger.Info("Precheck passed: Service restart will always be attempted.")
	return false, nil
}

func (s *RestartKubeControllerManagerStep) Run(ctx runtime.ExecutionContext) error {
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

func (s *RestartKubeControllerManagerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Restart step has no specific rollback action.")
	return nil
}

// Ensure the struct implements the Step interface.
var _ step.Step = (*RestartKubeControllerManagerStep)(nil)
