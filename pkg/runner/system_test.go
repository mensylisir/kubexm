package runner

import (
	"context"
	"errors" // For errors.New
	"fmt"    // For fmt.Errorf
	"strings"
	"testing"
	// "time" // Only needed if Reboot test was here and used time.Duration

	"github.com/mensylisir/kubexm/pkg/connector" // For connector.CommandError
)

// newTestRunnerForSystemTests can be a simple helper for now.
func newTestRunnerForSystemTests(t *testing.T) (Runner, *MockConnector) {
	mockConn := NewMockConnector()
	r := NewRunner()
	// Provide a default LookPathFunc that can be overridden by tests if needed
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		// t.Logf("Default LookPathFunc in newTestRunnerForSystemTests looking for: %s", file)
		// Allow common tools to be "found" by default
		commonTools := []string{"modprobe", "lsmod", "sysctl", "timedatectl", "swapon", "swapoff", "cat", "test", "sed", "rm", "ln", "mkdir", "chmod"}
		for _, tool := range commonTools {
			if file == tool {
				return "/usr/bin/" + file, nil
			}
		}
		return "", fmt.Errorf("LookPath mock (system_test): tool %s not found", file)
	}
	// Provide a default ExecFunc that can be overridden.
	mockConn.ExecFunc = func(ctx context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
		// t.Logf("Default ExecFunc in newTestRunnerForSystemTests: cmd=%q, sudo=%v", cmd, opts.Sudo)
		return nil, nil, fmt.Errorf("unexpected command in default mock exec for system tests: %s", cmd)
	}
	return r, mockConn
}

func TestRunner_LoadModule(t *testing.T) {
	ctx := context.Background()
	moduleName := "overlay"
	params := []string{"d_type=on"}

	tests := []struct {
		name          string
		setupMock     func(m *MockConnector)
		moduleName    string
		params        []string
		expectError   bool
		errorContains string
	}{
		{
			name: "success loading module with params",
			setupMock: func(m *MockConnector) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					expectedCmd := fmt.Sprintf("modprobe %s %s", moduleName, strings.Join(params, " "))
					if cmd == expectedCmd && opts.Sudo {
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("LoadModule: unexpected exec: %s", cmd)
				}
			},
			moduleName:  moduleName,
			params:      params,
			expectError: false,
		},
		{
			name: "success loading module no params",
			setupMock: func(m *MockConnector) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					expectedCmd := fmt.Sprintf("modprobe %s", "dummy")
					if cmd == expectedCmd && opts.Sudo {
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("LoadModule: unexpected exec: %s", cmd)
				}
			},
			moduleName:  "dummy",
			params:      nil,
			expectError: false,
		},
		{
			name: "modprobe command fails",
			setupMock: func(m *MockConnector) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if strings.HasPrefix(cmd, "modprobe") {
						return nil, []byte("modprobe error"), errors.New("modprobe failed")
					}
					return nil, nil, fmt.Errorf("LoadModule: unexpected exec: %s", cmd)
				}
			},
			moduleName:    moduleName,
			expectError:   true,
			errorContains: "failed to load module",
		},
		{
			name:          "empty moduleName",
			setupMock:     func(m *MockConnector) {},
			moduleName:    "",
			expectError:   true,
			errorContains: "moduleName cannot be empty",
		},
		{
			name:          "invalid moduleName characters",
			setupMock:     func(m *MockConnector) {},
			moduleName:    "mod;ule",
			expectError:   true,
			errorContains: "invalid characters in moduleName",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForSystemTests(t)
			tt.setupMock(mockConn)

			err := r.LoadModule(ctx, mockConn, nil, tt.moduleName, tt.params...) // Facts not used by LoadModule

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

func TestRunner_IsModuleLoaded(t *testing.T) {
	ctx := context.Background()
	moduleName := "br_netfilter"

	tests := []struct {
		name          string
		setupMock     func(m *MockConnector, module string)
		moduleName    string
		expectedLoaded bool
		expectError   bool
		errorContains string
	}{
		{
			name: "module is loaded",
			setupMock: func(m *MockConnector, module string) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					expectedCmd := fmt.Sprintf("test -d /sys/module/%s", module)
					if cmd == expectedCmd && !opts.Sudo {
						return nil, nil, nil // Exit 0 means directory exists, module loaded
					}
					return nil, nil, fmt.Errorf("IsModuleLoaded: unexpected exec: %s", cmd)
				}
			},
			moduleName:    moduleName,
			expectedLoaded: true,
			expectError:   false,
		},
		{
			name: "module is not loaded",
			setupMock: func(m *MockConnector, module string) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					expectedCmd := fmt.Sprintf("test -d /sys/module/%s", module)
					if cmd == expectedCmd && !opts.Sudo {
						return nil, nil, &connector.CommandError{ExitCode: 1} // Exit 1 means not loaded
					}
					return nil, nil, fmt.Errorf("IsModuleLoaded: unexpected exec: %s", cmd)
				}
			},
			moduleName:    moduleName,
			expectedLoaded: false,
			expectError:   false,
		},
		{
			name: "check command execution error",
			setupMock: func(m *MockConnector, module string) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					return nil, nil, errors.New("exec arbitrary error")
				}
			},
			moduleName:    moduleName,
			expectedLoaded: false,
			expectError:   true,
			errorContains: "error checking for module",
		},
		{
			name:          "empty moduleName",
			setupMock:     func(m *MockConnector, module string) {},
			moduleName:    "",
			expectedLoaded: false,
			expectError:   true,
			errorContains: "moduleName cannot be empty",
		},
		{
			name:          "invalid moduleName characters",
			setupMock:     func(m *MockConnector, module string) {},
			moduleName:    "mod;ule",
			expectedLoaded: false,
			expectError:   true,
			errorContains: "invalid characters in moduleName",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForSystemTests(t)
			tt.setupMock(mockConn, tt.moduleName)

			loaded, err := r.IsModuleLoaded(ctx, mockConn, tt.moduleName)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, got nil")
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errorContains)
				}
			} else if err != nil {
				t.Errorf("Did not expect an error, got %v", err)
			}

			if loaded != tt.expectedLoaded {
				t.Errorf("Expected loaded %v, got %v", tt.expectedLoaded, loaded)
			}
		})
	}
}

