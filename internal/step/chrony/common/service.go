package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// ChronyServiceAction defines the action for chrony service.
type ChronyServiceAction string

const (
	ChronyActionStart   ChronyServiceAction = "start"
	ChronyActionStop    ChronyServiceAction = "stop"
	ChronyActionEnable  ChronyServiceAction = "enable"
	ChronyActionDisable ChronyServiceAction = "disable"
)

// ChronyServiceStep manages chrony service.
type ChronyServiceStep struct {
	step.Base
	ServiceName string
	Action      ChronyServiceAction
}

type ChronyServiceStepBuilder struct {
	step.Builder[ChronyServiceStepBuilder, *ChronyServiceStep]
}

func NewChronyServiceStepBuilder(ctx runtime.ExecutionContext, instanceName, serviceName string, action ChronyServiceAction) *ChronyServiceStepBuilder {
	s := &ChronyServiceStep{
		ServiceName: serviceName,
		Action:      action,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>%s chrony service", instanceName, action)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(ChronyServiceStepBuilder).Init(s)
}

func (s *ChronyServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ChronyServiceStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *ChronyServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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
	case ChronyActionStart:
		runErr = runner.StartService(ctx.GoContext(), conn, facts, s.ServiceName)
	case ChronyActionStop:
		runErr = runner.StopService(ctx.GoContext(), conn, facts, s.ServiceName)
	case ChronyActionEnable:
		runErr = runner.EnableService(ctx.GoContext(), conn, facts, s.ServiceName)
	case ChronyActionDisable:
		runErr = runner.DisableService(ctx.GoContext(), conn, facts, s.ServiceName)
	default:
		runErr = fmt.Errorf("unsupported action '%s' for chrony service", s.Action)
	}

	if runErr != nil {
		if !s.Base.IgnoreError {
			result.MarkFailed(runErr, fmt.Sprintf("failed to %s chrony", s.Action))
			return result, runErr
		}
	}

	logger.Infof("Chrony service %s completed", s.Action)
	result.MarkCompleted(fmt.Sprintf("Chrony %s completed", s.Action))
	return result, nil
}

func (s *ChronyServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*ChronyServiceStep)(nil)
