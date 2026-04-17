package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// ContainerdServiceStep manages containerd service.
type ContainerdServiceStep struct {
	step.Base
	ServiceName string
	Action      string
}

type ContainerdServiceStepBuilder struct {
	step.Builder[ContainerdServiceStepBuilder, *ContainerdServiceStep]
}

func NewContainerdServiceStepBuilder(ctx runtime.ExecutionContext, instanceName, serviceName, action string) *ContainerdServiceStepBuilder {
	s := &ContainerdServiceStep{
		ServiceName: serviceName,
		Action:      action,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>%s containerd service", instanceName, action)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(ContainerdServiceStepBuilder).Init(s)
}

func (s *ContainerdServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ContainerdServiceStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *ContainerdServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}
	facts, _ := ctx.GetHostFacts(ctx.GetHost())

	var runErr error
	switch s.Action {
	case "start":
		runErr = runner.StartService(ctx.GoContext(), conn, facts, s.ServiceName)
	case "stop":
		runErr = runner.StopService(ctx.GoContext(), conn, facts, s.ServiceName)
	case "restart":
		runErr = runner.RestartService(ctx.GoContext(), conn, facts, s.ServiceName)
	case "enable":
		runErr = runner.EnableService(ctx.GoContext(), conn, facts, s.ServiceName)
	case "disable":
		runErr = runner.DisableService(ctx.GoContext(), conn, facts, s.ServiceName)
	default:
		runErr = fmt.Errorf("unsupported action '%s' for containerd service", s.Action)
	}

	if runErr != nil {
		if !s.Base.IgnoreError {
			result.MarkFailed(runErr, fmt.Sprintf("failed to %s containerd", s.Action))
			return result, runErr
		}
	}

	logger.Infof("Containerd service %s completed", s.Action)
	result.MarkCompleted(fmt.Sprintf("Containerd %s completed", s.Action))
	return result, nil
}

func (s *ContainerdServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*ContainerdServiceStep)(nil)
