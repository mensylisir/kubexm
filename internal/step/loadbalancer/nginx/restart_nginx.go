package nginx

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type RestartNginxStep struct {
	step.Base
}

type RestartNginxStepBuilder struct {
	step.Builder[RestartNginxStepBuilder, *RestartNginxStep]
}

func NewRestartNginxStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RestartNginxStepBuilder {
	s := &RestartNginxStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Restart NGINX service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	b := new(RestartNginxStepBuilder).Init(s)
	return b
}

func (s *RestartNginxStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartNginxStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		return false, err
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, nginxServiceName)
	if err != nil {
		return false, fmt.Errorf("failed to determine nginx service status: %w", err)
	}

	if !active {
		logger.Infof("NGINX service is not active. Nothing to restart, skipping.")
		return true, nil
	}

	logger.Info("NGINX service is active. Step needs to run to restart it.")
	return false, nil
}

func (s *RestartNginxStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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

	logger.Infof("Restarting NGINX service...")
	if err := runner.RestartService(ctx.GoContext(), conn, facts, nginxServiceName); err != nil {
		result.MarkFailed(err, "failed to restart NGINX service")
		return result, err
	}

	active, err := runner.IsServiceActive(ctx.GoContext(), conn, facts, nginxServiceName)
	if err != nil {
		result.MarkFailed(err, "failed to verify NGINX service status after restart")
		return result, fmt.Errorf("failed to verify NGINX service status after restart: %w", err)
	}
	if !active {
		result.MarkFailed(err, "NGINX service did not become active after restart command")
		return result, fmt.Errorf("NGINX service did not become active after restart command")
	}

	logger.Info("NGINX service restarted successfully.")
	result.MarkCompleted("NGINX service restarted successfully")
	return result, nil
}

func (s *RestartNginxStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name)
	logger.Info("Rollback for a restart step is not applicable. No action taken.")
	return nil
}

var _ step.Step = (*RestartNginxStep)(nil)
