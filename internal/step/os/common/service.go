package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// ServiceAction defines the action to perform on a service.
type ServiceAction string

const (
	ServiceActionStart   ServiceAction = "start"
	ServiceActionStop    ServiceAction = "stop"
	ServiceActionRestart ServiceAction = "restart"
	ServiceActionEnable  ServiceAction = "enable"
	ServiceActionDisable ServiceAction = "disable"
	ServiceActionStatus  ServiceAction = "status"
	ServiceActionReload  ServiceAction = "reload"
)

// ManageServiceStep manages system services.
type ManageServiceStep struct {
	step.Base
	ServiceName string
	Action      ServiceAction
}

type ManageServiceStepBuilder struct {
	step.Builder[ManageServiceStepBuilder, *ManageServiceStep]
}

func NewManageServiceStepBuilder(ctx runtime.ExecutionContext, instanceName, serviceName string, action ServiceAction) *ManageServiceStepBuilder {
	s := &ManageServiceStep{
		ServiceName: serviceName,
		Action:      action,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>%s service %s", instanceName, action, serviceName)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(ManageServiceStepBuilder).Init(s)
}

func (s *ManageServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ManageServiceStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *ManageServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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
	case ServiceActionStart:
		runErr = runner.StartService(ctx.GoContext(), conn, facts, s.ServiceName)
	case ServiceActionStop:
		runErr = runner.StopService(ctx.GoContext(), conn, facts, s.ServiceName)
	case ServiceActionRestart:
		runErr = runner.RestartService(ctx.GoContext(), conn, facts, s.ServiceName)
	case ServiceActionEnable:
		runErr = runner.EnableService(ctx.GoContext(), conn, facts, s.ServiceName)
	case ServiceActionDisable:
		runErr = runner.DisableService(ctx.GoContext(), conn, facts, s.ServiceName)
	case ServiceActionReload:
		// systemctl reload doesn't have a direct Runner method, use DaemonReload for systemctl reload-daemon
		runErr = runner.DaemonReload(ctx.GoContext(), conn, facts)
	case ServiceActionStatus:
		// Status check would need a different approach, for now use Run
		cmd := fmt.Sprintf("systemctl status %s", s.ServiceName)
		_, runErr = runner.Run(ctx.GoContext(), conn, cmd, s.Base.Sudo)
	default:
		result.MarkFailed(fmt.Errorf("unknown action: %s", s.Action), "invalid action")
		return result, fmt.Errorf("unknown action: %s", s.Action)
	}

	if runErr != nil {
		if !s.Base.IgnoreError {
			result.MarkFailed(runErr, fmt.Sprintf("failed to %s service %s", s.Action, s.ServiceName))
			return result, runErr
		}
		logger.Warnf("Command failed (ignored): %v", runErr)
	}

	logger.Infof("Service %s %s completed successfully", s.ServiceName, s.Action)
	result.MarkCompleted(fmt.Sprintf("Service %s %s completed", s.ServiceName, s.Action))
	return result, nil
}

func (s *ManageServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*ManageServiceStep)(nil)
