package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// RegistryServiceStep manages registry service.
type RegistryServiceStep struct {
	step.Base
	ServiceName string
	Action      string
}

type RegistryServiceStepBuilder struct {
	step.Builder[RegistryServiceStepBuilder, *RegistryServiceStep]
}

func NewRegistryServiceStepBuilder(ctx runtime.ExecutionContext, instanceName, serviceName, action string) *RegistryServiceStepBuilder {
	s := &RegistryServiceStep{
		ServiceName: serviceName,
		Action:      action,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>%s registry service %s", instanceName, action, serviceName)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(RegistryServiceStepBuilder).Init(s)
}

func (s *RegistryServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RegistryServiceStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *RegistryServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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
		runErr = fmt.Errorf("unsupported action '%s' for registry service", s.Action)
	}

	if runErr != nil {
		if !s.Base.IgnoreError {
			result.MarkFailed(runErr, fmt.Sprintf("failed to %s registry service", s.Action))
			return result, runErr
		}
	}

	logger.Infof("Registry service %s completed", s.Action)
	result.MarkCompleted(fmt.Sprintf("Registry %s completed", s.Action))
	return result, nil
}

func (s *RegistryServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*RegistryServiceStep)(nil)
