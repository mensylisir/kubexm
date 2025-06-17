package command

import (
	"context"
	"errors" // Required for errors.As
	"fmt"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
)

// CommandStep executes an arbitrary shell command on a host.
type CommandStep struct {
	// StepName is a descriptive name for this command execution step.
	// If not set, a default name will be generated from the command.
	StepName string

	// Cmd is the shell command to be executed.
	Cmd string

	// Sudo specifies whether to execute the command with sudo privileges.
	Sudo bool

	// IgnoreError, if true, means that a non-zero exit code from this command
	// will not cause the step to be marked as "Failed" (it will be "Succeeded" instead),
	// though the error details will still be captured in the Result.
	// This is useful for commands where a non-zero exit might be an expected outcome
	// that doesn't signify a true failure of the step's intent.
	IgnoreError bool

	// Timeout specifies a duration after which the command will be forcefully terminated.
	// If zero or negative, no specific timeout is applied by this step for the command execution,
	// relying on the GoContext's deadline or the connector's default behavior.
	Timeout time.Duration

	// Env is a slice of environment variables to set for the command, in "KEY=VALUE" format.
	Env []string

	// ExpectedExitCode defines the exit code that is considered a success for this command.
	// Defaults to 0 if not set. If IgnoreError is true, this field is less critical
	// as all exit codes (after successful execution of the command itself) are typically treated as success.
	// If IgnoreError is false, and the command exits with a code different from ExpectedExitCode,
	// the step is marked as "Failed".
	ExpectedExitCode int

	// CheckCmd is an optional command that, if specified, will be run by the Check method.
	// If CheckCmd exits with CheckExpectedExitCode (or 0 if CheckExpectedExitCode for CheckCmd is not set),
	// the main Cmd of this step will be skipped, making the step idempotent.
	CheckCmd string
	// CheckSudo specifies whether the CheckCmd should be run with sudo.
	CheckSudo bool
	// CheckExpectedExitCode defines the success exit code for the CheckCmd. Defaults to 0.
	CheckExpectedExitCode int
}

// NewCommandStep creates a new CommandStep with required fields.
// The command to execute and whether to use sudo are fundamental.
// Optional configurations can be set using builder-style methods or direct field access.
func NewCommandStep(command string, sudo bool) *CommandStep {
	return &CommandStep{
		Cmd:  command,
		Sudo: sudo,
		// StepName will be auto-generated if not set via WithName().
		// ExpectedExitCode defaults to 0.
		// CheckExpectedExitCode defaults to 0.
	}
}

// WithName sets a custom name for the step, used in logging and results.
func (s *CommandStep) WithName(name string) *CommandStep {
	s.StepName = name
	return s
}

// WithIgnoreError sets the IgnoreError flag. If true, non-zero exit codes
// from the main command do not mark the step as Failed.
func (s *CommandStep) WithIgnoreError(ignore bool) *CommandStep {
	s.IgnoreError = ignore
	return s
}

// WithTimeout sets a specific timeout for the command execution.
func (s *CommandStep) WithTimeout(timeout time.Duration) *CommandStep {
	s.Timeout = timeout
	return s
}

// WithEnv sets environment variables (KEY=VALUE format) for the command.
func (s *CommandStep) WithEnv(env []string) *CommandStep {
	s.Env = env
	return s
}

// WithExpectedExitCode sets the exit code that signifies success for the main command.
// This is only relevant if IgnoreError is false.
func (s *CommandStep) WithExpectedExitCode(code int) *CommandStep {
	s.ExpectedExitCode = code
	return s
}

// WithCheckCmd defines an idempotency check.
// If checkCmd runs and its exit code matches checkExpectedExitCode (variadic, defaults to 0),
// the main Cmd of this step will be skipped.
func (s *CommandStep) WithCheckCmd(checkCmd string, checkSudo bool, checkExpectedExitCode ...int) *CommandStep {
	s.CheckCmd = checkCmd
	s.CheckSudo = checkSudo
	if len(checkExpectedExitCode) > 0 {
		s.CheckExpectedExitCode = checkExpectedExitCode[0]
	} else {
		s.CheckExpectedExitCode = 0 // Default success exit code for check command is 0
	}
	return s
}


