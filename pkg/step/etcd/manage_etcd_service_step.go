package etcd

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
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
	ActionIsActive      ServiceAction = "is-active" // For precheck
	ActionIsEnabled     ServiceAction = "is-enabled"  // For precheck
)

// ManageEtcdServiceStep performs a systemctl action on the etcd service.
type ManageEtcdServiceStep struct {
	meta        spec.StepMeta
	Action      ServiceAction
	ServiceName string // Defaults to "etcd"
	Sudo        bool
}

// NewManageEtcdServiceStep creates a new ManageEtcdServiceStep.
func NewManageEtcdServiceStep(instanceName string, action ServiceAction, serviceName string, sudo bool) step.Step {
	name := instanceName
	svcName := serviceName
	if svcName == "" {
		svcName = "etcd"
	}
	if name == "" {
		name = fmt.Sprintf("ManageEtcdService-%s-%s", strings.Title(string(action)), svcName)
	}

	return &ManageEtcdServiceStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Performs systemctl %s on %s service.", action, svcName),
		},
		Action:      action,
		ServiceName: svcName,
		Sudo:        true, // Systemctl actions usually require sudo
	}
}

func (s *ManageEtcdServiceStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *ManageEtcdServiceStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Precheck logic depends on the action
	switch s.Action {
	case ActionDaemonReload:
		logger.Info("Daemon-reload action does not have a direct precheck state, will always run if scheduled.")
		return false, nil // Always run daemon-reload if it's part of the plan
	case ActionStart, ActionRestart: // For start/restart, precheck if it's already active
		active, err := runnerSvc.IsServiceActive(ctx.GoContext(), conn, nil /*facts*/, s.ServiceName)
		if err != nil {
			logger.Warn("Failed to check if service is active, assuming it needs action.", "service", s.ServiceName, "error", err)
			return false, nil
		}
		if s.Action == ActionStart && active {
			logger.Info("Service is already active, start action satisfied.", "service", s.ServiceName)
			return true, nil
		}
		if s.Action == ActionRestart && !active { // If restarting a stopped service, that's fine, Run will start it.
			logger.Info("Service is not active, restart will effectively start it.", "service", s.ServiceName)
			return false, nil // Let Run proceed
		}
		if s.Action == ActionRestart && active {
			logger.Info("Service is active, restart action will proceed.", "service", s.ServiceName)
			return false, nil // Let Run proceed to actually restart
		}
		return false, nil // Needs start/restart
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
		return false, nil // Needs stop
	case ActionEnable:
		// Runner needs an IsServiceEnabled method. For now, assume we always run 'enable'.
		// Or, parse `systemctl is-enabled <service>` output.
		// Simplified: always run 'enable' as it's usually harmless to re-enable.
		// A more robust check would parse "systemctl is-enabled serviceName"
		cmd := fmt.Sprintf("systemctl is-enabled %s", s.ServiceName)
		stdout, _, err := runnerSvc.Run(ctx.GoContext(), conn, cmd, false) // is-enabled usually doesn't need sudo
		if err != nil {
			// Non-zero exit for 'disabled' or 'static', error for not found.
			// If it errors (e.g. service not found), let Run proceed and fail there.
			logger.Warn("Failed to check if service is enabled, assuming it needs action.", "command", cmd, "error", err)
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
		stdout, _, err := runnerSvc.Run(ctx.GoContext(), conn, cmd, false)
		if err != nil {
			logger.Warn("Failed to check if service is enabled for disable precheck, assuming it needs action.", "command", cmd, "error", err)
			return false, nil
		}
		// is-enabled returns "disabled" or "static" (or others) if not enabled.
		// If it's anything other than "enabled", disable action might be considered done or not applicable.
		if strings.TrimSpace(string(stdout)) != "enabled" {
			logger.Info("Service is already not enabled (or static), disable action satisfied.", "service", s.ServiceName, "status_output", string(stdout))
			return true, nil
		}
		return false, nil // Needs disable
	case ActionIsActive, ActionIsEnabled: // These are query actions, not state-changing for Precheck.
		logger.Info("Query actions (is-active, is-enabled) used in Precheck context are for assertion by the caller, not for skipping this step.", "action", s.Action)
		return false, nil // This step itself, if it's a query, should always run to perform the query.
	default:
		return false, fmt.Errorf("unsupported service action for precheck: %s", s.Action)
	}
}

func (s *ManageEtcdServiceStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	var cmd string
	if s.Action == ActionDaemonReload {
		cmd = "systemctl daemon-reload"
	} else if s.Action == ActionIsActive || s.Action == ActionIsEnabled {
		cmd = fmt.Sprintf("systemctl %s %s", s.Action, s.ServiceName)
	} else {
		cmd = fmt.Sprintf("systemctl %s %s", s.Action, s.ServiceName)
	}

	logger.Info("Executing systemctl command.", "command", cmd)
	// For state-changing commands, output is usually not critical unless there's an error.
	// For query commands (is-active, is-enabled), output and exit code are important.
	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: s.Sudo, Check: true})

	if err != nil {
		// For 'is-active' and 'is-enabled', a non-zero exit code is expected if the state is false.
		// The 'Check: true' option in RunWithOptions means `err` will be a CommandError for non-zero exits.
		if cmdErr, ok := err.(*connector.CommandError); ok {
			if s.Action == ActionIsActive { // is-active: 0 for active, non-zero for inactive/failed
				// Store result in cache? Or rely on caller to check CommandError.ExitCode
				logger.Info("Service is-active check result.", "exit_code", cmdErr.ExitCode, "stdout", string(stdout), "stderr", string(stderr))
				if cmdErr.ExitCode == 0 { // Active
					// Potentially set cache: ctx.TaskCache().Set(fmt.Sprintf("service.%s.isActive", s.ServiceName), true)
				} else { // Inactive or other state
					// Potentially set cache: ctx.TaskCache().Set(fmt.Sprintf("service.%s.isActive", s.ServiceName), false)
				}
				return nil // The command ran, result is in exit code / output.
			}
			if s.Action == ActionIsEnabled { // is-enabled: 0 for enabled, 1 for disabled/static/masked etc.
				logger.Info("Service is-enabled check result.", "exit_code", cmdErr.ExitCode, "stdout", string(stdout), "stderr", string(stderr))
				// Potentially set cache based on strings.TrimSpace(string(stdout))
				return nil // Command ran.
			}
		}
		// For other actions, any error is a failure.
		logger.Error("Systemctl command failed.", "command", cmd, "error", err, "stdout", string(stdout), "stderr", string(stderr))
		return fmt.Errorf("systemctl command '%s' failed: %w. Stderr: %s", cmd, err, string(stderr))
	}

	logger.Info("Systemctl command executed successfully.", "command", cmd, "stdout", string(stdout))
	return nil
}

func (s *ManageEtcdServiceStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	// Rollback for service management is tricky and context-dependent.
	// E.g., if Action was 'start', rollback might be 'stop'.
	// If Action was 'enable', rollback might be 'disable'.
	// Daemon-reload has no direct rollback.
	// For simplicity, this generic step does not implement intelligent rollback of the specific action.
	// Tasks requiring specific rollback sequences for service states should plan them explicitly.
	logger.Info("No specific rollback action defined for generic service management step.", "original_action", s.Action)
	return nil
}

var _ step.Step = (*ManageEtcdServiceStep)(nil)
