package docker

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	// Re-using ServiceAction type from a common place would be ideal,
	// but for now, defining it locally or assuming string for action.
	// For this implementation, let's use plain strings and validate.
)

// AllowedDockerServiceActions lists the valid actions for ManageDockerServiceStepSpec.
var AllowedDockerServiceActions = map[string]bool{
	"start":         true,
	"stop":          true,
	"restart":       true,
	"enable":        true,
	"disable":       true,
	"daemon-reload": true,
}

// ManageDockerServiceStepSpec defines parameters for managing the Docker systemd service.
type ManageDockerServiceStepSpec struct {
	spec.StepMeta `json:",inline"`

	ServiceName string `json:"serviceName,omitempty"`
	Action      string `json:"action,omitempty"` // "start", "stop", "restart", "enable", "disable", "daemon-reload"
	Sudo        bool   `json:"sudo,omitempty"`
}

// NewManageDockerServiceStepSpec creates a new ManageDockerServiceStepSpec.
func NewManageDockerServiceStepSpec(name, description, action string) *ManageDockerServiceStepSpec {
	finalName := name
	if finalName == "" {
		finalName = fmt.Sprintf("%s Docker service", strings.Title(action))
	}
	finalDescription := description
	// Description refined in populateDefaults

	return &ManageDockerServiceStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		Action: action,
		// ServiceName and Sudo defaulted in populateDefaults
	}
}

// Name returns the step's name.
func (s *ManageDockerServiceStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *ManageDockerServiceStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *ManageDockerServiceStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *ManageDockerServiceStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *ManageDockerServiceStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ManageDockerServiceStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *ManageDockerServiceStepSpec) populateDefaults(logger runtime.Logger) {
	if s.ServiceName == "" {
		s.ServiceName = "docker"
		logger.Debug("ServiceName defaulted.", "name", s.ServiceName)
	}
	s.Action = strings.ToLower(s.Action)
	if !AllowedDockerServiceActions[s.Action] {
		logger.Warn("Invalid or empty Action specified, defaulting to 'status' for safety (no-op).", "action", s.Action)
		// This default means it won't do anything harmful if action is wrong.
		// Alternatively, could error out in Precheck/Run if action is invalid.
		// For now, let's make it a no-op by setting a non-modifying action if invalid.
		// However, the prompt implies action is provided. Let's assume it's valid or Run will error.
	}

	if !s.Sudo { // Default to true if not explicitly set false
		s.Sudo = true
		logger.Debug("Sudo defaulted to true.")
	}

	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Performs action '%s' on Docker service '%s'.", s.Action, s.ServiceName)
	}
}

// Precheck determines if the Docker service is already in the desired state.
func (s *ManageDockerServiceStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.ServiceName == "" || s.Action == "" {
		return false, fmt.Errorf("ServiceName and Action must be specified for %s", s.GetName())
	}
	if !AllowedDockerServiceActions[s.Action] {
		return false, fmt.Errorf("invalid Action '%s' specified for %s", s.Action, s.GetName())
	}

	if s.Action == "daemon-reload" {
		logger.Debug("Action is daemon-reload, Precheck returns false to ensure it runs.")
		return false, nil
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	var checkCmd string
	var expectedToFind bool // True if the success of checkCmd (exit 0) means "done"

	switch s.Action {
	case "start", "restart":
		checkCmd = fmt.Sprintf("systemctl is-active %s", s.ServiceName)
		expectedToFind = true // "active" (exit 0) means done for start, restart always runs (handled by returning false)
		if s.Action == "restart" { expectedToFind = false } // Restart should always run
	case "enable":
		checkCmd = fmt.Sprintf("systemctl is-enabled %s", s.ServiceName)
		expectedToFind = true // "enabled" (exit 0) means done
	case "stop":
		checkCmd = fmt.Sprintf("systemctl is-active %s", s.ServiceName)
		expectedToFind = false // "inactive" (non-exit 0) means done
	case "disable":
		checkCmd = fmt.Sprintf("systemctl is-enabled %s", s.ServiceName)
		expectedToFind = false // "disabled" (non-exit 0 for systemctl is-enabled) means done
	default:
		logger.Debug("No specific precheck logic for action, will run.", "action", s.Action)
		return false, nil
	}

	logger.Debug("Executing service status check command.", "command", checkCmd)
	stdout, _, err := conn.Exec(ctx.GoContext(), checkCmd, execOpts)
	outputStatus := strings.TrimSpace(string(stdout))

	if err != nil { // Command failed (e.g., service not found, or is-active/is-enabled returned non-zero)
		if expectedToFind { // e.g., wanted 'active' or 'enabled', but got error (implying not in that state)
			logger.Info("Service is not in the desired state (check command failed).", "action", s.Action, "output", outputStatus, "error", err)
			return false, nil
		}
		// Wanted 'inactive' or 'disabled'. Error from is-active/is-enabled often means it is indeed in that state.
		logger.Info("Service is likely in the desired state (check command failed as expected for inactive/disabled).", "action", s.Action, "output", outputStatus, "error", err)
		return true, nil
	}

	// Command succeeded (exit code 0)
	if expectedToFind { // e.g., is-active returned "active", or is-enabled returned "enabled"
		logger.Info("Service is already in the desired state.", "action", s.Action, "status", outputStatus)
		return true, nil
	}
	// e.g., is-active returned "active" (command success) but we want to stop it.
	logger.Info("Service is not in the desired state (check command succeeded but indicates wrong state).", "action", s.Action, "status", outputStatus)
	return false, nil
}

// Run executes the systemctl command for the Docker service.
func (s *ManageDockerServiceStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.ServiceName == "" || s.Action == "" {
		return fmt.Errorf("ServiceName and Action must be specified for %s", s.GetName())
	}
	if !AllowedDockerServiceActions[s.Action] {
		return fmt.Errorf("invalid Action '%s' specified for %s", s.Action, s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	var cmd string
	if s.Action == "daemon-reload" {
		cmd = "systemctl daemon-reload"
	} else {
		cmd = fmt.Sprintf("systemctl %s %s", s.Action, s.ServiceName)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	logger.Info("Executing Docker service command.", "command", cmd)
	_, stderr, err := conn.Exec(ctx.GoContext(), cmd, execOpts)
	if err != nil {
		// For stop/disable, if service not found, it's often not a true error for the desired state.
		errMsg := strings.ToLower(string(stderr) + err.Error())
		if (s.Action == "stop" || s.Action == "disable") &&
		   (strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "no such file") || strings.Contains(errMsg, "does not exist")) {
			logger.Warn("Service not found while trying to stop/disable, considering it a success for the action.", "command", cmd, "stderr", string(stderr))
			return nil
		}
		return fmt.Errorf("failed to execute command '%s' on host %s (stderr: %s): %w", cmd, host.GetName(), string(stderr), err)
	}

	logger.Info("Docker service command executed successfully.", "command", cmd)
	return nil
}

// Rollback for Docker service management.
func (s *ManageDockerServiceStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger)
	// A simple rollback might try the inverse of start/stop or enable/disable.
	// However, this can be risky or lead to unexpected states.
	// For this generic step, a no-op with logging is safest.
	logger.Warn("Rollback for Docker service management action is not automatically performed or is best-effort.", "action", s.Action)
	return nil
}

var _ step.Step = (*ManageDockerServiceStepSpec)(nil)
