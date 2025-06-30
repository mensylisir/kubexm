package runner

import (
	"context"
	// "errors" // Removed as not directly used by test logic; mock errors are from fmt or connector
	"fmt"
	"strings"
	"testing"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// Helper to quickly get a runner with a mock connector for user tests
func newTestRunnerForUser(t *testing.T) (Runner, *MockConnector) {
	mockConn := NewMockConnector()
	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return &connector.OS{ID: "linux-test", Arch: "amd64", Kernel: "test-kernel"}, nil
	}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if isExecCmdForFactsInUserTest(cmd) { return []byte("dummy"), nil, nil }
		return []byte(""), nil, nil
	}
	r := NewRunner()
	return r, mockConn
}


func TestRunner_UserExists_True(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	username := "testuser"

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if strings.Contains(cmd, fmt.Sprintf("id -u %s", username)) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("UserExists True: unexpected cmd %s", cmd)
	}

	exists, err := r.UserExists(context.Background(), mockConn, username)
	if err != nil {
		t.Fatalf("UserExists() error = %v", err)
	}
	if !exists {
		t.Error("UserExists() = false, want true")
	}
}

func TestRunner_UserExists_False(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	username := "nosuchuser"

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if strings.Contains(cmd, fmt.Sprintf("id -u %s", username)) {
			return nil, []byte("id: 'nosuchuser': no such user"), &connector.CommandError{ExitCode: 1}
		}
		return nil, nil, fmt.Errorf("UserExists False: unexpected cmd %s", cmd)
	}

	exists, err := r.UserExists(context.Background(), mockConn, username)
	if err != nil {
		t.Fatalf("UserExists() error = %v (expected nil from UserExists itself)", err)
	}
	if exists {
		t.Error("UserExists() = true, want false")
	}
}

func TestRunner_GroupExists_True(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	groupname := "testgroup"

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if strings.Contains(cmd, fmt.Sprintf("getent group %s", groupname)) {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("GroupExists True: unexpected cmd %s", cmd)
	}
	exists, err := r.GroupExists(context.Background(), mockConn, groupname)
	if err != nil {
		t.Fatalf("GroupExists() error = %v", err)
	}
	if !exists {
		t.Error("GroupExists() = false, want true")
	}
}

func TestRunner_AddUser_Success(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	username := "newuser"
	group := "users"
	shell := "/bin/bash"
	homeDir := "/home/newuser"

	var userExistsCmdCalled, userAddCmdCalled bool

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options

		expectedUserExistsCmd := fmt.Sprintf("id -u %s", username)
		if cmd == expectedUserExistsCmd {
			userExistsCmdCalled = true
			return nil, nil, &connector.CommandError{ExitCode: 1} // Simulate user does not exist
		}

		// Check for useradd command (be a bit flexible with flag order)
		if strings.HasPrefix(cmd, "useradd") && strings.Contains(cmd, username) && options.Sudo {
			userAddCmdCalled = true
			if !strings.Contains(cmd, username) { t.Errorf("AddUser cmd missing username: %s", cmd)}
			if !strings.Contains(cmd, "-g "+group) { t.Errorf("AddUser cmd missing group: %s", cmd)}
			if !strings.Contains(cmd, "-s "+shell) { t.Errorf("AddUser cmd missing shell: %s", cmd)}
			if !strings.Contains(cmd, "-m") { t.Errorf("AddUser cmd missing -m (createHome): %s", cmd)}
			if !strings.Contains(cmd, "-d "+homeDir) {t.Errorf("AddUser cmd missing -d homeDir: %s", cmd)}
			return nil, nil, nil // Simulate useradd success
		}

		if isExecCmdForFactsInUserTest(cmd) { return []byte("dummy"), nil, nil }
		return nil, nil, fmt.Errorf("TestRunner_AddUser_Success: unexpected cmd %s", cmd)
	}

	err := r.AddUser(context.Background(), mockConn, username, group, shell, homeDir, true, false)
	if err != nil {
		t.Fatalf("AddUser() error = %v", err)
	}
	if !userExistsCmdCalled {
		t.Error("UserExists command (id -u) was not called by AddUser")
	}
	if !userAddCmdCalled {
		t.Error("useradd command was not called by AddUser")
	}
}

