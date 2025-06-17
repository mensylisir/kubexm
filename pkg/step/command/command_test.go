package command

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kubexms/kubexms/pkg/connector"
	// "github.com/kubexms/kubexms/pkg/runner" // Not directly needed if using step's mock helper
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step" // Import step to use its test helpers
)

// newTestContextForCommandStep is a local alias for the shared helper if preferred,
// or just use step.newTestContextForStep directly.
// For this test, we'll use the one from the step package to ensure it's accessible.
// var newTestContextForCommandStep = step.newTestContextForStep


func TestCommandStep_Run_Success(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	// Facts are not strictly needed for basic command step, use nil to get defaults from helper
	ctx := step.newTestContextForStep(t, mockConn, nil)

	cs := NewCommandStep("echo hello", false).WithName("Test Echo")

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == "echo hello" && !options.Sudo {
			return []byte("hello"), nil, nil
		}
		return nil, nil, fmt.Errorf("unexpected exec call: %s with sudo %v", cmd, options.Sudo)
	}

	res := cs.Run(ctx)

	if res.Status != "Succeeded" {
		t.Errorf("Run status = %s, want Succeeded. Error: %v", res.Status, res.Error)
	}
	if res.Stdout != "hello" {
		t.Errorf("Run stdout = %s, want hello", res.Stdout)
	}
	if res.Error != nil {
		t.Errorf("Run error = %v, want nil", res.Error)
	}
	if len(mockConn.ExecHistory) != 1 || mockConn.ExecHistory[0] != "echo hello" {
		t.Errorf("ExecHistory incorrect, got: %v", mockConn.ExecHistory)
	}
}

func TestCommandStep_Run_Error_NonZeroExit(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := step.newTestContextForStep(t, mockConn, nil)

	cs := NewCommandStep("exit 123", false)
	expectedCmdErr := &connector.CommandError{Cmd: "exit 123", ExitCode: 123, Stderr: "failed"}

	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		return nil, []byte("failed"), expectedCmdErr
	}

	res := cs.Run(ctx)

	if res.Status != "Failed" {
		t.Errorf("Run status = %s, want Failed", res.Status)
	}
	// Check if res.Error is the specific CommandError instance or wraps it.
	var cmdError *connector.CommandError
	if !errors.As(res.Error, &cmdError) {
		t.Errorf("Run error type = %T, want to be or wrap *connector.CommandError", res.Error)
	} else {
		if cmdError.ExitCode != 123 {
			t.Errorf("CommandError exit code = %d, want 123", cmdError.ExitCode)
		}
	}
	if res.Stderr != "failed" {
		t.Errorf("Run stderr = %s, want 'failed'", res.Stderr)
	}
}

func TestCommandStep_Run_IgnoreError(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := step.newTestContextForStep(t, mockConn, nil)

	cs := NewCommandStep("exit 1", false).WithIgnoreError(true)
	cmdErr := &connector.CommandError{Cmd: "exit 1", ExitCode: 1, Stderr: "ignored error output"}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		return nil, []byte("ignored error output"), cmdErr
	}
	res := cs.Run(ctx)
	if res.Status != "Succeeded" {
		t.Errorf("Run status with IgnoreError = %s, want Succeeded", res.Status)
	}
	// When IgnoreError is true, res.Error should be nil (or the original error if we change the spec).
	// The current CommandStep sets res.Error = nil when IgnoreError is true and error is CommandError.
	if res.Error != nil {
		t.Errorf("Run error with IgnoreError = %v, want nil for status Succeeded", res.Error)
	}
	if !strings.Contains(res.Message, "error was ignored") {
		t.Errorf("Run message = %q, expected to contain 'error was ignored'", res.Message)
	}
	if res.Stderr != "ignored error output" { // Stderr should still be captured
		t.Errorf("Run stderr = %q, want 'ignored error output'", res.Stderr)
	}
}

func TestCommandStep_Run_ExpectedExitCode_Match(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := step.newTestContextForStep(t, mockConn, nil)

	cs := NewCommandStep("mycmd_exits_5", false).WithExpectedExitCode(5)
	cmdErr := &connector.CommandError{Cmd: "mycmd_exits_5", ExitCode: 5} // Simulates command exiting with 5
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		return nil, nil, cmdErr
	}
	res := cs.Run(ctx)
	if res.Status != "Succeeded" {
		t.Errorf("Run status with ExpectedExitCode match = %s, want Succeeded", res.Status)
	}
	if res.Error != nil {
		t.Errorf("Run error with ExpectedExitCode match = %v, want nil", res.Error)
	}
}

