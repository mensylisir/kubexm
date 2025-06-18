package command

import (
	"context" // Required by runtime.Context, not directly by this file's logic for context.Background()
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/spec" // Import the spec package
	"github.com/kubexms/kubexms/pkg/step"  // Import for step.Result, step.Register, etc.
)

// CommandStepSpec holds the declarative parameters for executing a shell command.
type CommandStepSpec struct {
	// SpecName is a descriptive name for this command execution step.
	// If not provided, a name will be generated based on the command.
	SpecName string

	// Cmd is the shell command to be executed.
	Cmd string

	// Sudo specifies whether to execute the command with sudo privileges.
	Sudo bool

	// IgnoreError, if true, means that a non-zero exit code from this command
	// (or if ExpectedExitCode is not met) will not cause the step to be marked as "Failed".
	// The step will be "Succeeded", but error details will be in Result.Error/Message.
	IgnoreError bool

	// Timeout specifies a duration after which the command will be forcefully terminated.
	// If zero, no timeout is applied beyond the context's deadline.
	Timeout time.Duration

	// Env is a slice of environment variables to set for the command, in "KEY=VALUE" format.
	Env []string

	// ExpectedExitCode defines the exit code considered a success. Defaults to 0.
	// If IgnoreError is false, a mismatch causes the step to be "Failed".
	ExpectedExitCode int

	// CheckCmd is an optional command for idempotency. If it runs and its exit code
	// matches CheckExpectedExitCode, the main Cmd is skipped.
	CheckCmd            string
	CheckSudo           bool
	CheckExpectedExitCode int // Defaults to 0 for CheckCmd if not set
}

// GetName returns the configured or generated name of the step spec.
func (s *CommandStepSpec) GetName() string {
	if s.SpecName != "" {
		return s.SpecName
	}
	if len(s.Cmd) > 30 {
		return fmt.Sprintf("Exec: %s...", s.Cmd[:30])
	}
	return fmt.Sprintf("Exec: %s", s.Cmd)
}

// Ensure CommandStepSpec implements spec.StepSpec
var _ spec.StepSpec = &CommandStepSpec{}


// CommandStepExecutor implements the logic for executing a CommandStepSpec.
type CommandStepExecutor struct{}

func init() {
	// Register this executor for the CommandStepSpec type.
	// The type name string must be unique and consistent.
	// Using a pointer to the zero value of the spec type for GetSpecTypeName.
	step.Register(step.GetSpecTypeName(&CommandStepSpec{}), &CommandStepExecutor{})
}

// Check determines if the main command needs to be run based on CheckCmd.
func (e *CommandStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	spec, ok := s.(*CommandStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected spec type %T for CommandStepExecutor Check method", s)
	}

	if spec.CheckCmd == "" {
		return false, nil // No check command, so main command should run
	}

	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", spec.GetName(), "phase", "Check").Sugar()
	hostCtxLogger.Debugf("Executing CheckCmd: %s", spec.CheckCmd)

	opts := &connector.ExecOptions{
		Sudo:    spec.CheckSudo,
		Timeout: spec.Timeout,
		Env:     spec.Env,
	}

	// Using ctx.Host.Runner.Run directly as it returns combined output, which is fine for Check's error reporting.
	// If specific stdout/stderr from CheckCmd were needed for logic, RunWithOptions would be better.
	_, checkCmdStderr, runErr := ctx.Host.Runner.RunWithOptions(ctx.GoContext, spec.CheckCmd, opts)

	if runErr != nil {
		var cmdErr *connector.CommandError
		if errors.As(runErr, &cmdErr) {
			if cmdErr.ExitCode == spec.CheckExpectedExitCode {
				hostCtxLogger.Debugf("CheckCmd '%s' completed with expected exit code %d. Main command will be skipped.", spec.CheckCmd, spec.CheckExpectedExitCode)
				return true, nil // isDone = true
			}
			hostCtxLogger.Debugf("CheckCmd '%s' failed with exit code %d (expected %d for 'done' state). Main command will run. Stderr: %s", spec.CheckCmd, cmdErr.ExitCode, spec.CheckExpectedExitCode, string(checkCmdStderr))
			return false, nil
		}
		hostCtxLogger.Errorf("Failed to execute CheckCmd '%s': %v. Stderr: %s", spec.CheckCmd, runErr, string(checkCmdStderr))
		return false, fmt.Errorf("check command '%s' execution failed: %w", spec.CheckCmd, runErr)
	}

	// CheckCmd executed successfully (implicit exit code 0)
	if spec.CheckExpectedExitCode == 0 {
		hostCtxLogger.Debugf("CheckCmd '%s' completed successfully (exit 0). Main command will be skipped.", spec.CheckCmd)
		return true, nil // isDone = true
	}

	hostCtxLogger.Debugf("CheckCmd '%s' completed with exit code 0, but expected %d for 'done' state. Main command will run.", spec.CheckCmd, spec.CheckExpectedExitCode)
	return false, nil
}

