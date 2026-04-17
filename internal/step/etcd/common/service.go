package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// EtcdServiceAction defines the action to perform on etcd service.
type EtcdServiceAction string

const (
	EtcdServiceActionStart   EtcdServiceAction = "start"
	EtcdServiceActionStop    EtcdServiceAction = "stop"
	EtcdServiceActionRestart EtcdServiceAction = "restart"
	EtcdServiceActionEnable  EtcdServiceAction = "enable"
	EtcdServiceActionDisable EtcdServiceAction = "disable"
	EtcdServiceActionStatus  EtcdServiceAction = "status"
)

// EtcdServiceStep manages etcd service (start/stop/enable/disable).
type EtcdServiceStep struct {
	step.Base
	ServiceName string
	Action      EtcdServiceAction
}

type EtcdServiceStepBuilder struct {
	step.Builder[EtcdServiceStepBuilder, *EtcdServiceStep]
}

func NewEtcdServiceStepBuilder(ctx runtime.ExecutionContext, instanceName, serviceName string, action EtcdServiceAction) *EtcdServiceStepBuilder {
	s := &EtcdServiceStep{
		ServiceName: serviceName,
		Action:      action,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>%s etcd service %s", instanceName, action, serviceName)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(EtcdServiceStepBuilder).Init(s)
}

func (s *EtcdServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EtcdServiceStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *EtcdServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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
	case EtcdServiceActionStart:
		runErr = runner.StartService(ctx.GoContext(), conn, facts, s.ServiceName)
	case EtcdServiceActionStop:
		runErr = runner.StopService(ctx.GoContext(), conn, facts, s.ServiceName)
	case EtcdServiceActionRestart:
		runErr = runner.RestartService(ctx.GoContext(), conn, facts, s.ServiceName)
	case EtcdServiceActionEnable:
		runErr = runner.EnableService(ctx.GoContext(), conn, facts, s.ServiceName)
	case EtcdServiceActionDisable:
		runErr = runner.DisableService(ctx.GoContext(), conn, facts, s.ServiceName)
	case EtcdServiceActionStatus:
		cmd := fmt.Sprintf("systemctl status %s", s.ServiceName)
		_, runErr = runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo)
	default:
		result.MarkFailed(fmt.Errorf("unknown action: %s", s.Action), "invalid action")
		return result, fmt.Errorf("unknown action: %s", s.Action)
	}

	if runErr != nil {
		if !s.Base.IgnoreError {
			result.MarkFailed(runErr, fmt.Sprintf("failed to %s etcd service", s.Action))
			return result, runErr
		}
		logger.Warnf("Command failed (ignored): %v", runErr)
	}

	logger.Infof("Etcd service %s completed successfully", s.Action)
	result.MarkCompleted(fmt.Sprintf("Etcd service %s completed", s.Action))
	return result, nil
}

func (s *EtcdServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*EtcdServiceStep)(nil)
