package cri_dockerd

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// AllowedCriDockerdServiceActions lists valid actions. Using a new map for clarity.
var AllowedCriDockerdServiceActions = map[string]bool{
	"start":         true,
	"stop":          true,
	"restart":       true,
	"enable":        true,
	"disable":       true,
	"daemon-reload": true,
}

// ManageCriDockerdServiceStepSpec defines parameters for managing the cri-dockerd service and socket.
type ManageCriDockerdServiceStepSpec struct {
	spec.StepMeta `json:",inline"`

	ServiceName  string `json:"serviceName,omitempty"`
	SocketName   string `json:"socketName,omitempty"`
	Action       string `json:"action,omitempty"` // "start", "stop", "restart", "enable", "disable", "daemon-reload"
	ManageSocket bool   `json:"manageSocket,omitempty"`
	EnableNow    bool   `json:"enableNow,omitempty"`
	Sudo         bool   `json:"sudo,omitempty"`
}

// NewManageCriDockerdServiceStepSpec creates a new ManageCriDockerdServiceStepSpec.
func NewManageCriDockerdServiceStepSpec(name, description, action string) *ManageCriDockerdServiceStepSpec {
	finalName := name
	if finalName == "" {
		finalName = fmt.Sprintf("%s cri-dockerd service", strings.Title(action))
	}
	finalDescription := description
	// Description refined in populateDefaults

	return &ManageCriDockerdServiceStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		Action:       action,
		ManageSocket: true, // Default ManageSocket to true
		// Other defaults in populateDefaults
	}
}

// Name returns the step's name.
func (s *ManageCriDockerdServiceStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *ManageCriDockerdServiceStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *ManageCriDockerdServiceStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *ManageCriDockerdServiceStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *ManageCriDockerdServiceStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ManageCriDockerdServiceStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *ManageCriDockerdServiceStepSpec) populateDefaults(logger runtime.Logger) {
	if s.ServiceName == "" {
		s.ServiceName = "cri-docker.service"
		logger.Debug("ServiceName defaulted.", "name", s.ServiceName)
	}
	if s.SocketName == "" {
		s.SocketName = "cri-docker.socket"
		logger.Debug("SocketName defaulted.", "name", s.SocketName)
	}

	// Default ManageSocket to true if not explicitly set by user.
	// Bool zero value is false. If user wants it true, they set it.
	// If they don't set it, it's false. To make it default true:
	// ManageSocket is now defaulted to true in the factory.
	// The following line could be used if we needed to ensure it's true if it somehow got reset,
	// but typically not needed if factory sets it and user can override.
	// if !s.ManageSocket && !utils.IsFieldExplicitlySet(s, "ManageSocket") { // Hypothetical IsFieldExplicitlySet
	//     s.ManageSocket = true
	//     logger.Debug("ManageSocket (re)defaulted to true.")
	// }

	s.Action = strings.ToLower(s.Action)
	if !AllowedCriDockerdServiceActions[s.Action] {
		logger.Warn("Invalid Action specified, defaulting to no-op (will error in Run/Precheck).", "action", s.Action)
	}

	if !s.Sudo { // Default to true if not explicitly set false
		s.Sudo = true
		logger.Debug("Sudo defaulted to true.")
	}

	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Manages cri-dockerd service ('%s') and socket ('%s') with action '%s'.",
			s.ServiceName, s.SocketName, s.Action)
	}
}

// isServiceStateCorrect checks if a unit (service or socket) is in the desired active/enabled state.
func isServiceStateCorrect(ctx runtime.StepContext, conn connector.Connector, unitName string, action string, sudo bool) (bool, error) {
	logger := ctx.GetLogger().With("unit", unitName, "action", action)
	execOpts := &connector.ExecOptions{Sudo: sudo}
	var checkCmd string
	var expectedState bool // true if command success (exit 0) means desired state

	switch action {
	case "start", "restart": // For precheck, if starting, done if active. Restart always runs.
		checkCmd = fmt.Sprintf("systemctl is-active %s", unitName)
		expectedState = true
		if action == "restart" { return false, nil } // Restart action should always run
	case "enable":
		checkCmd = fmt.Sprintf("systemctl is-enabled %s", unitName)
		expectedState = true
	case "stop":
		checkCmd = fmt.Sprintf("systemctl is-active %s", unitName)
		expectedState = false // Done if "inactive" (command fails/non-zero exit)
	case "disable":
		checkCmd = fmt.Sprintf("systemctl is-enabled %s", unitName)
		expectedState = false // Done if "disabled" (command returns non-zero for is-enabled)
	default:
		return false, fmt.Errorf("unsupported action for state check: %s", action)
	}

	stdout, _, err := conn.Exec(ctx.GoContext(), checkCmd, execOpts)
	outputStatus := strings.TrimSpace(string(stdout))

	if err != nil { // Command failed (non-zero exit)
		if !expectedState { // e.g., for stop/disable, non-zero exit means it's already stopped/disabled
			logger.Debug("Unit is in desired state (command failed as expected).", "status", outputStatus, "error", err)
			return true, nil
		}
		logger.Debug("Unit not in desired state (command failed).", "status", outputStatus, "error", err)
		return false, nil
	}
	// Command succeeded (exit 0)
	if expectedState { // e.g., for start/enable, exit 0 means it's active/enabled
		logger.Debug("Unit is in desired state (command succeeded).", "status", outputStatus)
		return true, nil
	}
	logger.Debug("Unit not in desired state (command succeeded but indicates wrong state).", "status", outputStatus)
	return false, nil
}

