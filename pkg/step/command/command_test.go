package command

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kubexms/kubexms/pkg/config"  // For config.Cluster in test helper
	"github.com/kubexms/kubexms/pkg/connector"
	"github.com/kubexms/kubexms/pkg/logger"
	"github.com/kubexms/kubexms/pkg/runner"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/spec" // For spec.StepSpec
	"github.com/kubexms/kubexms/pkg/step" // For step.Result and mock helpers
)

// newTestContextForCommandStep uses the shared helper from pkg/step.
// It primarily sets up runtime.Context with a mock runner whose connector can be controlled.
func newTestContextForCommandStep(t *testing.T, mockConn *step.MockStepConnector) *runtime.Context {
	t.Helper()
	// Facts are not strictly needed by CommandStepExecutor itself, default facts from newTestContextForStep are fine.
	return step.newTestContextForStep(t, mockConn, nil)
}


func TestCommandStepExecutor_Execute_Success(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForCommandStep(t, mockConn)

	cmdSpec := &CommandStepSpec{Cmd: "echo hello", Sudo: false, SpecName: "Test Echo"}
	// Get the executor from the registry using the spec's type name
	executor := step.GetExecutor(step.GetSpecTypeName(cmdSpec))
	if executor == nil {
		t.Fatalf("Executor not registered for CommandStepSpec (type name: %s)", step.GetSpecTypeName(cmdSpec))
	}

	expectedStdout := "hello"
	mockConn.ExecFunc = func(ctxEx context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == cmdSpec.Cmd && options.Sudo == cmdSpec.Sudo {
			return []byte(expectedStdout), nil, nil
		}
		return nil, nil, fmt.Errorf("unexpected exec call: %s, sudo: %v", cmd, options.Sudo)
	}

	res := executor.Execute(cmdSpec, ctx)

	if res.Status != "Succeeded" {
		t.Errorf("Execute status = %s, want Succeeded. Error: %v", res.Status, res.Error)
	}
	if res.Stdout != expectedStdout {
		t.Errorf("Execute stdout = %s, want %s", res.Stdout, expectedStdout)
	}
	if res.Error != nil {
		t.Errorf("Execute error = %v, want nil", res.Error)
	}
}

func TestCommandStepExecutor_Execute_Error_NonZeroExit(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForCommandStep(t, mockConn)

	cmdSpec := &CommandStepSpec{Cmd: "exit 123", Sudo: false}
	executor := step.GetExecutor(step.GetSpecTypeName(cmdSpec))
	if executor == nil { t.Fatal("Executor not registered") }

	expectedErr := &connector.CommandError{Cmd: cmdSpec.Cmd, ExitCode: 123, Stderr: "failed"}
	mockConn.ExecFunc = func(ctxC context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		return nil, []byte("failed"), expectedErr
	}

	res := executor.Execute(cmdSpec, ctx)

	if res.Status != "Failed" { t.Errorf("Execute status = %s, want Failed", res.Status) }

	var cmdError *connector.CommandError
	if !errors.As(res.Error, &cmdError) {
		t.Errorf("Execute error type = %T, want to be or wrap *connector.CommandError", res.Error)
	} else {
		if cmdError.ExitCode != 123 {
			t.Errorf("CommandError exit code = %d, want 123", cmdError.ExitCode)
		}
	}
	if res.Stderr != "failed" { t.Errorf("Execute stderr = %s, want 'failed'", res.Stderr) }
}

func TestCommandStepExecutor_Execute_IgnoreError(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForCommandStep(t, mockConn)

	cmdSpec := &CommandStepSpec{Cmd: "exit 1", Sudo: false, IgnoreError: true}
	executor := step.GetExecutor(step.GetSpecTypeName(cmdSpec))
	if executor == nil { t.Fatal("Executor not registered") }

	cmdErr := &connector.CommandError{Cmd: cmdSpec.Cmd, ExitCode: 1, Stderr: "ignored error output"}
	mockConn.ExecFunc = func(ctxC context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		return nil, []byte("ignored error output"), cmdErr
	}
	res := executor.Execute(cmdSpec, ctx)
	if res.Status != "Succeeded" { t.Errorf("Execute status with IgnoreError = %s, want Succeeded", res.Status) }
	// As per current CommandStepExecutor logic, res.Error is nil when IgnoreError=true and status is Succeeded
	if res.Error != nil { t.Errorf("Execute error with IgnoreError = %v, want nil", res.Error) }
	if !strings.Contains(res.Message, "error was ignored") {
		t.Errorf("Execute message = %q, expected to contain 'error was ignored'", res.Message)
	}
	if res.Stderr != "ignored error output" { // Stderr should still be captured
		t.Errorf("Execute stderr = %q, want 'ignored error output'", res.Stderr)
	}
}

func TestCommandStepExecutor_Check_NoCheckCmd(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForCommandStep(t, mockConn)
	cmdSpec := &CommandStepSpec{Cmd: "echo main"}
	executor := step.GetExecutor(step.GetSpecTypeName(cmdSpec))
	if executor == nil { t.Fatal("Executor not registered") }

	isDone, err := executor.Check(cmdSpec, ctx)
	if err != nil { t.Fatalf("Check() with no CheckCmd error = %v", err) }
	if isDone { t.Error("Check() with no CheckCmd = true, want false") }
}

func TestCommandStepExecutor_Check_CheckCmd_Succeeds_SkipsMain(t *testing.T) {
	mockConn := step.NewMockStepConnector()
	ctx := newTestContextForCommandStep(t, mockConn)

	cmdSpec := &CommandStepSpec{
		Cmd: "echo main",
		CheckCmd: "check_if_done",
		CheckSudo: false,
		CheckExpectedExitCode: 0,
	}
	executor := step.GetExecutor(step.GetSpecTypeName(cmdSpec))
	if executor == nil { t.Fatal("Executor not registered") }

	mockConn.ExecFunc = func(ctxGo context.Context, cmd string, options *connector.ExecOptions) ([]byte, []byte, error) {
		if cmd == "check_if_done" {
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("Check (skip): unexpected cmd %s", cmd)
	}

	isDone, err := executor.Check(cmdSpec, ctx)
	if err != nil { t.Fatalf("Check() with succeeding CheckCmd error = %v", err) }
	if !isDone { t.Error("Check() with succeeding CheckCmd = false, want true (skip main)") }
}


// Test builder functions (optional, but good for coverage if they are used)
func TestCommandStepSpec_BuilderMethods(t *testing.T) {
	spec := NewCommandSpec("base_cmd", true).
		WithName("MyCmd").
		WithIgnoreError(true).
		WithTimeout(5 * time.Second).
		WithEnv([]string{"VAR=val"}).
		WithExpectedExitCode(0). // Explicitly 0
		WithCheckCmd("check", false, 1)

	if spec.Cmd != "base_cmd" || !spec.Sudo { t.Error("NewCommandSpec base failed") }
	if spec.SpecName != "MyCmd" {t.Error("WithName failed")}
	if !spec.IgnoreError {t.Error("WithIgnoreError failed")}
	if spec.Timeout != 5*time.Second {t.Error("WithTimeout failed")}
	if len(spec.Env) != 1 || spec.Env[0] != "VAR=val" {t.Error("WithEnv failed")}
	if spec.ExpectedExitCode != 0 {t.Error("WithExpectedExitCode failed")}
	if spec.CheckCmd != "check" || spec.CheckSudo != false || spec.CheckExpectedExitCode != 1 {
		t.Error("WithCheckCmd failed")
	}
}
