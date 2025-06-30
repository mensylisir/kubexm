package runner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// shellEscapeFileTest is used for constructing expected command strings in tests.
func shellEscapeFileTest(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func newTestRunnerForFileTests(t *testing.T) (Runner, *MockConnector) {
	mockConn := NewMockConnector()
	r := NewRunner()
	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return &connector.OS{ID: "linux-filetest", Arch: "amd64"}, nil
	}
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		commonTools := []string{"mountpoint", "mkdir", "chmod", "ln", "mkfs.ext4", "mkfs.xfs", "mount", "grep", "sh", "cat", "df"}
		for _, tool := range commonTools {
			if file == tool {
				return "/usr/bin/" + file, nil
			}
		}
		// t.Logf("newTestRunnerForFileTests: LookPath mock: tool %s not found by default", file)
		return "", fmt.Errorf("LookPath mock (file_test): tool %s not found", file)
	}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
		// t.Logf("newTestRunnerForFileTests: Default Mock Exec: cmd=%q, sudo=%v", cmd, opts.Sudo)
		// This default ExecFunc should ideally not be hit if tests correctly mock specific commands.
		return nil, nil, fmt.Errorf("unexpected command in default mock exec for file tests: %s", cmd)
	}
	return r, mockConn
}

func TestRunner_IsMounted(t *testing.T) {
	ctx := context.Background()
	mountPath := "/mnt/mydata"

	tests := []struct {
		name            string
		setupMock       func(m *MockConnector, path string, ttName string)
		path            string
		expectedMounted bool
		expectError     bool
		errorContains   string
	}{
		{
			name: "is mounted - mountpoint cmd success",
			setupMock: func(m *MockConnector, path string, ttName string) {
				m.LookPathFunc = func(c context.Context, file string) (string, error) {
					if file == "mountpoint" { return "/usr/bin/mountpoint", nil }
					return "", fmt.Errorf("[%s] unexpected lookpath: %s", ttName, file)
				}
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if cmd == fmt.Sprintf("mountpoint -q %s", path) && !opts.Sudo {
						return nil, nil, nil // Exit 0 means it is a mountpoint
					}
					return nil, nil, fmt.Errorf("[%s] unexpected exec: %s", ttName, cmd)
				}
			},
			path:            mountPath,
			expectedMounted: true,
			expectError:     false,
		},
		{
			name: "is not mounted - mountpoint cmd exit 1",
			setupMock: func(m *MockConnector, path string, ttName string) {
				m.LookPathFunc = func(c context.Context, file string) (string, error) {
					if file == "mountpoint" { return "/usr/bin/mountpoint", nil }
					return "", fmt.Errorf("[%s] unexpected lookpath: %s", ttName, file)
				}
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if cmd == fmt.Sprintf("mountpoint -q %s", path) && !opts.Sudo {
						return nil, nil, &connector.CommandError{ExitCode: 1} // Exit 1 means not a mountpoint
					}
					return nil, nil, fmt.Errorf("[%s] unexpected exec: %s", ttName, cmd)
				}
			},
			path:            mountPath,
			expectedMounted: false,
			expectError:     false,
		},
		{
			name: "mountpoint command not found",
			setupMock: func(m *MockConnector, path string, ttName string) {
				m.LookPathFunc = func(c context.Context, file string) (string, error) {
					if file == "mountpoint" { return "", errors.New("mountpoint not found by mock") }
					return "", fmt.Errorf("[%s] unexpected lookpath: %s", ttName, file)
				}
			},
			path:            mountPath,
			expectedMounted: false,
			expectError:     true,
			errorContains:   "IsMounted: 'mountpoint' command not found",
		},
		{
			name:            "empty path",
			setupMock:       func(m *MockConnector, path string, ttName string) {},
			path:            "  ",
			expectedMounted: false,
			expectError:     true,
			errorContains:   "path cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForFileTests(t)
			tt.setupMock(mockConn, tt.path, tt.name)
			isMounted, err := r.IsMounted(ctx, mockConn, tt.path)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, got nil")
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errorContains)
				}
			} else if err != nil {
				t.Errorf("Did not expect an error, got %v", err)
			}
			if isMounted != tt.expectedMounted {
				t.Errorf("Expected mounted %v, got %v", tt.expectedMounted, isMounted)
			}
		})
	}
}

