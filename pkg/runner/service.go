package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// Constants for ServiceInfo are in interface.go / runner.go

// detectInitSystem is now a private method of defaultRunner, located in runner.go

// manageService is a helper function, converted to a private method of defaultRunner.
// It's not part of the Runner interface but used internally by other service methods.
func (r *defaultRunner) manageService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName, commandTemplate string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if facts == nil || facts.InitSystem == nil {
		return fmt.Errorf("init system facts not available")
	}
	if strings.TrimSpace(serviceName) == "" {
		return fmt.Errorf("serviceName cannot be empty")
	}
	if commandTemplate == "" {
		return fmt.Errorf("internal error: command template is empty for service management")
	}

	svcInfo := facts.InitSystem
	cmd := fmt.Sprintf(commandTemplate, serviceName)
	_, _, execErr := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: sudo})
	if execErr != nil {
		return fmt.Errorf("failed to manage service %s with command '%s' using %s: %w", serviceName, cmd, svcInfo.Type, execErr)
	}
	return nil
}

// StartService starts a service.
func (r *defaultRunner) StartService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error {
	if facts == nil || facts.InitSystem == nil {
		return fmt.Errorf("init system facts not available for StartService")
	}
	return r.manageService(ctx, conn, facts, serviceName, facts.InitSystem.StartCmd, true)
}

// StopService stops a service.
func (r *defaultRunner) StopService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error {
	if facts == nil || facts.InitSystem == nil {
		return fmt.Errorf("init system facts not available for StopService")
	}
	return r.manageService(ctx, conn, facts, serviceName, facts.InitSystem.StopCmd, true)
}

// RestartService restarts a service.
func (r *defaultRunner) RestartService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error {
	if facts == nil || facts.InitSystem == nil {
		return fmt.Errorf("init system facts not available for RestartService")
	}
	return r.manageService(ctx, conn, facts, serviceName, facts.InitSystem.RestartCmd, true)
}

// EnableService enables a service to start on boot.
func (r *defaultRunner) EnableService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error {
	if facts == nil || facts.InitSystem == nil {
		return fmt.Errorf("init system facts not available for EnableService")
	}
	svcInfo := facts.InitSystem
	if svcInfo.Type == InitSystemSysV && (svcInfo.EnableCmd == "" || !strings.Contains(svcInfo.EnableCmd, "%s")) {
		return fmt.Errorf("enabling service '%s' is not reliably supported for detected SysV init variant (command: '%s')", serviceName, svcInfo.EnableCmd)
	}
	return r.manageService(ctx, conn, facts, serviceName, svcInfo.EnableCmd, true)
}

// DisableService disables a service from starting on boot.
func (r *defaultRunner) DisableService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error {
	if facts == nil || facts.InitSystem == nil {
		return fmt.Errorf("init system facts not available for DisableService")
	}
	svcInfo := facts.InitSystem
	if svcInfo.Type == InitSystemSysV && (svcInfo.DisableCmd == "" || !strings.Contains(svcInfo.DisableCmd, "%s")) {
		return fmt.Errorf("disabling service '%s' is not reliably supported for detected SysV init variant (command: '%s')", serviceName, svcInfo.DisableCmd)
	}
	return r.manageService(ctx, conn, facts, serviceName, svcInfo.DisableCmd, true)
}

// IsServiceActive checks if a service is currently active/running.
func (r *defaultRunner) IsServiceActive(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	if facts == nil || facts.InitSystem == nil {
		return false, fmt.Errorf("init system facts not available for IsServiceActive")
	}
	if strings.TrimSpace(serviceName) == "" {
		return false, fmt.Errorf("serviceName cannot be empty")
	}

	svcInfo := facts.InitSystem
	cmd := fmt.Sprintf(svcInfo.IsActiveCmd, serviceName)

	if svcInfo.Type == InitSystemSystemd {
		return r.Check(ctx, conn, cmd, false)
	} else if svcInfo.Type == InitSystemSysV {
		stdout, _, execErr := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: false})
		if execErr == nil { // Exit code 0
			// Basic SysV check: For a more robust check, parse stdout for "running" or "active".
			// This simplified version assumes exit code 0 means active for many `service status` calls.
			// Some `service status` might return 0 even if not truly "active" but just "known".
			// Consider enhancing this if more precise SysV status is needed.
			outputStr := strings.ToLower(string(stdout))
			if strings.Contains(outputStr, "is running") || strings.Contains(outputStr, "running...") || strings.Contains(outputStr, "active (running)"){
				return true, nil
			}
			// If output doesn't explicitly say running, but exit code was 0, it's ambiguous.
			// For now, let's be conservative for SysV if output isn't clearly "running".
			// However, many scripts use exit code 0 for running.
			// This part can be refined. If exit code 0 means "definitely running" for target SysV scripts, this is fine.
			return true, nil // Defaulting to true if exit 0 for SysV status.
		}
		// Check if the error is a CommandError, indicating the command ran but exited non-zero.
		var cmdError *connector.CommandError
		if errors.As(execErr, &cmdError) { // Non-zero exit from status command
			return false, nil // Service is not active
		}
		// Other errors (e.g., command not found, connection issues)
		return false, fmt.Errorf("failed to check SysV service status for %s: %w", serviceName, execErr)
	}
	return false, fmt.Errorf("IsServiceActive not fully implemented for init system type: %s", svcInfo.Type)
}

// DaemonReload reloads the init system's configuration.
func (r *defaultRunner) DaemonReload(ctx context.Context, conn connector.Connector, facts *Facts) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if facts == nil || facts.InitSystem == nil {
		return fmt.Errorf("init system facts not available for DaemonReload")
	}
	svcInfo := facts.InitSystem

	if svcInfo.DaemonReloadCmd == "" {
		if svcInfo.Type == InitSystemSysV {
			return nil // No-op for basic SysV
		}
		return fmt.Errorf("daemon-reload command not defined for init system type: %s", svcInfo.Type)
	}
	cmd := svcInfo.DaemonReloadCmd
	_, _, execErr := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
	if execErr != nil {
		return fmt.Errorf("failed to execute daemon-reload using %s: %w", svcInfo.Type, execErr)
	}
	return nil
}
