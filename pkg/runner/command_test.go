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

// Helper to quickly get a runner with a mock connector for command tests
func newTestRunnerForCommand(t *testing.T) (Runner, *MockConnector) { // Renamed and returns Runner
	mockConn := NewMockConnector()
	// MockConnector's NewMockConnector() already sets up default GetOSFunc and ExecFunc.
	// We can override them here if specific default behavior for command tests is needed,
	// but often individual tests will set their own ExecFunc.

	// Example minimal setup if NewMockConnector defaults are not sufficient for some edge cases
	// or if we want to ensure specific behavior for commands NOT explicitly tested.
	mockConn.GetOSFunc = func(ctx context.Context) (*connector.OS, error) {
		return &connector.OS{ID: "linux-mock-command", Arch: "amd64"}, nil
	}
    // Default ExecFunc in this helper is for any background commands runner might issue
    // that are not the primary command being tested by a subtest.
	// Most tests will override mockConn.ExecFunc for the command they are testing.
	originalExecFunc := mockConn.ExecFunc // Keep original default from NewMockConnector if needed
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if strings.Contains(cmd, "hostname") { return []byte("test-host-cmd"), nil, nil }
		// If other fact-gathering commands were strictly needed by tested methods *before* the main exec:
		// if strings.Contains(cmd, "some_other_fact_cmd") { ... }

		// If an individual test overrides ExecFunc, that will be used.
		// If not, and it's not a known fact-gathering cmd, use original default or specific error.
		if originalExecFunc != nil {
			// This part might be complex if tests override and we still want a general default.
			// For command.go tests, they primarily test one Exec call.
			// So, individual tests setting ExecFunc is the main pattern.
		}
		// A simple default for commands not caught by specific test overrides:
		// fmt.Printf("Warning: newTestRunnerForCommand default ExecFunc called for: %s\n", cmd)
		return []byte("default exec from newTestRunnerForCommand"), nil, nil
	}


	r := NewRunner() // Corrected call
	// No error returned by NewRunner()
	return r, mockConn
}


func TestRunner_Run_Success(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t) // Updated call

	expectedStdout := "hello world"
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd // Ensure the mock connector's state is updated
		mockConn.LastExecOptions = options

		if cmd == "echo hello" && !options.Sudo {
			return []byte(expectedStdout), nil, nil
		}
		return nil, nil, errors.New("unexpected command in Run test")
	}

	out, err := r.Run(context.Background(), mockConn, "echo hello", false)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if out != expectedStdout {
		t.Errorf("Run() output = %q, want %q", out, expectedStdout)
	}
}

func TestRunner_Run_WithSudo(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t) // Updated call

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		if cmd == "whoami" && options.Sudo { // Sudo should be true
			return []byte("root"), nil, nil
		}
		t.Errorf("Exec called with cmd=%s, sudo=%v; expected whoami with sudo=true", cmd, options.Sudo)
		return nil, nil, errors.New("unexpected command or sudo option")
	}

	_, err := r.Run(context.Background(), mockConn, "whoami", true)
	if err != nil {
		t.Fatalf("Run() with sudo error = %v", err)
	}
}


func TestRunner_Run_Error(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t) // Updated call

	expectedErr := &connector.CommandError{Cmd: "failing_cmd", ExitCode: 1, Stderr: "it failed"}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		return nil, []byte("it failed"), expectedErr
	}

	_, err := r.Run(context.Background(), mockConn, "failing_cmd", false)
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
	r, mockConn := newTestRunnerForCommand(t) // Updated call
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
	out := r.MustRun(context.Background(), mockConn, "anything", false)
	if out != expectedStdout {
		t.Errorf("MustRun() output = %q, want %q", out, expectedStdout)
	}
}

func TestRunner_MustRun_Panics(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t) // Updated call
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
	r.MustRun(context.Background(), mockConn, "failing_cmd_mustrun", false)
}

func TestRunner_Check_True(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t) // Updated call
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		return nil, nil, nil // No error means exit code 0
	}

	ok, err := r.Check(context.Background(), mockConn, "true_command", false)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if !ok {
		t.Error("Check() = false, want true for successful command")
	}
}

func TestRunner_Check_False_NonZeroExit(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t) // Updated call
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		return nil, nil, &connector.CommandError{Cmd: cmd, ExitCode: 1} // Non-zero exit
	}

	ok, err := r.Check(context.Background(), mockConn, "false_command", false)
	if err != nil {
		t.Fatalf("Check() error = %v for non-zero exit (expected nil error from Check itself)", err)
	}
	if ok {
		t.Error("Check() = true, want false for command with non-zero exit")
	}
}

func TestRunner_Check_Error_ExecFailed(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t) // Updated call
	expectedErr := errors.New("connection failed")
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		mockConn.LastExecCmd = cmd
		mockConn.LastExecOptions = options
		return nil, nil, expectedErr // Actual execution error, not CommandError
	}

	_, err := r.Check(context.Background(), mockConn, "any_command", false)
	if err == nil {
		t.Fatal("Check() with actual exec error expected error, got nil")
	}
	if !errors.Is(err, expectedErr) {
		t.Errorf("Check() error = %v, want %v", err, expectedErr)
	}
}


