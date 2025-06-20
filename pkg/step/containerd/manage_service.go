package containerd

import (
	"fmt"
	"strings" // Needed for strings.Title in constructor

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for spec.StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
)

const containerdServiceNameDefault = "containerd"

// ServiceAction defines the desired action for a service.
type ServiceAction string

const (
	ServiceActionStart        ServiceAction = "start"
	ServiceActionStop         ServiceAction = "stop"
	ServiceActionRestart      ServiceAction = "restart"
	ServiceActionEnable       ServiceAction = "enable"
	ServiceActionDisable      ServiceAction = "disable"
	ServiceActionReload       ServiceAction = "reload"          // systemctl reload
	ServiceActionDaemonReload ServiceAction = "daemon-reload" // systemctl daemon-reload
)

// ManageContainerdServiceStepSpec manages the containerd systemd service.
type ManageContainerdServiceStepSpec struct {
	spec.StepMeta `json:",inline"`
	ServiceName   string        `json:"serviceName,omitempty"`
	Action        ServiceAction `json:"action,omitempty"`
}

// NewManageContainerdServiceStepSpec creates a new ManageContainerdServiceStepSpec.
func NewManageContainerdServiceStep(action ServiceAction, customStepName ...string) *ManageContainerdServiceStepSpec {
	// Use customStepName for StepMeta.Name, generate description based on action and service.
	name := ""
	if len(customStepName) > 0 && customStepName[0] != "" {
		name = customStepName[0]
	} else {
		name = fmt.Sprintf("%s %s service", strings.Title(string(action)), containerdServiceNameDefault)
	}
	description := fmt.Sprintf("Performs action '%s' on service '%s'.", action, containerdServiceNameDefault)

	return &ManageContainerdServiceStepSpec{
		StepMeta: spec.StepMeta{
			Name:        name,
			Description: description,
		},
		ServiceName: containerdServiceNameDefault, // Hardcoded for this specific containerd step
		Action:      action,
	}
}

// Name returns the step's name (implementing step.Step).
func (s *ManageContainerdServiceStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description (implementing step.Step).
func (s *ManageContainerdServiceStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *ManageContainerdServiceStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *ManageContainerdServiceStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *ManageContainerdServiceStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ManageContainerdServiceStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

// Precheck attempts to determine if the service is already in the desired state.
func (s *ManageContainerdServiceStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")

	if s.ServiceName == "" || s.Action == "" {
		return false, fmt.Errorf("ServiceName and Action must be specified for %s", s.GetName())
	}

	if s.Action == ServiceActionDaemonReload { // Match constant name
	    logger.Debug("Action is daemon-reload, Precheck returns false to ensure it runs.")
	    return false, nil
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	var checkCmd string
	var expectedToFind bool

	switch s.Action {
	case ServiceActionStart, ServiceActionRestart:
		checkCmd = fmt.Sprintf("systemctl is-active %s", s.ServiceName)
		expectedToFind = true
	case ServiceActionEnable:
		checkCmd = fmt.Sprintf("systemctl is-enabled %s", s.ServiceName)
		expectedToFind = true
	case ServiceActionStop:
		checkCmd = fmt.Sprintf("systemctl is-active %s", s.ServiceName)
		expectedToFind = false
	case ServiceActionDisable:
		checkCmd = fmt.Sprintf("systemctl is-enabled %s", s.ServiceName)
		expectedToFind = false
	default:
		logger.Debug("No precheck logic for action.", "action", s.Action)
		return false, nil
	}

	logger.Debug("Executing service status check command.", "command", checkCmd)
	stdout, _, err := conn.Exec(ctx.GoContext(), checkCmd, &connector.ExecOptions{Sudo: true})
	output := strings.TrimSpace(string(stdout))

	if err != nil {
		if expectedToFind {
			logger.Info("Service is not in the desired state (command failed).", "action", s.Action, "output", output, "error", err)
			return false, nil
		}
		logger.Info("Service is likely in the desired state (command failed as expected for inactive/disabled).", "action", s.Action, "output", output, "error", err)
		return true, nil
	}

	if expectedToFind {
		logger.Info("Service is already in the desired state.", "action", s.Action, "status", output)
		return true, nil
	}
	logger.Info("Service is not in the desired state (command succeeded but indicates wrong state).", "action", s.Action, "status", output)
	return false, nil
}

// Run executes the service management command using systemctl.
func (s *ManageContainerdServiceStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")

	if s.ServiceName == "" || s.Action == "" {
		return fmt.Errorf("ServiceName and Action must be specified for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	var cmd string
	if s.Action == ServiceActionDaemonReload { // Match constant name
		cmd = "systemctl daemon-reload"
	} else {
		cmd = fmt.Sprintf("systemctl %s %s", s.Action, s.ServiceName)
	}

	logger.Info("Executing service command.", "command", cmd)
	_, stderr, err := conn.Exec(ctx.GoContext(), cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to execute command '%s' on host %s (stderr: %s): %w", cmd, host.GetName(), string(stderr), err)
	}

	logger.Info("Service command executed successfully.", "command", cmd)
	return nil
}

// Rollback for containerd service management.
func (s *ManageContainerdServiceStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Info("ManageContainerdServiceStep Rollback is a no-op by default for most actions.")
	return nil
}

// Ensure ManageContainerdServiceStepSpec implements the step.Step interface.
var _ step.Step = (*ManageContainerdServiceStepSpec)(nil)
