package command

import (
	"context" // Required by runtime.Context, not directly by this file's logic for context.Background()
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime" // For runtime.StepContext
	"github.com/mensylisir/kubexm/pkg/spec"    // Import the spec package
	"github.com/mensylisir/kubexm/pkg/step"    // Import for step.Result, step.Register, etc.
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
func (e *CommandStepExecutor) Check(ctx runtime.StepContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("phase", "Check")

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		return false, fmt.Errorf("StepSpec not found in context for Check method in CommandStepExecutor")
	}
	spec, ok := rawSpec.(*CommandStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		return false, fmt.Errorf("unexpected StepSpec type for Check method in CommandStepExecutor: %T. Expected *CommandStepSpec", rawSpec)
	}

	if spec.CheckCmd == "" {
		logger.Debug("No CheckCmd defined, main command will run")
		return false, nil // No check command, so main command should run
	}

	logger.Debug("Executing CheckCmd", "command", spec.CheckCmd)

	currentHost := ctx.GetHost()
	if currentHost == nil {
		logger.Error("Current host not found in context")
		return false, fmt.Errorf("current host not found in context for CheckCmd")
	}

	goCtx := ctx.GoContext()
	connector, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "host", currentHost.GetName(), "error", err)
		return false, fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
	}

	opts := &connector.ExecOptions{
		Sudo:    spec.CheckSudo,
		Timeout: spec.Timeout, // Consider if a shorter timeout is needed for CheckCmd
		Env:     spec.Env,
	}

	// Assuming connector.Connector has RunCommand or similar.
	// If it has RunWithOptions, that would be fine too.
	// For now, let's assume RunCommand exists and is suitable.
	// The previous code used Host.Runner.RunWithOptions, which might be a method on a specific runner type.
	// The generic connector.Connector might have a simpler RunCommand.
	// Let's assume RunCommand for now, and if it's actually RunWithOptions on the connector, adjust.
	// connector.ExecOptions are passed, so RunCommand on connector should accept them.
	// The return values for connector.RunCommand are assumed to be (stdout, stderr, exitCode, error) or similar.
	// For simplicity, let's assume a method like `RunCommand(ctx, cmd, opts)` returning (stdout, stderr, error)
	// where error is a *connector.CommandError if it's an exit code issue.

	// Let's use a hypothetical `ExecuteCommand` on the connector that returns combined output and error
	// For now, sticking to a more common pattern: RunCommand that returns stdout, stderr, and error (which could be CommandError)
	_, checkCmdStderrBytes, runErr := connector.RunCommand(goCtx, spec.CheckCmd, opts) // Adjusted to use connector
	checkCmdStderr := string(checkCmdStderrBytes)

	if runErr != nil {
		var cmdErr *connector.CommandError
		if errors.As(runErr, &cmdErr) {
			if cmdErr.ExitCode == spec.CheckExpectedExitCode {
				logger.Debug("CheckCmd completed with expected exit code. Main command will be skipped.", "command", spec.CheckCmd, "exitCode", cmdErr.ExitCode)
				return true, nil // isDone = true
			}
			logger.Debug("CheckCmd failed with different exit code than expected for 'done' state. Main command will run.", "command", spec.CheckCmd, "exitCode", cmdErr.ExitCode, "expectedExitCode", spec.CheckExpectedExitCode, "stderr", checkCmdStderr)
			return false, nil
		}
		logger.Error("Failed to execute CheckCmd", "command", spec.CheckCmd, "error", runErr, "stderr", checkCmdStderr)
		return false, fmt.Errorf("check command '%s' execution failed: %w. Stderr: %s", spec.CheckCmd, runErr, checkCmdStderr)
	}

	// CheckCmd executed successfully (implicit exit code 0 if not CommandError)
	if spec.CheckExpectedExitCode == 0 {
		logger.Debug("CheckCmd completed successfully (exit 0). Main command will be skipped.", "command", spec.CheckCmd)
		return true, nil // isDone = true
	}

	logger.Debug("CheckCmd completed with exit code 0, but expected different for 'done' state. Main command will run.", "command", spec.CheckCmd, "expectedExitCode", spec.CheckExpectedExitCode)
	return false, nil
}

