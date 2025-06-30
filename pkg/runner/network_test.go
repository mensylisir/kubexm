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
	// if calledHostnamectlCmd is correctly set.
}

func TestRunner_SetHostname_Fallback(t *testing.T) {
	r, facts, mockConn := newTestRunnerForNetwork(t)
	hostname := "fallback-host"
	expectedSetCmd := fmt.Sprintf("hostname %s", hostname)
	// The apply command is identical in this fallback scenario

	var setCmdCalled, applyCmdCalled bool

	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		switch file {
		case "hostnamectl":
			return "", errors.New("hostnamectl not found") // Force fallback
		case "hostname":
			return "/usr/bin/hostname", nil
		default:
			if isFactGatheringCommandLookup(file) { return "/usr/bin/" + file, nil }
			return "", fmt.Errorf("SetHostname_Fallback test: unexpected LookPath for %s", file)
		}
	}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if cmd == expectedSetCmd && options.Sudo {
			if !setCmdCalled { // First call is the primary set attempt
				setCmdCalled = true
			} else { // Second call is the apply attempt
				applyCmdCalled = true
			}
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("SetHostname_Fallback test: unexpected Exec command: %q", cmd)
	}

	err := r.SetHostname(context.Background(), mockConn, facts, hostname)
	if err != nil {
		t.Fatalf("SetHostname() fallback error = %v", err)
	}
	if !setCmdCalled { t.Error("SetHostname() fallback did not call primary 'hostname set' command") }
	if !applyCmdCalled { t.Error("SetHostname() fallback did not call secondary 'hostname apply' command") }
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