// Execute runs the command defined in the CommandStepSpec.
func (e *CommandStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	spec, ok := s.(*CommandStepSpec)
	if !ok {
		err := fmt.Errorf("Execute: unexpected spec type %T for CommandStepExecutor", s)
		// Attempt to get a name for the result, even if spec is wrong type.
		specName := "UnknownStep (type error)"
		if s != nil { // s might be nil if called incorrectly, though unlikely with registry
			specName = s.GetName()
		}
		return step.NewResult(specName, ctx.Host.Name, time.Now(), err)
	}

	startTime := time.Now()
	res := step.NewResult(spec.GetName(), ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", spec.GetName(), "phase", "Execute").Sugar()


	opts := &connector.ExecOptions{
		Sudo:    spec.Sudo,
		Timeout: spec.Timeout,
		Env:     spec.Env,
	}

	hostCtxLogger.Infof("Running command: %s (Sudo: %v)", spec.Cmd, spec.Sudo)
	stdout, stderr, runErr := ctx.Host.Runner.RunWithOptions(ctx.GoContext, spec.Cmd, opts)

	res.Stdout = string(stdout)
	res.Stderr = string(stderr)
	res.EndTime = time.Now()

	if runErr != nil {
		var cmdErr *connector.CommandError
		if errors.As(runErr, &cmdErr) {
			res.Error = cmdErr // Store the original CommandError
			if spec.IgnoreError {
				hostCtxLogger.Warnf("Command '%s' exited with code %d (stderr: %s), but error is ignored. Step considered successful.", spec.Cmd, cmdErr.ExitCode, string(stderr))
				res.Status = "Succeeded"
				// res.Error is kept for information, but status indicates success.
				// If truly ignoring means "no error recorded", then set res.Error = nil here.
				// For now, let's keep res.Error but override status.
				res.Message = fmt.Sprintf("Command executed with exit code %d and error '%v', but it was ignored. Original Stderr: %s", cmdErr.ExitCode, cmdErr, string(stderr))
			} else if cmdErr.ExitCode == spec.ExpectedExitCode {
				hostCtxLogger.Infof("Command '%s' completed with expected exit code %d.", spec.Cmd, spec.ExpectedExitCode)
				res.Status = "Succeeded"
				res.Error = nil // Not an application-level error for this step as exit code matched.
			} else {
				hostCtxLogger.Errorf("Command '%s' failed with exit code %d (expected %d). Stderr: %s", spec.Cmd, cmdErr.ExitCode, spec.ExpectedExitCode, string(stderr))
				res.Status = "Failed"
				// res.Error is already cmdErr
			}
		} else { // Not a CommandError, so a more fundamental execution failure
			hostCtxLogger.Errorf("Failed to execute command '%s': %v. Stderr: %s", spec.Cmd, runErr, string(stderr))
			res.Error = runErr
			res.Status = "Failed"
		}
	} else { // runErr is nil (command executed successfully with exit code 0)
		if spec.ExpectedExitCode == 0 {
			hostCtxLogger.Successf("Command '%s' completed successfully.", spec.Cmd)
			res.Status = "Succeeded"
			// res.Error is already nil
		} else {
			// Command succeeded with 0, but a different exit code was expected.
			errMsg := fmt.Sprintf("command '%s' exited 0, but expected exit code %d", spec.Cmd, spec.ExpectedExitCode)
			hostCtxLogger.Errorf("%s. Marked as failed.", errMsg)
			res.Error = errors.New(errMsg)
			res.Status = "Failed"
		}
	}
	return res
}

// Ensure CommandStepExecutor implements step.StepExecutor interface
var _ step.StepExecutor = &CommandStepExecutor{}

// Builder functions for CommandStepSpec for convenience.
// These allow fluent construction of the spec.

// NewCommandSpec creates a new CommandStepSpec.
func NewCommandSpec(command string, sudo bool) *CommandStepSpec {
	return &CommandStepSpec{Cmd: command, Sudo: sudo}
}

// WithName sets a custom name for the step spec.
func (s *CommandStepSpec) WithName(name string) *CommandStepSpec {
	s.SpecName = name
	return s
}

// WithIgnoreError sets the IgnoreError flag for the spec.
func (s *CommandStepSpec) WithIgnoreError(ignore bool) *CommandStepSpec {
	s.IgnoreError = ignore
	return s
}

// WithTimeout sets a timeout for the command in the spec.
func (s *CommandStepSpec) WithTimeout(timeout time.Duration) *CommandStepSpec {
	s.Timeout = timeout
	return s
}

// WithEnv sets environment variables for the command in the spec.
func (s *CommandStepSpec) WithEnv(env []string) *CommandStepSpec {
	s.Env = env
	return s
}

// WithExpectedExitCode sets the expected exit code for success in the spec.
func (s *CommandStepSpec) WithExpectedExitCode(code int) *CommandStepSpec {
	s.ExpectedExitCode = code
	return s
}

// WithCheckCmd defines an idempotency check command for the spec.
func (s *CommandStepSpec) WithCheckCmd(checkCmd string, checkSudo bool, checkExpectedExitCode ...int) *CommandStepSpec {
	s.CheckCmd = checkCmd
	s.CheckSudo = checkSudo
	if len(checkExpectedExitCode) > 0 {
		s.CheckExpectedExitCode = checkExpectedExitCode[0]
	} else {
		s.CheckExpectedExitCode = 0
	}
	return s
}
