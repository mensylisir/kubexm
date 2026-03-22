package registry

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
		"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type RestartRegistryServiceStep struct {
	step.Base
}

type RestartRegistryServiceStepBuilder struct {
	step.Builder[RestartRegistryServiceStepBuilder, *RestartRegistryServiceStep]
}

func NewRestartRegistryServiceStepBuilder(ctx runtime.ExecutionContext, instanceName string) *RestartRegistryServiceStepBuilder {

	s := &RestartRegistryServiceStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Restart registry systemd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(RestartRegistryServiceStepBuilder).Init(s)
	return b
}

func (s *RestartRegistryServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestartRegistryServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *RestartRegistryServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	logger.Info("Executing 'systemctl daemon-reload' to ensure service definition is up-to-date.")
	if _, err := runner.Run(ctx.GoContext(), conn, "systemctl daemon-reload", s.Sudo); err != nil {
		result.MarkFailed(err, "failed to run daemon-reload")
		return result, err
	}

	restartCmd := "systemctl restart registry.service"
	logger.Infof("Executing remote command: %s", restartCmd)
	if _, err := runner.Run(ctx.GoContext(), conn, restartCmd, s.Sudo); err != nil {
		statusCmd := "systemctl status registry.service"
		statusOutput, _ := runner.Run(ctx.GoContext(), conn, statusCmd, s.Sudo)
		journalCmd := "journalctl -u registry.service -n 50 --no-pager"
		journalOutput, _ := runner.Run(ctx.GoContext(), conn, journalCmd, s.Sudo)

		result.MarkFailed(err, fmt.Sprintf("failed to restart registry service:\nStatus:\n%s\nJournal logs:\n%s", statusOutput, journalOutput))
		return result, err
	}

	time.Sleep(3 * time.Second)
	statusCmd := "systemctl is-active registry.service"
	output, err := runner.Run(ctx.GoContext(), conn, statusCmd, s.Sudo)
	if err != nil || strings.TrimSpace(output) != "active" {
		journalCmd := "journalctl -u registry.service -n 50 --no-pager"
		journalOutput, _ := runner.Run(ctx.GoContext(), conn, journalCmd, s.Sudo)
		result.MarkFailed(err, fmt.Sprintf("registry service failed to become active after restart. Journal logs:\n%s", journalOutput))
		return result, err
	}

	logger.Info("Registry service has been restarted successfully and is active.")
	result.MarkCompleted("registry service restarted successfully")
	return result, nil
}

func (s *RestartRegistryServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warn("Rolling back by stopping the registry service.")
	_, _ = runner.Run(ctx.GoContext(), conn, "systemctl stop registry.service", s.Sudo)
	return nil
}

var _ step.Step = (*RestartRegistryServiceStep)(nil)