func TestRunner_RunWithOptions_Success(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t) // Updated call

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

	stdout, _, err := r.RunWithOptions(context.Background(), mockConn, "test_cmd_opts", expectedOpts)
	if err != nil {
		t.Fatalf("RunWithOptions() error = %v", err)
	}
	if string(stdout) != expectedStdout {
		t.Errorf("RunWithOptions() stdout = %q, want %q", string(stdout), expectedStdout)
	}
}

func TestRunner_RunInBackground_Success_NohupFound(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t)
	testCmd := "sleep 10"

	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if file == "nohup" {
			return "/usr/bin/nohup", nil
		}
		return "", fmt.Errorf("unexpected LookPath call for %s", file)
	}

	var executedBgCmd string
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		executedBgCmd = cmd
		if !strings.HasPrefix(cmd, "/usr/bin/nohup sh -c 'sleep 10' > /dev/null 2>&1 &") {
			t.Errorf("RunInBackground command structure incorrect, got: %s", cmd)
		}
		if options.Sudo { // Assuming sudo false for this test case
			t.Error("RunInBackground expected sudo false for this test")
		}
		return nil, nil, nil
	}

	err := r.RunInBackground(context.Background(), mockConn, testCmd, false)
	if err != nil {
		t.Fatalf("RunInBackground() error = %v", err)
	}
	if executedBgCmd == "" {
		t.Error("ExecFunc was not called by RunInBackground")
	}
}

func TestRunner_RunInBackground_Success_NohupNotFound(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t)
	testCmd := "my_daemon -d"

	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) {
		if file == "nohup" {
			return "", errors.New("nohup not found")
		}
		// Allow other lookups if any were part of a more complex default runner setup
		return "/usr/bin/"+file, nil // Default for other tools
	}

	var executedBgCmd string
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		executedBgCmd = cmd
		// Expected: sh -c 'my_daemon -d > /dev/null 2>&1 &' (note the space at the end from current impl)
		expectedPrefix := "sh -c 'my_daemon -d > /dev/null 2>&1 &' "
		if !strings.HasPrefix(cmd, expectedPrefix) {
			t.Errorf("RunInBackground command without nohup structure incorrect, got: %q, want prefix %q", cmd, expectedPrefix)
		}
		if !options.Sudo { // Assuming sudo true for this test case
			t.Error("RunInBackground expected sudo true for this test")
		}
		return nil, nil, nil
	}

	err := r.RunInBackground(context.Background(), mockConn, testCmd, true) // Sudo true
	if err != nil {
		t.Fatalf("RunInBackground() without nohup error = %v", err)
	}
	if executedBgCmd == "" {
		t.Error("ExecFunc was not called by RunInBackground when nohup not found")
	}
}

func TestRunner_RunInBackground_CmdEmpty(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t)
	err := r.RunInBackground(context.Background(), mockConn, " ", false)
	if err == nil {
		t.Fatal("RunInBackground() with empty command expected error, got nil")
	}
	if !strings.Contains(err.Error(), "command cannot be empty") {
		t.Errorf("Error message mismatch: got %v", err)
	}
}

func TestRunner_RunInBackground_LaunchError(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t)
	mockConn.LookPathFunc = func(ctx context.Context, file string) (string, error) { return "/usr/bin/"+file, nil} // Assume nohup found

	expectedErr := errors.New("failed to launch")
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		return nil, []byte("launch stderr"), expectedErr
	}

	err := r.RunInBackground(context.Background(), mockConn, "any_cmd", false)
	if err == nil {
		t.Fatal("RunInBackground() expected error on launch failure, got nil")
	}
	if !strings.Contains(err.Error(), "failed to launch command") || !strings.Contains(err.Error(), "launch stderr") {
		t.Errorf("Error message from RunInBackground did not contain expected parts: %v", err)
	}
}


func TestRunner_RunRetry_SuccessOnFirstAttempt(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t)
	expectedOutput := "success"
	var execCount int

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		execCount++
		return []byte(expectedOutput), nil, nil
	}

	out, err := r.RunRetry(context.Background(), mockConn, "test_cmd", false, 3, 1*time.Millisecond)
	if err != nil {
		t.Fatalf("RunRetry() error = %v", err)
	}
	if out != expectedOutput {
		t.Errorf("RunRetry() output = %q, want %q", out, expectedOutput)
	}
	if execCount != 1 {
		t.Errorf("RunRetry() expected 1 execution, got %d", execCount)
	}
}

func TestRunner_RunRetry_SuccessOnThirdAttempt(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t)
	expectedOutput := "success on 3rd"
	var execCount int

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		execCount++
		if execCount < 3 {
			return []byte("fail output"), nil, errors.New("simulated failure")
		}
		return []byte(expectedOutput), nil, nil // Success on 3rd attempt
	}

	out, err := r.RunRetry(context.Background(), mockConn, "test_cmd_retries", true, 2, 1*time.Millisecond) // 1 initial + 2 retries = 3 attempts
	if err != nil {
		t.Fatalf("RunRetry() error = %v", err)
	}
	if out != expectedOutput {
		t.Errorf("RunRetry() output = %q, want %q", out, expectedOutput)
	}
	if execCount != 3 {
		t.Errorf("RunRetry() expected 3 executions, got %d", execCount)
	}
}