func TestRunner_ConfigureModuleOnBoot(t *testing.T) {
	ctx := context.Background()
	moduleName := "nf_conntrack"
	params := []string{"hashsize=131072"}

	tests := []struct {
		name          string
		setupMock     func(m *MockConnector)
		moduleName    string
		params        []string
		expectError   bool
		errorContains string
	}{
		{
			name: "success configure module and params",
			setupMock: func(m *MockConnector) {
				// var mkdirCalled, writeModulesLoadCalled, writeModprobeDCalled bool // Unused
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					// Mkdirp for /etc/modules-load.d and /etc/modprobe.d
					if strings.HasPrefix(cmd, "mkdir -p") && (strings.Contains(cmd, "/etc/modules-load.d") || strings.Contains(cmd, "/etc/modprobe.d")) && opts.Sudo {
						// mkdirCalled = true
						return nil, nil, nil
					}
					// Chmod for these dirs (from Mkdirp)
					if strings.HasPrefix(cmd, "chmod 0755") && (strings.Contains(cmd, "/etc/modules-load.d") || strings.Contains(cmd, "/etc/modprobe.d")) && opts.Sudo {
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("ConfigureModuleOnBoot: unexpected exec: %s", cmd)
				}
				m.WriteFileFunc = func(c context.Context, content []byte, destPath string, opts *connector.FileTransferOptions) error {
					if destPath == fmt.Sprintf("/etc/modules-load.d/%s.conf", moduleName) && string(content) == moduleName+"\n" && opts.Sudo && opts.Permissions == "0644" {
						// writeModulesLoadCalled = true
						return nil
					}
					if destPath == fmt.Sprintf("/etc/modprobe.d/%s.conf", moduleName) && string(content) == fmt.Sprintf("options %s %s\n", moduleName, strings.Join(params, " ")) && opts.Sudo && opts.Permissions == "0644" {
						// writeModprobeDCalled = true
						return nil
					}
					return fmt.Errorf("ConfigureModuleOnBoot: unexpected WriteFile to %s with content %s", destPath, string(content))
				}
			},
			moduleName:  moduleName,
			params:      params,
			expectError: false,
		},
		{
			name: "success configure module no params",
			setupMock: func(m *MockConnector) {
				// var mkdirCalled, writeModulesLoadCalled bool // Unused
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if strings.HasPrefix(cmd, "mkdir -p") && strings.Contains(cmd, "/etc/modules-load.d") && opts.Sudo {
						// mkdirCalled = true
						return nil, nil, nil
					}
					if strings.HasPrefix(cmd, "chmod 0755") && strings.Contains(cmd, "/etc/modules-load.d") && opts.Sudo {
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("ConfigureModuleOnBoot: unexpected exec: %s", cmd)
				}
				m.WriteFileFunc = func(c context.Context, content []byte, destPath string, opts *connector.FileTransferOptions) error {
					if destPath == fmt.Sprintf("/etc/modules-load.d/%s.conf", "dummy") && string(content) == "dummy\n" && opts.Sudo {
						// writeModulesLoadCalled = true
						return nil
					}
					return fmt.Errorf("ConfigureModuleOnBoot: unexpected WriteFile to %s", destPath)
				}
			},
			moduleName:  "dummy",
			params:      nil,
			expectError: false,
		},
		{
			name:          "empty moduleName",
			setupMock:     func(m *MockConnector) {},
			moduleName:    "",
			expectError:   true,
			errorContains: "moduleName cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForSystemTests(t)
			tt.setupMock(mockConn)

			err := r.ConfigureModuleOnBoot(ctx, mockConn, nil, tt.moduleName, tt.params...) // Facts not used yet by this impl

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

func TestRunner_SetSysctl(t *testing.T) {
	ctx := context.Background()
	sysctlKey := "net.ipv4.ip_forward"
	sysctlValue := "1"
	sysctlConfFile := "/etc/sysctl.d/99-kubexm-runner.conf"

	tests := []struct {
		name          string
		setupMock     func(m *MockConnector)
		key           string
		value         string
		persistent    bool
		expectError   bool
		errorContains string
	}{
		{
			name: "set temporary only",
			setupMock: func(m *MockConnector) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					expectedCmd := fmt.Sprintf("sysctl -w %s=\"%s\"", sysctlKey, sysctlValue)
					if cmd == expectedCmd && opts.Sudo {
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("SetSysctl temporary: unexpected exec: %s", cmd)
				}
			},
			key:         sysctlKey,
			value:       sysctlValue,
			persistent:  false,
			expectError: false,
		},
		{
			name: "set persistent - entry not exists",
			setupMock: func(m *MockConnector) {
				var execCallCount int
				// var writtenContent string // Unused
				// Ensure LookPath allows necessary commands for Mkdirp
				m.LookPathFunc = func(ctx context.Context, file string) (string, error) {
					if file == "mkdir" || file == "chmod" || file == "sysctl" || file == "grep" || file == "tee" || file == "sh" {
						return "/usr/bin/" + file, nil
					}
					return "", fmt.Errorf("SetSysctl persistent LookPath: unexpected %s", file)
				}
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					execCallCount++
					// t.Logf("SetSysctl persistent exec call %d: %s", execCallCount, cmd)
					switch execCallCount {
					case 1: // Temporary set
						if cmd == fmt.Sprintf("sysctl -w %s=\"%s\"", sysctlKey, sysctlValue) && opts.Sudo {
							return nil, nil, nil
						}
					case 2: // Mkdirp for /etc/sysctl.d
						if strings.HasPrefix(cmd, "mkdir -p") && strings.Contains(cmd, "/etc/sysctl.d") && opts.Sudo {
							return nil, nil, nil
						}
					case 3: // Chmod for /etc/sysctl.d (from Mkdirp)
						if strings.HasPrefix(cmd, "chmod 0755") && strings.Contains(cmd, "/etc/sysctl.d") && opts.Sudo {
							return nil, nil, nil
						}
					case 4: // Check existing entry in conf file (grep)
						expectedLine := fmt.Sprintf("%s = %s", sysctlKey, sysctlValue)
						if cmd == fmt.Sprintf("grep -Fxq -- %s %s", shellEscape(expectedLine), shellEscape(sysctlConfFile)) && !opts.Sudo {
							return nil, nil, &connector.CommandError{ExitCode: 1} // Not found
						}
					case 5: // Append to conf file (echo | tee -a)
						expectedLine := fmt.Sprintf("%s = %s", sysctlKey, sysctlValue)
						if strings.HasPrefix(cmd, "echo "+shellEscape(expectedLine)) && strings.Contains(cmd, "tee -a "+shellEscape(sysctlConfFile)) && opts.Sudo {
							// writtenContent = expectedLine // Unused
							return nil, nil, nil
						}
					case 6: // Apply settings (sysctl -p)
						if cmd == "sysctl -p" && opts.Sudo {
							return nil, nil, nil
						}
					default:
						return nil, nil, fmt.Errorf("SetSysctl persistent: unexpected exec call %d: %s", execCallCount, cmd)
					}
					return nil, nil, fmt.Errorf("SetSysctl persistent: fallthrough in mock exec call %d for cmd: %s", execCallCount, cmd)
				}
			},
			key:         sysctlKey,
			value:       sysctlValue,
			persistent:  true,
			expectError: false,
		},
		{
			name:          "empty key",
			setupMock:     func(m *MockConnector) {},
			key:           "",
			value:         sysctlValue,
			persistent:    false,
			expectError:   true,
			errorContains: "key cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForSystemTests(t)
			tt.setupMock(mockConn)

			err := r.SetSysctl(ctx, mockConn, tt.key, tt.value, tt.persistent)

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

func TestRunner_SetTimezone(t *testing.T) {
	ctx := context.Background()
	timezone := "America/New_York"

	tests := []struct {
		name          string
		setupMock     func(m *MockConnector)
		timezone      string
		expectError   bool
		errorContains string
	}{
		{
			name: "success with timedatectl",
			setupMock: func(m *MockConnector) {
				m.LookPathFunc = func(c context.Context, file string) (string, error) {
					if file == "timedatectl" { return "/usr/bin/timedatectl", nil }
					return "", fmt.Errorf("SetTimezone: unexpected lookpath %s", file)
				}
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					expectedCmd := fmt.Sprintf("timedatectl set-timezone %s", shellEscape(timezone))
					if cmd == expectedCmd && opts.Sudo {
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("SetTimezone: unexpected exec: %s", cmd)
				}
			},
			timezone:    timezone,
			expectError: false,
		},
		{
			name: "timedatectl not found",
			setupMock: func(m *MockConnector) {
				m.LookPathFunc = func(c context.Context, file string) (string, error) {
					if file == "timedatectl" { return "", errors.New("timedatectl not found") }
					return "", fmt.Errorf("SetTimezone: unexpected lookpath %s", file)
				}
			},
			timezone:      timezone,
			expectError:   true,
			errorContains: "'timedatectl' command not found",
		},
		{
			name:          "empty timezone",
			setupMock:     func(m *MockConnector) {},
			timezone:      "",
			expectError:   true,
			errorContains: "timezone cannot be empty",
		},
		{
			name:          "invalid timezone format",
			setupMock:     func(m *MockConnector) {},
			timezone:      "../danger",
			expectError:   true,
			errorContains: "invalid characters or format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForSystemTests(t)
			tt.setupMock(mockConn)

			err := r.SetTimezone(ctx, mockConn, nil, tt.timezone) // Facts not used by current SetTimezone

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

func TestRunner_DisableSwap(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		setupMock     func(m *MockConnector)
		expectError   bool
		errorContains string
	}{
		{
			name: "success disabling swap",
			setupMock: func(m *MockConnector) {
				// var swapoffCalled, sedCalled bool // Unused
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if cmd == "swapoff -a" && opts.Sudo {
						// swapoffCalled = true
						return nil, nil, nil
					}
					if strings.HasPrefix(cmd, "sed -i'.kubexm-runner.bak' -E") && strings.Contains(cmd, "/etc/fstab") && opts.Sudo {
						// sedCalled = true
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("DisableSwap: unexpected exec: %s", cmd)
				}
			},
			expectError: false,
		},
		{
			name: "swapoff fails",
			setupMock: func(m *MockConnector) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if cmd == "swapoff -a" && opts.Sudo {
						return nil, []byte("swapoff error"), errors.New("swapoff failed")
					}
					return nil, nil, fmt.Errorf("DisableSwap: unexpected exec: %s", cmd)
				}
			},
			expectError:   true,
			errorContains: "failed to execute 'swapoff -a'",
		},
		{
			name: "sed command fails",
			setupMock: func(m *MockConnector) {
				// var swapoffCalled bool // Unused
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if cmd == "swapoff -a" && opts.Sudo {
						// swapoffCalled = true
						return nil, nil, nil
					}
					if strings.HasPrefix(cmd, "sed -i'.kubexm-runner.bak' -E") && opts.Sudo {
						return nil, []byte("sed error"), errors.New("sed failed")
					}
					return nil, nil, fmt.Errorf("DisableSwap: unexpected exec: %s", cmd)
				}
			},
			expectError:   true,
			errorContains: "failed to comment out swap entries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForSystemTests(t)
			tt.setupMock(mockConn)

			err := r.DisableSwap(ctx, mockConn, nil) // Facts not used by current DisableSwap

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

func TestRunner_IsSwapEnabled(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name            string
		setupMock       func(m *MockConnector)
		expectedEnabled bool
		expectError     bool
		errorContains   string
	}{
		{
			name: "swap is enabled - multiple entries",
			setupMock: func(m *MockConnector) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if cmd == "cat /proc/swaps" && !opts.Sudo {
						return []byte("Filename\t\t\t\tType\t\tSize\tUsed\tPriority\n" +
							"/dev/sda2\t\t\tpartition\t8388604\t0\t-2\n" +
							"/swapfile\t\t\tfile\t\t2097148\t0\t-3\n"), nil, nil
					}
					return nil, nil, fmt.Errorf("IsSwapEnabled: unexpected exec: %s", cmd)
				}
			},
			expectedEnabled: true,
			expectError:     false,
		},
		{
			name: "swap is enabled - one entry",
			setupMock: func(m *MockConnector) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if cmd == "cat /proc/swaps" && !opts.Sudo {
						return []byte("Filename\t\t\t\tType\t\tSize\tUsed\tPriority\n" +
							"/dev/sda2\t\t\tpartition\t8388604\t0\t-2\n"), nil, nil
					}
					return nil, nil, fmt.Errorf("IsSwapEnabled: unexpected exec: %s", cmd)
				}
			},
			expectedEnabled: true,
			expectError:     false,
		},
		{
			name: "swap is not enabled - only header",
			setupMock: func(m *MockConnector) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if cmd == "cat /proc/swaps" && !opts.Sudo {
						return []byte("Filename\t\t\t\tType\t\tSize\tUsed\tPriority\n"), nil, nil
					}
					return nil, nil, fmt.Errorf("IsSwapEnabled: unexpected exec: %s", cmd)
				}
			},
			expectedEnabled: false,
			expectError:     false,
		},
		{
			name: "swap is not enabled - empty output",
			setupMock: func(m *MockConnector) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if cmd == "cat /proc/swaps" && !opts.Sudo {
						return []byte(""), nil, nil
					}
					return nil, nil, fmt.Errorf("IsSwapEnabled: unexpected exec: %s", cmd)
				}
			},
			expectedEnabled: false,
			expectError:     false,
		},
		{
			name: "cat /proc/swaps fails",
			setupMock: func(m *MockConnector) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if cmd == "cat /proc/swaps" {
						return nil, []byte("cat error"), errors.New("cat /proc/swaps failed")
					}
					return nil, nil, fmt.Errorf("IsSwapEnabled: unexpected exec: %s", cmd)
				}
			},
			expectedEnabled: false,
			expectError:     true,
			errorContains:   "failed to read /proc/swaps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForSystemTests(t)
			tt.setupMock(mockConn)

			enabled, err := r.IsSwapEnabled(ctx, mockConn)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, got nil")
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errorContains)
				}
			} else if err != nil {
				t.Errorf("Did not expect an error, got %v", err)
			}

			if enabled != tt.expectedEnabled {
				t.Errorf("Expected enabled %v, got %v", tt.expectedEnabled, enabled)
			}
		})
	}
}
