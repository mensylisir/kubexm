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

func TestRunner_IsServiceActive_SysV(t *testing.T) {
	serviceName := "apache-httpd"

	tests := []struct {
		name           string
		mockStdout     string
		mockErr        error
		expectedActive bool
		expectError    bool
		errorContains  string
	}{
		{
			name:           "active_running_in_stdout",
			mockStdout:     "apache-httpd (pid 1234) is running...",
			mockErr:        nil, // Exit 0
			expectedActive: true,
		},
		{
			name:           "active_exit_0_generic_stdout",
			mockStdout:     "Apache HTTP Server is configured to start.", // No explicit "running"
			mockErr:        nil,                                      // Exit 0
			expectedActive: true, // Current behavior defaults to true on exit 0
		},
		{
			name:           "inactive_command_error_exit_3", // e.g. LSB status codes often use 3 for not running
			mockStdout:     "apache-httpd is stopped",
			mockErr:        &connector.CommandError{ExitCode: 3, Stderr: "Not running"},
			expectedActive: false,
		},
		{
			name:          "execution_error",
			mockStdout:    "",
			mockErr:       errors.New("failed to execute service command"),
			expectError:   true,
			errorContains: "failed to check SysV service status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, facts, mockConn := newTestRunnerForService(t, false) // SysV
			if facts.InitSystem.Type != InitSystemSysV {
				t.Fatal("Test setup error: Expected SysV init system")
			}
			expectedCmd := fmt.Sprintf(facts.InitSystem.IsActiveCmd, serviceName)

			mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
				if isExecCmdForFactsInServiceTest(cmd) { return []byte("dummy"), nil, nil }
				if cmd == expectedCmd && (options == nil || !options.Sudo) {
					return []byte(tt.mockStdout), nil, tt.mockErr
				}
				return nil, nil, fmt.Errorf("IsServiceActive SysV test: unexpected cmd %s", cmd)
			}

			active, err := r.IsServiceActive(context.Background(), mockConn, facts, serviceName)

			if tt.expectError {
				if err == nil { t.Errorf("Expected error containing %q, got nil", tt.errorContains) }
				if err != nil && !strings.Contains(err.Error(), tt.errorContains) { t.Errorf("Error message %q does not contain %q", err.Error(), tt.errorContains) }
			} else if err != nil {
				t.Errorf("Did not expect error, got %v", err)
			}
			if active != tt.expectedActive { t.Errorf("Expected active %v, got %v", tt.expectedActive, active) }
		})
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

func TestRunner_EnableService_SysV_Unsupported(t *testing.T) {
	r, facts, mockConn := newTestRunnerForService(t, false) // SysV
	serviceName := "legacy-app"

	// Modify facts to simulate an unsupported SysV enable command
	originalEnableCmd := facts.InitSystem.EnableCmd
	facts.InitSystem.EnableCmd = "chkconfig_no_placeholder" // Not a template string

	err := r.EnableService(context.Background(), mockConn, facts, serviceName)
	if err == nil {
		t.Fatalf("EnableService for SysV with bad command template expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not reliably supported for detected SysV init variant") {
		t.Errorf("Error message mismatch: got %q", err.Error())
	}

	facts.InitSystem.EnableCmd = "" // Empty command
	err = r.EnableService(context.Background(), mockConn, facts, serviceName)
	if err == nil {
		t.Fatalf("EnableService for SysV with empty command template expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not reliably supported for detected SysV init variant") {
		t.Errorf("Error message mismatch for empty cmd: got %q", err.Error())
	}
	facts.InitSystem.EnableCmd = originalEnableCmd // Restore
}

func TestRunner_EnableService_SysV_Supported(t *testing.T) {
	r, facts, mockConn := newTestRunnerForService(t, false) // SysV
	serviceName := "mycustom-svc"

	// Ensure facts.InitSystem.EnableCmd is a valid template for this test
	facts.InitSystem.EnableCmd = "update-rc.d %s defaults" // A common SysV enable pattern
	expectedCmd := fmt.Sprintf(facts.InitSystem.EnableCmd, serviceName)
	var cmdExecuted string

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if isExecCmdForFactsInServiceTest(cmd) { return []byte("dummy"), nil, nil }
		if cmd == expectedCmd && options.Sudo {
			cmdExecuted = cmd; return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("EnableService SysV (supported): unexpected cmd %s", cmd)
	}

	err := r.EnableService(context.Background(), mockConn, facts, serviceName)
	if err != nil { t.Fatalf("EnableService for SysV (supported) error: %v", err) }
	if cmdExecuted != expectedCmd { t.Errorf("EnableService SysV (supported) cmd = %q, want %q", cmdExecuted, expectedCmd) }
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
