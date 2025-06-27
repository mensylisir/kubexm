package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// Helper to quickly get a runner with a mock connector for network tests
func newTestRunnerForNetwork(t *testing.T) (Runner, *Facts, *MockConnector) { // Updated signature
	mockConn := NewMockConnector()
	// Setup mockConn.GetOSFunc and mockConn.ExecFunc for basic fact gathering
	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return &connector.OS{ID: "linux-network-test", Arch: "amd64", Kernel: "net-kernel"}, nil
	}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if mockConn.ExecHistory == nil { mockConn.ExecHistory = []string{} }
		mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)

		if strings.Contains(cmd, "hostname") { return []byte("network-test-host"), nil, nil }
		if strings.Contains(cmd, "nproc") { return []byte("2"), nil, nil }
		if strings.Contains(cmd, "grep MemTotal") { return []byte("2048000"), nil, nil } // 2GB
		if strings.Contains(cmd, "ip -4 route get 8.8.8.8") { return []byte("8.8.8.8 dev eth1 src 10.0.0.10"), nil, nil }
		if strings.Contains(cmd, "ip -6 route get") { return nil, nil, fmt.Errorf("no ipv6 for network test") }
		if strings.Contains(cmd, "command -v") { return []byte("/usr/bin/" + strings.Fields(cmd)[2]), nil, nil } // Basic mock for command -v
		if strings.HasPrefix(cmd, "test -e /etc/init.d") { return nil, nil, errors.New("no /etc/init.d for this mock")}

		// Fallback for commands not specific to fact gathering for network tests
		// Individual tests will override this for commands like 'ss', 'netstat', 'hostnamectl'
		// fmt.Printf("newTestRunnerForNetwork: Default ExecFunc called for: %s\n", cmd)
		return []byte(""), nil, nil
	}
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		 switch file {
			case "ss", "netstat", "hostnamectl", "hostname", "grep", "awk", "ip", "cat", "uname", "nproc", "systemctl", "service", "apt-get", "yum", "dnf":
				return "/usr/bin/" + file, nil
			default:
				return "", fmt.Errorf("LookPath mock (network): command %s not found", file)
		}
	}

	r := NewRunner()
	facts, err := r.GatherFacts(context.Background(), mockConn)
	if err != nil {
		t.Fatalf("newTestRunnerForNetwork: Failed to gather facts: %v", err)
	}
	if facts == nil {
        t.Fatalf("newTestRunnerForNetwork: GatherFacts returned nil facts")
    }
	return r, facts, mockConn
}

// isFactGatheringCommandLookup is a helper for LookPath during NewRunner
func isFactGatheringCommandLookup(cmd string) bool {
	return cmd == "hostname" || cmd == "uname" || cmd == "nproc" || cmd == "grep" || cmd == "awk" || cmd == "ip" || cmd == "cat"
}


func TestRunner_IsPortOpen_True_ss(t *testing.T) {
	r, facts, mockConn := newTestRunnerForNetwork(t)
	port := 8080

	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if file == "ss" { return "/usr/bin/ss", nil }
		if isFactGatheringCommandLookup(file) { return "/usr/bin/" + file, nil}
		return "", errors.New("unexpected LookPath call in IsPortOpen test")
	}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if strings.Contains(cmd, "ss -ltn") && strings.Contains(cmd, fmt.Sprintf(":%d ", port)) {
			return nil, nil, nil
		}
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		return nil, nil, fmt.Errorf("IsPortOpen ss: unexpected cmd %s", cmd)
	}

	isOpen, err := r.IsPortOpen(context.Background(), mockConn, facts, port)
	if err != nil {
		t.Fatalf("IsPortOpen() error = %v", err)
	}
	if !isOpen {
		t.Error("IsPortOpen() = false, want true")
	}
}

func TestRunner_IsPortOpen_False_netstat(t *testing.T) {
	r, facts, mockConn := newTestRunnerForNetwork(t)
	port := 80

	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if file == "ss" { return "", errors.New("ss not found") }
		if file == "netstat" { return "/usr/bin/netstat", nil }
		if isFactGatheringCommandLookup(file) { return "/usr/bin/" + file, nil}
		return "", errors.New("unexpected LookPath call")
	}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if strings.Contains(cmd, "netstat -ltn") && strings.Contains(cmd, fmt.Sprintf(":%d\b.*LISTEN", port)) {
			return nil, nil, &connector.CommandError{ExitCode: 1}
		}
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		return nil, nil, fmt.Errorf("IsPortOpen netstat: unexpected cmd %s", cmd)
	}

	isOpen, err := r.IsPortOpen(context.Background(), mockConn, facts, port)
	if err != nil {
		t.Fatalf("IsPortOpen() error = %v (expected nil from IsPortOpen itself)", err)
	}
	if isOpen {
		t.Error("IsPortOpen() = true, want false")
	}
}

