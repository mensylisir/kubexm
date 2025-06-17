package runner

import (
	"context"
	"fmt"
	"strings"

	"github.com/kubexms/kubexms/pkg/connector" // Assuming this is the correct path
)

// InitSystemType defines the type of init system.
type InitSystemType string

const (
	InitSystemUnknown InitSystemType = "unknown"
	InitSystemSystemd InitSystemType = "systemd"
	InitSystemSysV    InitSystemType = "sysvinit" // Basic support for SysV
)

// ServiceInfo holds details about the detected init system and its commands.
type ServiceInfo struct {
	Type         InitSystemType
	StartCmd     string // %s for service name
	StopCmd      string
	EnableCmd    string
	DisableCmd   string
	RestartCmd   string
	IsActiveCmd  string
	DaemonReloadCmd string
}

var (
	systemdInfo = ServiceInfo{
		Type:          InitSystemSystemd,
		StartCmd:      "systemctl start %s",
		StopCmd:       "systemctl stop %s",
		EnableCmd:     "systemctl enable %s",
		DisableCmd:    "systemctl disable %s",
		RestartCmd:    "systemctl restart %s",
		IsActiveCmd:   "systemctl is-active --quiet %s", // --quiet sets exit code
		DaemonReloadCmd: "systemctl daemon-reload",
	}
	sysvinitInfo = ServiceInfo{ // Simplified SysV support
		Type:          InitSystemSysV,
		StartCmd:      "service %s start", // or /etc/init.d/%s start
		StopCmd:       "service %s stop",
		EnableCmd:     "chkconfig %s on",    // Varies (chkconfig, update-rc.d)
		DisableCmd:    "chkconfig %s off",   // Varies
		RestartCmd:    "service %s restart",
		IsActiveCmd:   "service %s status", // Output parsing needed, not just exit code
		DaemonReloadCmd: "", // Typically no direct equivalent for sysvinit in one command
	}
)

// detectInitSystem attempts to identify the init system on the host.
// It caches the result in Runner's Facts if a field is added there.
func (r *Runner) detectInitSystem(ctx context.Context) (*ServiceInfo, error) {
	if r.Facts == nil || r.Facts.OS == nil {
		return nil, fmt.Errorf("OS facts not available, cannot detect init system")
	}
	// For now, detect every time. Caching could be added to Facts.

	// Systemd is common on modern Linux. Check for systemctl.
	if _, err := r.LookPath(ctx, "systemctl"); err == nil {
		return &systemdInfo, nil
	}

	// Fallback to checking for SysV 'service' or init.d scripts if systemctl not found.
	if _, err := r.LookPath(ctx, "service"); err == nil {
		// This is a basic assumption. More checks might be needed to confirm SysV.
		return &sysvinitInfo, nil
	}
	// Check for /etc/init.d as another indicator for SysV
	if exists, _ := r.Exists(ctx, "/etc/init.d"); exists {
		return &sysvinitInfo, nil
	}

	return nil, fmt.Errorf("unable to detect a supported init system (systemd, sysvinit) on OS ID: %s", r.Facts.OS.ID)
}

func (r *Runner) manageService(ctx context.Context, serviceName, commandTemplate string, sudo bool) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}
	if strings.TrimSpace(serviceName) == "" {
		return fmt.Errorf("serviceName cannot be empty")
	}

	svcInfo, err := r.detectInitSystem(ctx)
	if err != nil {
		return err
	}
	if commandTemplate == "" {
		return fmt.Errorf("internal error: command template is empty for service management")
	}

	cmd := fmt.Sprintf(commandTemplate, serviceName)
	_, stderr, execErr := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: sudo})
	if execErr != nil {
		return fmt.Errorf("failed to manage service %s with command '%s' using %s: %w (stderr: %s)", serviceName, cmd, svcInfo.Type, execErr, string(stderr))
	}
	return nil
}

// StartService starts a service.
func (r *Runner) StartService(ctx context.Context, serviceName string) error {
	svcInfo, err := r.detectInitSystem(ctx)
	if err != nil { return err }
	return r.manageService(ctx, serviceName, svcInfo.StartCmd, true)
}

// StopService stops a service.
func (r *Runner) StopService(ctx context.Context, serviceName string) error {
	svcInfo, err := r.detectInitSystem(ctx)
	if err != nil { return err }
	return r.manageService(ctx, serviceName, svcInfo.StopCmd, true)
}

