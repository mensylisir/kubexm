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
					// Use shellEscapeFileTest to match the actual command being generated
					if cmd == fmt.Sprintf("mountpoint -q %s", shellEscapeFileTest(path)) && !opts.Sudo {
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
					if cmd == fmt.Sprintf("mountpoint -q %s", shellEscapeFileTest(path)) && !opts.Sudo {
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


func TestRunner_Unmount(t *testing.T) {
	ctx := context.Background()
	mountPath := "/mnt/mydata"

	tests := []struct {
		name             string
		mountPoint       string
		force            bool
		sudo             bool
		setupMock        func(m *MockConnector, tt *testing.T, currentTestName string)
		expectError      bool
		errorMsgContains string
	}{
		{
			name:       "success_unmount_not_forced_sudo",
			mountPoint: mountPath,
			force:      false,
			sudo:       true,
			setupMock: func(m *MockConnector, tt *testing.T, currentTestName string) {
				// IsMounted check
				m.LookPathFunc = func(c context.Context, file string) (string, error) {
					if file == "mountpoint" { return "/usr/bin/mountpoint", nil }
					return "", fmt.Errorf("[%s] IsMounted LookPath: unexpected %s", currentTestName, file)
				}
				isMountedCalled := false
				umountCalled := false
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if strings.HasPrefix(cmd, "mountpoint -q") {
						isMountedCalled = true
						return nil, nil, nil // Simulate is mounted
					}
					if strings.HasPrefix(cmd, "umount") && strings.Contains(cmd, shellEscape(mountPath)) && !strings.Contains(cmd, "-f") && opts.Sudo {
						umountCalled = true
						return nil, nil, nil // umount success
					}
					return nil, nil, fmt.Errorf("[%s] Unmount: unexpected cmd %q, sudo %v", currentTestName, cmd, opts.Sudo)
				}
				// Add a cleanup check to ensure functions were called
				t.Cleanup(func() {
					if !isMountedCalled { tt.Errorf("[%s] Expected IsMounted to be called", currentTestName) }
					if !umountCalled { tt.Errorf("[%s] Expected umount command to be called", currentTestName) }
				})
			},
		},
		{
			name:       "success_unmount_forced_no_sudo",
			mountPoint: mountPath,
			force:      true,
			sudo:       false,
			setupMock: func(m *MockConnector, tt *testing.T, currentTestName string) {
				m.LookPathFunc = func(c context.Context, file string) (string, error) {
					if file == "mountpoint" { return "/usr/bin/mountpoint", nil }
					return "", fmt.Errorf("[%s] IsMounted LookPath: unexpected %s", currentTestName, file)
				}
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if strings.HasPrefix(cmd, "mountpoint -q") { return nil, nil, nil } // Is mounted
					if strings.HasPrefix(cmd, "umount -f") && strings.Contains(cmd, shellEscape(mountPath)) && !opts.Sudo {
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("[%s] Unmount: unexpected cmd %q, sudo %v", currentTestName, cmd, opts.Sudo)
				}
			},
		},
		{
			name:       "not_mounted_idempotency",
			mountPoint: mountPath,
			force:      false,
			sudo:       false,
			setupMock: func(m *MockConnector, tt *testing.T, currentTestName string) {
				m.LookPathFunc = func(c context.Context, file string) (string, error) {
					if file == "mountpoint" { return "/usr/bin/mountpoint", nil }
					return "", fmt.Errorf("[%s] IsMounted LookPath: unexpected %s", currentTestName, file)
				}
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if strings.HasPrefix(cmd, "mountpoint -q") {
						return nil, nil, &connector.CommandError{ExitCode: 1} // Not mounted
					}
					// umount should not be called if not mounted
					return nil, nil, fmt.Errorf("[%s] Unmount: umount should not be called if not mounted, cmd %q", currentTestName, cmd)
				}
			},
		},
		{
			name:             "empty_mountPoint",
			mountPoint:       " ",
			expectError:      true,
			errorMsgContains: "mountPoint cannot be empty",
		},
		{
			name:       "isMounted_check_fails",
			mountPoint: mountPath,
			setupMock: func(m *MockConnector, tt *testing.T, currentTestName string) {
				m.LookPathFunc = func(c context.Context, file string) (string, error) {
					if file == "mountpoint" { return "/usr/bin/mountpoint", nil }
					return "", fmt.Errorf("[%s] IsMounted LookPath: unexpected %s", currentTestName, file)
				}
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if strings.HasPrefix(cmd, "mountpoint -q") {
						return nil, nil, errors.New("failed to run mountpoint") // IsMounted check fails
					}
					return nil, nil, fmt.Errorf("[%s] Unmount: unexpected cmd %q", currentTestName, cmd)
				}
			},
			expectError:      true,
			errorMsgContains: "failed to check if /mnt/mydata is mounted",
		},
		{
			name:       "umount_fails_not_mounted_error_idempotency",
			mountPoint: mountPath,
			sudo:       true,
			setupMock: func(m *MockConnector, tt *testing.T, currentTestName string) {
				m.LookPathFunc = func(c context.Context, file string) (string, error) {
					if file == "mountpoint" { return "/usr/bin/mountpoint", nil }
					return "", fmt.Errorf("[%s] IsMounted LookPath: unexpected %s", currentTestName, file)
				}
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if strings.HasPrefix(cmd, "mountpoint -q") { return nil, nil, nil } // Is mounted
					if strings.HasPrefix(cmd, "umount") && opts.Sudo {
						return nil, []byte("umount: /mnt/mydata: not mounted."), &connector.CommandError{ExitCode: 1, Stderr: "umount: /mnt/mydata: not mounted."}
					}
					return nil, nil, fmt.Errorf("[%s] Unmount: unexpected cmd %q", currentTestName, cmd)
				}
			},
			expectError: false, // Should be idempotent
		},
		{
			name:       "umount_fails_other_error",
			mountPoint: mountPath,
			sudo:       true,
			setupMock: func(m *MockConnector, tt *testing.T, currentTestName string) {
				m.LookPathFunc = func(c context.Context, file string) (string, error) {
					if file == "mountpoint" { return "/usr/bin/mountpoint", nil }
					return "", fmt.Errorf("[%s] IsMounted LookPath: unexpected %s", currentTestName, file)
				}
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if strings.HasPrefix(cmd, "mountpoint -q") { return nil, nil, nil } // Is mounted
					if strings.HasPrefix(cmd, "umount") && opts.Sudo {
						return nil, []byte("device is busy"), &connector.CommandError{ExitCode: 32, Stderr: "device is busy"}
					}
					return nil, nil, fmt.Errorf("[%s] Unmount: unexpected cmd %q", currentTestName, cmd)
				}
			},
			expectError:      true,
			errorMsgContains: "failed to unmount /mnt/mydata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForFileTests(t)
			if tt.setupMock != nil {
				tt.setupMock(mockConn, t, tt.name)
			}

			err := r.Unmount(ctx, mockConn, tt.mountPoint, tt.force, tt.sudo)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error containing %q, got nil", tt.errorMsgContains)
				} else if !strings.Contains(err.Error(), tt.errorMsgContains) {
					t.Fatalf("Error message %q does not contain %q", err.Error(), tt.errorMsgContains)
				}
			} else if err != nil {
				t.Fatalf("Did not expect error, got %v", err)
			}
		})
	}
}

