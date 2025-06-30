package runner

import (
	"context"
	"errors" // Now used for errors.New
	"fmt"
	"strings"
	"testing"
	// "time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// shellEscapeUserTest is used for constructing expected command strings in tests.
func shellEscapeUserTest(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func newTestRunnerForUser(t *testing.T) (Runner, *MockConnector) {
	mockConn := NewMockConnector()
	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return &connector.OS{ID: "linux-test", Arch: "amd64", Kernel: "test-kernel"}, nil
	}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isExecCmdForFactsInUserTest(cmd) {
			return []byte("dummy fact output for " + cmd), nil, nil
		}
		return nil, nil, fmt.Errorf("newTestRunnerForUser: unhandled default exec: %s", cmd)
	}
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		commonTools := []string{"id", "getent", "useradd", "groupadd", "visudo", "mv", "chmod", "chown", "rm", "mkdir", "tee"}
		for _, tool := range commonTools {
			if file == tool {
				return "/usr/bin/" + file, nil
			}
		}
		if isExecCmdForFactsInUserTest(file) {
			return "/usr/bin/" + file, nil
		}
		return "", fmt.Errorf("newTestRunnerForUser: LookPath mock, tool %s not found", file)
	}
	r := NewRunner()
	return r, mockConn
}

func TestRunner_UserExists_True(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	username := "testuser"
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == fmt.Sprintf("id -u %s", username) && !options.Sudo {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("UserExists True: unexpected cmd %s", cmd)
	}
	exists, err := r.UserExists(context.Background(), mockConn, username)
	if err != nil { t.Fatalf("UserExists() error = %v", err) }
	if !exists { t.Error("UserExists() = false, want true") }
}

func TestRunner_UserExists_False(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	username := "nosuchuser"
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == fmt.Sprintf("id -u %s", username) && !options.Sudo {
			return nil, []byte("id: 'nosuchuser': no such user"), &connector.CommandError{ExitCode: 1}
		}
		return nil, nil, fmt.Errorf("UserExists False: unexpected cmd %s", cmd)
	}
	exists, err := r.UserExists(context.Background(), mockConn, username)
	if err != nil { t.Fatalf("UserExists() error = %v", err) } // UserExists itself should return nil error for check failures
	if exists { t.Error("UserExists() = true, want false") }
}

func TestRunner_GroupExists_True(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	groupname := "testgroup"
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == fmt.Sprintf("getent group %s", groupname) && !options.Sudo {
			return []byte(fmt.Sprintf("%s:x:1001:", groupname)), nil, nil
		}
		return nil, nil, fmt.Errorf("GroupExists True: unexpected cmd %s", cmd)
	}
	exists, err := r.GroupExists(context.Background(), mockConn, groupname)
	if err != nil { t.Fatalf("GroupExists() error = %v", err) }
	if !exists { t.Error("GroupExists() = false, want true") }
}

func TestRunner_GroupExists_False(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	groupname := "nosuchgroup"
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == fmt.Sprintf("getent group %s", groupname) && !options.Sudo {
			return nil, nil, &connector.CommandError{ExitCode: 2} // getent usually exits 2 if not found
		}
		return nil, nil, fmt.Errorf("GroupExists False: unexpected cmd %s", cmd)
	}
	exists, err := r.GroupExists(context.Background(), mockConn, groupname)
	if err != nil {
		t.Fatalf("GroupExists() error = %v", err)
	}
	if exists {t.Error("GroupExists() = true, want false")}
}