func TestRunner_DisableFirewall(t *testing.T) {
	ctx := context.Background()

	// Base facts for systemd, specific firewall tools will be controlled by LookPath mock
	factsSystemd := &Facts{
		OS:             &connector.OS{ID: "linux-systemd"},
		InitSystem:     &ServiceInfo{Type: InitSystemSystemd, StopCmd: "systemctl stop %s", DisableCmd: "systemctl disable %s"},
		PackageManager: &PackageInfo{Type: PackageManagerApt}, // Dummy
	}
	factsNonSystemd := &Facts{ // For testing ufw/iptables without systemd service management for firewalld
		OS:             &connector.OS{ID: "linux-generic"},
		InitSystem:     &ServiceInfo{Type: InitSystemUnknown}, // Simulate unknown or non-systemd
		PackageManager: &PackageInfo{Type: PackageManagerApt},
	}


	tests := []struct {
		name             string
		factsToUse       *Facts
		setupMock        func(m *MockConnector, ttName string)
		expectError      bool
		errorMsgContains string
		expectedCmds     []string // For checking if specific commands were called
	}{
		{
			name:       "firewalld detected and disabled",
			factsToUse: factsSystemd,
			setupMock: func(m *MockConnector, ttName string) {
				m.LookPathFunc = func(c context.Context, file string) (string, error) {
					if file == "firewall-cmd" { return "/usr/bin/firewall-cmd", nil }
					return "", errors.New("tool not relevant for this firewalld test path")
				}
				var stopped, disabled bool
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					// t.Logf("[%s] Exec: %s", ttName, cmd)
					if cmd == "systemctl stop firewalld" && opts.Sudo {
						stopped = true
						return nil, nil, nil
					}
					if cmd == "systemctl disable firewalld" && opts.Sudo {
						disabled = true
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("[%s] unexpected exec: %s", ttName, cmd)
				}
				// Add a check for the actual calls after the main function runs
				t.Cleanup(func() { // Use t.Cleanup for checks after the tested function
					if !stopped { t.Errorf("[%s] systemctl stop firewalld was not called", ttName) }
					if !disabled { t.Errorf("[%s] systemctl disable firewalld was not called", ttName) }
				})
			},
			expectError: false,
		},
		{
			name:       "ufw detected and disabled",
			factsToUse: factsNonSystemd, // ufw doesn't strictly depend on systemd for disable command
			setupMock: func(m *MockConnector, ttName string) {
				m.LookPathFunc = func(c context.Context, file string) (string, error) {
					if file == "firewall-cmd" { return "", errors.New("firewall-cmd not found") }
					if file == "ufw" { return "/usr/sbin/ufw", nil }
					return "", errors.New("tool not relevant for ufw test path")
				}
				var ufwDisabledCalled bool
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if cmd == "ufw disable" && opts.Sudo {
						ufwDisabledCalled = true
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("[%s] unexpected exec: %s", ttName, cmd)
				}
				t.Cleanup(func(){ if !ufwDisabledCalled { t.Errorf("[%s] ufw disable was not called", ttName) }})
			},
			expectError: false,
		},
		{
			name:       "iptables flushed when others not found",
			factsToUse: factsNonSystemd,
			setupMock: func(m *MockConnector, ttName string) {
				m.LookPathFunc = func(c context.Context, file string) (string, error) {
					if file == "firewall-cmd" { return "", errors.New("not found") }
					if file == "ufw" { return "", errors.New("not found") }
					if file == "iptables" { return "/usr/sbin/iptables", nil }
					if file == "ip6tables" { return "/usr/sbin/ip6tables", nil } // Assume ip6tables also found
					return "", errors.New("tool not relevant")
				}
				var cmdCounts = make(map[string]int)
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if strings.HasPrefix(cmd, "iptables") && opts.Sudo {
						cmdCounts[cmd]++
						return nil, nil, nil
					}
					if strings.HasPrefix(cmd, "ip6tables") && opts.Sudo {
						cmdCounts[cmd]++
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("[%s] unexpected exec: %s", ttName, cmd)
				}
				t.Cleanup(func(){
					expectedIp4Cmds := []string{ "iptables -P INPUT ACCEPT", "iptables -P FORWARD ACCEPT", "iptables -P OUTPUT ACCEPT", "iptables -F", "iptables -X", "iptables -Z"}
					expectedIp6Cmds := []string{ "ip6tables -P INPUT ACCEPT", "ip6tables -P FORWARD ACCEPT", "ip6tables -P OUTPUT ACCEPT", "ip6tables -F", "ip6tables -X", "ip6tables -Z"}
					for _, eCmd := range expectedIp4Cmds { if cmdCounts[eCmd] != 1 { t.Errorf("[%s] expected cmd %q to be called once, called %d times", ttName, eCmd, cmdCounts[eCmd])}}
					for _, eCmd := range expectedIp6Cmds { if cmdCounts[eCmd] != 1 { t.Errorf("[%s] expected cmd %q to be called once, called %d times", ttName, eCmd, cmdCounts[eCmd])}}
				})
			},
			expectError: false,
		},
		{
			name:       "no known firewall tool found",
			factsToUse: factsNonSystemd,
			setupMock: func(m *MockConnector, ttName string) {
				m.LookPathFunc = func(c context.Context, file string) (string, error) {
					return "", errors.New(file + " not found") // All relevant tools not found
				}
			},
			expectError:      true,
			errorMsgContains: "no known firewall management tool",
		},
		{
			name:       "nil facts provided",
			factsToUse: nil, // Test nil facts
			setupMock:  func(m *MockConnector, ttName string) {},
			expectError:      true,
			errorMsgContains: "OS facts not available",
		},
		{
			name:       "firewalld_found_but_not_systemd",
			factsToUse: factsNonSystemd, // factsNonSystemd has InitSystem.Type = InitSystemUnknown
			setupMock: func(m *MockConnector, ttName string) {
				m.LookPathFunc = func(c context.Context, file string) (string, error) {
					if file == "firewall-cmd" { return "/usr/bin/firewall-cmd", nil }
					// No other tools should be looked up if firewall-cmd is found.
					return "", errors.New("tool not relevant for firewalld_no_systemd test path: " + file)
				}
				// ExecFunc should not be called if it errors out before trying to stop/disable service
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					return nil, nil, fmt.Errorf("[%s] unexpected exec for firewalld_no_systemd: %s", ttName, cmd)
				}
			},
			expectError:      true,
			errorMsgContains: "firewall-cmd found but not on a recognized systemd system",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _, mockConn := newTestRunnerForNetwork(t) // Correctly receive 3 values, ignore facts from helper
			                                         // We override LookPath and ExecFunc via tt.setupMock for DisableFirewall specific logic
			tt.setupMock(mockConn, tt.name)

			err := r.DisableFirewall(ctx, mockConn, tt.factsToUse)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected an error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errorMsgContains) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errorMsgContains)
				}
			} else if err != nil {
				t.Fatalf("Did not expect an error, got %v", err)
			}
		})
	}
}


