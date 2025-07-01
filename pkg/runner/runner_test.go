package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time" // Needed for DeployAndEnableService and Reboot tests

	"github.com/mensylisir/kubexm/pkg/connector"
)

// TestNewRunner tests the NewRunner constructor.
func TestNewRunner(t *testing.T) {
	r := NewRunner()
	if r == nil {
		t.Fatal("NewRunner() returned nil")
	}
	if _, ok := r.(*defaultRunner); !ok {
		t.Errorf("NewRunner() did not return a *defaultRunner, got %T", r)
	}
}

// TestDefaultRunner_GatherFacts tests the GatherFacts method of defaultRunner.
func TestDefaultRunner_GatherFacts(t *testing.T) {
	ctx := context.Background()

	// Subtest for successful fact gathering
	t.Run("success", func(t *testing.T) {
		mockConn := NewMockConnector() // Assumes NewMockConnector() is defined and working
		mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
			return &connector.OS{ID: "linux", Arch: "amd64", Kernel: "5.4.0-generic"}, nil
		}
		mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
			if file == "apt-get" { return "/usr/bin/apt-get", nil }
			if file == "dnf" { return "", errors.New("dnf not found by LookPath") }
			if file == "yum" { return "", errors.New("yum not found by LookPath") }
			if file == "systemctl" { return "/usr/bin/systemctl", nil }
			// Default for other commands like hostname, nproc, etc. if LookPath were used for them.
			// For GatherFacts, these are typically direct exec.
			return "/usr/bin/" + file, nil
		}
		mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
			mockConn.LastExecCmd = cmd
			mockConn.LastExecOptions = options
			if mockConn.ExecHistory == nil {
				mockConn.ExecHistory = []string{}
			}
			mockConn.ExecHistory = append(mockConn.ExecHistory, cmd)

			if strings.Contains(cmd, "hostname -f") {
				return []byte("test-host-fqdn"), nil, nil
			}
			if strings.Contains(cmd, "hostname") && !strings.Contains(cmd, "hostname -f") { // Fallback
				return []byte("test-host"), nil, nil
			}
			if strings.Contains(cmd, "nproc") {
				return []byte("4"), nil, nil
			}
			if strings.Contains(cmd, "grep MemTotal /proc/meminfo") {
				return []byte("8192000"), nil, nil // 8GB in KB
			}
			if strings.Contains(cmd, "ip -4 route get 8.8.8.8") {
				// Simulate output that includes the IP address in the 7th field
				// The actual command uses awk '{print $7}', so mock should return just that.
				return []byte("192.168.1.100"), nil, nil
			}
			if strings.Contains(cmd, "ip -6 route get") {
				return nil, nil, fmt.Errorf("no ipv6 route") // Simulate no IPv6
			}
			// No need to mock "command -v" for package/init if LookPathFunc is correctly set
			return nil, nil, fmt.Errorf("GatherFacts.success: unhandled mock Exec command: %s", cmd)
		}

		r := NewRunner()
		facts, err := r.GatherFacts(ctx, mockConn)
		if err != nil {
			t.Fatalf("GatherFacts() error = %v, wantErr nil", err)
		}
		if facts == nil {
			t.Fatal("GatherFacts() returned nil facts, want non-nil")
		}
		if facts.Hostname != "test-host-fqdn" {
			t.Errorf("Facts.Hostname = %s, want test-host-fqdn", facts.Hostname)
		}
		if facts.TotalCPU != 4 {
			t.Errorf("Facts.TotalCPU = %d, want 4", facts.TotalCPU)
		}
		if facts.TotalMemory != 8000 { // 8192000 KB / 1024 = 8000 MB
			t.Errorf("Facts.TotalMemory = %d, want 8000", facts.TotalMemory)
		}
		if facts.OS == nil || facts.OS.ID != "linux" {
			t.Errorf("Facts.OS.ID = %v, want linux", facts.OS)
		}
		if facts.Kernel != "5.4.0-generic" { // Assuming OS.Kernel is used
			t.Errorf("Facts.Kernel = %s, want 5.4.0-generic", facts.Kernel)
		}
		if facts.IPv4Default != "192.168.1.100" {
			t.Errorf("Facts.IPv4Default = %s, want 192.168.1.100", facts.IPv4Default)
		}
		if facts.PackageManager == nil || facts.PackageManager.Type != PackageManagerApt {
			t.Errorf("Facts.PackageManager.Type = %v, want %v", facts.PackageManager, PackageManagerApt)
		}
		if facts.InitSystem == nil || facts.InitSystem.Type != InitSystemSystemd {
			t.Errorf("Facts.InitSystem.Type = %v, want %v", facts.InitSystem, InitSystemSystemd)
		}
	})

	t.Run("cpu_info_fails", func(t *testing.T) {
		mockConn := NewMockConnector() // GetOSFunc will use default mock (success: linux)
		expectedErr := errors.New("nproc command failed")
		mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
			if strings.Contains(cmd, "hostname -f") { // Hostname succeeds
				return []byte("test-host"), nil, nil
			}
			if strings.Contains(cmd, "nproc") { // CPU info fails
				return nil, nil, expectedErr
			}
			// Other commands for other facts succeed
			if strings.Contains(cmd, "grep MemTotal") { return []byte("1024000"), nil, nil } // 1GB
			if strings.Contains(cmd, "ip -4 route") { return []byte("1.1.1.1 dev eth0 src 1.1.1.1"), nil, nil }
			if strings.Contains(cmd, "ip -6 route") { return nil, nil, fmt.Errorf("no ipv6") }
			if strings.Contains(cmd, "command -v") { return []byte("found"), nil, nil} // for package/init detection
			return nil, nil, fmt.Errorf("cpu_info_fails: unhandled mock command: %s", cmd)
		}

		r := NewRunner()
		_, err := r.GatherFacts(ctx, mockConn)
		if err == nil {
			t.Fatalf("GatherFacts() with nproc failing expected error, got nil")
		}
		// errgroup returns the first error encountered by any of its Go routines.
		if !strings.Contains(err.Error(), "failed during concurrent fact gathering") || !errors.Is(err, expectedErr) {
			t.Errorf("GatherFacts() error = %v, want error containing 'failed during concurrent fact gathering' and wrapping mock nproc error", err)
		}
	})

	t.Run("success_centos_yum", func(t *testing.T) {
		mockConn := NewMockConnector()
		mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
			return &connector.OS{ID: "centos", Arch: "amd64", Kernel: "3.10.0-generic"}, nil
		}
		mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
			if file == "dnf" { return "", errors.New("dnf not found by LookPath") }
			if file == "yum" { return "/usr/bin/yum", nil }
			if file == "systemctl" { return "/usr/bin/systemctl", nil }
			return "/usr/bin/" + file, nil
		}
		mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
			mockConn.LastExecCmd = cmd
			if strings.Contains(cmd, "hostname -f") { return []byte("centos-host.local"), nil, nil }
			if strings.Contains(cmd, "nproc") { return []byte("2"), nil, nil }
			if strings.Contains(cmd, "grep MemTotal /proc/meminfo") { return []byte("4096000"), nil, nil } // 4GB in KB
			if strings.Contains(cmd, "ip -4 route get 8.8.8.8") { return []byte("10.0.2.15"), nil, nil } // awk '{print $7}'
			if strings.Contains(cmd, "ip -6 route get") { return nil, nil, fmt.Errorf("no ipv6") }
			// "command -v" for dnf/yum not needed if LookPathFunc is specific
			return nil, nil, fmt.Errorf("GatherFacts.success_centos_yum: unhandled mock Exec command: %s", cmd)
		}

		r := NewRunner()
		facts, err := r.GatherFacts(ctx, mockConn)
		if err != nil {
			t.Fatalf("GatherFacts() for centos/yum error = %v, wantErr nil", err)
		}
		if facts == nil {
			t.Fatal("GatherFacts() for centos/yum returned nil facts, want non-nil")
		}
		if facts.Hostname != "centos-host.local" {
			t.Errorf("Facts.Hostname = %s, want centos-host.local", facts.Hostname)
		}
		if facts.PackageManager == nil || facts.PackageManager.Type != PackageManagerYum {
			t.Errorf("Facts.PackageManager.Type = %v, want %v", facts.PackageManager.Type, PackageManagerYum)
		}
		if facts.InitSystem == nil || facts.InitSystem.Type != InitSystemSystemd {
			t.Errorf("Facts.InitSystem.Type = %v, want %v", facts.InitSystem.Type, InitSystemSystemd)
		}
		if facts.IPv4Default != "10.0.2.15" {
			t.Errorf("Facts.IPv4Default = %s, want 10.0.2.15", facts.IPv4Default)
		}
	})

	t.Run("success_hostname_fallback", func(t *testing.T) {
		mockConn := NewMockConnector()
		mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
			return &connector.OS{ID: "linux", Arch: "amd64", Kernel: "5.4.0-generic"}, nil
		}
		mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
			if strings.Contains(cmd, "hostname -f") {
				return nil, nil, errors.New("hostname -f failed") // Simulate hostname -f failure
			}
			if strings.Contains(cmd, "hostname") && !strings.Contains(cmd, "hostname -f") { // Fallback
				return []byte("fallback-host"), nil, nil
			}
			// Provide minimal successful responses for other commands
			if strings.Contains(cmd, "nproc") { return []byte("1"), nil, nil }
			if strings.Contains(cmd, "grep MemTotal") { return []byte("1024000"), nil, nil }
			if strings.Contains(cmd, "ip -4 route") { return []byte("default via 192.168.1.1 dev eth0 src 192.168.1.101 "), nil, nil }
			if strings.Contains(cmd, "ip -6 route") { return nil, nil, fmt.Errorf("no ipv6") }
			if strings.Contains(cmd, "command -v") { return []byte("/usr/bin/somecmd"), nil, nil }
			return nil, nil, fmt.Errorf("GatherFacts.success_hostname_fallback: unhandled mock command: %s", cmd)
		}

		r := NewRunner()
		facts, err := r.GatherFacts(ctx, mockConn)
		if err != nil {
			t.Fatalf("GatherFacts() with hostname fallback error = %v", err)
		}
		if facts.Hostname != "fallback-host" {
			t.Errorf("Facts.Hostname = %s, want fallback-host", facts.Hostname)
		}
	})

	t.Run("get_os_fails", func(t *testing.T) {
		mockConn := NewMockConnector()
		expectedErr := errors.New("mock GetOS failed")
		mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
			return nil, expectedErr
		}
		// No need to set ExecFunc as GetOS is the first call.

		r := NewRunner()
		_, err := r.GatherFacts(ctx, mockConn)
		if err == nil {
			t.Fatalf("GatherFacts() with GetOS failing expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to get OS info") || !errors.Is(err, expectedErr) {
			t.Errorf("GatherFacts() error = %v, want error containing 'failed to get OS info' and wrapping mock error", err)
		}
	})

	t.Run("hostname_fails", func(t *testing.T) {
		mockConn := NewMockConnector() // GetOSFunc will use default mock (success: linux)
		expectedErr := errors.New("hostname command failed")
		mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
			if strings.Contains(cmd, "hostname") { // Both hostname -f and hostname
				return nil, nil, expectedErr
			}
			// Other commands for other facts succeed to isolate hostname failure
			if strings.Contains(cmd, "nproc") { return []byte("1"), nil, nil }
			if strings.Contains(cmd, "grep MemTotal") { return []byte("1024000"), nil, nil } // 1GB
			if strings.Contains(cmd, "ip -4 route") { return []byte("1.1.1.1 dev eth0 src 1.1.1.1"), nil, nil }
			if strings.Contains(cmd, "ip -6 route") { return nil, nil, fmt.Errorf("no ipv6") }
			if strings.Contains(cmd, "command -v") { return []byte("found"), nil, nil} // for package/init detection
			return nil, nil, fmt.Errorf("hostname_fails: unhandled mock command: %s", cmd)
		}

		r := NewRunner()
		_, err := r.GatherFacts(ctx, mockConn)
		if err == nil {
			t.Fatalf("GatherFacts() with hostname failing expected error, got nil")
		}
		// errgroup returns the first error, so we expect the hostname error.
		if !strings.Contains(err.Error(), "failed during concurrent fact gathering") || !errors.Is(err, expectedErr) {
			t.Errorf("GatherFacts() error = %v, want error containing 'failed during concurrent fact gathering' and wrapping mock hostname error", err)
		}
	})

	t.Run("connector_nil", func(t *testing.T) {
		r := NewRunner()
		_, err := r.GatherFacts(ctx, nil)
		if err == nil {
			t.Error("GatherFacts() with nil connector expected error, got nil")
		}
		if !strings.Contains(err.Error(), "connector cannot be nil") {
			t.Errorf("Error message mismatch, got %q, want to contain 'connector cannot be nil'", err.Error())
		}
	})

	t.Run("connector_not_connected", func(t *testing.T) {
		mockConn := NewMockConnector()
		mockConn.IsConnectedFunc = func() bool { return false } // Simulate disconnected

		r := NewRunner()
		_, err := r.GatherFacts(ctx, mockConn)
		if err == nil {
			t.Error("GatherFacts() with disconnected connector expected error, got nil")
		}
		if !strings.Contains(err.Error(), "connector is not connected") {
			t.Errorf("Error message mismatch, got %q, want to contain 'connector is not connected'", err.Error())
		}
	})
}

