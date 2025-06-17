package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/kubexms/kubexms/pkg/connector"
)

// Helper to setup runner with a specific init system for service tests
func newTestRunnerForService(t *testing.T, isSystemd bool) (*Runner, *MockConnector) {
	mockConn := NewMockConnector()
	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return &connector.OS{ID: "linux-test", Arch: "amd64", Kernel: "test-kernel"}, nil
	}
	// Default Exec for NewRunner fact gathering & other commands if not overridden
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		// Fallback for service commands if not specifically mocked in a test
		return []byte(""), nil, nil
	}
	// Default LookPath for commands that might be checked by detectInitSystem
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if isFactGatheringCommandLookup(file) { return "/usr/bin/" + file, nil}
		if isSystemd {
			if file == "systemctl" { return "/usr/bin/systemctl", nil }
			if file == "service" { return "", errors.New("service command not found when systemd is primary")}
		} else { // SysV
			if file == "systemctl" { return "", errors.New("systemctl not found for SysV") }
			if file == "service" { return "/usr/sbin/service", nil }
		}
		return "", fmt.Errorf("LookPath mock: command %s not expected for init system test (systemd: %v)", file, isSystemd)
	}
	// Default Stat for Exists check (e.g. /etc/init.d)
	mockConn.StatFunc = func(ctx context.Context, path string) (*connector.FileStat, error) {
		if !isSystemd && path == "/etc/init.d" { // For SysV detection fallback
			return &connector.FileStat{Name: "init.d", IsExist: true, IsDir: true}, nil
		}
		return &connector.FileStat{Name: path, IsExist: false}, nil // Default to not found for other paths
	}


	r, err := NewRunner(context.Background(), mockConn)
	if err != nil {
		t.Fatalf("Failed to create runner for service tests (systemd: %v): %v", isSystemd, err)
	}
	return r, mockConn
}


func TestRunner_StartService_Systemd(t *testing.T) {
	r, mockConn := newTestRunnerForService(t, true) // Systemd
	serviceName := "nginx"
	expectedCmd := fmt.Sprintf(systemdInfo.StartCmd, serviceName)
	var cmdExecuted string

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd; mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		if cmd == expectedCmd && options.Sudo {
			cmdExecuted = cmd
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("StartService systemd: unexpected cmd %s or sudo %v", cmd, options.Sudo)
	}
	err := r.StartService(context.Background(), serviceName)
	if err != nil {t.Fatalf("StartService systemd error = %v", err)}
	if cmdExecuted != expectedCmd {t.Errorf("StartService systemd cmd = %q, want %q", cmdExecuted, expectedCmd)}
}

func TestRunner_StopService_SysV(t *testing.T) {
	r, mockConn := newTestRunnerForService(t, false) // SysV
	serviceName := "apache2"
	expectedCmd := fmt.Sprintf(sysvinitInfo.StopCmd, serviceName)
	var cmdExecuted string

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd; mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		if cmd == expectedCmd && options.Sudo {
			cmdExecuted = cmd
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("StopService sysv: unexpected cmd %s or sudo %v", cmd, options.Sudo)
	}
	err := r.StopService(context.Background(), serviceName)
	if err != nil {t.Fatalf("StopService sysv error = %v", err)}
	if cmdExecuted != expectedCmd {t.Errorf("StopService sysv cmd = %q, want %q", cmdExecuted, expectedCmd)}
}

func TestRunner_IsServiceActive_Systemd_True(t *testing.T) {
	r, mockConn := newTestRunnerForService(t, true)
	serviceName := "sshd"

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd; mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		// systemctl is-active --quiet sshd
		if strings.Contains(cmd, "systemctl is-active --quiet") && strings.Contains(cmd, serviceName) && !options.Sudo {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("IsServiceActive systemd: unexpected cmd %s or sudo %v", cmd, options.Sudo)
	}
	active, err := r.IsServiceActive(context.Background(), serviceName)
	if err != nil {t.Fatalf("IsServiceActive systemd error = %v", err)}
	if !active {t.Error("IsServiceActive systemd = false, want true")}
}

func TestRunner_IsServiceActive_Systemd_False(t *testing.T) {
	r, mockConn := newTestRunnerForService(t, true)
	serviceName := "inactive-svc"
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd; mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		if strings.Contains(cmd, "systemctl is-active --quiet") && strings.Contains(cmd, serviceName) {
			return nil, nil, &connector.CommandError{ExitCode: 3}
		}
		return nil, nil, errors.New("IsServiceActive systemd false: unexpected cmd")
	}
	active, err := r.IsServiceActive(context.Background(), serviceName)
	if err != nil {t.Fatalf("IsServiceActive systemd (false) error = %v (expected nil from IsServiceActive itself)", err)}
	if active {t.Error("IsServiceActive systemd (false) = true, want false")}
}


func TestRunner_DaemonReload_Systemd(t *testing.T) {
	r, mockConn := newTestRunnerForService(t, true)
	expectedCmd := systemdInfo.DaemonReloadCmd
	var cmdExecuted string
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd; mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		if cmd == expectedCmd && options.Sudo {
			cmdExecuted = cmd
			return nil, nil, nil
		}
		return nil, nil, errors.New("DaemonReload systemd: unexpected cmd")
	}
	err := r.DaemonReload(context.Background())
	if err != nil {t.Fatalf("DaemonReload systemd error = %v", err)}
	if cmdExecuted != expectedCmd {t.Errorf("DaemonReload systemd cmd = %q, want %q", cmdExecuted, expectedCmd)}
}

func TestRunner_EnableService_Systemd(t *testing.T) {
	r, mockConn := newTestRunnerForService(t, true)
	serviceName := "my-app"
	expectedCmd := fmt.Sprintf(systemdInfo.EnableCmd, serviceName)
	var cmdExecuted string

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd; mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		if cmd == expectedCmd && options.Sudo {
			cmdExecuted = cmd
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("EnableService systemd: unexpected command %s", cmd)
	}
	err := r.EnableService(context.Background(), serviceName)
	if err != nil {
		t.Fatalf("EnableService for systemd error: %v", err)
	}
	if cmdExecuted != expectedCmd {
		t.Errorf("EnableService systemd command mismatch. Got: %s, Want: %s", cmdExecuted, expectedCmd)
	}
}
