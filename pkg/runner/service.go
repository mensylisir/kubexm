package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
)

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

func (r *defaultRunner) StartService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error {
	if facts == nil || facts.InitSystem == nil {
		return fmt.Errorf("init system facts not available for StartService")
	}
	return r.manageService(ctx, conn, facts, serviceName, facts.InitSystem.StartCmd, true)
}

func (r *defaultRunner) StopService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error {
	if facts == nil || facts.InitSystem == nil {
		return fmt.Errorf("init system facts not available for StopService")
	}
	return r.manageService(ctx, conn, facts, serviceName, facts.InitSystem.StopCmd, true)
}

func (r *defaultRunner) RestartService(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) error {
	if facts == nil || facts.InitSystem == nil {
		return fmt.Errorf("init system facts not available for RestartService")
	}
	return r.manageService(ctx, conn, facts, serviceName, facts.InitSystem.RestartCmd, true)
}

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
		if execErr == nil {
			outputStr := strings.ToLower(string(stdout))
			if strings.Contains(outputStr, "is running") || strings.Contains(outputStr, "running...") || strings.Contains(outputStr, "active (running)") {
				return true, nil
			}
			return false, nil
		}
		var cmdError *connector.CommandError
		if errors.As(execErr, &cmdError) { // Non-zero exit from status command
			return false, nil // Service is not active
		}
		return false, fmt.Errorf("failed to check SysV service status for %s: %w", serviceName, execErr)
	}
	return false, fmt.Errorf("IsServiceActive not fully implemented for init system type: %s", svcInfo.Type)
}

func (r *defaultRunner) IsServiceEnabled(ctx context.Context, conn connector.Connector, facts *Facts, serviceName string) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	if facts == nil || facts.InitSystem == nil {
		return false, fmt.Errorf("init system facts not available for IsServiceEnabled")
	}
	if strings.TrimSpace(serviceName) == "" {
		return false, fmt.Errorf("serviceName cannot be empty")
	}

	svcInfo := facts.InitSystem

	switch svcInfo.Type {
	case InitSystemSystemd:
		cmd := fmt.Sprintf("systemctl is-enabled --quiet %s", serviceName)
		_, _, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: false})

		if err == nil {
			return true, nil
		}
		if _, ok := err.(*connector.CommandError); ok {
			return false, nil
		}
		return false, err

	case InitSystemSysV:
		osFamily := strings.ToLower(facts.OS.ID)

		if osFamily == "rhel" || osFamily == "centos" || osFamily == "fedora" {
			if _, err := r.LookPath(ctx, conn, "chkconfig"); err == nil {
				cmd := fmt.Sprintf("chkconfig %s", serviceName)
				_, _, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: false})
				if err == nil {
					return true, nil
				}
				if _, ok := err.(*connector.CommandError); ok {
					return false, nil
				}
				return false, err
			}
		}
		cmd := fmt.Sprintf("ls /etc/rc?.d/S* | grep -qE '/S[0-9]+%s$'", serviceName)
		return r.Check(ctx, conn, cmd, false)

	default:
		return false, fmt.Errorf("IsServiceEnabled not implemented for init system type: %s", svcInfo.Type)
	}
}

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