func TestRunner_AddUser_Success(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	username, group, shell, homeDir := "newuser", "users", "/bin/bash", "/home/newuser"
	var userExistsCmdCalled, userAddCmdCalled bool
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == fmt.Sprintf("id -u %s", username) {
			userExistsCmdCalled = true
			return nil, nil, &connector.CommandError{ExitCode: 1}
		}
		if strings.HasPrefix(cmd, "useradd") && strings.Contains(cmd, username) && options.Sudo {
			userAddCmdCalled = true; return nil, nil, nil
		}
		if isExecCmdForFactsInUserTest(cmd) { return []byte("dummy"), nil, nil }
		return nil, nil, fmt.Errorf("TestRunner_AddUser_Success: unexpected cmd %s", cmd)
	}
	err := r.AddUser(context.Background(), mockConn, username, group, shell, homeDir, true, false)
	if err != nil { t.Fatalf("AddUser() error = %v", err) }
	if !userExistsCmdCalled { t.Error("UserExists (id -u) was not called") }
	if !userAddCmdCalled { t.Error("useradd was not called") }
}

func TestRunner_AddGroup_Success(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	groupname := "newgroup"
	var groupExistsCmdCalled, groupAddCmdCalled bool
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == fmt.Sprintf("getent group %s", groupname) {
			groupExistsCmdCalled = true
			return nil, nil, &connector.CommandError{ExitCode: 1}
		}
		if strings.HasPrefix(cmd, "groupadd") && strings.Contains(cmd, groupname) && options.Sudo {
			groupAddCmdCalled = true; return nil, nil, nil
		}
		if isExecCmdForFactsInUserTest(cmd) { return []byte("dummy"), nil, nil }
		return nil, nil, fmt.Errorf("TestRunner_AddGroup_Success: unexpected cmd %s", cmd)
	}
	err := r.AddGroup(context.Background(), mockConn, groupname, false)
	if err != nil { t.Fatalf("AddGroup() error = %v", err) }
	if !groupExistsCmdCalled { t.Error("GroupExists (getent group) was not called") }
	if !groupAddCmdCalled { t.Error("groupadd was not called") }
}

func TestRunner_AddUser_AlreadyExists(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	username := "existinguser"
	var userExistsCmdCalled, userAddCmdCalled bool
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == fmt.Sprintf("id -u %s", username) { // UserExists check
			userExistsCmdCalled = true
			return nil, nil, nil // Simulate user exists (exit 0)
		}
		if strings.HasPrefix(cmd, "useradd") { // Should not be called
			userAddCmdCalled = true
		}
		if isExecCmdForFactsInUserTest(cmd) { return []byte("dummy"), nil, nil }
		return nil, nil, fmt.Errorf("TestRunner_AddUser_AlreadyExists: unexpected cmd %s", cmd)
	}
	err := r.AddUser(context.Background(), mockConn, username, "group", "shell", "home", false, false)
	if err != nil { t.Fatalf("AddUser() when user exists, error = %v, want nil", err) }
	if !userExistsCmdCalled {t.Error("UserExists check (id -u) was not called")}
	if userAddCmdCalled { t.Error("useradd was called unexpectedly when user already exists") }
}

func TestRunner_AddGroup_AlreadyExists(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	groupname := "existinggroup"
	var groupExistsCmdCalled, groupAddCmdCalled bool
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == fmt.Sprintf("getent group %s", groupname) { // GroupExists check
			groupExistsCmdCalled = true; return nil, nil, nil // Simulate group exists (exit 0)
		}
		if strings.HasPrefix(cmd, "groupadd") { groupAddCmdCalled = true } // Should not be called
		if isExecCmdForFactsInUserTest(cmd) { return []byte("dummy"), nil, nil }
		return nil, nil, fmt.Errorf("TestRunner_AddGroup_AlreadyExists: unexpected cmd %s", cmd)
	}
	err := r.AddGroup(context.Background(), mockConn, groupname, false)
	if err != nil { t.Fatalf("AddGroup() when group exists, error = %v, want nil", err) }
	if !groupExistsCmdCalled {t.Error("GroupExists check (getent group) was not called")}
	if groupAddCmdCalled { t.Error("groupadd was called unexpectedly when group already exists") }
}

