package common

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	// Ensure runtime is imported if its types are used directly,
	// but step.StepContext interface should be used in method signatures.
	// "github.com/mensylisir/kubexm/pkg/runtime"
)

// ServiceAction defines the allowed actions for managing a systemd service.
type ServiceAction string

const (
	ActionStart         ServiceAction = "start"
	ActionStop          ServiceAction = "stop"
	ActionRestart       ServiceAction = "restart"
	ActionEnable        ServiceAction = "enable"
	ActionDisable       ServiceAction = "disable"
	ActionDaemonReload  ServiceAction = "daemon-reload"
	ActionIsActive      ServiceAction = "is-active" // For precheck/query
	ActionIsEnabled     ServiceAction = "is-enabled"  // For precheck/query
)

// ManageServiceStep performs a systemctl action on a given service.
type ManageServiceStep struct {
	meta        spec.StepMeta
	Action      ServiceAction
	ServiceName string // Mandatory: the name of the service to manage
	Sudo        bool
}

// NewManageServiceStep creates a new ManageServiceStep.
// serviceName is mandatory.
func NewManageServiceStep(instanceName string, action ServiceAction, serviceName string, sudo bool) step.Step {
	name := instanceName
	if serviceName == "" {
		// This should ideally be caught by validation or be a programming error,
		// as serviceName is critical for this step.
		// However, to prevent panic if it somehow happens:
		panic("serviceName cannot be empty for NewManageServiceStep")
	}
	if name == "" {
		name = fmt.Sprintf("ManageService-%s-%s", strings.Title(string(action)), serviceName)
	}

	return &ManageServiceStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Performs systemctl %s on %s service.", action, serviceName),
		},
		Action:      action,
		ServiceName: serviceName,
		Sudo:        sudo, // Sudo is often required for systemctl
	}
}

func (s *ManageServiceStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *ManageServiceStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
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
		if s.Action == ActionRestart { // For restart, always proceed if no error
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
		// Use RunWithOptions with Check:true to handle non-zero exits gracefully for is-enabled
		stdout, _, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: false, Check: true})
		if err != nil {
			if cmdErr, ok := err.(*connector.CommandError); ok && cmdErr.ExitCode != 0 { // e.g. exit code 1 for "disabled"
                 // This means it's not "enabled"
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
		stdout, _, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: false, Check: true})
		if err != nil {
			if cmdErr, ok := err.(*connector.CommandError); ok && cmdErr.ExitCode != 0 { // e.g. exit code 1 for "disabled"
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

func (s *ManageServiceStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	var cmd string
	if s.Action == ActionDaemonReload {
		cmd = "systemctl daemon-reload"
	} else {
		cmd = fmt.Sprintf("systemctl %s %s", s.Action, s.ServiceName)
	}

	logger.Info("Executing systemctl command.", "command", cmd)
	// For is-active and is-enabled, Check:true allows us to inspect the exit code.
	// For other actions, we want an error if the command fails (non-zero exit).
	runOpts := &connector.ExecOptions{Sudo: s.Sudo}
	if s.Action == ActionIsActive || s.Action == ActionIsEnabled {
		runOpts.Check = true
	}

	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, runOpts)

	if err != nil {
		if cmdErr, ok := err.(*connector.CommandError); ok {
			if s.Action == ActionIsActive {
				logger.Info("Service is-active check result.", "exit_code", cmdErr.ExitCode, "stdout", string(stdout), "stderr", string(stderr))
				// Store actual state in cache for other steps/tasks to query if needed
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

	// Success (exit code 0)
	if s.Action == ActionIsActive {
		ctx.GetTaskCache().Set(fmt.Sprintf("service.%s.isActive", s.ServiceName), true)
	}
	if s.Action == ActionIsEnabled {
		ctx.GetTaskCache().Set(fmt.Sprintf("service.%s.isEnabled", s.ServiceName), strings.TrimSpace(string(stdout)) == "enabled")
	}

	logger.Info("Systemctl command executed successfully.", "command", cmd, "stdout", string(stdout))
	return nil
}

func (s *ManageServiceStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("No specific rollback action defined for generic service management step.", "original_action", s.Action)
	return nil
}

var _ step.Step = (*ManageServiceStep)(nil)
