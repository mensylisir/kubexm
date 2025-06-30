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
		if file == "grep" { return "/usr/bin/grep", nil } // Grep is also looked up by some Check impls if not piped
		if isFactGatheringCommandLookup(file) { return "/usr/bin/" + file, nil}
		return "", errors.New("unexpected LookPath call in IsPortOpen_False_netstat")
	}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		// Construct the exact command string IsPortOpen is expected to generate for netstat
		expectedNetstatCmd := fmt.Sprintf("netstat -ltn | grep -q ':%d\\b.*LISTEN'", port)
		if cmd == expectedNetstatCmd {
			return nil, nil, &connector.CommandError{ExitCode: 1} // Simulate grep not finding (exit code 1)
		}
		// Fallback for any other unexpected command during this specific test
		return nil, nil, fmt.Errorf("IsPortOpen_False_netstat test: unexpected cmd %s", cmd)
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
	// t.Unskip("Re-evaluating TestRunner_WaitForPort_Success") // Unskipping
	r, facts, mockConn := newTestRunnerForNetwork(t)
	port := 1234
	callCount := 0

	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if file == "ss" {
			// t.Logf("WaitForPort_Success: LookPathFunc found 'ss'")
			return "/usr/bin/ss", nil
		}
		// t.Logf("WaitForPort_Success: LookPathFunc unexpected file '%s'", file)
		return "", fmt.Errorf("WaitForPort_Success test: unexpected LookPath for %s", file)
	}

	ssCmdToMatch := fmt.Sprintf("ss -ltn | grep -q ':%d '", port)
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd // Still useful for some checks
		// t.Logf("WaitForPort_Success: ExecFunc received cmd: %q, callCount: %d", cmd, callCount)
		if cmd == ssCmdToMatch {
			callCount++
			if callCount >= 3 { // Succeed on 3rd or later attempt
				// t.Logf("WaitForPort_Success: ExecFunc success for %q (attempt %d)", cmd, callCount)
				return nil, nil, nil // Port is open
			}
			// t.Logf("WaitForPort_Success: ExecFunc fail (CmdError Exit 1) for %q (attempt %d)", cmd, callCount)
			return nil, nil, &connector.CommandError{ExitCode: 1} // Port not open
		}
		// t.Logf("WaitForPort_Success: ExecFunc unexpected command %q", cmd)
		return nil, nil, fmt.Errorf("WaitForPort_Success test: unexpected command %s", cmd)
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
	expectedHostnamectlCmd := fmt.Sprintf("hostnamectl set-hostname %s", hostname)
	// In this test case, hostnamectl is found, so the plain "hostname" apply command should not run.

	var calledHostnamectlCmd string

	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		switch file {
		case "hostnamectl":
			return "/usr/bin/hostnamectl", nil
		case "hostname": // May be looked up by SetHostname if hostnamectl fails, or for apply.
			return "/usr/bin/hostname", nil
		default:
			if isFactGatheringCommandLookup(file) { return "/usr/bin/" + file, nil }
			return "", fmt.Errorf("SetHostname_Success test: unexpected LookPath for %s", file)
		}
	}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		// t.Logf("SetHostname_Success Mock Exec: cmd=%q, sudo=%v", cmd, options.Sudo) // DEBUGGING

		if cmd == expectedHostnamectlCmd && options.Sudo {
			calledHostnamectlCmd = cmd
			return nil, nil, nil
		}
		// The plain 'hostname <hostname>' command should NOT be called if hostnamectl was used.
		// If it were, it would be an error in the SetHostname logic or this test's premise.
		plainHostnameCmd := fmt.Sprintf("hostname %s", hostname)
		if cmd == plainHostnameCmd && options.Sudo {
			return nil, nil, fmt.Errorf("SetHostname_Success test: plain hostname command %q was called unexpectedly (hostnamectl should have been used and sufficed)", cmd)
		}
		return nil, nil, fmt.Errorf("SetHostname_Success test: unexpected Exec command: %q", cmd)
	}

	err := r.SetHostname(context.Background(), mockConn, facts, hostname)
	if err != nil {
		t.Fatalf("SetHostname() error = %v", err)
	}
	if calledHostnamectlCmd == "" {
		t.Error("SetHostname() did not call the expected hostnamectl command")
	}
	// Depending on SetHostname logic, if hostnamectl is used, the direct 'hostname' apply might be skipped.
	// The original test logged if applyHostnameCmd was empty, this check should be more specific
	// to the path taken (hostnamectl vs plain hostname). For this test, hostnamectl path is taken.
	// So applyHostnameCmd (if we were capturing it separately) should be empty.
	// The current failure "did not call the apply hostname command" is only relevant if the fallback path was taken.
	// For now, we just check hostnamectlCmd was called. A separate test should cover the fallback.
	// The original test's error message "SetHostname() did not call the apply hostname command" will go away
	// if calledHostnamectlCmd is correctly set, because the test structure was:
	// if hostnamectlCmd == "" { t.Error("did not call hostnamectl") }
	// if applyHostnameCmd == "" { t.Log("did not call apply...") } -> this was not an t.Error()
	// The actual FAIL was from "did not call hostnamectl"
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
		// if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil } // Not needed if ExecFunc is specific

		expectedGrepCmd := fmt.Sprintf("grep -Fxq '%s' /etc/hosts", expectedEntry)
		expectedAppendCmd := fmt.Sprintf("echo '%s' >> /etc/hosts", expectedEntry)

		if cmd == expectedGrepCmd && !options.Sudo { // grep is not sudo
			grepCalled = true
			return nil, nil, &connector.CommandError{ExitCode: 1} // Simulate entry not found
		}
		if cmd == expectedAppendCmd && options.Sudo { // append is sudo
			echoCalled = true
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("AddHostEntry new: unexpected cmd %q", cmd)
	}

	err := r.AddHostEntry(context.Background(), mockConn, ip, fqdn, hostname)
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
		// if isFactGatheringCommand(cmd) { return []byte("dummy"), nil, nil }

		expectedGrepCmd := fmt.Sprintf("grep -Fxq '%s' /etc/hosts", expectedEntry)
		if cmd == expectedGrepCmd && !options.Sudo {
			grepCalled = true
			return nil, nil, nil // Simulate entry found
		}
		if strings.Contains(cmd, "echo") { // Should not be called
			echoCalled = true
		}
		return nil, nil, fmt.Errorf("AddHostEntry exists: unexpected cmd %q", cmd)
	}

	err := r.AddHostEntry(context.Background(), mockConn, ip, fqdn)
	if err != nil {
		t.Fatalf("AddHostEntry() when entry exists error = %v", err)
	}
	if !grepCalled {t.Error("AddHostEntry (exists) did not call grep")}
	if echoCalled {t.Error("AddHostEntry (exists) unexpectedly called echo to add entry")}
}

func TestRunner_DisableFirewall_NotImplemented(t *testing.T) {
	r, facts, mockConn := newTestRunnerForNetwork(t)
	err := r.DisableFirewall(context.Background(), mockConn, facts)
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("DisableFirewall expected 'not implemented' error, got %v", err)
	}
}

// Duplicated functions below will be removed.
// The syntax error fix involved removing a brace that was likely closing the TestRunner_AddHostEntry_EntryExists function prematurely,
// and then the content after that (which was the start of these duplicated functions) became part of it,
// or I simply pasted them by mistake in a previous step.