func TestRunner_AddUser_Fails(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	username := "failuser"
	expectedErr := errors.New("useradd command failed")
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == fmt.Sprintf("id -u %s", username) { // UserExists check
			return nil, nil, &connector.CommandError{ExitCode: 1} // User does not exist
		}
		if strings.HasPrefix(cmd, "useradd") && strings.Contains(cmd, username) && options.Sudo {
			return nil, []byte("useradd error output"), expectedErr
		}
		if isExecCmdForFactsInUserTest(cmd) { return []byte("dummy"), nil, nil }
		return nil, nil, fmt.Errorf("TestRunner_AddUser_Fails: unexpected cmd %s", cmd)
	}
	err := r.AddUser(context.Background(), mockConn, username, "group", "shell", "home", false, false)
	if err == nil {
		t.Fatal("AddUser() expected an error when useradd command fails, got nil")
	}
	if !strings.Contains(err.Error(), "failed to add user") || !errors.Is(err, expectedErr) {
		t.Errorf("AddUser() error = %v, want error containing 'failed to add user' and wrapping %v", err, expectedErr)
	}
}

// Helper for creating string pointers for UserModifications
func stringPtr(s string) *string { return &s }

func TestRunner_ModifyUser(t *testing.T) {
	ctx := context.Background()
	username := "testuser"

	tests := []struct {
		name              string
		username          string
		modifications     UserModifications
		mockUserExists    bool // true if user exists, false otherwise
		mockUserExistsErr error
		mockUsermodErr    error
		expectedCmdParts  []string // Expected flag-value pairs or standalone flags
		expectError       bool
		errorMsgContains  string
	}{
		{
			name:     "modify_shell",
			username: username,
			modifications: UserModifications{
				NewShell: stringPtr("/bin/zsh"),
			},
			mockUserExists:   true,
			expectedCmdParts: []string{"-s", "/bin/zsh"},
		},
		{
			name:     "modify_primary_group",
			username: username,
			modifications: UserModifications{
				NewPrimaryGroup: stringPtr("newgroup"),
			},
			mockUserExists:   true,
			expectedCmdParts: []string{"-g", "newgroup"},
		},
		{
			name:     "append_to_secondary_groups",
			username: username,
			modifications: UserModifications{
				AppendToGroups: []string{"dev", "ops"},
			},
			mockUserExists:   true,
			expectedCmdParts: []string{"-aG", "dev,ops"},
		},
		{
			name:     "set_secondary_groups",
			username: username,
			modifications: UserModifications{
				SetSecondaryGroups: []string{"users", "staff"},
			},
			mockUserExists:   true,
			expectedCmdParts: []string{"-G", "users,staff"},
		},
		{
			name:     "change_homedir_and_move_contents",
			username: username,
			modifications: UserModifications{
				NewHomeDir:          stringPtr("/home/newtestuser"),
				MoveHomeDirContents: true,
			},
			mockUserExists:   true,
			expectedCmdParts: []string{"-d", "/home/newtestuser", "-m"},
		},
		{
			name:     "change_comment",
			username: username,
			modifications: UserModifications{
				NewComment: stringPtr("Test User Full Name"),
			},
			mockUserExists:   true,
			expectedCmdParts: []string{"-c", "'Test User Full Name'"}, // Value is shellEscaped
		},
		{
			name:     "change_login_name",
			username: username,
			modifications: UserModifications{
				NewUsername: stringPtr("newtestuserlogin"),
			},
			mockUserExists:   true,
			expectedCmdParts: []string{"-l", "newtestuserlogin"},
		},
		{
			name:     "multiple_modifications",
			username: username,
			modifications: UserModifications{
				NewShell:       stringPtr("/bin/fish"),
				NewPrimaryGroup: stringPtr("fishers"),
				AppendToGroups:  []string{"cool"},
			},
			mockUserExists: true,
			// Order of parts can vary, so test logic needs to be robust to this
			expectedCmdParts: []string{"-s", "/bin/fish", "-g", "fishers", "-aG", "cool"},
		},
		{
			name:             "user_not_found",
			username:         "nosuchuser",
			modifications:    UserModifications{NewShell: stringPtr("/bin/false")},
			mockUserExists:   false,
			expectError:      true,
			errorMsgContains: "user nosuchuser does not exist",
		},
		{
			name:     "usermod_command_fails",
			username: username,
			modifications: UserModifications{
				NewShell: stringPtr("/bin/bash"),
			},
			mockUserExists:   true,
			mockUsermodErr:   errors.New("usermod failed"),
			expectedCmdParts: []string{"-s", "/bin/bash"},
			expectError:      true,
			errorMsgContains: "failed to modify user",
		},
		{
			name:           "no_modifications_requested",
			username:       username,
			modifications:  UserModifications{},
			mockUserExists: true,
		},
		{
			name:             "empty_new_username",
			username:         username,
			modifications:    UserModifications{NewUsername: stringPtr(" ")},
			mockUserExists:   true,
			expectError:      true,
			errorMsgContains: "new username cannot be empty",
		},
		{
			name:     "move_home_without_new_home",
			username: username,
			modifications: UserModifications{
				MoveHomeDirContents: true,
			},
			mockUserExists:   true,
			expectError:      true,
			errorMsgContains: "MoveHomeDirContents can only be true if NewHomeDir is also specified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForUser(t)
			var usermodCmdExecuted string

			mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
				if cmd == fmt.Sprintf("id -u %s", tt.username) {
					if tt.mockUserExistsErr != nil {
						return nil, nil, tt.mockUserExistsErr
					}
					if tt.mockUserExists {
						return nil, nil, nil
					}
					return nil, nil, &connector.CommandError{ExitCode: 1}
				}

				if strings.HasPrefix(cmd, "usermod ") {
					usermodCmdExecuted = cmd
					if tt.mockUsermodErr != nil {
						return nil, []byte("usermod error"), tt.mockUsermodErr
					}
					return nil, nil, nil
				}
				if isExecCmdForFactsInUserTest(cmd) { return []byte("dummy"), nil, nil }
				return nil, nil, fmt.Errorf("ModifyUser test: unexpected command %q", cmd)
			}

			err := r.ModifyUser(ctx, mockConn, tt.username, tt.modifications)

			if tt.expectError {
				if err == nil {
					t.Fatalf("Expected error containing %q, got nil", tt.errorMsgContains)
				}
				if !strings.Contains(err.Error(), tt.errorMsgContains) {
					t.Errorf("Error message %q does not contain %q", err.Error(), tt.errorMsgContains)
				}
			} else if err != nil {
				t.Fatalf("Did not expect error, got %v. usermod cmd: %s", err, usermodCmdExecuted)
			}

			if !tt.expectError && len(tt.expectedCmdParts) > 0 {
				if usermodCmdExecuted == "" {
					t.Errorf("usermod command was not executed when modifications were expected")
				} else {
					cmdFields := strings.Fields(usermodCmdExecuted)
					if cmdFields[0] != "usermod" {
						t.Errorf("Expected command to start with 'usermod', got %q", usermodCmdExecuted)
					}
					if cmdFields[len(cmdFields)-1] != tt.username {
						t.Errorf("Expected command to end with username %q, got %q (full command: %s)", tt.username, cmdFields[len(cmdFields)-1], usermodCmdExecuted)
					}

					actualCmdOptions := make(map[string]string)
					isStandalone := make(map[string]bool)

					cmdOptionsAndArgs := cmdFields[1 : len(cmdFields)-1] // parts between "usermod" and username

					idx := 0
					for idx < len(cmdOptionsAndArgs) {
						opt := cmdOptionsAndArgs[idx]
						if strings.HasPrefix(opt, "-") {
							if idx+1 < len(cmdOptionsAndArgs) && !strings.HasPrefix(cmdOptionsAndArgs[idx+1], "-") {
								// Option with a value
								val := cmdOptionsAndArgs[idx+1]
								// Reconstruct value if it was quoted and split by strings.Fields
								if strings.HasPrefix(val, "'") && !strings.HasSuffix(val, "'") {
									for valIdx := idx + 2; valIdx < len(cmdOptionsAndArgs); valIdx++ {
										val += " " + cmdOptionsAndArgs[valIdx]
										if strings.HasSuffix(cmdOptionsAndArgs[valIdx], "'") {
											idx = valIdx // Update outer loop index
											break
										}
										// If no closing quote found, this indicates an issue or very complex arg.
										// For this test, assume simple shellEscape quoting.
									}
								}
								actualCmdOptions[opt] = val
								idx += 2 // Consumed option and its value (or parts of it)
							} else {
								// Standalone option
								isStandalone[opt] = true
								idx++
							}
						} else {
							// Should not happen for well-formed usermod options
							t.Errorf("Unexpected non-option argument %q in command options part: %v", opt, cmdOptionsAndArgs)
							idx++
						}
					}

					// Check expected options
					for i := 0; i < len(tt.expectedCmdParts); {
						expOpt := tt.expectedCmdParts[i]
						if i+1 < len(tt.expectedCmdParts) && !strings.HasPrefix(tt.expectedCmdParts[i+1], "-") {
							// Expected option with value
							expVal := tt.expectedCmdParts[i+1]
							if actVal, ok := actualCmdOptions[expOpt]; ok {
								if actVal != expVal {
									t.Errorf("For option %s, expected value %q, got %q", expOpt, expVal, actVal)
								}
							} else {
								t.Errorf("Expected option %s with value %q not found", expOpt, expVal)
							}
							i += 2
						} else {
							// Expected standalone option
							if _, ok := isStandalone[expOpt]; !ok {
								t.Errorf("Expected standalone option %s not found", expOpt)
							}
							i++
						}
					}
				}
			} else if !tt.expectError && len(tt.expectedCmdParts) == 0 && usermodCmdExecuted != "" {
				t.Errorf("usermod command %q was executed unexpectedly (no modifications requested)", usermodCmdExecuted)
			}
		})
	}
}

