package common

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// ServiceStep represents a service management operation.
type ServiceStep struct {
	step.Base
	ServiceName string
}

type ServiceStepBuilder struct {
	step.Builder[ServiceStepBuilder, *ServiceStep]
}

func NewServiceStepBuilder(ctx runtime.ExecutionContext, instanceName, serviceName string) *ServiceStepBuilder {
	s := &ServiceStep{ServiceName: serviceName}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Base service step for %s (should not be instantiated directly)", instanceName, serviceName)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(ServiceStepBuilder).Init(s)
}

func (s *ServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ServiceStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *ServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	err := fmt.Errorf("ServiceStep Run should not be called directly")
	result.MarkFailed(err, "base step not implemented")
	return result, err
}

// ServiceAction defines the action to perform.
type ServiceAction string

const (
	ServiceActionStart   ServiceAction = "start"
	ServiceActionStop    ServiceAction = "stop"
	ServiceActionEnable  ServiceAction = "enable"
	ServiceActionDisable ServiceAction = "disable"
	ServiceActionReload  ServiceAction = "reload"
	ServiceActionRestart ServiceAction = "restart"
)

// EnableServiceStep enables a systemd service.
type EnableServiceStep struct {
	ServiceStep
}

type EnableServiceStepBuilder struct {
	step.Builder[EnableServiceStepBuilder, *EnableServiceStep]
}

func NewEnableServiceStepBuilder(ctx runtime.ExecutionContext, instanceName, serviceName string) *EnableServiceStepBuilder {
	s := &EnableServiceStep{}
	s.ServiceName = serviceName
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Enable service %s", instanceName, serviceName)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(EnableServiceStepBuilder).Init(s)
}

func (s *EnableServiceStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	runResult, err := runner.Run(ctx.GoContext(), conn, fmt.Sprintf("systemctl is-enabled %s", s.ServiceName), s.Sudo)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(runResult.Stdout) == "enabled", nil
}

func (s *EnableServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		result.MarkFailed(err, "failed to get host facts")
		return result, err
	}

	if err := runner.EnableService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to enable %s", s.ServiceName))
		return result, err
	}

	result.MarkCompleted(fmt.Sprintf("Service %s enabled", s.ServiceName))
	return result, nil
}

func (s *EnableServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*EnableServiceStep)(nil)

// StartServiceStep starts a systemd service.
type StartServiceStep struct {
	ServiceStep
}

type StartServiceStepBuilder struct {
	step.Builder[StartServiceStepBuilder, *StartServiceStep]
}

func NewStartServiceStepBuilder(ctx runtime.ExecutionContext, instanceName, serviceName string) *StartServiceStepBuilder {
	s := &StartServiceStep{}
	s.ServiceName = serviceName
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Start service %s", instanceName, serviceName)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(StartServiceStepBuilder).Init(s)
}

func (s *StartServiceStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	runResult, err := runner.Run(ctx.GoContext(), conn, fmt.Sprintf("systemctl is-active %s", s.ServiceName), s.Sudo)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(runResult.Stdout) == "active", nil
}

func (s *StartServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		result.MarkFailed(err, "failed to get host facts")
		return result, err
	}

	if err := runner.StartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to start %s", s.ServiceName))
		return result, err
	}

	result.MarkCompleted(fmt.Sprintf("Service %s started", s.ServiceName))
	return result, nil
}

func (s *StartServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*StartServiceStep)(nil)

// RestartServiceStep restarts a systemd service.
type RestartServiceStep struct {
	ServiceStep
}

type RestartServiceStepBuilder struct {
	step.Builder[RestartServiceStepBuilder, *RestartServiceStep]
}

func NewRestartServiceStepBuilder(ctx runtime.ExecutionContext, instanceName, serviceName string) *RestartServiceStepBuilder {
	s := &RestartServiceStep{}
	s.ServiceName = serviceName
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Restart service %s", instanceName, serviceName)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return new(RestartServiceStepBuilder).Init(s)
}

func (s *RestartServiceStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	// Restart should always run — checking if the service was recently restarted
	// would add complexity without much benefit. Idempotency for restart means
	// "bring to running state", not "skip if running".
	return false, nil
}

func (s *RestartServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		result.MarkFailed(err, "failed to get host facts")
		return result, err
	}

	if err := runner.RestartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
		result.MarkFailed(err, fmt.Sprintf("failed to restart %s", s.ServiceName))
		return result, err
	}

	result.MarkCompleted(fmt.Sprintf("Service %s restarted", s.ServiceName))
	return result, nil
}

func (s *RestartServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*RestartServiceStep)(nil)
