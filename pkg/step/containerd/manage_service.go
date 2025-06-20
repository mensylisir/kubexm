package containerd

import (
	"fmt"
	"strings" // Needed for strings.Title in constructor

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	// No longer need spec, time, or context directly
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
	ServiceName string
	Action      ServiceAction
	StepName    string
}

// NewManageContainerdServiceStep creates a new ManageContainerdServiceStep.
func NewManageContainerdServiceStep(action ServiceAction, customStepName ...string) step.Step {
	name := fmt.Sprintf("%s %s service", strings.Title(string(action)), containerdServiceName)
	if len(customStepName) > 0 && customStepName[0] != "" {
		name = customStepName[0]
	}
	return &ManageContainerdServiceStep{
		ServiceName: containerdServiceName,
		Action:      action,
		StepName:    name,
	}
}

func (s *ManageContainerdServiceStep) Name() string {
	return s.StepName
}

func (s *ManageContainerdServiceStep) Description() string {
	return fmt.Sprintf("Performs action '%s' on service '%s'.", s.Action, s.ServiceName)
}

func (s *ManageContainerdServiceStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Precheck")

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.Name(), err)
	}

	// Assuming connector methods: IsServiceActive(ctx, name), IsServiceEnabled(ctx, name)
	isActive, errActive := conn.IsServiceActive(ctx.GoContext(), s.ServiceName)
	if errActive != nil {
		logger.Warn("Failed to check service active status. Assuming action is required.", "service", s.ServiceName, "error", errActive)
		return false, nil
	}

	isEnabled, errEnabled := conn.IsServiceEnabled(ctx.GoContext(), s.ServiceName)
	if errEnabled != nil {
	    logger.Warn("Failed to check service enabled status. Proceeding based on active status.", "service", s.ServiceName, "error", errEnabled)
	    isEnabled = isActive // Fallback logic: if can't check enabled, assume enabled if active.
	}

	switch s.Action {
	case ServiceActionStart:
		if isActive {
			logger.Info("Service is already active.", "service", s.ServiceName)
			return true, nil
		}
		return false, nil
	case ServiceActionEnable:
		if isEnabled {
		    logger.Info("Service is already enabled.", "service", s.ServiceName)
		    return true, nil
		}
		// If enabling, we also typically want it started. If it's already active but not enabled,
		// the 'enable' action is still needed. If it's not active and not enabled, both are needed.
		// So, just checking isEnabled is sufficient for the "is this step done?" question.
		return false, nil
	case ServiceActionStop:
		if !isActive {
			logger.Info("Service is already stopped.", "service", s.ServiceName)
			return true, nil
		}
		return false, nil
	case ServiceActionDisable:
	    if !isEnabled { // If already not enabled, the primary goal of 'disable' is met.
	        logger.Info("Service is already disabled.", "service", s.ServiceName)
	        // We might also want to ensure it's stopped if we disable it.
	        // However, for precheck, if it's not enabled, the disable action itself is "done".
	        return true, nil
	    }
	    return false, nil
	case ServiceActionRestart, ServiceActionReload, ServiceActionDaemonReload:
		// These actions are generally performed regardless of current state if requested.
		// For example, 'restart' implies a desire to stop and start, even if already running.
		// 'daemon-reload' has effects beyond a single service's state.
		logger.Debug("Action type implies it should always run if requested.", "action", s.Action)
		return false, nil
	default:
		return false, fmt.Errorf("unknown service action '%s' for step %s on host %s", s.Action, s.Name(), host.GetName())
	}
}

