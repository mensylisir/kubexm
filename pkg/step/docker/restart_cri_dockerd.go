package docker

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RestartCriDockerdStep struct {
	step.Base
}

type RestartCriDockerdStepBuilder struct {
	step.Builder[RestartCriDockerdStepBuilder, *RestartCriDockerdStep]
}

func NewRestartCriDockerdStepBuilder(ctx runtime.Context, instanceName string) *RestartCriDockerdStepBuilder {
	s := &RestartCriDockerdStep{}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Restart cri-dockerd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(RestartCriDockerdStepBuilder).Init(s)
	return b
}

func (s *RestartCriDockerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartCriDockerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *RestartCriDockerdStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Restarting cri-dockerd service...")
	if err := runner.RestartService(ctx.GoContext(), conn, facts, CriDockerdServiceName); err != nil {
		logsCmd := fmt.Sprintf("journalctl -u %s --no-pager -n 50", CriDockerdServiceName)
		out, _, _ := runner.OriginRun(ctx.GoContext(), conn, logsCmd, s.Sudo)
		logger.Errorf("Failed to restart cri-dockerd service. Recent logs:\n%s", out)
		return fmt.Errorf("failed to restart cri-dockerd service: %w", err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, CriDockerdServiceName)
	if err != nil {
		return fmt.Errorf("failed to verify cri-dockerd service status after restarting: %w", err)
	}
	if !active {
		return fmt.Errorf("cri-dockerd service did not become active after restart command")
	}

	logger.Info("CriDockerd service restarted successfully.")
	return nil
}

func (s *RestartCriDockerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Restart step has no specific rollback action.")
	return nil
}