func TestRunner_AddGroup_Fails(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	groupname := "failgroup"
	expectedErr := errors.New("groupadd command failed")
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == fmt.Sprintf("getent group %s", groupname) { // GroupExists check
			return nil, nil, &connector.CommandError{ExitCode: 1} // Group does not exist
		}
		if strings.HasPrefix(cmd, "groupadd") && strings.Contains(cmd, groupname) && options.Sudo {
			return nil, []byte("groupadd error output"), expectedErr
		}
		if isExecCmdForFactsInUserTest(cmd) { return []byte("dummy"), nil, nil }
		return nil, nil, fmt.Errorf("TestRunner_AddGroup_Fails: unexpected cmd %s", cmd)
	}
	err := r.AddGroup(context.Background(), mockConn, groupname, false)
	if err == nil { t.Fatal("AddGroup() expected an error when groupadd command fails, got nil") }
	if !strings.Contains(err.Error(), "failed to add group") || !errors.Is(err, expectedErr) {
		t.Errorf("AddGroup() error = %v, want error containing 'failed to add group' and wrapping %v", err, expectedErr)
	}
}

func TestRunner_AddUser_SystemUserNoHome(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	username := "sysuser"
	var userExistsCmdCalled, userAddCmdCalled bool
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == fmt.Sprintf("id -u %s", username) {
			userExistsCmdCalled = true
			return nil, nil, &connector.CommandError{ExitCode: 1}
		}
		if strings.HasPrefix(cmd, "useradd") && strings.Contains(cmd, username) && options.Sudo {
			userAddCmdCalled = true
			if !strings.Contains(cmd, "-r") { t.Errorf("AddUser (system) missing -r: %s", cmd)}
			if !strings.Contains(cmd, "-M") { t.Errorf("AddUser (system, no home) missing -M: %s", cmd)}
			return nil, nil, nil
		}
		if isExecCmdForFactsInUserTest(cmd) { return []byte("dummy"), nil, nil }
		return nil, nil, fmt.Errorf("TestRunner_AddUser_SystemUserNoHome: unexpected cmd %s", cmd)
	}
	err := r.AddUser(context.Background(), mockConn, username, "", "", "", false, true)
	if err != nil { t.Fatalf("AddUser() for system user error = %v", err) }
	if !userExistsCmdCalled { t.Error("UserExists (id -u) was not called") }
	if !userAddCmdCalled { t.Error("useradd was not called") }
}

