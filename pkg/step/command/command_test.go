package command

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockStepConnector is a mock implementation of connector.Connector for testing steps.
type MockStepConnector struct {
	ExecFunc func(ctx context.Context, cmd string, opts *connector.ExecOptions) (stdout, stderr []byte, err error)
	// Add other methods like Copy, Stat, etc., if CommandStep starts using them.
}

func (m *MockStepConnector) Connect(ctx context.Context, cfg connector.ConnectionCfg) error { return nil }
func (m *MockStepConnector) Close() error                                                 { return nil }
func (m *MockStepConnector) IsConnected() bool                                            { return true }
func (m *MockStepConnector) GetOS(ctx context.Context) (*connector.OS, error) {
	return &connector.OS{ID: "linux", Arch: "amd64"}, nil
}
func (m *MockStepConnector) Exec(ctx context.Context, cmd string, opts *connector.ExecOptions) ([]byte, []byte, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(ctx, cmd, opts)
	}
	return nil, nil, fmt.Errorf("ExecFunc not implemented in mock")
}
func (m *MockStepConnector) CopyContent(ctx context.Context, content []byte, destPath string, options *connector.FileTransferOptions) error {
	return nil
}
func (m *MockStepConnector) Stat(ctx context.Context, path string) (*connector.FileStat, error) {
	return &connector.FileStat{IsExist: false}, nil
}
func (m *MockStepConnector) LookPath(ctx context.Context, file string) (string, error) { return file, nil }
func (m *MockStepConnector) ReadFile(ctx context.Context, path string) ([]byte, error) { return nil, nil }
func (m *MockStepConnector) WriteFile(ctx context.Context, content []byte, destPath, permissions string, sudo bool) error {
	return nil
}


// newTestRuntimeContext creates a runtime.Context suitable for testing steps.
func newTestRuntimeContext(t *testing.T, conn connector.Connector) *runtime.Context {
	t.Helper()
	log := logger.Get() // Use a real logger for tests for now

	// Create a mock host
	mockHost := &step.MockHost{
		MockName:    "test-host",
		MockAddress: "127.0.0.1",
		MockRoles:   []string{"test"},
	}

	rtCtx := &runtime.Context{
		GoCtx:  context.Background(),
		Logger: log,
		Runner: runner.New(), // Using real runner, which relies on the provided connector
		HostRuntimes: map[string]*runtime.HostRuntime{
			mockHost.GetName(): {
				Host:  mockHost,
				Conn:  conn, // Use the provided connector
				Facts: &runner.Facts{OS: &connector.OS{ID: "linux", Arch: "amd64", PrettyName: "Test Linux"}},
			},
		},
		CurrentHost: mockHost, // Set CurrentHost for StepContext methods like GetHost()
	}
	return rtCtx.ForHost(mockHost) // Return a StepContext
}


func TestCommandStep_Run_Success(t *testing.T) {
	mockConn := &MockStepConnector{}
	rtCtx := newTestRuntimeContext(t, mockConn) // Pass the mock connector
	host := rtCtx.GetHost() // Get the host from the context

	cmdStep := NewCommandStep("TestEcho", "echo hello", false, false, 0, nil, 0, "", false, 0, "", false)

	expectedStdout := "hello"
	mockConn.ExecFunc = func(ctxEx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		require.Equal(t, "echo hello", cmd)
		require.False(t, options.Sudo)
		return []byte(expectedStdout), nil, nil
	}

	err := cmdStep.Run(rtCtx, host)
	assert.NoError(t, err)
	// Note: CommandStep itself doesn't return stdout/stderr from Run.
	// These would typically be captured by the engine or a higher-level result collector.
	// If we need to test stdout/stderr capture, we'd have to check the mock connector's calls or use a step result object.
}

func TestCommandStep_Run_Error_NonZeroExit(t *testing.T) {
	mockConn := &MockStepConnector{}
	rtCtx := newTestRuntimeContext(t, mockConn)
	host := rtCtx.GetHost()

	cmdStep := NewCommandStep("TestExit123", "exit 123", false, false, 0, nil, 0, "", false, 0, "", false)

	expectedCmdErr := &connector.CommandError{Cmd: "exit 123", ExitCode: 123, Stderr: "failed"}
	mockConn.ExecFunc = func(ctxC context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		require.Equal(t, "exit 123", cmd)
		return nil, []byte("failed"), expectedCmdErr
	}

	err := cmdStep.Run(rtCtx, host)
	require.Error(t, err)

	var cmdError *connector.CommandError
	require.True(t, errors.As(err, &cmdError), "error should be or wrap *connector.CommandError")
	assert.Equal(t, 123, cmdError.ExitCode)
	assert.Equal(t, "failed", cmdError.Stderr)
}