func TestRunner_WaitForPort_Success(t *testing.T) {
	r, facts, mockConn := newTestRunnerForNetwork(t)
	port := 1234
	callCount := 0

	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if file == "ss" { return "/usr/bin/ss", nil }
		if isFactGatheringCommandLookup(file) { return "/usr/bin/" + file, nil}
		return "/usr/bin/ss", nil // Default to ss for simplicity
	}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }

		// For IsPortOpen calls within WaitForPort
		if strings.Contains(cmd, "ss -ltn") && strings.Contains(cmd, fmt.Sprintf(":%d ", port)) {
			callCount++
			if callCount < 3 {
				return nil, nil, &connector.CommandError{ExitCode: 1}
			}
			return nil, nil, nil // Port open on 3rd call
		}
		return nil, nil, fmt.Errorf("WaitForPort Success: unexpected cmd %s", cmd)
	}

	err := r.WaitForPort(context.Background(), mockConn, facts, port, 5*time.Second)
	if err != nil {
		t.Fatalf("WaitForPort() error = %v", err)
	}
	if callCount < 3 {
		t.Errorf("WaitForPort() IsPortOpen was called %d times, expected at least 3", callCount)
	}
}

func TestRunner_WaitForPort_Timeout(t *testing.T) {
	r, facts, mockConn := newTestRunnerForNetwork(t)
	port := 4321

	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if file == "ss" { return "/usr/bin/ss", nil }
		if isFactGatheringCommandLookup(file) { return "/usr/bin/" + file, nil}
		return "/usr/bin/ss", nil
	}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		if strings.Contains(cmd, "ss -ltn") { // IsPortOpen check
			return nil, nil, &connector.CommandError{ExitCode: 1} // Always not open
		}
		return nil, nil, fmt.Errorf("WaitForPort Timeout: unexpected cmd %s", cmd)
	}

	err := r.WaitForPort(context.Background(), mockConn, facts, port, 100*time.Millisecond)
	if err == nil {
		t.Fatal("WaitForPort() expected timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "timed out waiting for port") {
		t.Errorf("Error message = %q, want to contain 'timed out waiting for port'", err.Error())
	}
}

func TestRunner_SetHostname_Success(t *testing.T) {
	r, facts, mockConn := newTestRunnerForNetwork(t)
	hostname := "new-test-host"

	var hostnamectlCmd, applyHostnameCmd string
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if file == "hostnamectl" { return "/usr/bin/hostnamectl", nil }
		if file == "hostname" { return "/usr/bin/hostname", nil } // For apply command
		if isFactGatheringCommandLookup(file) { return "/usr/bin/" + file, nil}
		return "", errors.New("hostnamectl not found for this test variation if it's called")
	}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }

		if strings.Contains(cmd, "hostnamectl set-hostname "+hostname) && options.Sudo {
			hostnamectlCmd = cmd
			return nil, nil, nil
		}
		if strings.Contains(cmd, "hostname "+hostname) && options.Sudo {
			applyHostnameCmd = cmd
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("SetHostname: unexpected cmd %s", cmd)
	}

	err := r.SetHostname(context.Background(), mockConn, facts, hostname)
	if err != nil {
		t.Fatalf("SetHostname() error = %v", err)
	}
	if hostnamectlCmd == "" {
		t.Error("SetHostname() did not call hostnamectl")
	}
	if applyHostnameCmd == "" {
		t.Log("SetHostname() did not call the apply hostname command, or it was not captured correctly.")
	}
}

func TestRunner_AddHostEntry_NewEntry(t *testing.T) {
	r, _, mockConn := newTestRunnerForNetwork(t) // Ignored facts
	ip := "10.0.0.5"
	fqdn := "server.example.com"
	hostname := "server"
	expectedEntry := fmt.Sprintf("%s %s %s", ip, fqdn, hostname)

	var grepCalled, echoCalled bool
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		if strings.Contains(cmd, "grep -Fxq") && strings.Contains(cmd, expectedEntry) {
			grepCalled = true
			return nil, nil, &connector.CommandError{ExitCode: 1}
		}
		if strings.Contains(cmd, fmt.Sprintf("echo '%s' >> /etc/hosts", expectedEntry)) && options.Sudo {
			echoCalled = true
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("AddHostEntry new: unexpected cmd %s", cmd)
	}

	err := r.AddHostEntry(context.Background(), mockConn, ip, fqdn, hostname) // Added mockConn, facts implicitly not used by AddHostEntry
	if err != nil {
		t.Fatalf("AddHostEntry() error = %v", err)
	}
	if !grepCalled {t.Error("AddHostEntry did not call grep to check existing")}
	if !echoCalled {t.Error("AddHostEntry did not call echo to add entry")}
}

func TestRunner_AddHostEntry_EntryExists(t *testing.T) {
	r, _, mockConn := newTestRunnerForNetwork(t) // Ignored facts
	ip := "10.0.0.6"
	fqdn := "db.example.com"
	expectedEntry := fmt.Sprintf("%s %s", ip, fqdn)

	var grepCalled, echoCalled bool
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }
		if strings.Contains(cmd, "grep -Fxq") && strings.Contains(cmd, expectedEntry) {
			grepCalled = true
			return nil, nil, nil
		}
		if strings.Contains(cmd, "echo") {
			echoCalled = true
		}
		return nil, nil, fmt.Errorf("AddHostEntry exists: unexpected cmd %s", cmd)
	}

	err := r.AddHostEntry(context.Background(), mockConn, ip, fqdn) // Added mockConn, facts implicitly not used
	if err != nil {
		t.Fatalf("AddHostEntry() when entry exists error = %v", err)
	}
	if !grepCalled {t.Error("AddHostEntry (exists) did not call grep")}
	if echoCalled {t.Error("AddHostEntry (exists) unexpectedly called echo to add entry")}
}
