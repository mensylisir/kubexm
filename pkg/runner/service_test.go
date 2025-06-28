package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// Helper to setup runner with a specific init system for service tests
func newTestRunnerForService(t *testing.T, isSystemd bool) (Runner, *Facts, *MockConnector) {
	mockConn := NewMockConnector()
	osIDForFacts := "linux-test-systemd"
	if !isSystemd {
		osIDForFacts = "linux-test-sysv"
	}

	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return &connector.OS{ID: osIDForFacts, Arch: "amd64", Kernel: "test-kernel"}, nil
	}

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if mockConn.ExecHistory == nil {
			mockConn.ExecHistory = []string{}
		}
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)

		if strings.Contains(cmd, "hostname") {
			return []byte("service-test-host"), nil, nil
		}
		if strings.Contains(cmd, "nproc") {
			return []byte("1"), nil, nil
		}
		if strings.Contains(cmd, "grep MemTotal") {
			return []byte("1024000"), nil, nil
		}
		if strings.Contains(cmd, "ip -4 route get 8.8.8.8") {
			return []byte("8.8.8.8 dev eth0 src 1.1.1.1"), nil, nil
		}
		if strings.Contains(cmd, "ip -6 route get") {
			return nil, nil, fmt.Errorf("no ipv6")
		}
		if strings.HasPrefix(cmd, "test -e /etc/init.d") {
			if isSystemd {
				return nil, nil, errors.New("/etc/init.d not relevant for systemd mock")
			}
			return nil, nil, nil
		}
		return []byte(""), nil, nil
	}

	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		// Use a more specific local helper for LookPath in service tests if needed,
		// or rely on the isExecCmdForFactsInServiceTest for broader ExecFunc mocking.
		// This mock prioritizes init system commands.
		if isSystemd {
			if file == "systemctl" {
				return "/usr/bin/systemctl", nil
			}
			if file == "service" {
				return "", errors.New("service command not found when systemd is primary")
			}
		} else { // SysV
			if file == "systemctl" {
				return "", errors.New("systemctl not found for SysV")
			}
			if file == "service" {
				return "/usr/sbin/service", nil
			}
		}
		// Fallback for other commands that might be looked up by GatherFacts indirectly
		factRelatedTools := []string{"hostname", "uname", "nproc", "grep", "awk", "ip", "cat", "test", "command"}
		for _, frt := range factRelatedTools {
			if strings.Contains(file, frt) { // Make it a bit more general
				return "/usr/bin/" + file, nil
			}
		}
		return "", fmt.Errorf("LookPath mock (service test): command %s not expected for init system (systemd: %v)", file, isSystemd)
	}

	r := NewRunner()
	facts, err := r.GatherFacts(context.Background(), mockConn)
	if err != nil {
		t.Fatalf("newTestRunnerForService: Failed to gather facts for init system (systemd: %v): %v", isSystemd, err)
	}
	if facts == nil {
		t.Fatalf("newTestRunnerForService: GatherFacts returned nil facts (systemd: %v)", isSystemd)
	}
	if facts.InitSystem == nil {
		t.Fatalf("newTestRunnerForService for OS '%s' (systemd: %v): facts.InitSystem is nil. Check mock LookPath for systemctl/service.", osIDForFacts, isSystemd)
	}
	return r, facts, mockConn
}

func TestRunner_StartService_Systemd(t *testing.T) {
	r, facts, mockConn := newTestRunnerForService(t, true) // Systemd
	serviceName := "nginx"
	expectedCmd := fmt.Sprintf(facts.InitSystem.StartCmd, serviceName)
	var cmdExecuted string

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isExecCmdForFactsInServiceTest(cmd) {
			return []byte("dummy"), nil, nil
		}
		if cmd == expectedCmd && options.Sudo {
			cmdExecuted = cmd
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("StartService systemd: unexpected cmd %s or sudo %v", cmd, options.Sudo)
	}
	err := r.StartService(context.Background(), mockConn, facts, serviceName)
	if err != nil {
		t.Fatalf("StartService systemd error = %v", err)
	}
	if cmdExecuted != expectedCmd {
		t.Errorf("StartService systemd cmd = %q, want %q", cmdExecuted, expectedCmd)
	}
}