func TestCommandStep_Run_IgnoreError(t *testing.T) {
	mockConn := &MockStepConnector{}
	rtCtx := newTestRuntimeContext(t, mockConn)
	host := rtCtx.GetHost()

	cmdStep := NewCommandStep("TestIgnoreError", "exit 1", false, true, 0, nil, 0, "", false, 0, "", false)

	cmdErr := &connector.CommandError{Cmd: "exit 1", ExitCode: 1, Stderr: "ignored error output"}
	mockConn.ExecFunc = func(ctxC context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		return nil, []byte("ignored error output"), cmdErr
	}

	err := cmdStep.Run(rtCtx, host)
	assert.NoError(t, err, "Run should succeed when IgnoreError is true")
}

func TestCommandStep_Run_ExpectedExitCode_Match(t *testing.T) {
	mockConn := &MockStepConnector{}
	rtCtx := newTestRuntimeContext(t, mockConn)
	host := rtCtx.GetHost()

	// Expect exit code 5, command exits with 5
	cmdStep := NewCommandStep("TestExpectedExit", "cmd-exits-5", false, false, 0, nil, 5, "", false, 0, "", false)

	cmdErr := &connector.CommandError{Cmd: "cmd-exits-5", ExitCode: 5, Stderr: "exited 5 as expected"}
	mockConn.ExecFunc = func(ctxC context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		return nil, []byte("exited 5 as expected"), cmdErr
	}

	err := cmdStep.Run(rtCtx, host)
	assert.NoError(t, err, "Run should succeed when command exit code matches ExpectedExitCode")
}

func TestCommandStep_Run_ExpectedExitCode_Mismatch(t *testing.T) {
	mockConn := &MockStepConnector{}
	rtCtx := newTestRuntimeContext(t, mockConn)
	host := rtCtx.GetHost()

	// Expect exit code 5, command exits with 0
	cmdStep := NewCommandStep("TestExpectedExitMismatch", "cmd-exits-0", false, false, 0, nil, 5, "", false, 0, "", false)

	mockConn.ExecFunc = func(ctxC context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		return []byte("cmd output"), nil, nil // Exits 0
	}

	err := cmdStep.Run(rtCtx, host)
	require.Error(t, err, "Run should fail when command exit code (0) does not match ExpectedExitCode (5)")
	var cmdError *connector.CommandError
	require.True(t, errors.As(err, &cmdError), "error should be or wrap *connector.CommandError")
	assert.Equal(t, 0, cmdError.ExitCode, "Error should reflect actual exit code")
}


func TestCommandStep_Precheck_NoCheckCmd(t *testing.T) {
	mockConn := &MockStepConnector{}
	rtCtx := newTestRuntimeContext(t, mockConn)
	host := rtCtx.GetHost()

	cmdStep := NewCommandStep("TestMain", "echo main", false, false, 0, nil, 0, "", false, 0, "", false)

	isDone, err := cmdStep.Precheck(rtCtx, host)
	assert.NoError(t, err)
	assert.False(t, isDone, "Precheck with no CheckCmd should return false (run main command)")
}

func TestCommandStep_Precheck_CheckCmd_Succeeds_SkipsMain(t *testing.T) {
	mockConn := &MockStepConnector{}
	rtCtx := newTestRuntimeContext(t, mockConn)
	host := rtCtx.GetHost()

	cmdStep := NewCommandStep("TestMain", "echo main", false, false, 0, nil, 0,
		"check_if_done", false, 0, // CheckCmd, CheckSudo, CheckExpectedExitCode (0 for success)
		"", false)

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		require.Equal(t, "check_if_done", cmd)
		return nil, nil, nil // CheckCmd succeeds (exit 0)
	}

	isDone, err := cmdStep.Precheck(rtCtx, host)
	assert.NoError(t, err)
	assert.True(t, isDone, "Precheck with succeeding CheckCmd (exit 0, expected 0) should return true (skip main)")
}

