package docker

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type RestartCriDockerdStep struct {
	step.Base
}

type RestartCriDockerdStepBuilder struct {
	step.Builder[RestartCriDockerdStepBuilder, *RestartCriDockerdStep]
}

func NewRestartCriDockerdStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RestartCriDockerdStepBuilder {
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

func (s *RestartCriDockerdStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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
		result.MarkFailed(err, "failed to gather facts to restart service")
		return result, fmt.Errorf("failed to gather facts to restart service: %w", err)
	}

	logger.Infof("Restarting cri-dockerd service...")
	if err := runner.RestartService(ctx.GoContext(), conn, facts, CriDockerdServiceName); err != nil {
		logsCmd := fmt.Sprintf("journalctl -u %s --no-pager -n 50", CriDockerdServiceName)
		out, _, _ := runner.OriginRun(ctx.GoContext(), conn, logsCmd, s.Sudo)
		logger.Errorf("Failed to restart cri-dockerd service. Recent logs:\n%s", out)
		result.MarkFailed(err, "failed to restart cri-dockerd service")
		return result, fmt.Errorf("failed to restart cri-dockerd service: %w", err)
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, CriDockerdServiceName)
	if err != nil {
		result.MarkFailed(err, "failed to verify cri-dockerd service status after restarting")
		return result, fmt.Errorf("failed to verify cri-dockerd service status after restarting: %w", err)
	}
	if !active {
		result.MarkFailed(err, "cri-dockerd service did not become active after restart command")
		return result, fmt.Errorf("cri-dockerd service did not become active after restart command")
	}

	logger.Info("CriDockerd service restarted successfully.")
	result.MarkCompleted("cri-dockerd service restarted successfully")
	return result, nil
}

func (s *RestartCriDockerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Restart step has no specific rollback action.")
	return nil
}
