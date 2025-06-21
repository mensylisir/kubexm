package containerd

import (
	"fmt"
	"strings" // Needed for strings.Title in constructor

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
)

const containerdServiceName = "containerd"

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

// ManageContainerdServiceStep manages the containerd systemd service.
type ManageContainerdServiceStep struct {
	meta        spec.StepMeta
	ServiceName string
	Action      ServiceAction
	Sudo        bool // Runner service methods typically handle sudo internally based on OS/facts or it's implied.
	             // However, if a generic `Run` is used for `reload`, sudo might be needed.
}

// NewManageContainerdServiceStep creates a new ManageContainerdServiceStep.
func NewManageContainerdServiceStep(instanceName string, action ServiceAction, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = fmt.Sprintf("%s %s service", strings.Title(string(action)), containerdServiceName)
	}
	return &ManageContainerdServiceStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Performs action '%s' on service '%s'.", action, containerdServiceName),
		},
		ServiceName: containerdServiceName,
		Action:      action,
		Sudo:        sudo, // Store sudo, primarily for potential 'reload' via generic Run
	}
}

// Meta returns the step's metadata.
func (s *ManageContainerdServiceStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *ManageContainerdServiceStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}
	facts, err := ctx.GetHostFacts(host)
	if err != nil {
		return false, fmt.Errorf("failed to get host facts for %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	isActive, errActive := runnerSvc.IsServiceActive(ctx.GoContext(), conn, facts, s.ServiceName)
	if errActive != nil {
		logger.Warn("Failed to check service active status. Assuming action is required.", "service", s.ServiceName, "error", errActive)
		return false, nil
	}

	// Runner interface does not have IsServiceEnabled.
	// Precheck for Enable/Disable actions will be simplified to always return false, letting Run attempt the action.
	// A more sophisticated check would require `runnerSvc.Run("systemctl is-enabled servicename")`.
	if s.Action == ServiceActionEnable || s.Action == ServiceActionDisable {
		logger.Debug("Precheck for Enable/Disable cannot reliably determine current enabled state via runner; action will proceed.")
		return false, nil
	}

	switch s.Action {
	case ServiceActionStart:
		if isActive {
			logger.Info("Service is already active.", "service", s.ServiceName)
			return true, nil
		}
		return false, nil
	case ServiceActionStop:
		if !isActive {
			logger.Info("Service is already stopped.", "service", s.ServiceName)
			return true, nil
		}
		return false, nil
	case ServiceActionRestart, ServiceActionReload, ServiceActionDaemonReload:
		logger.Debug("Action type implies it should always run if requested.", "action", s.Action)
		return false, nil
	default:
		return false, fmt.Errorf("unknown service action '%s' for step %s on host %s", s.Action, s.meta.Name, host.GetName())
	}
}

func (s *ManageContainerdServiceStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}
	facts, err := ctx.GetHostFacts(host) // Runner methods need facts
	if err != nil {
		return fmt.Errorf("failed to get host facts for %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	switch s.Action {
	case ServiceActionStart:
		logger.Info("Starting service.", "service", s.ServiceName)
		if err := runnerSvc.StartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
			return fmt.Errorf("failed to start service %s on host %s: %w", s.ServiceName, host.GetName(), err)
		}
		isActive, verifyErr := runnerSvc.IsServiceActive(ctx.GoContext(), conn, facts, s.ServiceName)
		if verifyErr != nil {
			return fmt.Errorf("failed to verify service status after start for %s on host %s: %w", s.ServiceName, host.GetName(), verifyErr)
		}
		if !isActive {
			return fmt.Errorf("service %s started but reported as not active on host %s", s.ServiceName, host.GetName())
		}
		logger.Info("Service started successfully.", "service", s.ServiceName)
	case ServiceActionStop:
		logger.Info("Stopping service.", "service", s.ServiceName)
		if err := runnerSvc.StopService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
			return fmt.Errorf("failed to stop service %s on host %s: %w", s.ServiceName, host.GetName(), err)
		}
		isActive, verifyErr := runnerSvc.IsServiceActive(ctx.GoContext(), conn, facts, s.ServiceName)
		if verifyErr != nil {
			logger.Warn("Failed to verify service status after stop. Assuming stopped.", "service", s.ServiceName, "error", verifyErr)
		} else if isActive {
			return fmt.Errorf("service %s stopped but reported as still active on host %s", s.ServiceName, host.GetName())
		}
		logger.Info("Service stopped successfully.", "service", s.ServiceName)
	case ServiceActionEnable:
		logger.Info("Enabling service.", "service", s.ServiceName)
		if err := runnerSvc.EnableService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
			return fmt.Errorf("failed to enable service %s on host %s: %w", s.ServiceName, host.GetName(), err)
		}
		logger.Info("Service enabled successfully.", "service", s.ServiceName)
	case ServiceActionDisable:
		logger.Info("Disabling service.", "service", s.ServiceName)
		if err := runnerSvc.DisableService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
			return fmt.Errorf("failed to disable service %s on host %s: %w", s.ServiceName, host.GetName(), err)
		}
		logger.Info("Service disabled successfully.", "service", s.ServiceName)
	case ServiceActionRestart:
		logger.Info("Restarting service.", "service", s.ServiceName)
		if err := runnerSvc.RestartService(ctx.GoContext(), conn, facts, s.ServiceName); err != nil {
			return fmt.Errorf("failed to restart service %s on host %s: %w", s.ServiceName, host.GetName(), err)
		}
		isActive, verifyErr := runnerSvc.IsServiceActive(ctx.GoContext(), conn, facts, s.ServiceName)
		if verifyErr != nil {
			return fmt.Errorf("failed to verify service status after restart for %s on host %s: %w", s.ServiceName, host.GetName(), verifyErr)
		}
		if !isActive {
			return fmt.Errorf("service %s restarted but reported as not active on host %s", s.ServiceName, host.GetName())
		}
		logger.Info("Service restarted successfully.", "service", s.ServiceName)
	case ServiceActionReload:
		logger.Info("Reloading service configuration.", "service", s.ServiceName)
		reloadCmd := fmt.Sprintf("systemctl reload %s", s.ServiceName)
		if _, err := runnerSvc.Run(ctx.GoContext(), conn, reloadCmd, s.Sudo); err != nil {
			return fmt.Errorf("failed to reload service %s on host %s: %w", s.ServiceName, host.GetName(), err)
		}
		logger.Info("Service reload signal sent.", "service", s.ServiceName)
	case ServiceActionDaemonReload:
		logger.Info("Performing systemctl daemon-reload.")
		if err := runnerSvc.DaemonReload(ctx.GoContext(), conn, facts); err != nil { // Sudo is handled by runner
			return fmt.Errorf("failed to perform daemon-reload on host %s: %w", host.GetName(), err)
		}
		logger.Info("Daemon-reload performed successfully.")
	default:
		return fmt.Errorf("unknown service action '%s' for step %s on host %s", s.Action, s.meta.Name, host.GetName())
	}
	return nil
}

func (s *ManageContainerdServiceStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")

	if s.Action == ServiceActionStart || s.Action == ServiceActionRestart || s.Action == ServiceActionEnable {
		logger.Info("Attempting to stop service as part of rollback (best effort).", "service", s.ServiceName)
		runnerSvc := ctx.GetRunner()
		conn, err := ctx.GetConnectorForHost(host)
		if err != nil {
			logger.Error("Failed to get connector for rollback, cannot stop service.", "error", err)
			return nil
		}
		facts, err := ctx.GetHostFacts(host)
		if err != nil {
			logger.Error("Failed to get host facts for rollback, cannot stop service.", "error", err)
			return nil
		}
		if errStop := runnerSvc.StopService(ctx.GoContext(), conn, facts, s.ServiceName); errStop != nil {
			logger.Warn("Failed to stop service during rollback (best effort).", "service", s.ServiceName, "error", errStop)
		} else {
			logger.Info("Service stopped during rollback (best effort).", "service", s.ServiceName)
		}
	} else {
		logger.Info("No specific rollback action defined for this service management step/action.", "action", s.Action)
	}
	return nil
}

// Ensure ManageContainerdServiceStep implements the step.Step interface.
var _ step.Step = (*ManageContainerdServiceStep)(nil)