func TestRunner_AddGroup_Success(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	groupname := "newgroup"

	var groupExistsCmdCalled, groupAddCmdCalled bool
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options

		expectedGroupExistsCmd := fmt.Sprintf("getent group %s", groupname)
		if cmd == expectedGroupExistsCmd {
			groupExistsCmdCalled = true
			return nil, nil, &connector.CommandError{ExitCode: 1} // Simulate group does not exist
		}

		if strings.HasPrefix(cmd, "groupadd") && strings.Contains(cmd, groupname) && options.Sudo {
			groupAddCmdCalled = true
			if !strings.Contains(cmd, groupname) {t.Errorf("AddGroup missing groupname: %s", cmd)}
			if strings.Contains(cmd, "-r") {t.Errorf("AddGroup unexpectedly has -r for non-system group: %s", cmd)}
			return nil, nil, nil // Simulate groupadd success
		}
		if isExecCmdForFactsInUserTest(cmd) { return []byte("dummy"), nil, nil }
		return nil, nil, fmt.Errorf("TestRunner_AddGroup_Success: unexpected cmd %s", cmd)
	}
	err := r.AddGroup(context.Background(), mockConn, groupname, false)
	if err != nil {
		t.Fatalf("AddGroup() error = %v", err)
	}
	if !groupExistsCmdCalled {
		t.Error("GroupExists command (getent group) was not called by AddGroup")
	}
	if !groupAddCmdCalled {
		t.Error("groupadd command was not called by AddGroup")
	}
}

func TestRunner_AddUser_SystemUserNoHome(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	username := "sysuser"

	var userExistsCmdCalled, userAddCmdCalled bool
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options

		expectedUserExistsCmd := fmt.Sprintf("id -u %s", username)
		if cmd == expectedUserExistsCmd {
			userExistsCmdCalled = true
			return nil, nil, &connector.CommandError{ExitCode: 1} // Simulate user does not exist
		}

		if strings.HasPrefix(cmd, "useradd") && strings.Contains(cmd, username) && options.Sudo {
			userAddCmdCalled = true
			if !strings.Contains(cmd, "-r") { t.Errorf("AddUser (system) cmd missing -r: %s", cmd)}
			if !strings.Contains(cmd, "-M") { t.Errorf("AddUser (system, no home) cmd missing -M: %s", cmd)}
			if strings.Contains(cmd, " -m ") {t.Errorf("AddUser (system, no home) cmd unexpectedly has -m: %s", cmd)} // Note: " -m " to avoid matching "-d /home/..."
			return nil, nil, nil // Simulate useradd success
		}
		if isExecCmdForFactsInUserTest(cmd) { return []byte("dummy"), nil, nil }

		return nil, nil, fmt.Errorf("TestRunner_AddUser_SystemUserNoHome: unexpected cmd %s", cmd)
	}
	err := r.AddUser(context.Background(), mockConn, username, "", "", "", false, true)
	if err != nil {
		t.Fatalf("AddUser() for system user error = %v", err)
	}
	if !userExistsCmdCalled {
		t.Error("UserExists command (id -u) was not called by AddUser (system user)")
	}
	if !userAddCmdCalled {
		t.Error("useradd command was not called by AddUser (system user)")
	}
}

// isExecCmdForFactsInUserTest helper (can be shared or local if variations needed)
func isExecCmdForFactsInUserTest(cmd string) bool {
	return strings.Contains(cmd, "hostname") ||
		strings.Contains(cmd, "uname -r") ||
		strings.Contains(cmd, "nproc") ||
		strings.Contains(cmd, "grep MemTotal") ||
		strings.Contains(cmd, "ip -4 route") ||
		strings.Contains(cmd, "ip -6 route") ||
		strings.Contains(cmd, "command -v") ||
		strings.Contains(cmd, "test -e /etc/init.d")
}

func TestRunner_ConfigureSudoer_NotImplemented(t *testing.T) {
	r, mockConn := newTestRunnerForUser(t)
	err := r.ConfigureSudoer(context.Background(), mockConn, "testsudoer", "test ALL=(ALL) NOPASSWD: ALL")
	if err == nil || !strings.Contains(err.Error(), "not implemented") {
		t.Errorf("ConfigureSudoer expected 'not implemented' error, got %v", err)
	}
}
