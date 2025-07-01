package runner

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/connector/mocks" // Import for mocks.Connector
	"github.com/stretchr/testify/mock"               // Import for mock.Anything etc.
)

// shellEscapeFileTest is used for constructing expected command strings in tests.
func shellEscapeFileTest(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func newTestRunnerForFileTests(t *testing.T) (Runner, *mocks.Connector) {
	mockConn := mocks.NewConnector(t) // Use generated testify mock
	r := NewRunner()

	// Setup minimal mocks for GatherFacts to avoid warnings/unexpected errors in file tests
	// if GatherFacts is called (e.g. if a file operation needs OS context from Facts)
	setupMockGatherFacts_Minimal(mockConn, "linux-filetest")

	// Specific LookPath behavior for file tests, if needed beyond what setupMockGatherFacts_Minimal provides
	// Note: setupMockGatherFacts_Minimal already provides some LookPath mocks.
	// Specific LookPath behavior for file tests should be set within each test case
	// if it's not covered adequately by setupMockGatherFacts_Minimal or if a specific outcome (e.g. error) is needed.
	// Removing generic LookPath mocks for commonFileTools from here to make tests more explicit.

	// Removing generic Exec mock from here. Each test should define its expected Exec calls.
	// This makes it easier to debug when an unexpected command is called, as it will fail
	// the mock expectation directly rather than hitting a generic catch-all.

	return r, mockConn
}

func TestRunner_IsMounted(t *testing.T) {
	ctx := context.Background()
	mountPath := "/mnt/mydata"

	tests := []struct {
		name            string
		setupMock       func(m *mocks.Connector, path string, ttName string) // Changed to *mocks.Connector
		path            string
		expectedMounted bool
		expectError     bool
		errorContains   string
	}{
		{
			name: "is mounted - mountpoint cmd success",
			setupMock: func(m *mocks.Connector, path string, ttName string) {
				m.On("LookPath", mock.Anything, "mountpoint").Return("/usr/bin/mountpoint", nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("mountpoint -q %s", shellEscapeFileTest(path)), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return !opts.Sudo })).Return(nil, nil, nil).Once()
			},
			path:            mountPath,
			expectedMounted: true,
			expectError:     false,
		},
		{
			name: "is not mounted - mountpoint cmd exit 1",
			setupMock: func(m *mocks.Connector, path string, ttName string) {
				m.On("LookPath", mock.Anything, "mountpoint").Return("/usr/bin/mountpoint", nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("mountpoint -q %s", shellEscapeFileTest(path)), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return !opts.Sudo })).Return(nil, nil, &connector.CommandError{ExitCode: 1}).Once()
			},
			path:            mountPath,
			expectedMounted: false,
			expectError:     false,
		},
		{
			name: "mountpoint command not found",
			setupMock: func(m *mocks.Connector, path string, ttName string) {
				// This LookPath is for the IsMounted logic itself.
				// setupMockGatherFacts_Minimal might provide a .Maybe() for "mountpoint",
				// but this specific .On() will take precedence for this test case.
				m.On("LookPath", mock.Anything, "mountpoint").Return("", errors.New("mountpoint not found by mock")).Once()
				// We also need to mock ReadFile for the fallback path in IsMounted
				m.On("ReadFile", mock.Anything, "/proc/mounts").Return(nil, errors.New("failed to read /proc/mounts for fallback")).Maybe()
			},
			path:            mountPath,
			expectedMounted: false,
			expectError:     true,
			errorContains:   "IsMounted: 'mountpoint' command not found",
		},
		{
			name:            "empty path",
			setupMock:       func(m *mocks.Connector, path string, ttName string) {}, // No specific mocks needed as it should fail validation early
			path:            "  ",
			expectedMounted: false,
			expectError:     true,
			errorContains:   "path cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForFileTests(t) // mockConn is now *mocks.Connector
			if tt.setupMock != nil {
				tt.setupMock(mockConn, tt.path, tt.name)
			}
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
		setupMock        func(m *mocks.Connector, t *testing.T, currentTestName string) // Changed to *mocks.Connector
		expectError      bool
		errorMsgContains string
	}{
		{
			name:       "success_unmount_not_forced_sudo",
			mountPoint: mountPath,
			force:      false,
			sudo:       true,
			setupMock: func(m *mocks.Connector, t *testing.T, currentTestName string) {
				// IsMounted check: LookPath for mountpoint
				m.On("LookPath", mock.Anything, "mountpoint").Return("/usr/bin/mountpoint", nil).Once()
				// IsMounted check: Exec mountpoint -q
				m.On("Exec", mock.Anything, fmt.Sprintf("mountpoint -q %s", shellEscapeFileTest(mountPath)),
					mock.MatchedBy(func(opts *connector.ExecOptions) bool { return !opts.Sudo })).Return(nil, nil, nil).Once() // Simulate is mounted
				// Unmount command
				m.On("Exec", mock.Anything, fmt.Sprintf("umount %s", shellEscapeFileTest(mountPath)),
					mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo })).Return(nil, nil, nil).Once() // umount success
			},
		},
		{
			name:       "success_unmount_forced_no_sudo",
			mountPoint: mountPath,
			force:      true,
			sudo:       false,
			setupMock: func(m *mocks.Connector, t *testing.T, currentTestName string) {
				m.On("LookPath", mock.Anything, "mountpoint").Return("/usr/bin/mountpoint", nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("mountpoint -q %s", shellEscapeFileTest(mountPath)), mock.Anything).Return(nil, nil, nil).Once() // Is mounted
				m.On("Exec", mock.Anything, fmt.Sprintf("umount -f %s", shellEscapeFileTest(mountPath)),
					mock.MatchedBy(func(opts *connector.ExecOptions) bool { return !opts.Sudo })).Return(nil, nil, nil).Once()
			},
		},
		{
			name:       "not_mounted_idempotency",
			mountPoint: mountPath,
			force:      false,
			sudo:       false,
			setupMock: func(m *mocks.Connector, t *testing.T, currentTestName string) {
				m.On("LookPath", mock.Anything, "mountpoint").Return("/usr/bin/mountpoint", nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("mountpoint -q %s", shellEscapeFileTest(mountPath)), mock.Anything).Return(nil, nil, &connector.CommandError{ExitCode: 1}).Once() // Not mounted
				// umount should not be called
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
			setupMock: func(m *mocks.Connector, t *testing.T, currentTestName string) {
				m.On("LookPath", mock.Anything, "mountpoint").Return("/usr/bin/mountpoint", nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("mountpoint -q %s", shellEscapeFileTest(mountPath)), mock.Anything).Return(nil, nil, errors.New("failed to run mountpoint")).Once() // IsMounted check fails
			},
			expectError:      true,
			errorMsgContains: "failed to check if /mnt/mydata is mounted",
		},
		{
			name:       "umount_fails_not_mounted_error_idempotency",
			mountPoint: mountPath,
			sudo:       true,
			setupMock: func(m *mocks.Connector, t *testing.T, currentTestName string) {
				m.On("LookPath", mock.Anything, "mountpoint").Return("/usr/bin/mountpoint", nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("mountpoint -q %s", shellEscapeFileTest(mountPath)), mock.Anything).Return(nil, nil, nil).Once() // Is mounted
				m.On("Exec", mock.Anything, fmt.Sprintf("umount %s", shellEscapeFileTest(mountPath)), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo })).Return(nil, []byte("umount: /mnt/mydata: not mounted."), &connector.CommandError{ExitCode: 1, Stderr: "umount: /mnt/mydata: not mounted."}).Once()
			},
			expectError: false, // Should be idempotent
		},
		{
			name:       "umount_fails_other_error",
			mountPoint: mountPath,
			sudo:       true,
			setupMock: func(m *mocks.Connector, t *testing.T, currentTestName string) {
				m.On("LookPath", mock.Anything, "mountpoint").Return("/usr/bin/mountpoint", nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("mountpoint -q %s", shellEscapeFileTest(mountPath)), mock.Anything).Return(nil, nil, nil).Once() // Is mounted
				m.On("Exec", mock.Anything, fmt.Sprintf("umount %s", shellEscapeFileTest(mountPath)), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo })).Return(nil, []byte("device is busy"), &connector.CommandError{ExitCode: 32, Stderr: "device is busy"}).Once()
			},
			expectError:      true,
			errorMsgContains: "failed to unmount /mnt/mydata",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForFileTests(t) // mockConn is now *mocks.Connector
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
			r, mockConn := newTestRunnerForFileTests(t) // mockConn is now *mocks.Connector

			// Setup: mountpoint command not found
			mockConn.On("LookPath", mock.Anything, "mountpoint").Return("", errors.New("mountpoint not found")).Once()

			// Mock ReadFile specifically for /proc/mounts for the fallback path
			if tt.mockReadFileErr != nil {
				mockConn.On("ReadFile", mock.Anything, "/proc/mounts").Return(nil, tt.mockReadFileErr).Once()
			} else {
				mockConn.On("ReadFile", mock.Anything, "/proc/mounts").Return([]byte(tt.mockProcMounts), nil).Once()
			}

			// If IsMounted had further LookPath calls for its fallback (it doesn't currently, beyond ReadFile), they'd be mocked here.

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
		setupMock     func(m *mocks.Connector, ttName string, tcPath string, tcSudo bool) // Path added for dynamic command matching
		expectError   bool
		errorContains string
	}{
		{
			name: "success_no_sudo",
			path: filePath,
			sudo: false,
			setupMock: func(m *mocks.Connector, ttName string, tcPath string, tcSudo bool) {
				dir := filepath.Dir(tcPath)
				m.On("Exec", mock.Anything, fmt.Sprintf("mkdir -p %s", dir), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("chmod %s %s", "0755", dir), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("touch %s", shellEscapeFileTest(tcPath)), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, nil).Once()
			},
		},
		{
			name: "success_with_sudo",
			path: "/root/testfile.touch",
			sudo: true,
			setupMock: func(m *mocks.Connector, ttName string, tcPath string, tcSudo bool) {
				dir := filepath.Dir(tcPath)
				m.On("Exec", mock.Anything, fmt.Sprintf("mkdir -p %s", dir), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("chmod %s %s", "0755", dir), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("touch %s", shellEscapeFileTest(tcPath)), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, nil).Once()
			},
		},
		{
			name:          "empty_path",
			path:          " ",
			// No setupMock needed, validation should catch it.
			expectError:   true,
			errorContains: "path cannot be empty",
		},
		{
			name: "mkdir_parent_fails",
			path: "/opt/newdir/file.touch",
			sudo: false,
			setupMock: func(m *mocks.Connector, ttName string, tcPath string, tcSudo bool) {
				dir := filepath.Dir(tcPath)
				m.On("Exec", mock.Anything, fmt.Sprintf("mkdir -p %s", dir), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, errors.New("mkdir failed")).Once()
				// Chmod and touch won't be called
			},
			expectError:   true,
			errorContains: "failed to create parent directory",
		},
		{
			name: "touch_command_fails",
			path: filePath,
			sudo: false,
			setupMock: func(m *mocks.Connector, ttName string, tcPath string, tcSudo bool) {
				dir := filepath.Dir(tcPath)
				m.On("Exec", mock.Anything, fmt.Sprintf("mkdir -p %s", dir), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("chmod %s %s", "0755", dir), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("touch %s", shellEscapeFileTest(tcPath)), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, errors.New("touch command failed")).Once()
			},
			expectError:   true,
			errorContains: "failed to touch file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForFileTests(t) // mockConn is now *mocks.Connector
			if tt.setupMock != nil {
				tt.setupMock(mockConn, tt.name, tt.path, tt.sudo)
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
			r, mockConn := newTestRunnerForFileTests(t) // mockConn is now *mocks.Connector

			if tt.path != "" { // Only setup mock if path is not empty (empty path is a validation test)
				expectedCmd := fmt.Sprintf("df -BM -P %s", shellEscapeFileTest(tt.path))
				mockConn.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool { return !opts.Sudo })).
					Return([]byte(tt.mockDfOutput), nil, tt.mockDfError).Once()
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
		setupMock     func(m *mocks.Connector, tcTarget, tcLink string, tcSudo bool, ttName string) // Added target and link
		target        string
		link          string
		sudo          bool
		expectError   bool
		errorContains string
	}{
		{
			name: "success no sudo",
			setupMock: func(m *mocks.Connector, tcTarget, tcLink string, tcSudo bool, ttName string) {
				linkDir := filepath.Dir(tcLink)
				m.On("Exec", mock.Anything, fmt.Sprintf("mkdir -p %s", linkDir), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("chmod %s %s", "0755", linkDir), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("ln -sf %s %s", shellEscapeFileTest(tcTarget), shellEscapeFileTest(tcLink)), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, nil).Once()
			},
			target: targetPath, link: linkPath, sudo: false, expectError: false,
		},
		{
			name: "success with sudo",
			setupMock: func(m *mocks.Connector, tcTarget, tcLink string, tcSudo bool, ttName string) {
				linkDir := filepath.Dir(tcLink)
				m.On("Exec", mock.Anything, fmt.Sprintf("mkdir -p %s", linkDir), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("chmod %s %s", "0755", linkDir), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("ln -sf %s %s", shellEscapeFileTest(tcTarget), shellEscapeFileTest(tcLink)), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, nil).Once()
			},
			target: targetPath, link: linkPath, sudo: true, expectError: false,
		},
		{
			name: "ln command fails",
			setupMock: func(m *mocks.Connector, tcTarget, tcLink string, tcSudo bool, ttName string) {
				linkDir := filepath.Dir(tcLink)
				m.On("Exec", mock.Anything, fmt.Sprintf("mkdir -p %s", linkDir), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("chmod %s %s", "0755", linkDir), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, nil, nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("ln -sf %s %s", shellEscapeFileTest(tcTarget), shellEscapeFileTest(tcLink)), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, []byte("ln error"), errors.New("ln failed")).Once()
			},
			target: targetPath, link: linkPath, sudo: false, expectError: true, errorContains: "failed to create symlink",
		},
		{
			name: "mkdir for parent fails",
			setupMock: func(m *mocks.Connector, tcTarget, tcLink string, tcSudo bool, ttName string) {
				linkDir := filepath.Dir(tcLink)
				m.On("Exec", mock.Anything, fmt.Sprintf("mkdir -p %s", linkDir), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo == tcSudo })).Return(nil, []byte("mkdir error"), errors.New("mkdir failed")).Once()
			},
			target: targetPath, link: linkPath, sudo: false, expectError: true, errorContains: "failed to create parent directory",
		},
		{
			name: "empty target", setupMock: func(m *mocks.Connector, tcTarget, tcLink string, tcSudo bool, ttName string) {}, // No mocks for validation errors
			target: "", link: linkPath, sudo: false, expectError: true, errorContains: "target cannot be empty",
		},
		{
			name: "empty linkPath", setupMock: func(m *mocks.Connector, tcTarget, tcLink string, tcSudo bool, ttName string) {},
			target: targetPath, link: "", sudo: false, expectError: true, errorContains: "linkPath cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForFileTests(t) // mockConn is now *mocks.Connector
			if tt.setupMock != nil {
				tt.setupMock(mockConn, tt.target, tt.link, tt.sudo, tt.name)
			}
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
		setupMock     func(m *mocks.Connector, device string, fsT string, force bool, ttName string)
		device        string
		fsType        string
		force         bool
		expectError   bool
		errorContains string
	}{
		{
			name: "success no force",
			setupMock: func(m *mocks.Connector, device string, fsT string, force bool, ttName string) {
				expectedCmd := fmt.Sprintf("mkfs.%s %s", fsT, shellEscapeFileTest(device))
				m.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo })).Return(nil, nil, nil).Once()
			},
			device: devicePath, fsType: fsType, force: false, expectError: false,
		},
		{
			name: "success with force",
			setupMock: func(m *mocks.Connector, device string, fsT string, force bool, ttName string) {
				expectedCmd := fmt.Sprintf("mkfs.%s -f %s", fsT, shellEscapeFileTest(device))
				m.On("Exec", mock.Anything, expectedCmd, mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo })).Return(nil, nil, nil).Once()
			},
			device: devicePath, fsType: fsType, force: true, expectError: false,
		},
		{
			name: "mkfs command fails",
			setupMock: func(m *mocks.Connector, device string, fsT string, force bool, ttName string) {
				// Command will vary based on 'force' flag, so match prefix and device
				m.On("Exec", mock.Anything, mock.MatchedBy(func(cmd string) bool {
					return strings.HasPrefix(cmd, "mkfs."+fsT) && strings.Contains(cmd, shellEscapeFileTest(device))
				}), mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo })).Return(nil, []byte("mkfs error"), errors.New("mkfs failed")).Once()
			},
			device: devicePath, fsType: fsType, force: false, expectError: true, errorContains: "failed to make filesystem",
		},
		{
			name: "empty device", setupMock: func(m *mocks.Connector, device string, fsT string, force bool, ttName string) {}, // No mocks for validation
			device: "", fsType: fsType, expectError: true, errorContains: "device cannot be empty",
		},
		{
			name: "empty fsType", setupMock: func(m *mocks.Connector, device string, fsT string, force bool, ttName string) {},
			device: devicePath, fsType: "", expectError: true, errorContains: "fsType cannot be empty",
		},
		{
			name: "invalid fsType characters", setupMock: func(m *mocks.Connector, device string, fsT string, force bool, ttName string) {},
			device: devicePath, fsType: "ext4;", expectError: true, errorContains: "invalid characters in fsType",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForFileTests(t) // mockConn is now *mocks.Connector
			if tt.setupMock != nil {
				tt.setupMock(mockConn, tt.device, tt.fsType, tt.force, tt.name)
			}
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
		setupMockSpecific   func(m *mocks.Connector, ttName string) // Changed MockConnector to mocks.Connector
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
			setupMockSpecific: func(m *mocks.Connector, ttName string) { // Changed MockConnector to mocks.Connector
				// IsMounted: mountpoint -q fails (not mounted)
				m.On("LookPath", mock.Anything, "mountpoint").Return("/usr/bin/mountpoint", nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("mountpoint -q %s", shellEscapeFileTest(mountPoint)),
					mock.MatchedBy(func(opts *connector.ExecOptions) bool { return !opts.Sudo })).
					Return(nil, nil, &connector.CommandError{ExitCode: 1}).Once() // Not mounted

				// Mkdirp
				m.On("Exec", mock.Anything, fmt.Sprintf("mkdir -p %s", mountPoint),
					mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo })).
					Return(nil, nil, nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("chmod %s %s", "0755", mountPoint),
					mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo })).
					Return(nil, nil, nil).Once()

				// Mount
				expectedMountCmd := fmt.Sprintf("mount -o %s -t %s %s %s", strings.Join(baseOptions, ","), fsType, shellEscapeFileTest(device), shellEscapeFileTest(mountPoint))
				m.On("Exec", mock.Anything, expectedMountCmd,
					mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo })).
					Return(nil, nil, nil).Once()

				// Check Fstab: grep -qE fails (not in fstab)
				expectedGrepCmd := fmt.Sprintf("grep -qE '^[[:space:]]*[^#]+[[:space:]]+%s[[:space:]]' /etc/fstab", shellEscapeFileTest(mountPoint))
				m.On("Exec", mock.Anything, expectedGrepCmd,
					mock.MatchedBy(func(opts *connector.ExecOptions) bool { return !opts.Sudo })).
					Return(nil, nil, &connector.CommandError{ExitCode: 1}).Once() // Not in fstab

				// Add to Fstab
				expectedFstabEntry := fmt.Sprintf("%s %s %s %s 0 0", device, mountPoint, fsType, strings.Join(baseOptions, ","))
				expectedAppendCmd := fmt.Sprintf("sh -c 'echo %s >> /etc/fstab'", shellEscapeFileTest(expectedFstabEntry))
				m.On("Exec", mock.Anything, expectedAppendCmd,
					mock.MatchedBy(func(opts *connector.ExecOptions) bool { return opts.Sudo })).
					Return(nil, nil, nil).Once()
			},
			device: device, mountPoint: mountPoint, fsType: fsType, options: baseOptions, persistent: true,
			expectError: false,
		},
		{
			name: "already mounted, persistent, fstab entry exists",
			setupMockSpecific: func(m *mocks.Connector, ttName string) { // Changed MockConnector to mocks.Connector
				// IsMounted: mountpoint -q succeeds
				m.On("LookPath", mock.Anything, "mountpoint").Return("/usr/bin/mountpoint", nil).Once()
				m.On("Exec", mock.Anything, fmt.Sprintf("mountpoint -q %s", shellEscapeFileTest(mountPoint)),
					mock.MatchedBy(func(opts *connector.ExecOptions) bool { return !opts.Sudo })).
					Return(nil, nil, nil).Once() // Is mounted

				// Check Fstab: grep -qE succeeds
				expectedGrepCmd := fmt.Sprintf("grep -qE '^[[:space:]]*[^#]+[[:space:]]+%s[[:space:]]' /etc/fstab", shellEscapeFileTest(mountPoint))
				m.On("Exec", mock.Anything, expectedGrepCmd,
					mock.MatchedBy(func(opts *connector.ExecOptions) bool { return !opts.Sudo })).
					Return(nil, nil, nil).Once() // Entry in fstab
			},
			device: device, mountPoint: mountPoint, fsType: fsType, options: baseOptions, persistent: true,
			expectError: false,
		},
		{
			name: "input validation - empty device",
			setupMockSpecific: func(m *mocks.Connector, ttName string) {}, // No mocks needed for validation error
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