func TestRunner_StopService_SysV(t *testing.T) {
	r, facts, mockConn := newTestRunnerForService(t, false) // SysV
	serviceName := "apache2"
	expectedCmd := fmt.Sprintf(facts.InitSystem.StopCmd, serviceName)
	var cmdExecuted string

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isExecCmdForFactsInServiceTest(cmd) {
			return []byte("dummy"), nil, nil
		}
		if cmd == expectedCmd && options.Sudo {
			cmdExecuted = cmd
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("StopService sysv: unexpected cmd %s or sudo %v", cmd, options.Sudo)
	}
	err := r.StopService(context.Background(), mockConn, facts, serviceName)
	if err != nil {
		t.Fatalf("StopService sysv error = %v", err)
	}
	if cmdExecuted != expectedCmd {
		t.Errorf("StopService sysv cmd = %q, want %q", cmdExecuted, expectedCmd)
	}
}

func TestRunner_IsServiceActive_Systemd_True(t *testing.T) {
	r, facts, mockConn := newTestRunnerForService(t, true)
	serviceName := "sshd"

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isExecCmdForFactsInServiceTest(cmd) {
			return []byte("dummy"), nil, nil
		}
		if strings.Contains(cmd, "systemctl is-active --quiet") && strings.Contains(cmd, serviceName) && (options == nil || !options.Sudo) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("IsServiceActive systemd: unexpected cmd %s or sudo %v", cmd, options != nil && options.Sudo)
	}
	active, err := r.IsServiceActive(context.Background(), mockConn, facts, serviceName)
	if err != nil {
		t.Fatalf("IsServiceActive systemd error = %v", err)
	}
	if !active {
		t.Error("IsServiceActive systemd = false, want true")
	}
}

func TestRunner_IsServiceActive_Systemd_False(t *testing.T) {
	r, facts, mockConn := newTestRunnerForService(t, true)
	serviceName := "inactive-svc"
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isExecCmdForFactsInServiceTest(cmd) {
			return []byte("dummy"), nil, nil
		}
		if strings.Contains(cmd, "systemctl is-active --quiet") && strings.Contains(cmd, serviceName) {
			return nil, nil, &connector.CommandError{ExitCode: 3} // systemctl is-active returns 3 for inactive
		}
		return nil, nil, errors.New("IsServiceActive systemd false: unexpected cmd")
	}
	active, err := r.IsServiceActive(context.Background(), mockConn, facts, serviceName)
	if err != nil {
		t.Fatalf("IsServiceActive systemd (false) error = %v (expected nil from IsServiceActive itself)", err)
	}
	if active {
		t.Error("IsServiceActive systemd (false) = true, want false")
	}
}

func TestRunner_DaemonReload_Systemd(t *testing.T) {
	r, facts, mockConn := newTestRunnerForService(t, true)
	expectedCmd := facts.InitSystem.DaemonReloadCmd
	var cmdExecuted string
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isExecCmdForFactsInServiceTest(cmd) {
			return []byte("dummy"), nil, nil
		}
		if cmd == expectedCmd && options.Sudo {
			cmdExecuted = cmd
			return nil, nil, nil
		}
		return nil, nil, errors.New("DaemonReload systemd: unexpected cmd")
	}
	err := r.DaemonReload(context.Background(), mockConn, facts)
	if err != nil {
		t.Fatalf("DaemonReload systemd error = %v", err)
	}
	if cmdExecuted != expectedCmd {
		t.Errorf("DaemonReload systemd cmd = %q, want %q", cmdExecuted, expectedCmd)
	}
}

func TestRunner_EnableService_Systemd(t *testing.T) {
	r, facts, mockConn := newTestRunnerForService(t, true)
	serviceName := "my-app"
	expectedCmd := fmt.Sprintf(facts.InitSystem.EnableCmd, serviceName)
	var cmdExecuted string

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isExecCmdForFactsInServiceTest(cmd) {
			return []byte("dummy"), nil, nil
		}
		if cmd == expectedCmd && options.Sudo {
			cmdExecuted = cmd
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("EnableService systemd: unexpected command %s", cmd)
	}
	err := r.EnableService(context.Background(), mockConn, facts, serviceName)
	if err != nil {
		t.Fatalf("EnableService for systemd error: %v", err)
	}
	if cmdExecuted != expectedCmd {
		t.Errorf("EnableService systemd command mismatch. Got: %s, Want: %s", cmdExecuted, expectedCmd)
	}
}

// isExecCmdForFactsInServiceTest helper (can be shared or local if variations needed)
// This is similar to the one in user_test.go, kept local for now.
func isExecCmdForFactsInServiceTest(cmd string) bool {
	return strings.Contains(cmd, "hostname") ||
		strings.Contains(cmd, "uname -r") ||
		strings.Contains(cmd, "nproc") ||
		strings.Contains(cmd, "grep MemTotal") ||
		strings.Contains(cmd, "ip -4 route") ||
		strings.Contains(cmd, "ip -6 route") ||
		strings.Contains(cmd, "command -v") ||
		strings.Contains(cmd, "test -e /etc/init.d")
}