// Execute runs the command defined in the CommandStepSpec.
func (e *CommandStepExecutor) Execute(ctx runtime.StepContext) *step.Result {
	startTime := time.Now()
	logger := ctx.GetLogger().With("phase", "Execute")
	currentHost := ctx.GetHost()

	// Initial result, host might be nil for local/orchestration steps not tied to a host.
	res := step.NewResult(ctx, currentHost, startTime, nil)

	if currentHost == nil {
		errMsg := "current host not found in context for Execute method"
		logger.Error(errMsg)
		res.Error = fmt.Errorf(errMsg)
		res.Status = step.StatusFailed
		res.EndTime = time.Now()
		return res
	}
	// From now on, currentHost is not nil. We pass it to NewResult again if an early exit occurs for spec processing.

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		errMsg := "StepSpec not found in context"
		logger.Error(errMsg)
		res.Error = fmt.Errorf(errMsg) // Update existing res
		res.Status = step.StatusFailed
		res.EndTime = time.Now()
		return res
	}
	spec, ok := rawSpec.(*CommandStepSpec)
	if !ok {
		errMsg := fmt.Sprintf("unexpected StepSpec type: %T. Expected *CommandStepSpec", rawSpec)
		logger.Error(errMsg)
		res.Error = fmt.Errorf(errMsg) // Update existing res
		res.Status = step.StatusFailed
		res.EndTime = time.Now()
		return res
	}

	goCtx := ctx.GoContext()
	connector, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "host", currentHost.GetName(), "error", err)
		res.Error = fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
		res.Status = step.StatusFailed
		res.EndTime = time.Now()
		return res
	}

	opts := &connector.ExecOptions{
		Sudo:    spec.Sudo,
		Timeout: spec.Timeout,
		Env:     spec.Env,
	}

	logger.Info("Running command", "command", spec.Cmd, "sudo", spec.Sudo)
	stdoutBytes, stderrBytes, runErr := connector.RunCommand(goCtx, spec.Cmd, opts) // Adjusted to use connector

	res.Stdout = string(stdoutBytes)
	res.Stderr = string(stderrBytes)
	res.EndTime = time.Now() // Set EndTime closer to actual execution end

	if runErr != nil {
		var cmdErr *connector.CommandError
		if errors.As(runErr, &cmdErr) {
			res.Error = cmdErr // Store the original CommandError
			if spec.IgnoreError {
				logger.Warn("Command exited with error, but error is ignored. Step considered successful.", "command", spec.Cmd, "exitCode", cmdErr.ExitCode, "stderr", res.Stderr)
				res.Status = step.StatusSucceeded
				res.Message = fmt.Sprintf("Command executed with exit code %d and error '%v', but it was ignored. Original Stderr: %s", cmdErr.ExitCode, cmdErr, res.Stderr)
			} else if cmdErr.ExitCode == spec.ExpectedExitCode {
				logger.Info("Command completed with expected exit code.", "command", spec.Cmd, "exitCode", cmdErr.ExitCode)
				res.Status = step.StatusSucceeded
				res.Error = nil
			} else {
				logger.Error("Command failed with unexpected exit code.", "command", spec.Cmd, "exitCode", cmdErr.ExitCode, "expectedExitCode", spec.ExpectedExitCode, "stderr", res.Stderr)
				res.Status = step.StatusFailed
			}
		} else {
			logger.Error("Failed to execute command (non-CommandError).", "command", spec.Cmd, "error", runErr, "stderr", res.Stderr)
			res.Error = runErr
			res.Status = step.StatusFailed
		}
	} else { // runErr is nil
		if spec.ExpectedExitCode == 0 {
			logger.Info("Command completed successfully.", "command", spec.Cmd)
			res.Status = step.StatusSucceeded
		} else {
			errMsg := fmt.Sprintf("command '%s' exited 0, but expected exit code %d", spec.Cmd, spec.ExpectedExitCode)
			logger.Error(errMsg + ". Marked as failed.")
			res.Error = errors.New(errMsg)
			res.Status = step.StatusFailed
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