func TestRunner_CreateSymlink(t *testing.T) {
	ctx := context.Background()
	targetPath := "/original/file"
	linkPath := "/path/to/symlink"

	tests := []struct {
		name          string
		setupMock     func(m *MockConnector, tcSudo bool, ttName string)
		target        string
		link          string
		sudo          bool
		expectError   bool
		errorContains string
	}{
		{
			name: "success no sudo",
			setupMock: func(m *MockConnector, tcSudo bool, ttName string) {
				var mkdirCalled, chmodCalled bool
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					linkDir := filepath.Dir(linkPath)
					expectedMkdirCmd := fmt.Sprintf("mkdir -p %s", linkDir)
					expectedChmodCmd := fmt.Sprintf("chmod %s %s", "0755", linkDir)
					expectedLnCmd := fmt.Sprintf("ln -sf %s %s", shellEscapeFileTest(targetPath), shellEscapeFileTest(linkPath))

					if !mkdirCalled && cmd == expectedMkdirCmd && opts.Sudo == tcSudo {
						mkdirCalled = true; return nil, nil, nil
					}
					if mkdirCalled && !chmodCalled && cmd == expectedChmodCmd && opts.Sudo == tcSudo {
						chmodCalled = true; return nil, nil, nil
					}
					if mkdirCalled && chmodCalled && cmd == expectedLnCmd && opts.Sudo == tcSudo {
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("[%s] CreateSymlink: unexpected cmd %q, sudo %v (expected %v)", ttName, cmd, opts.Sudo, tcSudo)
				}
			},
			target: targetPath, link: linkPath, sudo: false, expectError: false,
		},
		{
			name: "success with sudo",
			setupMock: func(m *MockConnector, tcSudo bool, ttName string) {
				var mkdirCalled, chmodCalled bool
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					linkDir := filepath.Dir(linkPath)
					expectedMkdirCmd := fmt.Sprintf("mkdir -p %s", linkDir)
					expectedChmodCmd := fmt.Sprintf("chmod %s %s", "0755", linkDir)
					expectedLnCmd := fmt.Sprintf("ln -sf %s %s", shellEscapeFileTest(targetPath), shellEscapeFileTest(linkPath))
					if !mkdirCalled && cmd == expectedMkdirCmd && opts.Sudo == tcSudo {
						mkdirCalled = true; return nil, nil, nil
					}
					if mkdirCalled && !chmodCalled && cmd == expectedChmodCmd && opts.Sudo == tcSudo {
						chmodCalled = true; return nil, nil, nil
					}
					if mkdirCalled && chmodCalled && cmd == expectedLnCmd && opts.Sudo == tcSudo {
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("[%s] CreateSymlink: unexpected cmd %q, sudo %v (expected %v)", ttName, cmd, opts.Sudo, tcSudo)
				}
			},
			target: targetPath, link: linkPath, sudo: true, expectError: false,
		},
		{
			name: "ln command fails",
			setupMock: func(m *MockConnector, tcSudo bool, ttName string) {
				var mkdirCalled, chmodCalled bool
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					linkDir := filepath.Dir(linkPath)
					expectedMkdirCmd := fmt.Sprintf("mkdir -p %s", linkDir)
					expectedChmodCmd := fmt.Sprintf("chmod %s %s", "0755", linkDir)
					expectedLnCmd := fmt.Sprintf("ln -sf %s %s", shellEscapeFileTest(targetPath), shellEscapeFileTest(linkPath))
					if !mkdirCalled && cmd == expectedMkdirCmd && opts.Sudo == tcSudo {
						mkdirCalled = true; return nil, nil, nil
					}
					if mkdirCalled && !chmodCalled && cmd == expectedChmodCmd && opts.Sudo == tcSudo {
						chmodCalled = true; return nil, nil, nil
					}
					if mkdirCalled && chmodCalled && cmd == expectedLnCmd && opts.Sudo == tcSudo {
						return nil, []byte("ln error"), errors.New("ln failed")
					}
					return nil, nil, fmt.Errorf("[%s] CreateSymlink: unexpected cmd %q", ttName, cmd)
				}
			},
			target: targetPath, link: linkPath, sudo: false, expectError: true, errorContains: "failed to create symlink",
		},
		{
			name: "mkdir for parent fails",
			setupMock: func(m *MockConnector, tcSudo bool, ttName string) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					linkDir := filepath.Dir(linkPath)
					expectedMkdirCmd := fmt.Sprintf("mkdir -p %s", linkDir)
					if cmd == expectedMkdirCmd && opts.Sudo == tcSudo {
						return nil, []byte("mkdir error"), errors.New("mkdir failed")
					}
					return nil, nil, fmt.Errorf("[%s] CreateSymlink: unexpected cmd %q", ttName, cmd)
				}
			},
			target: targetPath, link: linkPath, sudo: false, expectError: true, errorContains: "failed to create parent directory",
		},
		{
			name: "empty target", setupMock: func(m *MockConnector, tcSudo bool, ttName string) {},
			target: "", link: linkPath, sudo: false, expectError: true, errorContains: "target cannot be empty",
		},
		{
			name: "empty linkPath", setupMock: func(m *MockConnector, tcSudo bool, ttName string) {},
			target: targetPath, link: "", sudo: false, expectError: true, errorContains: "linkPath cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForFileTests(t)
			tt.setupMock(mockConn, tt.sudo, tt.name)
			err := r.CreateSymlink(ctx, mockConn, tt.target, tt.link, tt.sudo)
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

func TestRunner_MakeFilesystem(t *testing.T) {
	ctx := context.Background()
	devicePath := "/dev/sdb1"
	fsType := "ext4"

	tests := []struct {
		name          string
		setupMock     func(m *MockConnector, device string, fsT string, force bool, ttName string)
		device        string
		fsType        string
		force         bool
		expectError   bool
		errorContains string
	}{
		{
			name: "success no force",
			setupMock: func(m *MockConnector, device string, fsT string, force bool, ttName string) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					expectedCmd := fmt.Sprintf("mkfs.%s %s", fsT, shellEscapeFileTest(device))
					if cmd == expectedCmd && opts.Sudo {
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("[%s] MakeFilesystem: unexpected cmd %s, sudo %v", ttName, cmd, opts.Sudo)
				}
			},
			device: devicePath, fsType: fsType, force: false, expectError: false,
		},
		{
			name: "success with force",
			setupMock: func(m *MockConnector, device string, fsT string, force bool, ttName string) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					expectedCmd := fmt.Sprintf("mkfs.%s -f %s", fsT, shellEscapeFileTest(device))
					if cmd == expectedCmd && opts.Sudo {
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("[%s] MakeFilesystem: unexpected cmd %s, sudo %v", ttName, cmd, opts.Sudo)
				}
			},
			device: devicePath, fsType: fsType, force: true, expectError: false,
		},
		{
			name: "mkfs command fails",
			setupMock: func(m *MockConnector, device string, fsT string, force bool, ttName string) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if strings.HasPrefix(cmd, "mkfs.") && strings.Contains(cmd, shellEscapeFileTest(device)) {
						return nil, []byte("mkfs error"), errors.New("mkfs failed")
					}
					return nil,nil, fmt.Errorf("[%s] MakeFilesystem: unexpected cmd %s", ttName, cmd)
				}
			},
			device: devicePath, fsType: fsType, force: false, expectError: true, errorContains: "failed to make filesystem",
		},
		{
			name: "empty device", setupMock: func(m *MockConnector, device string, fsT string, force bool, ttName string) {},
			device: "", fsType: fsType, expectError: true, errorContains: "device cannot be empty",
		},
		{
			name: "empty fsType", setupMock: func(m *MockConnector, device string, fsT string, force bool, ttName string) {},
			device: devicePath, fsType: "", expectError: true, errorContains: "fsType cannot be empty",
		},
		{
			name: "invalid fsType characters", setupMock: func(m *MockConnector, device string, fsT string, force bool, ttName string) {},
			device: devicePath, fsType: "ext4;", expectError: true, errorContains: "invalid characters in fsType",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForFileTests(t)
			tt.setupMock(mockConn, tt.device, tt.fsType, tt.force, tt.name)
			err := r.MakeFilesystem(ctx, mockConn, tt.device, tt.fsType, tt.force)
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

func TestRunner_EnsureMount(t *testing.T) {
	ctx := context.Background()
	device := "/dev/sdb1"
	mountPoint := "/mnt/data"
	fsType := "ext4"
	baseOptions := []string{"rw", "relatime"}

	tests := []struct {
		name                string
		setupMockSpecific   func(m *MockConnector, ttName string)
		device              string
		mountPoint          string
		fsType              string
		options             []string
		persistent          bool
		expectError         bool
		errorMsgContains    string
	}{
		{
			name: "success - not mounted, persistent, fstab entry added",
			setupMockSpecific: func(m *MockConnector, ttName string) {
				var execCallCount int
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					execCallCount++
					// t.Logf("[%s] EnsureMount Mock Exec Call %d: cmd=%q, sudo=%v", ttName, execCallCount, cmd, opts.Sudo)
					switch execCallCount {
					case 1: // IsMounted -> mountpoint -q
						if cmd == fmt.Sprintf("mountpoint -q %s", mountPoint) && !opts.Sudo {
							return nil, nil, &connector.CommandError{ExitCode: 1} // Not mounted
						}
					case 2: // Mkdirp -> mkdir -p (EnsureMount implementation uses sudo for this Mkdirp)
						if cmd == fmt.Sprintf("mkdir -p %s", mountPoint) && opts.Sudo {
							return nil, nil, nil
						}
					case 3: // Mkdirp -> chmod
						if cmd == fmt.Sprintf("chmod %s %s", "0755", mountPoint) && opts.Sudo {
							return nil, nil, nil
						}
					case 4: // Mount command
						expectedMountCmd := fmt.Sprintf("mount -o %s -t %s %s %s", strings.Join(baseOptions, ","), fsType, shellEscapeFileTest(device), shellEscapeFileTest(mountPoint))
						if cmd == expectedMountCmd && opts.Sudo {
							return nil, nil, nil
						}
					case 5: // Check Fstab: grep -qE
						expectedGrepCmd := fmt.Sprintf("grep -qE '^[[:space:]]*[^#]+[[:space:]]+%s[[:space:]]' /etc/fstab", shellEscapeFileTest(mountPoint))
						if cmd == expectedGrepCmd && !opts.Sudo {
							return nil, nil, &connector.CommandError{ExitCode: 1} // Not in fstab
						}
					case 6: // Add to Fstab: sh -c 'echo ...'
						expectedFstabEntry := fmt.Sprintf("%s %s %s %s 0 0", device, mountPoint, fsType, strings.Join(baseOptions, ","))
						expectedAppendCmd := fmt.Sprintf("sh -c 'echo %s >> /etc/fstab'", shellEscapeFileTest(expectedFstabEntry))
						if cmd == expectedAppendCmd && opts.Sudo {
							return nil, nil, nil
						}
					default:
						return nil, nil, fmt.Errorf("[%s] EnsureMount: unexpected command call %d: %q, sudo: %v", ttName, execCallCount, cmd, opts.Sudo)
					}
					return nil, nil, fmt.Errorf("[%s] EnsureMount: fallthrough in mock exec call %d for cmd %q, sudo: %v", ttName, execCallCount, cmd, opts.Sudo)
				}
			},
			device: device, mountPoint: mountPoint, fsType: fsType, options: baseOptions, persistent: true,
			expectError: false,
		},
		{
			name: "already mounted, persistent, fstab entry exists",
			setupMockSpecific: func(m *MockConnector, ttName string) {
				var execCallCount int
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					execCallCount++
					switch execCallCount {
					case 1: // IsMounted -> mountpoint -q
						if cmd == fmt.Sprintf("mountpoint -q %s", mountPoint) && !opts.Sudo {
							return nil, nil, nil // Is mounted
						}
					case 2: // Check Fstab: grep -qE
						expectedGrepCmd := fmt.Sprintf("grep -qE '^[[:space:]]*[^#]+[[:space:]]+%s[[:space:]]' /etc/fstab", shellEscapeFileTest(mountPoint))
						if cmd == expectedGrepCmd && !opts.Sudo {
							return nil, nil, nil // Entry in fstab
						}
					default:
						return nil, nil, fmt.Errorf("[%s] EnsureMount: unexpected command call %d: %q, sudo: %v", ttName, execCallCount, cmd, opts.Sudo)
					}
					return nil, nil, fmt.Errorf("[%s] EnsureMount: fallthrough in mock exec call %d for cmd %q, sudo: %v", ttName, execCallCount, cmd, opts.Sudo)
				}
			},
			device: device, mountPoint: mountPoint, fsType: fsType, options: baseOptions, persistent: true,
			expectError: false,
		},
		{
			name: "input validation - empty device",
			setupMockSpecific: func(m *MockConnector, ttName string) {},
			device: "", mountPoint: mountPoint, fsType: fsType, options: baseOptions, persistent: false,
			expectError: true, errorMsgContains: "device, mountPoint, and fsType must be specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForFileTests(t)
			tt.setupMockSpecific(mockConn, tt.name)

			err := r.EnsureMount(ctx, mockConn, tt.device, tt.mountPoint, tt.fsType, tt.options, tt.persistent)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected an error, got nil")
				} else if !strings.Contains(err.Error(), tt.errorMsgContains) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errorMsgContains)
				}
			} else if err != nil {
				t.Errorf("Did not expect an error, got %v", err)
			}
		})
	}
}