func TestCommandStep_Precheck_CheckCmd_Fails_RunsMain(t *testing.T) {
	mockConn := &MockStepConnector{}
	rtCtx := newTestRuntimeContext(t, mockConn)
	host := rtCtx.GetHost()

	cmdStep := NewCommandStep("TestMain", "echo main", false, false, 0, nil, 0,
		"check_if_done_fails", false, 0, // CheckCmd, CheckSudo, CheckExpectedExitCode (0 for success)
		"", false)

	checkCmdErr := &connector.CommandError{Cmd: "check_if_done_fails", ExitCode: 1, Stderr: "check failed"}
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		require.Equal(t, "check_if_done_fails", cmd)
		return nil, []byte("check failed"), checkCmdErr // CheckCmd fails (exit 1)
	}

	isDone, err := cmdStep.Precheck(rtCtx, host)
	assert.NoError(t, err) // Precheck itself doesn't fail if CheckCmd fails, it just means main cmd should run
	assert.False(t, isDone, "Precheck with failing CheckCmd (exit 1, expected 0) should return false (run main)")
}

func TestCommandStep_Precheck_CheckCmd_ExpectedNonZeroExit_SkipsMain(t *testing.T) {
	mockConn := &MockStepConnector{}
	rtCtx := newTestRuntimeContext(t, mockConn)
	host := rtCtx.GetHost()

	cmdStep := NewCommandStep("TestMain", "echo main", false, false, 0, nil, 0,
		"check_returns_5", false, 5, // CheckCmd, CheckSudo, CheckExpectedExitCode (5 for "done")
		"", false)

	checkCmdErr := &connector.CommandError{Cmd: "check_returns_5", ExitCode: 5, Stderr: "check returned 5"}
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		require.Equal(t, "check_returns_5", cmd)
		return nil, []byte("check returned 5"), checkCmdErr // CheckCmd exits with 5
	}

	isDone, err := cmdStep.Precheck(rtCtx, host)
	assert.NoError(t, err)
	assert.True(t, isDone, "Precheck with CheckCmd (exit 5, expected 5) should return true (skip main)")
}

func TestCommandStep_Rollback_Success(t *testing.T) {
	mockConn := &MockStepConnector{}
	rtCtx := newTestRuntimeContext(t, mockConn)
	host := rtCtx.GetHost()

	cmdStep := NewCommandStep("TestMain", "echo main", false, false, 0, nil, 0,
		"", false, 0,
		"echo rollback", false) // RollbackCmd, RollbackSudo

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		require.Equal(t, "echo rollback", cmd)
		require.False(t, options.Sudo)
		return []byte("rollback done"), nil, nil
	}

	err := cmdStep.Rollback(rtCtx, host)
	assert.NoError(t, err)
}

func TestCommandStep_Rollback_NoRollbackCmd(t *testing.T) {
	mockConn := &MockStepConnector{}
	rtCtx := newTestRuntimeContext(t, mockConn)
	host := rtCtx.GetHost()

	cmdStep := NewCommandStep("TestMain", "echo main", false, false, 0, nil, 0,
		"", false, 0,
		"", false) // No RollbackCmd

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		t.Fatalf("ExecFunc should not be called for rollback if RollbackCmd is empty")
		return nil, nil, nil
	}

	err := cmdStep.Rollback(rtCtx, host)
	assert.NoError(t, err)
}

func TestCommandStep_Rollback_Error(t *testing.T) {
	mockConn := &MockStepConnector{}
	rtCtx := newTestRuntimeContext(t, mockConn)
	host := rtCtx.GetHost()

	cmdStep := NewCommandStep("TestMain", "echo main", false, false, 0, nil, 0,
		"", false, 0,
		"rollback_fails", false)

	rollbackErr := &connector.CommandError{Cmd: "rollback_fails", ExitCode: 1, Stderr: "rollback error"}
	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		require.Equal(t, "rollback_fails", cmd)
		return nil, []byte("rollback error"), rollbackErr
	}

	err := cmdStep.Rollback(rtCtx, host)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rollback_fails")
	assert.Contains(t, err.Error(), "rollback error")

	var cmdError *connector.CommandError
	require.True(t, errors.As(err, &cmdError) || errors.As(errors.Unwrap(err), &cmdError), "error from Rollback should be or wrap *connector.CommandError")
	if cmdError != nil { // Check if unwrapping was successful
		assert.Equal(t, 1, cmdError.ExitCode)
	}
}