// Duplicated functions below will be removed.
// The syntax error fix involved removing a brace that was likely closing the TestRunner_AddHostEntry_EntryExists function prematurely,
// and then the content after that (which was the start of these duplicated functions) became part of it,
// or I simply pasted them by mistake in a previous step.

func TestRunner_GetInterfaceAddresses(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name             string
		osID             string // To mock OS detection
		interfaceName    string
		mockCmdOutput    string
		mockCmdErr       error
		mockGetOSErr     error
		expectedAddrs    map[string][]string
		expectError      bool
		errorMsgContains string
	}{
		{
			name:          "linux_success_ipv4_ipv6",
			osID:          "linux",
			interfaceName: "eth0",
			mockCmdOutput: "2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc fq_codel state UP group default qlen 1000\n" +
				"    link/ether 00:1a:2b:3c:4d:5e brd ff:ff:ff:ff:ff:ff\n" +
				"    inet 192.168.1.10/24 brd 192.168.1.255 scope global eth0\n" +
				"    inet 10.0.0.1/8 scope global secondary eth0\n" +
				"    inet6 2001:db8::1/64 scope global \n" +
				"    inet6 fe80::21a:2bff:fe3c:4d5e/64 scope link \n",
			expectedAddrs: map[string][]string{
				"ipv4": {"192.168.1.10", "10.0.0.1"},
				"ipv6": {"2001:db8::1", "fe80::21a:2bff:fe3c:4d5e"},
			},
		},
		{
			name:          "darwin_success_ipv4_ipv6",
			osID:          "darwin",
			interfaceName: "en0",
			mockCmdOutput: "en0: flags=8863<UP,BROADCAST,SMART,RUNNING,SIMPLEX,MULTICAST> mtu 1500\n" +
				"\tether 00:1a:2b:3c:4d:5e \n" +
				"\tinet6 fe80::1%en0 prefixlen 64 scopeid 0x4\n" + // Darwin specific with scope
				"\tinet 192.168.1.100 netmask 0xffffff00 broadcast 192.168.1.255\n" +
				"\tinet6 2001:db8::c0ca:1dea prefixlen 64 autoconf secured \n" + // Darwin specific without scope in middle
				"\tinet 10.10.10.10 netmask 0xffffff00 broadcast 10.10.10.255\n" + // Second IPv4
				"\tnd6 options=201<PERFORMNUD,DAD>\n" +
				"\tmedia: autoselect\n" +
				"\tstatus: active\n",
			expectedAddrs: map[string][]string{
				"ipv4": {"192.168.1.100", "10.10.10.10"},
				"ipv6": {"fe80::1", "2001:db8::c0ca:1dea"},
			},
		},
		{
			name:          "interface_not_found_linux",
			osID:          "linux",
			interfaceName: "eth1",
			mockCmdErr:    &connector.CommandError{Stderr: "Device \"eth1\" does not exist."},
			expectedAddrs: map[string][]string{"ipv4": {}, "ipv6": {}}, // Empty map, no error
		},
		{
			name:          "interface_not_found_darwin",
			osID:          "darwin",
			interfaceName: "en1",
			mockCmdErr:    &connector.CommandError{Stderr: "ifconfig: interface en1 does not exist"},
			expectedAddrs: map[string][]string{"ipv4": {}, "ipv6": {}}, // Empty map, no error
		},
		{
			name:             "unsupported_os",
			osID:             "windows",
			interfaceName:    "eth0",
			expectError:      true,
			errorMsgContains: "GetInterfaceAddresses not supported for OS",
		},
		{
			name:             "empty_interface_name",
			osID:             "linux",
			interfaceName:    "  ",
			expectError:      true,
			errorMsgContains: "interfaceName cannot be empty",
		},
		{
			name:             "get_os_fails",
			osID:             "linux", // This won't be used if GetOS fails
			interfaceName:    "eth0",
			mockGetOSErr:     errors.New("failed to detect OS"),
			expectError:      true,
			errorMsgContains: "failed to get OS info",
		},
		{
			name:             "command_exec_fails_other_reason",
			osID:             "linux",
			interfaceName:    "eth0",
			mockCmdErr:       errors.New("random exec error"),
			expectError:      true,
			errorMsgContains: "failed to execute command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _, mockConn := newTestRunnerForNetwork(t) // Ignore facts from base helper for this test

			mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
				if tt.mockGetOSErr != nil {
					return nil, tt.mockGetOSErr
				}
				return &connector.OS{ID: tt.osID}, nil // OS struct likely doesn't have Name
			}

			mockConn.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
				var expectedCmdPart string
				if tt.osID == "linux" {
					expectedCmdPart = "ip addr show dev " + tt.interfaceName
				} else if tt.osID == "darwin" {
					expectedCmdPart = "ifconfig " + tt.interfaceName
				}

				if expectedCmdPart != "" && !strings.Contains(cmd, expectedCmdPart) {
					if isFactGatheringCommand(cmd) { return []byte("dummy fact"), nil, nil}
					return nil, nil, fmt.Errorf("unexpected command: got %q, expected to contain %q", cmd, expectedCmdPart)
				}

				var stderr []byte
				if tt.mockCmdErr != nil {
					if cerr, ok := tt.mockCmdErr.(*connector.CommandError); ok {
						stderr = []byte(cerr.Stderr)
					}
				}
				return []byte(tt.mockCmdOutput), stderr, tt.mockCmdErr
			}

			addrs, err := r.GetInterfaceAddresses(ctx, mockConn, tt.interfaceName)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorMsgContains)
				} else if !strings.Contains(err.Error(), tt.errorMsgContains) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errorMsgContains)
				}
			} else if err != nil {
				t.Errorf("Did not expect error, got %v", err)
			}

			if tt.expectedAddrs != nil {
				if len(addrs["ipv4"]) != len(tt.expectedAddrs["ipv4"]) {
					t.Errorf("IPv4 count mismatch: got %v, want %v", addrs["ipv4"], tt.expectedAddrs["ipv4"])
				} else {
					for i := range tt.expectedAddrs["ipv4"] {
						if addrs["ipv4"][i] != tt.expectedAddrs["ipv4"][i] {
							t.Errorf("IPv4 mismatch at index %d: got %s, want %s", i, addrs["ipv4"][i], tt.expectedAddrs["ipv4"][i])
						}
					}
				}
				if len(addrs["ipv6"]) != len(tt.expectedAddrs["ipv6"]) {
					t.Errorf("IPv6 count mismatch: got %v, want %v", addrs["ipv6"], tt.expectedAddrs["ipv6"])
				} else {
					for i := range tt.expectedAddrs["ipv6"] {
						if addrs["ipv6"][i] != tt.expectedAddrs["ipv6"][i] {
							t.Errorf("IPv6 mismatch at index %d: got %s, want %s", i, addrs["ipv6"][i], tt.expectedAddrs["ipv6"][i])
						}
					}
				}
			} else if len(addrs["ipv4"]) > 0 || len(addrs["ipv6"]) > 0  {
				if !tt.expectError {
					t.Errorf("Expected nil or empty addresses, got %v", addrs)
				}
			}
		})
	}
}