// Name returns the name of the step. If StepName is not explicitly set,
// it generates a default name based on the command being executed.
func (s *CommandStep) Name() string {
	if s.StepName != "" {
		return s.StepName
	}
	// Generate a default name if not provided.
	if len(s.Cmd) > 30 {
		return fmt.Sprintf("Exec: %s...", s.Cmd[:30])
	}
	return fmt.Sprintf("Exec: %s", s.Cmd)
}

// Check determines if the main command (Cmd) needs to be run.
// If CheckCmd is defined, this method executes it. If the CheckCmd execution
// results in the CheckExpectedExitCode, Check returns (true, nil), indicating
// the main command is already "done" or its condition is met, so it should be skipped.
// If CheckCmd is not defined, or if it runs but does not result in the expected outcome,
// Check returns (false, nil), indicating the main command should run.
// An error is returned if the CheckCmd itself fails to execute (e.g., connection issue).
func (s *CommandStep) Check(ctx *runtime.Context) (isDone bool, err error) {
	if s.CheckCmd == "" {
		return false, nil // No check command defined, so main command should always run.
	}

	startTime := time.Now()
	// Create a temporary result for logging/debugging the check command itself, not returned directly.
	// The primary output of this Check method is `isDone` and `err`.
	_ = step.NewResult(fmt.Sprintf("Check for '%s'", s.Name()), ctx.Host.Name, startTime, nil)


	opts := &connector.ExecOptions{
		Sudo:    s.CheckSudo,
		Timeout: s.Timeout, // Potentially use a shorter, specific timeout for checks.
		Env:     s.Env,     // Inherit environment variables for the check.
	}

	ctx.Logger.Debugf("Running check command for step '%s': %s (Sudo: %v)", s.Name(), s.CheckCmd, s.CheckSudo)
	stdout, stderr, runErr := ctx.Host.Runner.RunWithOptions(ctx.GoContext, s.CheckCmd, opts)

	if runErr != nil {
		var cmdErr *connector.CommandError
		if errors.As(runErr, &cmdErr) {
			// CheckCmd executed but returned an exit code.
			if cmdErr.ExitCode == s.CheckExpectedExitCode {
				ctx.Logger.Debugf("Check command '%s' for step '%s' completed with expected exit code %d. Main step will be skipped.", s.CheckCmd, s.Name(), s.CheckExpectedExitCode)
				return true, nil // isDone = true, main Cmd will be skipped.
			}
			// CheckCmd exited with a code different from what we consider "done".
			ctx.Logger.Debugf("Check command '%s' for step '%s' failed with exit code %d (expected %d for 'done' state). Main command will run. Stderr: %s", s.CheckCmd, s.Name(), cmdErr.ExitCode, s.CheckExpectedExitCode, string(stderr))
			return false, nil // Not done, main Cmd should run.
		}
		// The execution of CheckCmd itself failed (e.g., connection error, command not found).
		// This is an actual error in the check process.
		ctx.Logger.Errorf("Failed to execute check command '%s' for step '%s': %v. Stdout: %s, Stderr: %s", s.CheckCmd, s.Name(), runErr, string(stdout), string(stderr))
		return false, fmt.Errorf("check command '%s' execution failed: %w", s.CheckCmd, runErr)
	}

	// CheckCmd executed successfully (runErr is nil, meaning exit code 0).
	if s.CheckExpectedExitCode == 0 {
		ctx.Logger.Debugf("Check command '%s' for step '%s' completed successfully (exit 0). Main step will be skipped.", s.CheckCmd, s.Name())
		return true, nil // isDone = true, main Cmd will be skipped.
	}

	// CheckCmd exited 0, but CheckExpectedExitCode was non-zero.
	ctx.Logger.Debugf("Check command '%s' for step '%s' completed with exit code 0, but expected %d for 'done' state. Main command will run.", s.CheckCmd, s.Name(), s.CheckExpectedExitCode)
	return false, nil // Not done, main Cmd should run.
}