func TestRunner_IsMounted_Fallback_ProcMounts(t *testing.T) {
	ctx := context.Background()
	mountPath := "/mnt/special"
	otherPath := "/data/regular"

	tests := []struct {
		name            string
		path            string
		mockProcMounts  string // Content of /proc/mounts
		mockReadFileErr error
		expectedMounted bool
		expectError     bool
		errorContains   string
	}{
		{
			name:            "is_mounted_found_in_/proc/mounts",
			path:            mountPath,
			mockProcMounts:  "sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0\nproc /proc proc rw,nosuid,nodev,noexec,relatime 0 0\n/dev/sda1 /mnt/special ext4 rw,relatime 0 0\n",
			expectedMounted: true,
		},
		{
			name:            "not_mounted_not_in_/proc/mounts",
			path:            otherPath,
			mockProcMounts:  "sysfs /sys sysfs rw,nosuid,nodev,noexec,relatime 0 0\nproc /proc proc rw,nosuid,nodev,noexec,relatime 0 0\n/dev/sda1 /mnt/special ext4 rw,relatime 0 0\n",
			expectedMounted: false,
		},
		{
			name:            "read_/proc/mounts_fails",
			path:            mountPath,
			mockReadFileErr: errors.New("failed to read"),
			expectError:     true,
			errorContains:   "failed to read /proc/mounts",
		},
		{
			name:            "empty_/proc/mounts_file",
			path:            mountPath,
			mockProcMounts:  "\n", // Empty or just newline
			expectedMounted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForFileTests(t)

			// Setup: mountpoint command not found, ReadFile for /proc/mounts is controlled
			mockConn.LookPathFunc = func(c context.Context, file string) (string, error) {
				if file == "mountpoint" {
					return "", errors.New("mountpoint not found")
				}
				// Allow other lookups if needed by ReadFile's cat fallback (though ReadFile mock below avoids this)
				return "/usr/bin/"+file, nil
			}

			// Mock ReadFile specifically for /proc/mounts
			// This assumes ReadFile is used by IsMounted; if IsMounted uses Exec("cat /proc/mounts"), then ExecFunc needs mocking.
			// The current IsMounted implementation calls r.ReadFile.
			mockConn.ReadFileFunc = func(c context.Context, path string) ([]byte, error) {
				if path == "/proc/mounts" {
					return []byte(tt.mockProcMounts), tt.mockReadFileErr
				}
				return nil, fmt.Errorf("IsMounted fallback test: unexpected ReadFile call for %s", path)
			}
			// If ReadFile has a cat fallback that uses Exec, that would also need mocking if ReadFileFunc is not set on mockConn.
			// However, newTestRunnerForFileTests sets up a general ExecFunc, so r.ReadFile will use that if ReadFileFunc is not set.
			// For clarity, setting ReadFileFunc is better.

			isMounted, err := r.IsMounted(ctx, mockConn, tt.path)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error containing %q, got nil", tt.errorContains)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Fatalf("Error message %q does not contain %q", err.Error(), tt.errorContains)
				}
			} else if err != nil {
				t.Fatalf("Did not expect error, got %v", err)
			}

			if isMounted != tt.expectedMounted {
				t.Errorf("Expected mounted %v, got %v", tt.expectedMounted, isMounted)
			}
		})
	}
}


