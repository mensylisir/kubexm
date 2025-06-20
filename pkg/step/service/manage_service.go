package service

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// ServiceAction defines the type for service management actions.
type ServiceAction string

const (
	ActionStart         ServiceAction = "start"
	ActionStop          ServiceAction = "stop"
	ActionRestart       ServiceAction = "restart"
	ActionEnable        ServiceAction = "enable"
	ActionDisable       ServiceAction = "disable"
	ActionDaemonReload  ServiceAction = "daemon-reload"
	ActionStatus        ServiceAction = "status" // For precheck
)

// ManageServiceStepSpec defines parameters for managing a system service (e.g., via systemd).
type ManageServiceStepSpec struct {
	spec.StepMeta `json:",inline"`

	ServiceName string        `json:"serviceName,omitempty"`
	Action      ServiceAction `json:"action,omitempty"`
	// ExpectedStatus string // For more advanced precheck, e.g. "active", "inactive", "enabled", "disabled"
}

// NewManageServiceStepSpec creates a new ManageServiceStepSpec.
func NewManageServiceStepSpec(name, description, serviceName string, action ServiceAction) *ManageServiceStepSpec {
	finalName := name
	if finalName == "" {
		finalName = fmt.Sprintf("%s service %s", strings.Title(string(action)), serviceName)
	}
	finalDescription := description
	if finalDescription == "" {
		finalDescription = fmt.Sprintf("Performs action '%s' on service '%s'", action, serviceName)
	}

	return &ManageServiceStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		ServiceName: serviceName,
		Action:      action,
	}
}

// Name returns the step's name.
func (s *ManageServiceStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *ManageServiceStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *ManageServiceStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *ManageServiceStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *ManageServiceStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ManageServiceStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

// Precheck attempts to determine if the service is already in the desired state.
// This is a simplified precheck. A more robust one would parse `systemctl status` or `is-active`/`is-enabled`.
func (s *ManageServiceStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")

	if s.ServiceName == "" || s.Action == "" {
		return false, fmt.Errorf("ServiceName and Action must be specified for ManageServiceStep: %s", s.GetName())
	}

	// Daemon-reload should always run if specified.
	if s.Action == ActionDaemonReload {
	    logger.Debug("Action is daemon-reload, Precheck returns false to ensure it runs.")
	    return false, nil
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	var checkCmd string
	var expectedToFind bool // True if finding the pattern means "done"

	switch s.Action {
	case ActionStart, ActionRestart: // If starting/restarting, done if already active
		checkCmd = fmt.Sprintf("systemctl is-active %s", s.ServiceName)
		expectedToFind = true // "active" means done
	case ActionEnable: // If enabling, done if already enabled
		checkCmd = fmt.Sprintf("systemctl is-enabled %s", s.ServiceName)
		expectedToFind = true // "enabled" means done
	case ActionStop, ActionDisable: // If stopping/disabling, done if already inactive/disabled
		// is-active for stop, is-enabled for disable
		if s.Action == ActionStop {
			checkCmd = fmt.Sprintf("systemctl is-active %s", s.ServiceName)
			expectedToFind = false // "inactive" (or error from is-active) means done
		} else { // ActionDisable
			checkCmd = fmt.Sprintf("systemctl is-enabled %s", s.ServiceName)
			expectedToFind = false // "disabled" (or error from is-enabled) means done
		}
	default:
		logger.Debug("No precheck logic for action.", "action", s.Action)
		return false, nil // Let Run handle it
	}

	logger.Debug("Executing service status check command.", "command", checkCmd)
	// systemctl is-active returns 0 if active, non-0 otherwise (usually 3 for inactive)
	// systemctl is-enabled returns 0 if enabled, 1 if disabled/static/indirect
	stdout, _, err := conn.Exec(ctx.GoContext(), checkCmd, &connector.ExecOptions{Sudo: true}) // systemctl usually needs sudo

	output := strings.TrimSpace(string(stdout))

	if err != nil { // Command failed (e.g. service not found, or returned non-zero for is-active/is-enabled)
		if expectedToFind { // e.g. wanted 'active' or 'enabled', but got error (implying not in that state)
			logger.Info("Service is not in the desired state (command failed).", "action", s.Action, "output", output, "error", err)
			return false, nil
		}
		// Wanted 'inactive' or 'disabled'. Error from is-active/is-enabled often means it is indeed inactive/disabled.
		logger.Info("Service is likely in the desired state (command failed as expected for inactive/disabled).", "action", s.Action, "output", output, "error", err)
		return true, nil
	}

	// Command succeeded (exit code 0)
	if expectedToFind { // e.g. is-active returned "active", or is-enabled returned "enabled"
		logger.Info("Service is already in the desired state.", "action", s.Action, "status", output)
		return true, nil
	}
	// e.g. is-active returned "active" but we want to stop it.
	logger.Info("Service is not in the desired state (command succeeded but indicates wrong state).", "action", s.Action, "status", output)
	return false, nil
}

// Run executes the service management command.
func (s *ManageServiceStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")

	if s.ServiceName == "" || s.Action == "" {
		return fmt.Errorf("ServiceName and Action must be specified for ManageServiceStep: %s", s.GetName())
	}

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

	logger.Info("Executing service command.", "command", cmd)
	// Most systemctl actions require sudo.
	_, stderr, err := conn.Exec(ctx.GoContext(), cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		// Special case for 'systemctl disable': if service is not found, it might return non-zero.
		// Some systems return 0 even if service doesn't exist for disable/stop.
		// This default implementation considers any error as a failure.
		return fmt.Errorf("failed to execute command '%s' on host %s (stderr: %s): %w", cmd, host.GetName(), string(stderr), err)
	}

	logger.Info("Service command executed successfully.", "command", cmd)
	return nil
}

// Rollback for service management can be complex.
// A simple approach might be to do nothing, or for 'start' try 'stop', for 'enable' try 'disable'.
// For this generic step, we'll make it a no-op unless specific inverse logic is required.
func (s *ManageServiceStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Info("ManageServiceStep Rollback is a no-op by default for most actions.")
	// Example of a more specific rollback:
	// if s.Action == ActionStart { // if we started it, try to stop it
	//    stopSpec := NewManageServiceStepSpec("", "", s.ServiceName, ActionStop)
	//    logger.Info("Attempting to stop service as part of rollback for start action.")
	//    return stopSpec.Run(ctx, host)
	// }
	return nil
}

var _ step.Step = (*ManageServiceStepSpec)(nil)