func (s *ManageContainerdServiceStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Run")

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.Name(), err)
	}

	// Assuming connector methods: StartService, StopService, EnableService, DisableService, RestartService, ReloadService, DaemonReload
	switch s.Action {
	case ServiceActionStart:
		logger.Info("Starting service.", "service", s.ServiceName)
		if err := conn.StartService(ctx.GoContext(), s.ServiceName); err != nil {
			return fmt.Errorf("failed to start service %s on host %s: %w", s.ServiceName, host.GetName(), err)
		}
		isActive, verifyErr := conn.IsServiceActive(ctx.GoContext(), s.ServiceName)
		if verifyErr != nil {
		    return fmt.Errorf("failed to verify service status after start for %s on host %s: %w", s.ServiceName, host.GetName(), verifyErr)
		}
		if !isActive {
		    return fmt.Errorf("service %s started but reported as not active on host %s", s.ServiceName, host.GetName())
		}
		logger.Info("Service started successfully.", "service", s.ServiceName)
	case ServiceActionStop:
		logger.Info("Stopping service.", "service", s.ServiceName)
		if err := conn.StopService(ctx.GoContext(), s.ServiceName); err != nil {
			return fmt.Errorf("failed to stop service %s on host %s: %w", s.ServiceName, host.GetName(), err)
		}
		isActive, verifyErr := conn.IsServiceActive(ctx.GoContext(), s.ServiceName)
		if verifyErr != nil {
		    logger.Warn("Failed to verify service status after stop. Assuming stopped.", "service", s.ServiceName, "error", verifyErr)
		} else if isActive {
		    return fmt.Errorf("service %s stopped but reported as still active on host %s", s.ServiceName, host.GetName())
		}
		logger.Info("Service stopped successfully.", "service", s.ServiceName)
	case ServiceActionEnable:
		logger.Info("Enabling service.", "service", s.ServiceName)
		if err := conn.EnableService(ctx.GoContext(), s.ServiceName); err != nil {
			return fmt.Errorf("failed to enable service %s on host %s: %w", s.ServiceName, host.GetName(), err)
		}
		logger.Info("Service enabled successfully.", "service", s.ServiceName)
	case ServiceActionDisable:
		logger.Info("Disabling service.", "service", s.ServiceName)
		if err := conn.DisableService(ctx.GoContext(), s.ServiceName); err != nil {
			return fmt.Errorf("failed to disable service %s on host %s: %w", s.ServiceName, host.GetName(), err)
		}
		logger.Info("Service disabled successfully.", "service", s.ServiceName)
	case ServiceActionRestart:
		logger.Info("Restarting service.", "service", s.ServiceName)
		if err := conn.RestartService(ctx.GoContext(), s.ServiceName); err != nil {
		    return fmt.Errorf("failed to restart service %s on host %s: %w", s.ServiceName, host.GetName(), err)
		}
		isActive, verifyErr := conn.IsServiceActive(ctx.GoContext(), s.ServiceName)
		if verifyErr != nil {
		    return fmt.Errorf("failed to verify service status after restart for %s on host %s: %w", s.ServiceName, host.GetName(), verifyErr)
		}
		if !isActive {
		    return fmt.Errorf("service %s restarted but reported as not active on host %s", s.ServiceName, host.GetName())
		}
		logger.Info("Service restarted successfully.", "service", s.ServiceName)
    case ServiceActionReload:
        logger.Info("Reloading service configuration.", "service", s.ServiceName)
        if err := conn.ReloadService(ctx.GoContext(), s.ServiceName); err != nil {
            return fmt.Errorf("failed to reload service %s on host %s: %w", s.ServiceName, host.GetName(), err)
        }
        logger.Info("Service reload signal sent.", "service", s.ServiceName)
    case ServiceActionDaemonReload:
        logger.Info("Performing systemctl daemon-reload.")
        if err := conn.DaemonReload(ctx.GoContext()); err != nil {
            return fmt.Errorf("failed to perform daemon-reload on host %s: %w", host.GetName(), err)
        }
        logger.Info("Daemon-reload performed successfully.")
	default:
		return fmt.Errorf("unknown service action '%s' for step %s on host %s", s.Action, s.Name(), host.GetName())
	}
	return nil
}

func (s *ManageContainerdServiceStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Rollback")

	// Best-effort rollback: if 'start' or 'enable' or 'restart' failed, try to stop.
	// For 'stop' or 'disable', usually no direct rollback action is taken for the service itself.
	// For 'reload' or 'daemon-reload', rollback is typically not applicable for the action itself.
	if s.Action == ServiceActionStart || s.Action == ServiceActionRestart || s.Action == ServiceActionEnable {
		logger.Info("Attempting to stop service as part of rollback (best effort).", "service", s.ServiceName)
		conn, err := ctx.GetConnectorForHost(host)
	    if err != nil {
		    logger.Error("Failed to get connector for rollback, cannot stop service.", "error", err)
            return nil // Can't do much if connector fails
	    }
        // Best effort stop
        if errStop := conn.StopService(ctx.GoContext(), s.ServiceName); errStop != nil {
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