func TestRunner_TouchFile(t *testing.T) {
	ctx := context.Background()
	filePath := "/tmp/testfile.touch"

	tests := []struct {
		name          string
		path          string
		sudo          bool
		setupMock     func(m *MockConnector, ttName string, expectSudo bool)
		expectError   bool
		errorContains string
	}{
		{
			name: "success_no_sudo",
			path: filePath,
			sudo: false,
			setupMock: func(m *MockConnector, ttName string, expectSudo bool) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if strings.HasPrefix(cmd, "mkdir -p") && strings.Contains(cmd, filepath.Dir(filePath)) && opts.Sudo == expectSudo {
						return nil, nil, nil
					}
					if strings.HasPrefix(cmd, "chmod") && strings.Contains(cmd, filepath.Dir(filePath)) && opts.Sudo == expectSudo {
						return nil, nil, nil
					}
					if cmd == fmt.Sprintf("touch %s", shellEscape(filePath)) && opts.Sudo == expectSudo {
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("[%s] unexpected exec: %s, sudo: %v", ttName, cmd, opts.Sudo)
				}
			},
		},
		{
			name: "success_with_sudo",
			path: "/root/testfile.touch", // Path that might need sudo for parent dir
			sudo: true,
			setupMock: func(m *MockConnector, ttName string, expectSudo bool) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if strings.HasPrefix(cmd, "mkdir -p") && opts.Sudo == expectSudo {
						return nil, nil, nil
					}
					if strings.HasPrefix(cmd, "chmod") && opts.Sudo == expectSudo {
						return nil, nil, nil
					}
					if strings.HasPrefix(cmd, "touch") && opts.Sudo == expectSudo {
						return nil, nil, nil
					}
					return nil, nil, fmt.Errorf("[%s] unexpected exec: %s, sudo: %v", ttName, cmd, opts.Sudo)
				}
			},
		},
		{
			name:          "empty_path",
			path:          " ",
			expectError:   true,
			errorContains: "path cannot be empty",
		},
		{
			name: "mkdir_parent_fails",
			path: "/opt/newdir/file.touch",
			sudo: false,
			setupMock: func(m *MockConnector, ttName string, expectSudo bool) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if strings.HasPrefix(cmd, "mkdir -p") {
						return nil, nil, errors.New("mkdir failed")
					}
					return nil, nil, fmt.Errorf("[%s] unexpected exec: %s", ttName, cmd)
				}
			},
			expectError:   true,
			errorContains: "failed to create parent directory",
		},
		{
			name: "touch_command_fails",
			path: filePath,
			sudo: false,
			setupMock: func(m *MockConnector, ttName string, expectSudo bool) {
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if strings.HasPrefix(cmd, "mkdir -p") { return nil, nil, nil }
					if strings.HasPrefix(cmd, "chmod") { return nil, nil, nil }
					if strings.HasPrefix(cmd, "touch") {
						return nil, nil, errors.New("touch command failed")
					}
					return nil, nil, fmt.Errorf("[%s] unexpected exec: %s", ttName, cmd)
				}
			},
			expectError:   true,
			errorContains: "failed to touch file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForFileTests(t)
			if tt.setupMock != nil {
				tt.setupMock(mockConn, tt.name, tt.sudo)
			}

			err := r.TouchFile(ctx, mockConn, tt.path, tt.sudo)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorContains)
				} else if !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errorContains)
				}
			} else if err != nil {
				t.Errorf("Did not expect error, got %v", err)
			}
		})
	}
}

