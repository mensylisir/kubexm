package common

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type ServiceAction string

const (
	ActionStart        ServiceAction = "start"
	ActionStop         ServiceAction = "stop"
	ActionRestart      ServiceAction = "restart"
	ActionEnable       ServiceAction = "enable"
	ActionDisable      ServiceAction = "disable"
	ActionMask         ServiceAction = "mask"
	ActionUnmask       ServiceAction = "unmask"
	ActionDaemonReload ServiceAction = "daemon-reload"
	ActionIsActive     ServiceAction = "is-active"
	ActionIsEnabled    ServiceAction = "is-enabled"
)

type ManageServiceStep struct {
	step.Base
	Action      ServiceAction
	ServiceName string
}

type ManageServiceStepBuilder struct {
	step.Builder[ManageServiceStepBuilder, *ManageServiceStep]
}

func NewManageServiceStepBuilder(instanceName, serviceName string, action ServiceAction) *ManageServiceStepBuilder {
	cs := &ManageServiceStep{
		ServiceName: serviceName,
		Action:      action,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>%s [%s]", string(action), serviceName)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 30 * time.Second
	return new(ManageServiceStepBuilder).Init(cs)
}

func (s *ManageServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ManageServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}
	switch s.Action {
	case ActionDaemonReload:
		logger.Info("Daemon-reload action does not have a direct precheck state, will always run if scheduled.")
		return false, nil
	case ActionStart, ActionRestart:
		active, err := runnerSvc.IsServiceActive(ctx.GoContext(), conn, nil, s.ServiceName)
		if err != nil {
			logger.Warn("Failed to check if service is active, assuming it needs action.", "service", s.ServiceName, "error", err)
			return false, nil
		}
		if s.Action == ActionStart && active {
			logger.Info("Service is already active, start action satisfied.", "service", s.ServiceName)
			return true, nil
		}
		if s.Action == ActionRestart {
			logger.Info("Restart action will proceed.", "service", s.ServiceName, "currently_active", active)
			return false, nil
		}
		return false, nil
	case ActionStop:
		active, err := runnerSvc.IsServiceActive(ctx.GoContext(), conn, nil, s.ServiceName)
		if err != nil {
			logger.Warn("Failed to check if service is active for stop precheck, assuming it needs action.", "service", s.ServiceName, "error", err)
			return false, nil
		}
		if !active {
			logger.Info("Service is already stopped, stop action satisfied.", "service", s.ServiceName)
			return true, nil
		}
		return false, nil
	case ActionEnable:
		cmd := fmt.Sprintf("systemctl is-enabled %s", s.ServiceName)
		stdout, _, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: false})
		if err != nil {
			if cmdErr, ok := err.(*connector.CommandError); ok && cmdErr.ExitCode != 0 {
				logger.Info("Service is not enabled (is-enabled returned non-zero).", "service", s.ServiceName, "exit_code", cmdErr.ExitCode, "stdout", string(stdout))
				return false, nil
			}
			logger.Warn("Failed to check if service is enabled (unexpected error), assuming it needs action.", "command", cmd, "error", err)
			return false, nil
		}
		if strings.TrimSpace(string(stdout)) == "enabled" {
			logger.Info("Service is already enabled.", "service", s.ServiceName)
			return true, nil
		}
		logger.Info("Service is not enabled.", "service", s.ServiceName, "status_output", string(stdout))
		return false, nil
	case ActionDisable:
		cmd := fmt.Sprintf("systemctl is-enabled %s", s.ServiceName)
		stdout, _, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: false})
		if err != nil {
			if cmdErr, ok := err.(*connector.CommandError); ok && cmdErr.ExitCode != 0 {
				logger.Info("Service is already not enabled (is-enabled returned non-zero), disable action satisfied.", "service", s.ServiceName, "exit_code", cmdErr.ExitCode, "stdout", string(stdout))
				return true, nil
			}
			logger.Warn("Failed to check if service is enabled for disable precheck (unexpected error), assuming it needs action.", "command", cmd, "error", err)
			return false, nil
		}
		if strings.TrimSpace(string(stdout)) != "enabled" {
			logger.Info("Service is already not enabled (or static), disable action satisfied.", "service", s.ServiceName, "status_output", string(stdout))
			return true, nil
		}
		return false, nil
	case ActionIsActive, ActionIsEnabled:
		logger.Info("Query actions used in Precheck context are for assertion by the caller, will always run if scheduled.", "action", s.Action)
		return false, nil
	default:
		return false, fmt.Errorf("unsupported service action for precheck: %s", s.Action)
	}
}

func (s *ManageServiceStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	var cmd string
	if s.Action == ActionDaemonReload {
		cmd = "systemctl daemon-reload"
	} else {
		cmd = fmt.Sprintf("systemctl %s %s", s.Action, s.ServiceName)
	}

	logger.Info("Executing systemctl command.", "command", cmd)
	runOpts := &connector.ExecOptions{Sudo: s.Sudo}

	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, runOpts)

	if err != nil {
		if cmdErr, ok := err.(*connector.CommandError); ok {
			if s.Action == ActionIsActive {
				logger.Info("Service is-active check result.", "exit_code", cmdErr.ExitCode, "stdout", string(stdout), "stderr", string(stderr))
				ctx.GetTaskCache().Set(fmt.Sprintf("service.%s.isActive", s.ServiceName), cmdErr.ExitCode == 0)
				return nil
			}
			if s.Action == ActionIsEnabled {
				logger.Info("Service is-enabled check result.", "exit_code", cmdErr.ExitCode, "stdout", string(stdout), "stderr", string(stderr))
				ctx.GetTaskCache().Set(fmt.Sprintf("service.%s.isEnabled", s.ServiceName), strings.TrimSpace(string(stdout)) == "enabled")
				return nil
			}
		}
		logger.Error("Systemctl command failed.", "command", cmd, "error", err, "stdout", string(stdout), "stderr", string(stderr))
		return fmt.Errorf("systemctl command '%s' failed: %w. Stderr: %s", cmd, err, string(stderr))
	}
	if s.Action == ActionIsActive {
		ctx.GetTaskCache().Set(fmt.Sprintf("service.%s.isActive", s.ServiceName), true)
	}
	if s.Action == ActionIsEnabled {
		ctx.GetTaskCache().Set(fmt.Sprintf("service.%s.isEnabled", s.ServiceName), strings.TrimSpace(string(stdout)) == "enabled")
	}

	logger.Info("Systemctl command executed successfully.", "command", cmd, "stdout", string(stdout))
	return nil
}

func (s *ManageServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("No specific rollback action defined for generic service management step.", "original_action", s.Action)
	return nil
}

var _ step.Step = (*ManageServiceStep)(nil)