// Run executes the main command defined in the CommandStep.
// It uses the associated Runner instance from the runtime.Context.
// It populates and returns a step.Result detailing the outcome.
func (s *CommandStep) Run(ctx *runtime.Context) *step.Result {
	startTime := time.Now()
	// Initialize result. Error and status will be updated based on execution.
	res := step.NewResult(s.Name(), ctx.Host.Name, startTime, nil)

	opts := &connector.ExecOptions{
		Sudo:    s.Sudo,
		Timeout: s.Timeout,
		Env:     s.Env,
	}

	ctx.Logger.Infof("Running command on host %s: %s (Sudo: %v)", ctx.Host.Name, s.Cmd, s.Sudo)
	stdout, stderr, runErr := ctx.Host.Runner.RunWithOptions(ctx.GoContext, s.Cmd, opts)

	res.Stdout = string(stdout)
	res.Stderr = string(stderr)
	res.EndTime = time.Now() // Explicitly set EndTime after command execution.

	if runErr != nil {
		var cmdErr *connector.CommandError
		if errors.As(runErr, &cmdErr) {
			// Command executed but returned an exit code (which might or might not be an error).
			res.Error = cmdErr // Store the full CommandError.

			if s.IgnoreError {
				ctx.Logger.Warnf("Command '%s' on host %s exited with code %d (stderr: %s), but error is ignored. Step considered successful.", s.Cmd, ctx.Host.Name, cmdErr.ExitCode, string(stderr))
				res.Status = "Succeeded"
				// Error is kept in res.Error for record, but status is Succeeded.
				// Optionally, clear res.Error if "ignored" means no error should be reported at all.
				// For now, let's keep it for diagnostics but override status.
				// res.Error = nil
			} else if cmdErr.ExitCode == s.ExpectedExitCode {
				ctx.Logger.Successf("Command '%s' on host %s completed with expected exit code %d.", s.Cmd, ctx.Host.Name, s.ExpectedExitCode)
				res.Status = "Succeeded"
				res.Error = nil // Matched expected exit code, so not an application-level error for this step.
			} else {
				ctx.Logger.Errorf("Command '%s' on host %s failed with exit code %d (expected %d). Stderr: %s", s.Cmd, ctx.Host.Name, cmdErr.ExitCode, s.ExpectedExitCode, string(stderr))
				res.Status = "Failed"
				// res.Error is already cmdErr
			}
		} else {
			// Execution of the command itself failed (e.g., connection error, command not found by shell).
			// This is different from the command running and returning a non-zero exit code.
			ctx.Logger.Errorf("Failed to execute command '%s' on host %s: %v. Stderr: %s", s.Cmd, ctx.Host.Name, runErr, string(stderr))
			res.Error = runErr
			res.Status = "Failed"
		}
	} else {
		// runErr is nil, meaning command executed and exited with 0.
		if s.ExpectedExitCode == 0 {
			ctx.Logger.Successf("Command '%s' on host %s completed successfully (exit 0).", s.Cmd, ctx.Host.Name)
			res.Status = "Succeeded"
			// res.Error is already nil
		} else {
			// Exited 0, but a different code was expected (and IgnoreError is false).
			errMsg := fmt.Errorf("command '%s' on host %s exited 0, but expected exit code %d", s.Cmd, ctx.Host.Name, s.ExpectedExitCode)
			ctx.Logger.Errorf(errMsg.Error() + ". Marked as failed.")
			res.Error = errMsg
			res.Status = "Failed"
		}
	}

	// Add a message if an error was ignored.
	if res.Status == "Succeeded" && s.IgnoreError && runErr != nil {
		// We need to check if runErr (the original error) was a CommandError to access its details.
		var originalCmdErr *connector.CommandError
		if errors.As(runErr, &originalCmdErr) {
			res.Message = fmt.Sprintf("Command executed with exit code %d, but error was ignored due to step configuration. Original Stderr: %s", originalCmdErr.ExitCode, string(originalCmdErr.Stderr))
		} else {
			res.Message = fmt.Sprintf("Command executed with an error, but it was ignored due to step configuration. Original error: %v", runErr)
		}
	}

	return res
}

// Ensure CommandStep implements Step interface
var _ step.Step = &CommandStep{}
