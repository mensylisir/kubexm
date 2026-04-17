package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// CrioServiceStep manages CRI-O service.
type CrioServiceStep struct {
	step.Base
	ServiceName string
	Action      string
}

type CrioServiceStepBuilder struct {
	step.Builder[CrioServiceStepBuilder, *CrioServiceStep]
}

func NewCrioServiceStepBuilder(ctx runtime.ExecutionContext, instanceName, serviceName, action string) *CrioServiceStepBuilder {
	s := &CrioServiceStep{
		ServiceName: serviceName,
		Action:      action,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>%s CRI-O service", instanceName, action)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(CrioServiceStepBuilder).Init(s)
}

func (s *CrioServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CrioServiceStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *CrioServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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
		runErr = fmt.Errorf("unsupported action '%s' for CRI-O service", s.Action)
	}

	if runErr != nil {
		if !s.Base.IgnoreError {
			result.MarkFailed(runErr, fmt.Sprintf("failed to %s CRI-O service", s.Action))
			return result, runErr
		}
	}

	logger.Infof("CRI-O service %s completed", s.Action)
	result.MarkCompleted(fmt.Sprintf("CRI-O %s completed", s.Action))
	return result, nil
}

func (s *CrioServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*CrioServiceStep)(nil)