func TestRunner_DeployAndEnableService(t *testing.T) {
	ctx := context.Background()
	serviceName := "myservice"
	configPath := "/etc/myservice/service.conf"
	configContent := "key=value"
	permissions := "0600"

	factsForSystemd := &Facts{
		OS: &connector.OS{ID: "linux"},
		InitSystem: &ServiceInfo{
			Type:            InitSystemSystemd,
			DaemonReloadCmd: "systemctl daemon-reload",
			EnableCmd:       "systemctl enable %s",
			RestartCmd:      "systemctl restart %s",
		},
		PackageManager: &PackageInfo{Type: PackageManagerApt}, // Needed for GatherFacts in helper
	}

	// Template data
	tmplString := "key={{.MyKey}}"
	templateData := struct{ MyKey string }{MyKey: "myValue"}
	expectedRenderedContent := "key=myValue"


	tests := []struct {
		name             string
		facts            *Facts
		configContent    string
		templateData     interface{}
		mockSetup        func(m *MockConnector, expectedContent string)
		expectError      bool
		errorContains    string
		expectedCmdOrder []string // To check sequence of main operations (simplified)
	}{
		{
			name:          "success with literal content",
			facts:         factsForSystemd,
			configContent: configContent,
			templateData:  nil,
			mockSetup: func(m *MockConnector, expectedContent string) {
				var writeFileCalled, daemonReloadCalled, enableCalled, restartCalled bool
				m.WriteFileFunc = func(ctx context.Context, content []byte, destPath string, opts *connector.FileTransferOptions) error {
					if destPath == configPath && string(content) == expectedContent && opts.Sudo && opts.Permissions == permissions {
						writeFileCalled = true
						return nil
					}
					return fmt.Errorf("unexpected WriteFile call: path=%s, content=%s", destPath, string(content))
				}
				m.ExecFunc = func(ctx context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if cmd == factsForSystemd.InitSystem.DaemonReloadCmd && opts.Sudo {
						daemonReloadCalled = true
						return nil, nil, nil
					}
					if cmd == fmt.Sprintf(factsForSystemd.InitSystem.EnableCmd, serviceName) && opts.Sudo {
						enableCalled = true
						return nil, nil, nil
					}
					if cmd == fmt.Sprintf(factsForSystemd.InitSystem.RestartCmd, serviceName) && opts.Sudo {
						restartCalled = true
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("DeployAndEnableService: unexpected exec: %s", cmd)
				}
				// For assertions outside mock
				t.Helper()
				go func() {
					time.Sleep(100 * time.Millisecond) // Give time for calls to occur
					if !writeFileCalled { t.Error("WriteFile was not called") }
					if !daemonReloadCalled { t.Error("DaemonReload was not called") }
					if !enableCalled { t.Error("EnableService was not called") }
					if !restartCalled { t.Error("RestartService was not called") }
				}()
			},
			expectError:   false,
		},
		{
			name:          "success with template rendering",
			facts:         factsForSystemd,
			configContent: tmplString,
			templateData:  templateData,
			mockSetup: func(m *MockConnector, expectedContent string) {
				// expectedContent here will be the rendered one
				var writeFileCalled, daemonReloadCalled, enableCalled, restartCalled bool
				m.WriteFileFunc = func(ctx context.Context, content []byte, destPath string, opts *connector.FileTransferOptions) error {
					if destPath == configPath && string(content) == expectedContent && opts.Sudo && opts.Permissions == permissions {
						writeFileCalled = true
						return nil
					}
					return fmt.Errorf("unexpected WriteFile call: path=%s, content=%s, expectedContent=%s", destPath, string(content), expectedContent)
				}
				m.ExecFunc = func(ctx context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if cmd == factsForSystemd.InitSystem.DaemonReloadCmd && opts.Sudo { daemonReloadCalled = true; return nil, nil, nil }
					if cmd == fmt.Sprintf(factsForSystemd.InitSystem.EnableCmd, serviceName) && opts.Sudo { enableCalled = true; return nil, nil, nil }
					if cmd == fmt.Sprintf(factsForSystemd.InitSystem.RestartCmd, serviceName) && opts.Sudo { restartCalled = true; return nil, nil, nil }
					return nil, nil, fmt.Errorf("DeployAndEnableService: unexpected exec: %s", cmd)
				}
				t.Helper(); go func() { time.Sleep(100*time.Millisecond); if !writeFileCalled || !daemonReloadCalled || !enableCalled || !restartCalled { t.Log("One of the core functions not called for template path") } }()
			},
			expectError:   false,
		},
		{
			name:          "WriteFile fails",
			facts:         factsForSystemd,
			configContent: configContent,
			mockSetup: func(m *MockConnector, expectedContent string) {
				m.WriteFileFunc = func(ctx context.Context, content []byte, destPath string, opts *connector.FileTransferOptions) error {
					return errors.New("mock WriteFile failed")
				}
			},
			expectError:   true,
			errorContains: "failed to write configuration file",
		},
		{
			name:          "DaemonReload fails",
			facts:         factsForSystemd,
			configContent: configContent,
			mockSetup: func(m *MockConnector, expectedContent string) {
				m.WriteFileFunc = func(ctx context.Context, content []byte, destPath string, opts *connector.FileTransferOptions) error { return nil }
				m.ExecFunc = func(ctx context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if cmd == factsForSystemd.InitSystem.DaemonReloadCmd { return nil, nil, errors.New("mock daemon-reload failed")}
					return nil, nil, nil
				}
			},
			expectError: true,
			errorContains: "failed to perform daemon-reload",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRunner() // Use defaultRunner directly
			mockConn := NewMockConnector() // Each test gets a fresh mock

			expectedContent := tt.configContent
			if tt.templateData != nil {
				expectedContent = expectedRenderedContent // For this specific template
			}
			tt.mockSetup(mockConn, expectedContent)

			err := r.DeployAndEnableService(ctx, mockConn, tt.facts, serviceName, tt.configContent, configPath, permissions, tt.templateData)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, got nil")
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errorContains)
				}
			} else if err != nil {
				t.Errorf("Did not expect an error, got %v", err)
			}
		})
	}
}