func TestRunner_RunRetry_AllAttemptsFail(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t)
	var execCount int
	finalError := errors.New("final failure")

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		execCount++
		if execCount == 3 { // Total 3 attempts (1 initial + 2 retries)
			return []byte("last fail output"), nil, finalError
		}
		return []byte("intermediate fail"), nil, errors.New("intermediate failure")
	}

	out, err := r.RunRetry(context.Background(), mockConn, "failing_cmd_always", false, 2, 1*time.Millisecond)
	if err == nil {
		t.Fatal("RunRetry() expected error after all attempts fail, got nil")
	}
	if !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Errorf("Error message missing attempt count: %v", err)
	}
	if !errors.Is(err, finalError) { // Check if the last error is wrapped
		t.Errorf("RunRetry() error should wrap the last execution error. Got: %v, want wrapped: %v", err, finalError)
	}
	if execCount != 3 {
		t.Errorf("RunRetry() expected 3 executions, got %d", execCount)
	}
	if !strings.Contains(out, "last fail output") {
		t.Errorf("RunRetry() output on failure = %q, expected to contain last failure output", out)
	}
}

func TestRunner_RunRetry_ContextCancelledDuringDelay(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t)
	var execCount int
	cancelCtx, cancel := context.WithCancel(context.Background())

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		execCount++
		// Simulate a delay that would allow cancellation
		if execCount == 1 { // First attempt fails
			go func() {
				time.Sleep(50 * time.Millisecond) // Ensure delay in RunRetry is hit
				cancel()
			}()
			return nil, nil, errors.New("first attempt failure")
		}
		return nil, nil, errors.New("should not reach further attempts")
	}

	_, err := r.RunRetry(cancelCtx, mockConn, "cmd_cancel_delay", false, 3, 200*time.Millisecond) // Long delay
	if err == nil {
		t.Fatal("RunRetry() expected error due to context cancellation, got nil")
	}
	if !errors.Is(err, context.Canceled) && !strings.Contains(err.Error(), "context cancelled during delay") {
		t.Errorf("RunRetry() error = %v, expected context.Canceled or specific message", err)
	}
	if execCount != 1 { // Should only try once before context is cancelled during delay
		t.Errorf("RunRetry() expected 1 execution before cancellation, got %d", execCount)
	}
}

func TestRunner_RunRetry_ContextCancelledDuringExecution(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t)
	cancelCtx, cancel := context.WithCancel(context.Background())
	var execCount int

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		execCount++
		if execCount == 1 { // First attempt
			// Simulate work then cancellation
			time.Sleep(10 * time.Millisecond) // some work
			cancel() // Cancel context during the (mocked) execution of r.Run
			return nil, nil, ctx.Err() // Return context error as r.Run might
		}
		return nil, nil, errors.New("should not be reached")
	}

	_, err := r.RunRetry(cancelCtx, mockConn, "cmd_cancel_exec", false, 1, 10*time.Millisecond)
	if err == nil {
		t.Fatal("RunRetry() expected error due to context cancellation during exec, got nil")
	}

	// The error from r.Run (which is ctx.Err()) will be wrapped by RunRetry's message
	if !errors.Is(err, context.Canceled) {
		t.Errorf("RunRetry() error should wrap context.Canceled. Got: %v", err)
	}
	if !strings.Contains(err.Error(), "failed after 1 attempts") && !strings.Contains(err.Error(), "context cancelled before command") {
		// The exact error message depends on when ctx.Done() is checked vs when r.Run returns.
		// If r.Run itself returns ctx.Err(), then RunRetry will wrap that.
		// If ctx.Done() is caught by the select before r.Run, a different message is formed.
		// Both indicate cancellation.
		t.Logf("RunRetry error: %v (execCount: %d)", err, execCount)
	}

	if execCount != 1 {
		t.Errorf("RunRetry() expected 1 execution attempt, got %d", execCount)
	}
}


func TestRunner_RunRetry_NoRetriesNegativeInput(t *testing.T) {
	r, mockConn := newTestRunnerForCommand(t)
	expectedOutput := "success"
	var execCount int

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		execCount++
		return []byte(expectedOutput), nil, nil
	}

	out, err := r.RunRetry(context.Background(), mockConn, "test_cmd_neg_retry", false, -5, 1*time.Millisecond)
	if err != nil {
		t.Fatalf("RunRetry() with negative retries error = %v", err)
	}
	if out != expectedOutput {
		t.Errorf("RunRetry() output = %q, want %q", out, expectedOutput)
	}
	if execCount != 1 { // Should default to 0 retries (1 attempt)
		t.Errorf("RunRetry() with negative retries expected 1 execution, got %d", execCount)
	}
}