func isExecCmdForFactsInUserTest(cmd string) bool {
	factCmds := []string{"hostname", "uname -r", "nproc", "grep MemTotal", "ip -4 route", "ip -6 route", "command -v", "test -e /etc/init.d"}
	for _, fc := range factCmds {
		if strings.Contains(cmd, fc) { return true }
	}
	return false
}

func TestRunner_ConfigureSudoer(t *testing.T) {
	ctx := context.Background()
	sudoerName := "test_sudoer_config"
	content := "testuser ALL=(ALL) NOPASSWD: /usr/bin/uptime"
	finalPath := fmt.Sprintf("/etc/sudoers.d/%s", sudoerName)

	defaultLookPathMock := func(m *MockConnector) {
		m.LookPathFunc = func(c context.Context, file string) (string, error) {
			commonTools := []string{"visudo", "mv", "chmod", "chown", "rm", "mkdir", "tee"}
			for _, tool := range commonTools {
				if file == tool { return "/usr/bin/" + file, nil }
			}
			if isExecCmdForFactsInUserTest(file) { return "/usr/bin/" + file, nil }
			return "", fmt.Errorf("ConfigureSudoer LookPath: unexpected tool %s", file)
		}
	}

	tests := []struct {
		name             string
		sudoerName       string
		content          string
		setupMock        func(m *MockConnector, ttName string)
		expectError      bool
		errorMsgContains string
	}{
		{
			name:       "success",
			sudoerName: sudoerName,
			content:    content,
			setupMock: func(m *MockConnector, ttName string) {
				defaultLookPathMock(m)
				var tempPathWritten string
				var visudoCalled, mkdirSudoersDCalled, mvCalled, chmodFinalCalled, chownFinalCalled bool

				m.WriteFileFunc = func(c context.Context, writeContent []byte, destPath string, opts *connector.FileTransferOptions) error {
					if strings.HasPrefix(destPath, "/tmp/kubexm_sudoer_") && string(writeContent) == content && opts.Permissions == "0600" && !opts.Sudo {
						tempPathWritten = destPath
						return nil
					}
					return fmt.Errorf("[%s] unexpected WriteFile: path=%s, sudo=%v, content=%s", ttName, destPath, opts.Sudo, string(writeContent))
				}

				execCallOrder := 0
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					execCallOrder++
					// t.Logf("[%s] ConfigureSudoer Success Mock Exec: call %d, cmd %q, sudo %v", ttName, execCallOrder, cmd, opts.Sudo)

					switch execCallOrder {
					case 1: // visudo
						if strings.HasPrefix(cmd, "visudo -cf ") && strings.Contains(cmd, tempPathWritten) && opts.Sudo {
							visudoCalled = true; return nil, nil, nil
						}
					case 2: // mkdir for /etc/sudoers.d
						if cmd == fmt.Sprintf("mkdir -p %s", "/etc/sudoers.d") && opts.Sudo {
							mkdirSudoersDCalled = true; return nil, nil, nil
						}
					case 3: // chmod for /etc/sudoers.d (from Mkdirp)
						if cmd == fmt.Sprintf("chmod %s %s", "0755", "/etc/sudoers.d") && opts.Sudo {
							return nil, nil, nil
						}
					case 4: // mv
						if cmd == fmt.Sprintf("mv %s %s", shellEscapeUserTest(tempPathWritten), shellEscapeUserTest(finalPath)) && opts.Sudo {
							mvCalled = true; return nil, nil, nil
						}
					case 5: // chmod final
						expectedCmd := fmt.Sprintf("chmod 0440 %s", finalPath) // Path is NOT shell-escaped by r.Chmod
						if cmd == expectedCmd && opts.Sudo {
							chmodFinalCalled = true; return nil, nil, nil
						}
					case 6: // chown final
						// Actual Chown implementation in file.go does:
						// cmd := fmt.Sprintf("chown %s %s %s", recursiveFlag, ownerGroupSpec, path)
						// For this case (non-recursive), recursiveFlag is empty. After space normalization, command is "chown owner:group path"
						expectedCmd := fmt.Sprintf("chown root:root %s", finalPath)
						if cmd == expectedCmd && opts.Sudo {
							chownFinalCalled = true; return nil, nil, nil
						}
					}
					return nil, nil, fmt.Errorf("[%s] unexpected Exec call %d: %q, sudo %v", ttName, execCallOrder, cmd, opts.Sudo)
				}
				m.RemoveFunc = func(c context.Context, path string, opts connector.RemoveOptions) error {
					if path == tempPathWritten && !opts.Sudo { return nil }
					return fmt.Errorf("[%s] unexpected Remove: %s", ttName, path)
				}
				m.StatFunc = func(c context.Context, path string) (*connector.FileStat, error) {
					if path == tempPathWritten { return &connector.FileStat{Name: path, IsExist: false}, nil }
					return &connector.FileStat{Name: path, IsExist: true}, nil
				}

				t.Cleanup(func(){
					if tempPathWritten == "" {t.Errorf("[%s] Temp file was not written", ttName)}
					if !visudoCalled {t.Errorf("[%s] visudo was not called", ttName)}
					if !mkdirSudoersDCalled {t.Errorf("[%s] mkdir for /etc/sudoers.d was not called", ttName)}
					if !mvCalled {t.Errorf("[%s] mv was not called", ttName)}
					if !chmodFinalCalled {t.Errorf("[%s] chmod 0440 was not called", ttName)}
					if !chownFinalCalled {t.Errorf("[%s] chown root:root was not called", ttName)}
				})
			},
			expectError: false,
		},
		{
			name:       "visudo validation fails",
			sudoerName: sudoerName,
			content:    "bad content",
			setupMock: func(m *MockConnector, ttName string) {
				defaultLookPathMock(m)
				var tempPathWritten string
				m.WriteFileFunc = func(c context.Context, writeContent []byte, destPath string, opts *connector.FileTransferOptions) error {
					if strings.HasPrefix(destPath, "/tmp/kubexm_sudoer_") {
						tempPathWritten = destPath; return nil
					}
					return fmt.Errorf("[%s] visudo fails: unexpected WriteFile to %s", ttName, destPath)
				}
				m.ExecFunc = func(c context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
					if strings.HasPrefix(cmd, "visudo -cf ") && strings.Contains(cmd, tempPathWritten) && opts.Sudo {
						return nil, []byte("syntax error near line 1"), &connector.CommandError{ExitCode: 1}
					}
					return nil, nil, fmt.Errorf("[%s] visudo fails: unexpected Exec: %s", ttName, cmd)
				}
				m.RemoveFunc = func(c context.Context, path string, opts connector.RemoveOptions) error {
					if path == tempPathWritten && !opts.Sudo { return nil }
					return fmt.Errorf("[%s] visudo fails: unexpected Remove: %s", ttName, path)
				}
				m.StatFunc = func(c context.Context, path string) (*connector.FileStat, error) {
					if path == tempPathWritten { return &connector.FileStat{Name:path, IsExist: true}, nil}
					return &connector.FileStat{Name:path, IsExist: false}, nil
				}
			},
			expectError:      true,
			errorMsgContains: "sudoer content validation failed",
		},
		{
			name: "invalid sudoerName", sudoerName: "test/../evil", content: content,
			setupMock: func(m *MockConnector, ttName string) {},
			expectError: true, errorMsgContains: "invalid characters in sudoerName",
		},
		{
			name: "empty content", sudoerName: sudoerName, content: " ",
			setupMock: func(m *MockConnector, ttName string) {},
			expectError: true, errorMsgContains: "content for sudoer file cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, mockConn := newTestRunnerForUser(t)
			tt.setupMock(mockConn, tt.name)
			err := r.ConfigureSudoer(ctx, mockConn, tt.sudoerName, tt.content)
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
