package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kubexms/kubexms/pkg/connector"
)

// Helper to quickly get a runner with a mock connector for command tests
func newTestRunnerWithMock(t *testing.T) (*Runner, *MockConnector) {
	mockConn := NewMockConnector()
	// Provide a minimal successful NewRunner setup
	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return &connector.OS{ID: "linux-test", Arch: "amd64", Kernel: "test-kernel"}, nil
	}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		// Default exec for fact gathering in NewRunner
		if strings.Contains(cmd, "hostname") { return []byte("test-host"), nil, nil }
		if strings.Contains(cmd, "uname -r") { return []byte("test-kernel"), nil, nil }
		if strings.Contains(cmd, "nproc") { return []byte("1"), nil, nil }
		if strings.Contains(cmd, "grep MemTotal") { return []byte("1024"), nil, nil } // 1MB
		if strings.Contains(cmd, "ip -4 route") { return []byte("1.1.1.1"), nil, nil }
		if strings.Contains(cmd, "ip -6 route") { return nil, nil, fmt.Errorf("no ipv6") }
		// Fallback for actual test commands if not overridden by specific tests
		return []byte(""), nil, nil
	}

	r, err := NewRunner(context.Background(), mockConn)
	if err != nil {
		t.Fatalf("Failed to create runner for command tests: %v", err)
	}
	return r, mockConn
}


func TestRunner_Run_Success(t *testing.T) {
	r, mockConn := newTestRunnerWithMock(t)

	expectedStdout := "hello world"
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd // Ensure the mock connector's state is updated
		mockConn.LastExecOptions = options

		if cmd == "echo hello" && !options.Sudo {
			return []byte(expectedStdout), nil, nil
		}
		return nil, nil, errors.New("unexpected command in Run test")
	}

	out, err := r.Run(context.Background(), "echo hello", false)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out != expectedStdout {
		t.Errorf("Run() output = %q, want %q", out, expectedStdout)
	}
}

func TestRunner_Run_WithSudo(t *testing.T) {
	r, mockConn := newTestRunnerWithMock(t)

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if cmd == "whoami" && options.Sudo { // Sudo should be true
			return []byte("root"), nil, nil
		}
		t.Errorf("Exec called with cmd=%s, sudo=%v; expected whoami with sudo=true", cmd, options.Sudo)
		return nil, nil, errors.New("unexpected command or sudo option")
	}

	_, err := r.Run(context.Background(), "whoami", true)
	if err != nil {
		t.Fatalf("Run() with sudo error = %v", err)
	}
}


func TestRunner_Run_Error(t *testing.T) {
	r, mockConn := newTestRunnerWithMock(t)

	expectedErr := &connector.CommandError{Cmd: "failing_cmd", ExitCode: 1, Stderr: "it failed"}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		return nil, []byte("it failed"), expectedErr
	}

	_, err := r.Run(context.Background(), "failing_cmd", false)
	if err == nil {
		t.Fatal("Run() with failing command expected error, got nil")
	}
	// Check if the returned error is the expected CommandError or wraps it.
	// Using errors.As for robust check for CommandError type.
	var cmdErr *connector.CommandError
	if !errors.As(err, &cmdErr) {
		t.Errorf("Run() error type = %T, want %T or wrapped", err, expectedErr)
	} else {
		if cmdErr.ExitCode != 1 || cmdErr.Stderr != "it failed" {
			t.Errorf("Run() CommandError fields mismatch: got %+v, want %+v", cmdErr, expectedErr)
		}
	}
}

func TestRunner_MustRun_Success(t *testing.T) {
	r, mockConn := newTestRunnerWithMock(t)
	expectedStdout := "must_succeed"
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		return []byte(expectedStdout), nil, nil
	}

	defer func() {
		if rec := recover(); rec != nil {
			t.Errorf("MustRun() panicked unexpectedly: %v", rec)
		}
	}()
	out := r.MustRun(context.Background(), "anything", false)
	if out != expectedStdout {
		t.Errorf("MustRun() output = %q, want %q", out, expectedStdout)
	}
}

func TestRunner_MustRun_Panics(t *testing.T) {
	r, mockConn := newTestRunnerWithMock(t)
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		return nil, nil, errors.New("command failed for MustRun")
	}

	defer func() {
		if rec := recover(); rec == nil {
			t.Error("MustRun() did not panic on error")
		} else {
			// Optionally check the panic message
			if !strings.Contains(fmt.Sprintf("%v", rec), "command failed for MustRun") {
				t.Errorf("MustRun() panic message mismatch: got %v", rec)
			}
		}
	}()
	r.MustRun(context.Background(), "failing_cmd_mustrun", false)
}

func TestRunner_Check_True(t *testing.T) {
	r, mockConn := newTestRunnerWithMock(t)
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		return nil, nil, nil // No error means exit code 0
	}

	ok, err := r.Check(context.Background(), "true_command", false)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !ok {
		t.Error("Check() = false, want true for successful command")
	}
}

func TestRunner_Check_False_NonZeroExit(t *testing.T) {
	r, mockConn := newTestRunnerWithMock(t)
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		return nil, nil, &connector.CommandError{Cmd: cmd, ExitCode: 1} // Non-zero exit
	}

	ok, err := r.Check(context.Background(), "false_command", false)
	if err != nil {
		t.Fatalf("Check() error = %v for non-zero exit (expected nil error from Check itself)", err)
	}
	if ok {
		t.Error("Check() = true, want false for command with non-zero exit")
	}
}

func TestRunner_Check_Error_ExecFailed(t *testing.T) {
	r, mockConn := newTestRunnerWithMock(t)
	expectedErr := errors.New("connection failed")
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		return nil, nil, expectedErr // Actual execution error, not CommandError
	}

	_, err := r.Check(context.Background(), "any_command", false)
	if err == nil {
		t.Fatal("Check() with actual exec error expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("Check() error = %v, want %v", err, expectedErr)
	}
}


func TestRunner_RunWithOptions_Success(t *testing.T) {
	r, mockConn := newTestRunnerWithMock(t)

	expectedStdout := "output with options"
	expectedOpts := &connector.ExecOptions{Sudo: true, Timeout: 5 * time.Second}

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if cmd != "test_cmd_opts" {
			t.Errorf("Exec called with cmd=%s, want test_cmd_opts", cmd)
		}
		if options == nil || !options.Sudo || options.Timeout != expectedOpts.Timeout {
			t.Errorf("Exec called with options=%+v, want %+v", options, expectedOpts)
		}
		return []byte(expectedStdout), nil, nil
	}

	stdout, _, err := r.RunWithOptions(context.Background(), "test_cmd_opts", expectedOpts)
	if err != nil {
		t.Fatalf("RunWithOptions() error = %v", err)
	}
	if string(stdout) != expectedStdout {
		t.Errorf("RunWithOptions() stdout = %q, want %q", string(stdout), expectedStdout)
	}
}
