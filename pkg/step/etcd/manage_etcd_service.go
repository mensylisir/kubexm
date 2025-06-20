package etcd

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// ServiceAction defines the action to be performed on a service.
// Re-declared here if not imported from a common place, or use common.ServiceAction.
// For now, assume it's fine to have it here for etcd-specific step.
type ServiceAction string

const (
	ActionStartEtcd         ServiceAction = "start"
	ActionStopEtcd          ServiceAction = "stop"
	ActionRestartEtcd       ServiceAction = "restart"
	ActionEnableEtcd        ServiceAction = "enable"
	ActionDisableEtcd       ServiceAction = "disable"
	ActionDaemonReloadEtcd  ServiceAction = "daemon-reload"
)

// ManageEtcdServiceStepSpec defines parameters for managing the etcd systemd service.
type ManageEtcdServiceStepSpec struct {
	spec.StepMeta // Embed common meta fields

	ServiceName string        `json:"serviceName,omitempty"` // Name of the etcd service, e.g., "etcd"
	Action      ServiceAction `json:"action,omitempty"`      // e.g., "start", "stop", "enable"
}

// NewManageEtcdServiceStepSpec creates a new ManageEtcdServiceStepSpec.
func NewManageEtcdServiceStepSpec(stepName, serviceName string, action ServiceAction) *ManageEtcdServiceStepSpec {
	sName := serviceName
	if sName == "" {
		sName = "etcd" // Default etcd service name
	}

	if stepName == "" {
		stepName = fmt.Sprintf("%s %s service", strings.Title(string(action)), sName)
	}

	return &ManageEtcdServiceStepSpec{
		StepMeta: spec.StepMeta{
			Name:        stepName,
			Description: fmt.Sprintf("Performs action '%s' on the %s systemd service.", action, sName),
		},
		ServiceName: sName,
		Action:      action,
	}
}

// GetName returns the step's name.
func (s *ManageEtcdServiceStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description.
func (s *ManageEtcdServiceStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec validates and returns the spec.
func (s *ManageEtcdServiceStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ManageEtcdServiceStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

// Name returns the step's name (implementing step.Step).
func (s *ManageEtcdServiceStepSpec) Name() string { return s.GetName() }

// Description returns the step's description (implementing step.Step).
func (s *ManageEtcdServiceStepSpec) Description() string { return s.GetDescription() }

// Precheck attempts to determine if the service is already in the desired state.
func (s *ManageEtcdServiceStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")

	if s.ServiceName == "" || s.Action == "" {
		return false, fmt.Errorf("ServiceName and Action must be specified for ManageEtcdServiceStep: %s", s.GetName())
	}

	if s.Action == ActionDaemonReloadEtcd {
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
	case ActionStartEtcd, ActionRestartEtcd:
		checkCmd = fmt.Sprintf("systemctl is-active %s", s.ServiceName)
		expectedToFind = true
	case ActionEnableEtcd:
		checkCmd = fmt.Sprintf("systemctl is-enabled %s", s.ServiceName)
		expectedToFind = true
	case ActionStopEtcd:
		checkCmd = fmt.Sprintf("systemctl is-active %s", s.ServiceName)
		expectedToFind = false
	case ActionDisableEtcd:
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

// Run executes the service management command.
func (s *ManageEtcdServiceStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")

	if s.ServiceName == "" || s.Action == "" {
		return fmt.Errorf("ServiceName and Action must be specified for ManageEtcdServiceStep: %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	var cmd string
	if s.Action == ActionDaemonReloadEtcd {
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

// Rollback for etcd service management.
func (s *ManageEtcdServiceStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	// Default is no-op, specific inverse actions could be added if critical.
	// For example, if ActionStart, rollback could be ActionStop.
	logger.Info("ManageEtcdServiceStep Rollback is a no-op by default for most actions.")
	return nil
}

var _ step.Step = (*ManageEtcdServiceStepSpec)(nil)