func TestRunner_GetDiskUsage(t *testing.T) {
	ctx := context.Background()
	testPath := "/mnt/data"

	tests := []struct {
		name             string
		path             string
		mockDfOutput     string
		mockDfError      error
		expectedTotal    uint64
		expectedFree     uint64
		expectedUsed     uint64
		expectError      bool
		errorMsgContains string
	}{
		{
			name:         "success",
			path:         testPath,
			mockDfOutput: "Filesystem     1M-blocks  Used Available Use% Mounted on\n/dev/sda1         19976M 7930M    11018M  42% /mnt/data\n",
			expectedTotal: 19976,
			expectedUsed:  7930,
			expectedFree:  11018,
		},
		{
			name:             "df_command_fails",
			path:             testPath,
			mockDfError:      errors.New("df execution failed"),
			expectError:      true,
			errorMsgContains: "failed to execute df command",
		},
		{
			name:             "df_output_too_few_lines",
			path:             testPath,
			mockDfOutput:     "Filesystem     1M-blocks  Used Available Use% Mounted on\n",
			expectError:      true,
			errorMsgContains: "not enough lines",
		},
		{
			name:             "df_output_too_few_fields",
			path:             testPath,
			mockDfOutput:     "Filesystem     1M-blocks  Used Available Use% Mounted on\n/dev/sda1 19976M 7930M\n",
			expectError:      true,
			errorMsgContains: "not enough fields",
		},
		{
			name:             "df_output_total_not_parseable",
			path:             testPath,
			mockDfOutput:     "Filesystem     1M-blocks  Used Available Use% Mounted on\n/dev/sda1         X19976M 7930M    11018M  42% /\n",
			expectError:      true,
			errorMsgContains: "failed to parse total disk space",
		},
		{
			name:             "df_output_used_no_M_suffix",
			path:             testPath,
			mockDfOutput:     "Filesystem     1M-blocks  Used Available Use% Mounted on\n/dev/sda1         19976M 7930    11018M  42% /\n",
			expectError:      true,
			errorMsgContains: "failed to parse used disk space",
		},
		{
			name:             "df_output_free_invalid_number",
			path:             testPath,
			mockDfOutput:     "Filesystem     1M-blocks  Used Available Use% Mounted on\n/dev/sda1         19976M 7930M    XYZM  42% /\n",
			expectError:      true,
			errorMsgContains: "failed to parse free disk space",
		},
		{
			name:             "empty_path",
			path:             "",
			expectError:      true,
			errorMsgContains: "path cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForFileTests(t)
			mockConn.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
				if strings.HasPrefix(cmd, "df -BM -P") && strings.Contains(cmd, shellEscape(tt.path)) {
					return []byte(tt.mockDfOutput), nil, tt.mockDfError
				}
				return nil, nil, fmt.Errorf("GetDiskUsage test: unexpected command %q", cmd)
			}

			total, free, used, err := r.GetDiskUsage(ctx, mockConn, tt.path)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorMsgContains)
				} else if !strings.Contains(err.Error(), tt.errorMsgContains) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errorMsgContains)
				}
			} else if err != nil {
				t.Errorf("Did not expect error, got %v", err)
			}

			if !tt.expectError {
				if total != tt.expectedTotal {
					t.Errorf("Expected total %d, got %d", tt.expectedTotal, total)
				}
				if free != tt.expectedFree {
					t.Errorf("Expected free %d, got %d", tt.expectedFree, free)
				}
				if used != tt.expectedUsed {
					t.Errorf("Expected used %d, got %d", tt.expectedUsed, used)
				}
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