func TestRunner_Reboot(t *testing.T) {
	ctx := context.Background()
	shortTimeout := 100 * time.Millisecond // For timeout test
	longTimeout := 7 * time.Second // For success test (e.g. 2 polls at 3s interval + initial sleep)


	tests := []struct {
		name           string
		timeout        time.Duration
		setupMock      func(m *MockConnector)
		expectError    bool
		errorContains  string
	}{
		{
			name: "reboot success after few checks",
			timeout: longTimeout,
			setupMock: func(m *MockConnector) {
				var execCallCount int
				rebootCmdIssued := false
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					execCallCount++
					// t.Logf("Reboot success mock: cmd=%s, count=%d", cmd, execCallCount)
					// Match the actual command pattern used by Reboot()
					if strings.Contains(cmd, "reboot") && strings.Contains(cmd, "sh -c") && opts.Sudo {
						rebootCmdIssued = true
						// Simulate connection drop by returning an error that Reboot() is expected to ignore
						return nil, nil, errors.New("session channel closed") // This error should be ignored by Reboot()
					}
					if rebootCmdIssued && cmd == "uptime" { // Liveness check
						if execCallCount >= 3 { // Succeeds on the 2nd uptime check (3rd exec overall)
							return []byte("uptime output"), nil, nil
						}
						return nil, nil, errors.New("host not responsive yet")
					}
					return nil, nil, fmt.Errorf("Reboot success mock: unexpected command %s (call %d)", cmd, execCallCount)
				}
			},
			expectError: false,
		},
		{
			name: "reboot times out",
			timeout: shortTimeout,
			setupMock: func(m *MockConnector) {
				rebootCmdIssued := false
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					// t.Logf("Reboot timeout mock: cmd=%s", cmd)
					// Match the actual command pattern used by Reboot()
					if strings.Contains(cmd, "reboot") && strings.Contains(cmd, "sh -c") && opts.Sudo {
						rebootCmdIssued = true
						// Simulate connection drop by returning an error that Reboot() is expected to ignore
						return nil, nil, errors.New("session channel closed") // This error should be ignored by Reboot()
					}
					if rebootCmdIssued && cmd == "uptime" { // Liveness check always fails
						return nil, nil, errors.New("host still not responsive")
					}
					return nil, nil, fmt.Errorf("Reboot timeout mock: unexpected command %s", cmd)
				}
			},
			expectError:   true,
			errorContains: "timed out waiting for host to become responsive",
		},
		{
			name: "reboot command itself fails (e.g., not found, permission)",
			timeout: shortTimeout, // Timeout doesn't matter much if initial cmd fails hard
			setupMock: func(m *MockConnector) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					// The actual command sent by Reboot is "sh -c 'sleep 2 && reboot > /dev/null 2>&1 &'"
					if strings.Contains(cmd, "reboot") && strings.Contains(cmd, "sh -c") && opts.Sudo { // More robust check
						// This error should NOT contain "context deadline exceeded", "session channel closed", "connection lost", or "EOF"
						// to trigger the early return from Reboot.
						return nil, []byte("critical reboot error"), &connector.CommandError{Stderr: "reboot command execution failed critically", ExitCode: 1}
					}
					return nil, nil, fmt.Errorf("Reboot cmd fail mock: unexpected command %s", cmd)
				}
			},
			expectError:   true,
			errorContains: "failed to issue reboot command", // This reflects the updated Reboot() behavior
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRunner()
			mockConn := NewMockConnector()
			// Ensure IsConnected is true for the initial reboot command attempt
			mockConn.IsConnectedFunc = func() bool { return true }
			tt.setupMock(mockConn)

			err := r.Reboot(ctx, mockConn, tt.timeout)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, got nil")
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errorContains)
				}
			} else if err != nil {
				t.Errorf("Did not expect an error, got %v", err)
			}
		})
	}
}