func TestCommandStep_Run_ExpectedExitCode_Mismatch(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := step.newTestContextForStep(t, mockConn, nil)

	cs := NewCommandStep("mycmd_exits_1", false).WithExpectedExitCode(0) // Expects 0, gets 1
	cmdErr := &connector.CommandError{Cmd: "mycmd_exits_1", ExitCode: 1, Stderr: "actual exit 1"}
	mockConn.ExecFunc = func(ctx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		return nil, []byte("actual exit 1"), cmdErr
	}
	res := cs.Run(ctx)
	if res.Status != "Failed" {
		t.Errorf("Run status with ExpectedExitCode mismatch = %s, want Failed", res.Status)
	}
	if res.Error == nil {
		t.Error("Run error with ExpectedExitCode mismatch is nil, want an error")
	} else {
		var actualCmdErr *connector.CommandError
		if !errors.As(res.Error, &actualCmdErr) || actualCmdErr.ExitCode != 1 {
			t.Errorf("Run error is not the expected CommandError or ExitCode mismatch: %v", res.Error)
		}
	}
}


func TestCommandStep_Check_NoCheckCmd(t *testing.T) {
	mockConn := step.NewMockStepConnector() // Not used as CheckCmd is empty
	ctx := step.newTestContextForStep(t, mockConn, nil)
	cs := NewCommandStep("echo main", false)

	isDone, err := cs.Check(ctx)
	if err != nil {
		t.Fatalf("Check() with no CheckCmd error = %v", err)
	}
	if isDone {
		t.Error("Check() with no CheckCmd = true, want false")
	}
}

func TestCommandStep_Check_CheckCmd_Succeeds_SkipsMain(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := step.newTestContextForStep(t, mockConn, nil)

	cs := NewCommandStep("echo main", false).WithCheckCmd("check_if_done", false, 0) // Expects exit 0 for done

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == "check_if_done" {
			return nil, nil, nil // CheckCmd succeeds (exit 0)
		}
		return nil, nil, fmt.Errorf("Check (skip): unexpected cmd %s", cmd)
	}

	isDone, err := cs.Check(ctx)
	if err != nil {
		t.Fatalf("Check() with succeeding CheckCmd error = %v", err)
	}
	if !isDone {
		t.Error("Check() with succeeding CheckCmd = false, want true (skip main)")
	}
	if len(mockConn.ExecHistory) != 1 || mockConn.ExecHistory[0] != "check_if_done" {
		t.Errorf("ExecHistory for CheckCmd success incorrect, got: %v", mockConn.ExecHistory)
	}
}

func TestCommandStep_Check_CheckCmd_Fails_RunsMain(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := step.newTestContextForStep(t, mockConn, nil)

	cs := NewCommandStep("echo main", false).WithCheckCmd("check_if_not_done", false, 0) // Expects 0 for done

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == "check_if_not_done" {
			return nil, nil, &connector.CommandError{ExitCode: 1} // CheckCmd "fails" (non-zero exit)
		}
		return nil, nil, fmt.Errorf("Check (run): unexpected cmd %s", cmd)
	}

	isDone, err := cs.Check(ctx)
	if err != nil {
		t.Fatalf("Check() with failing CheckCmd error = %v", err)
	}
	if isDone {
		t.Error("Check() with failing CheckCmd = true, want false (run main)")
	}
}

func TestCommandStep_Name_Default(t *testing.T) {
	cs := NewCommandStep("a_very_long_command_that_should_be_truncated_for_the_name", false)
	expectedName := "Exec: a_very_long_command_that_sho..."
	if cs.Name() != expectedName {
		t.Errorf("Name() default = %q, want %q", cs.Name(), expectedName)
	}

	csShort := NewCommandStep("short_cmd", false)
	expectedShortName := "Exec: short_cmd"
	if csShort.Name() != expectedShortName {
		t.Errorf("Name() default short = %q, want %q", csShort.Name(), expectedShortName)
	}
}

func TestCommandStep_Name_Custom(t *testing.T) {
	customName := "My Custom Command Step"
	cs := NewCommandStep("any_cmd", false).WithName(customName)
	if cs.Name() != customName {
		t.Errorf("Name() custom = %q, want %q", cs.Name(), customName)
	}
}