// RestartService restarts a service.
func (r *Runner) RestartService(ctx context.Context, serviceName string) error {
	svcInfo, err := r.detectInitSystem(ctx)
	if err != nil { return err }
	return r.manageService(ctx, serviceName, svcInfo.RestartCmd, true)
}

// EnableService enables a service to start on boot.
func (r *Runner) EnableService(ctx context.Context, serviceName string) error {
	svcInfo, err := r.detectInitSystem(ctx)
	if err != nil { return err }
	if svcInfo.Type == InitSystemSysV && (svcInfo.EnableCmd == "" || !strings.Contains(svcInfo.EnableCmd, "%s")) {
		return fmt.Errorf("enabling service '%s' is not reliably supported for detected SysV init variant (command: '%s')", serviceName, svcInfo.EnableCmd)
	}
	return r.manageService(ctx, serviceName, svcInfo.EnableCmd, true)
}

// DisableService disables a service from starting on boot.
func (r *Runner) DisableService(ctx context.Context, serviceName string) error {
	svcInfo, err := r.detectInitSystem(ctx)
	if err != nil { return err }
	if svcInfo.Type == InitSystemSysV && (svcInfo.DisableCmd == "" || !strings.Contains(svcInfo.DisableCmd, "%s")) {
		return fmt.Errorf("disabling service '%s' is not reliably supported for detected SysV init variant (command: '%s')", serviceName, svcInfo.DisableCmd)
	}
	return r.manageService(ctx, serviceName, svcInfo.DisableCmd, true)
}

// IsServiceActive checks if a service is currently active/running.
func (r *Runner) IsServiceActive(ctx context.Context, serviceName string) (bool, error) {
	if r.Conn == nil {
		return false, fmt.Errorf("runner has no valid connector")
	}
	if strings.TrimSpace(serviceName) == "" {
		return false, fmt.Errorf("serviceName cannot be empty")
	}

	svcInfo, err := r.detectInitSystem(ctx)
	if err != nil {
		return false, err
	}

	cmd := fmt.Sprintf(svcInfo.IsActiveCmd, serviceName)

	// For systemd, `systemctl is-active --quiet` exits 0 if active.
	// For SysV, `service status` exit codes are not always standardized.
	// Output parsing might be needed for SysV.
	if svcInfo.Type == InitSystemSystemd {
		// Sudo is typically not required for `is-active`.
		return r.Check(ctx, cmd, false)
	} else if svcInfo.Type == InitSystemSysV {
		// `service <name> status` often returns 0 if running, but output is more reliable.
		// This is a simplified check. A robust SysV check would parse output.
		// For now, we'll assume exit code 0 from `service status` means active for SysV.
		// This might need adjustment.
		stdout, _, execErr := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: false})
		if execErr == nil { // Exit code 0
			// Further check output for SysV if needed, e.g. look for "running" or "active"
			// For example:
			// outputStr := strings.ToLower(string(stdout))
			// return strings.Contains(outputStr, "running") || strings.Contains(outputStr, "active"), nil
			return true, nil
		}
		// If CommandError with non-zero exit, assume not active for SysV for this basic check.
		if _, ok := execErr.(*connector.CommandError); ok {
			return false, nil
		}
		return false, fmt.Errorf("failed to check SysV service status for %s: %w", serviceName, execErr)
	}

	return false, fmt.Errorf("IsServiceActive not fully implemented for init system type: %s", svcInfo.Type)
}

// DaemonReload reloads the init system's configuration (e.g., systemctl daemon-reload).
func (r *Runner) DaemonReload(ctx context.Context) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}
	svcInfo, err := r.detectInitSystem(ctx)
	if err != nil {
		return err
	}

	if svcInfo.DaemonReloadCmd == "" {
		// For SysV, there isn't always a direct equivalent.
		// This might be a no-op or return an error indicating not supported.
		if svcInfo.Type == InitSystemSysV {
			return nil // No-op for basic SysV, or log a warning.
		}
		return fmt.Errorf("daemon-reload command not defined for init system type: %s", svcInfo.Type)
	}

	cmd := svcInfo.DaemonReloadCmd
	// Daemon reload usually requires sudo.
	_, stderr, execErr := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: true})
	if execErr != nil {
		return fmt.Errorf("failed to execute daemon-reload using %s: %w (stderr: %s)", svcInfo.Type, execErr, string(stderr))
	}
	return nil
}

// TODO: Add more service management functions:
// - GetServiceStatus(ctx context.Context, serviceName string) (status string, err error) (more detailed status)
// - MaskService(ctx context.Context, serviceName string) error
// - UnmaskService(ctx context.Context, serviceName string) error
