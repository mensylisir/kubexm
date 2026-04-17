package registry

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/runner"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type StopRegistryServiceStep struct {
	step.Base
}

type StopRegistryServiceStepBuilder struct {
	step.Builder[StopRegistryServiceStepBuilder, *StopRegistryServiceStep]
}

func NewStopRegistryServiceStepBuilder(ctx runtime.ExecutionContext, instanceName string) *StopRegistryServiceStepBuilder {
	s := &StopRegistryServiceStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Stop registry systemd service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(StopRegistryServiceStepBuilder).Init(s)
	return b
}

func (s *StopRegistryServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *StopRegistryServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, "systemctl is-active registry.service", &runner.ExecOptions{Sudo: s.Sudo})
	if err != nil {
		var cmdErr *runner.CommandError
		if errors.As(err, &cmdErr) {
			combined := strings.TrimSpace(cmdErr.Stderr + "\n" + string(stderr) + "\n" + string(stdout))
			if cmdErr.ExitCode != 0 && (strings.Contains(combined, "inactive") || strings.Contains(combined, "unknown") || strings.Contains(combined, "could not be found") || strings.Contains(combined, "not-found") || strings.Contains(combined, "not found") || strings.Contains(combined, "loaded: not-found")) {
				return true, nil
			}
		}
		return false, fmt.Errorf("failed to determine registry service status: %w", err)
	}
	return strings.TrimSpace(string(stdout)) != "active", nil
}

func (s *StopRegistryServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	ctx.SetStepResult(result)
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, "systemctl is-active registry.service", &runner.ExecOptions{Sudo: s.Sudo})
	if err != nil {
		var cmdErr *runner.CommandError
		if errors.As(err, &cmdErr) {
			combined := strings.TrimSpace(cmdErr.Stderr + "\n" + string(stderr) + "\n" + string(stdout))
			if cmdErr.ExitCode != 0 && (strings.Contains(combined, "inactive") || strings.Contains(combined, "unknown") || strings.Contains(combined, "could not be found") || strings.Contains(combined, "not-found") || strings.Contains(combined, "not found") || strings.Contains(combined, "loaded: not-found")) {
				result.SetMetadata("registry_was_active", false)
				result.MarkCompleted("registry service already inactive")
				return result, nil
			}
		}
		result.MarkFailed(err, "failed to determine registry service status before stop")
		return result, fmt.Errorf("failed to determine registry service status before stop: %w", err)
	}

	wasActive := strings.TrimSpace(string(stdout)) == "active"
	result.SetMetadata("registry_was_active", wasActive)
	if !wasActive {
		result.MarkCompleted("registry service already inactive")
		return result, nil
	}

	if _, err := runnerSvc.Run(ctx.GoContext(), conn, "systemctl stop registry.service", s.Sudo); err != nil {
		result.MarkFailed(err, "failed to stop registry service")
		return result, err
	}
	result.MarkCompleted("registry service stopped successfully")
	return result, nil
}

func (s *StopRegistryServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	stepResult := ctx.GetStepResult()
	if stepResult == nil {
		return nil
	}

	wasActiveValue, ok := stepResult.GetMetadata("registry_was_active")
	if !ok {
		return nil
	}

	wasActive, ok := wasActiveValue.(bool)
	if !ok || !wasActive {
		return nil
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}
	_, _ = runner.Run(ctx.GoContext(), conn, "systemctl start registry.service", s.Sudo)
	return nil
}

var _ step.Step = (*StopRegistryServiceStep)(nil)