// Precheck determines if the cri-dockerd service/socket are already in the desired state.
func (s *ManageCriDockerdServiceStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger, ctx.GetStepID()) // Pass StepID for unique TargetDir in populateDefaults

	if s.Action == "" || !AllowedCriDockerdServiceActions[s.Action] {
		return false, fmt.Errorf("invalid or unspecified Action '%s' for %s", s.Action, s.GetName())
	}
	if s.Action == "daemon-reload" {
		logger.Debug("Action is daemon-reload, Precheck returns false to ensure it runs.")
		return false, nil
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	serviceDone, err := isServiceStateCorrect(ctx, conn, s.ServiceName, s.Action, s.Sudo)
	if err != nil { return false, err } // Error during check

	if s.ManageSocket && (s.Action == "start" || s.Action == "stop" || s.Action == "enable" || s.Action == "disable") {
		socketDone, err := isServiceStateCorrect(ctx, conn, s.SocketName, s.Action, s.Sudo)
		if err != nil { return false, err }
		if serviceDone && socketDone {
			logger.Info("Both cri-dockerd service and socket are already in the desired state.")
			return true, nil
		}
	} else if serviceDone { // Only service or action doesn't involve socket in precheck logic like restart
		logger.Info("cri-dockerd service is already in the desired state.")
		return true, nil
	}

	logger.Info("cri-dockerd service/socket not in desired state. Step needs to run.")
	return false, nil
}

// Run executes systemctl commands for cri-dockerd service and socket.
func (s *ManageCriDockerdServiceStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger, ctx.GetStepID())

	if s.Action == "" || !AllowedCriDockerdServiceActions[s.Action] {
		return fmt.Errorf("invalid or unspecified Action '%s' for %s", s.Action, s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	if s.Action == "daemon-reload" {
		logger.Info("Executing systemctl daemon-reload.")
		_, stderr, err := conn.Exec(ctx.GoContext(), "systemctl daemon-reload", execOpts)
		if err != nil {
			return fmt.Errorf("failed to execute daemon-reload (stderr: %s): %w", string(stderr), err)
		}
		logger.Info("Systemctl daemon-reload executed successfully.")
		return nil
	}

	unitsToManage := []string{}
	if s.ManageSocket && (s.Action == "enable" || s.Action == "disable" || s.Action == "start" || s.Action == "stop") {
		// Order can matter: for enable/start, often socket first. For stop/disable, service first.
		if s.Action == "enable" || s.Action == "start" {
			unitsToManage = append(unitsToManage, s.SocketName)
			unitsToManage = append(unitsToManage, s.ServiceName)
		} else { // stop, disable
			unitsToManage = append(unitsToManage, s.ServiceName)
			unitsToManage = append(unitsToManage, s.SocketName)
		}
	} else {
		unitsToManage = append(unitsToManage, s.ServiceName)
	}

	for _, unitName := range unitsToManage {
		if unitName == "" { continue }

		actionCmd := s.Action
		if s.Action == "enable" && s.EnableNow {
			// If it's the socket, "enable --now" is fine.
			// If it's the service, and socket is also managed, "enable --now" on socket might start service.
			// Or, just "enable" then "start". For simplicity, if EnableNow is true, apply to both.
			actionCmd = "enable --now"
		} else if s.Action == "disable" && s.ManageSocket {
		    // If disabling socket, also stop it. If disabling service, also stop it.
		    // This is a common pattern. "systemctl disable foo" doesn't stop "foo".
		    // We can issue a stop command before disable for the unit.
		    stopUnitCmd := fmt.Sprintf("systemctl stop %s", unitName)
		    logger.Info("Stopping unit before disabling.", "command", stopUnitCmd)
		    if _, stderr, errStop := conn.Exec(ctx.GoContext(), stopUnitCmd, execOpts); errStop != nil {
		        logger.Warn("Failed to stop unit before disabling (might be already stopped or not exist).", "unit", unitName, "stderr", string(stderr), "error", errStop)
		    }
		}


		cmd := fmt.Sprintf("systemctl %s %s", actionCmd, unitName)
		logger.Info("Executing command for unit.", "unit", unitName, "command", cmd)
		_, stderr, errCmd := conn.Exec(ctx.GoContext(), cmd, execOpts)
		if errCmd != nil {
			errMsg := strings.ToLower(string(stderr) + errCmd.Error())
			isNotFound := strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "no such file") || strings.Contains(errMsg, "does not exist")
			if (s.Action == "stop" || s.Action == "disable") && isNotFound {
				logger.Info("Command to stop/disable unit indicated it was not found. Considered successful for the action.", "unit", unitName, "command", cmd, "stderr", string(stderr))
			} else {
				return fmt.Errorf("failed to execute '%s' for unit %s (stderr: %s): %w", cmd, unitName, string(stderr), errCmd)
			}
		} else {
			logger.Info("Command executed successfully for unit.", "unit", unitName, "command", cmd)
		}
	}
	return nil
}

// Rollback for cri-dockerd service management.
func (s *ManageCriDockerdServiceStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger, ctx.GetStepID())
	logger.Warn("Rollback for cri-dockerd service management action is not automatically performed or is best-effort.", "action", s.Action)
	// Consider inverse actions for 'start' -> 'stop', 'enable' -> 'disable' if critical.
	return nil
}

var _ step.Step = (*ManageCriDockerdServiceStepSpec)(nil)
