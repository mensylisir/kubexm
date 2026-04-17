package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// HarborServiceAction defines the action for Harbor service.
type HarborServiceAction string

const (
	HarborActionStart   HarborServiceAction = "start"
	HarborActionStop    HarborServiceAction = "stop"
	HarborActionRestart HarborServiceAction = "restart"
	HarborActionEnable  HarborServiceAction = "enable"
	HarborActionDisable HarborServiceAction = "disable"
)

// HarborServiceStep manages Harbor service.
type HarborServiceStep struct {
	step.Base
	ServiceName string
	Action      HarborServiceAction
}

type HarborServiceStepBuilder struct {
	step.Builder[HarborServiceStepBuilder, *HarborServiceStep]
}

func NewHarborServiceStepBuilder(ctx runtime.ExecutionContext, instanceName, serviceName string, action HarborServiceAction) *HarborServiceStepBuilder {
	s := &HarborServiceStep{
		ServiceName: serviceName,
		Action:      action,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>%s Harbor service", instanceName, action)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(HarborServiceStepBuilder).Init(s)
}

func (s *HarborServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *HarborServiceStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *HarborServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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
	case HarborActionStart:
		runErr = runner.StartService(ctx.GoContext(), conn, facts, s.ServiceName)
	case HarborActionStop:
		runErr = runner.StopService(ctx.GoContext(), conn, facts, s.ServiceName)
	case HarborActionRestart:
		runErr = runner.RestartService(ctx.GoContext(), conn, facts, s.ServiceName)
	case HarborActionEnable:
		runErr = runner.EnableService(ctx.GoContext(), conn, facts, s.ServiceName)
	case HarborActionDisable:
		runErr = runner.DisableService(ctx.GoContext(), conn, facts, s.ServiceName)
	default:
		runErr = fmt.Errorf("unsupported action '%s' for Harbor service", s.Action)
	}

	if runErr != nil {
		if !s.Base.IgnoreError {
			result.MarkFailed(runErr, fmt.Sprintf("failed to %s Harbor", s.Action))
			return result, runErr
		}
	}

	logger.Infof("Harbor service %s completed", s.Action)
	result.MarkCompleted(fmt.Sprintf("Harbor %s completed", s.Action))
	return result, nil
}

func (s *HarborServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*HarborServiceStep)(nil)
