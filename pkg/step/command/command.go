package command

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for spec.StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
)

// CommandStep executes a shell command on a target host.
type CommandStep struct {
	meta                spec.StepMeta // Contains Name and Description
	Cmd                 string
	Sudo                bool
	IgnoreError         bool
	Timeout             time.Duration
	Env                 []string
	ExpectedExitCode    int
	CheckCmd            string
	CheckSudo           bool
	CheckExpectedExitCode int
	RollbackCmd         string // Optional: command to run for rollback
	RollbackSudo        bool   // Optional: sudo for rollback command
}

// NewCommandStep creates a new CommandStep.
// Parameters for checkCmd, checkSudo, checkExpectedExitCode, rollbackCmd, rollbackSudo are optional and can be empty/zero.
func NewCommandStep(
	instanceName string, // Used for StepMeta.Name
	cmd string,
	sudo bool,
	ignoreError bool,
	timeout time.Duration,
	env []string,
	expectedExitCode int,
	checkCmd string,
	checkSudo bool,
	checkExpectedExitCode int,
	rollbackCmd string,
	rollbackSudo bool,
) step.Step {
	metaName := instanceName
	if metaName == "" {
		// Generate a default name if instanceName is not provided.
		if len(cmd) > 30 {
			metaName = "Exec: " + cmd[:30] + "..."
		} else {
			metaName = "Exec: " + cmd
		}
	}
	return &CommandStep{
		meta: spec.StepMeta{
			Name:        metaName,
			Description: fmt.Sprintf("Executes command: '%s'", cmd),
		},
		Cmd:                 cmd,
		Sudo:                sudo,
		IgnoreError:         ignoreError,
		Timeout:             timeout,
		Env:                 env,
		ExpectedExitCode:    expectedExitCode,
		CheckCmd:            checkCmd,
		CheckSudo:           checkSudo,
		CheckExpectedExitCode: checkExpectedExitCode,
		RollbackCmd:         rollbackCmd,
		RollbackSudo:        rollbackSudo,
	}
}

// Meta returns the step's metadata.
func (s *CommandStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *CommandStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", host.GetName(), "phase", "Precheck")

	if s.CheckCmd == "" {
		logger.Debug("No CheckCmd defined, main command will run")
		return false, nil
	}

	logger.Debug("Executing CheckCmd", "command", s.CheckCmd)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.Meta().Name, err)
	}

	opts := &connector.ExecOptions{
		Sudo:    s.CheckSudo,
		Timeout: s.Timeout, // Consider if a shorter/different timeout is needed for CheckCmd
		Env:     s.Env,
	}

	// Assuming conn.RunCommand exists and is the method to use.
	// The prompt uses conn.RunCommand, which is not standard on connector.Connector.
	// connector.Connector has Exec(). Let's adapt to use Exec().
	// Exec(ctx context.Context, cmd string, opts *ExecOptions) (stdout, stderr []byte, err error)
	_, stderrBytes, runErr := conn.Exec(ctx.GoContext(), s.CheckCmd, opts)
	checkCmdStderr := string(stderrBytes)

	if runErr != nil {
		var cmdErr *connector.CommandError // Assuming connector.CommandError exists and is used by Exec
		if errors.As(runErr, &cmdErr) {
			if cmdErr.ExitCode == s.CheckExpectedExitCode {
				logger.Debug("CheckCmd completed with expected exit code. Main command will be skipped.", "exitCode", cmdErr.ExitCode)
				return true, nil // isDone = true
			}
			logger.Debug("CheckCmd failed with different exit code than expected for 'done' state. Main command will run.", "exitCode", cmdErr.ExitCode, "expected", s.CheckExpectedExitCode, "stderr", checkCmdStderr)
			return false, nil
		}
		logger.Error("Failed to execute CheckCmd", "error", runErr, "stderr", checkCmdStderr)
		return false, fmt.Errorf("check command '%s' execution failed for step %s on host %s: %w. Stderr: %s", s.CheckCmd, s.Meta().Name, host.GetName(), runErr, checkCmdStderr)
	}

	// CheckCmd executed successfully (implicit exit code 0 if not CommandError from Exec)
	if s.CheckExpectedExitCode == 0 {
		logger.Debug("CheckCmd completed successfully (exit 0). Main command will be skipped.")
		return true, nil // isDone = true
	}

	logger.Debug("CheckCmd completed with exit code 0, but expected different for 'done' state. Main command will run.", "expected", s.CheckExpectedExitCode)
	return false, nil
}

func (s *CommandStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", host.GetName(), "phase", "Run")

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.Meta().Name, err)
	}

	opts := &connector.ExecOptions{
		Sudo:    s.Sudo,
		Timeout: s.Timeout,
		Env:     s.Env,
	}

	logger.Info("Running command", "command", s.Cmd, "sudo", s.Sudo)
	// Assuming conn.RunCommand is actually conn.Exec
	stdoutBytes, stderrBytes, runErr := conn.Exec(ctx.GoContext(), s.Cmd, opts)

	// Stashing stdout/stderr is optional as per comments.
	// Example:
	// ctx.StepCache().Set(s.Name()+"#stdout", string(stdoutBytes))
	// ctx.StepCache().Set(s.Name()+"#stderr", string(stderrBytes))


	if runErr != nil {
		var cmdErr *connector.CommandError // Assuming connector.CommandError
		if errors.As(runErr, &cmdErr) {
			// Populate CommandError with Stdout/Stderr if not already done by Exec
			// cmdErr.Stdout = string(stdoutBytes) // Assuming CommandError has these fields
			// cmdErr.Stderr = string(stderrBytes)

			if s.IgnoreError {
				logger.Warn("Command exited with error, but error is ignored.", "command", s.Cmd, "exitCode", cmdErr.ExitCode, "stdout", string(stdoutBytes), "stderr", string(stderrBytes))
				return nil
			}
			if cmdErr.ExitCode == s.ExpectedExitCode {
				logger.Info("Command completed with expected (non-zero) exit code.", "exitCode", cmdErr.ExitCode, "stdout", string(stdoutBytes), "stderr", string(stderrBytes))
				return nil
			}
			logger.Error("Command failed with unexpected exit code.", "exitCode", cmdErr.ExitCode, "expected", s.ExpectedExitCode, "stdout", string(stdoutBytes), "stderr", string(stderrBytes))
			return cmdErr // Return the CommandError
		}
		// Non-CommandError type
		logger.Error("Failed to execute command (non-CommandError).", "error", runErr, "stdout", string(stdoutBytes), "stderr", string(stderrBytes))
		return fmt.Errorf("command '%s' failed for step %s on host %s (non-CommandError): %w. Stdout: %s, Stderr: %s", s.Cmd, s.Meta().Name, host.GetName(), runErr, string(stdoutBytes), string(stderrBytes))
	}

	// runErr is nil (command exited 0)
	if s.ExpectedExitCode == 0 {
		logger.Info("Command completed successfully (exit 0).", "stdout", string(stdoutBytes), "stderr", string(stderrBytes))
		return nil // Success
	}

	// Command exited 0, but a different code was expected.
	errMsg := fmt.Sprintf("command '%s' exited 0 (stdout: %s, stderr: %s), but expected exit code %d for step %s on host %s", s.Cmd, string(stdoutBytes), string(stderrBytes), s.ExpectedExitCode, s.Meta().Name, host.GetName())
	logger.Error(errMsg)
	// Create a CommandError to carry the actual exit code (0) and output
	return &connector.CommandError{
		Cmd:      s.Cmd, // Populate Cmd field in CommandError
		Underlying: errors.New(errMsg), // Populate Underlying field
		ExitCode: 0, // Actual exit code
		Stdout:   string(stdoutBytes),
		Stderr:   string(stderrBytes),
	}
}

func (s *CommandStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", host.GetName(), "phase", "Rollback")

	if s.RollbackCmd == "" {
		logger.Debug("No RollbackCmd defined for this command step.")
		return nil
	}

	logger.Info("Attempting to run rollback command", "command", s.RollbackCmd, "sudo", s.RollbackSudo)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for host during rollback", "error", err)
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", host.GetName(), s.Meta().Name, err)
	}

	opts := &connector.ExecOptions{
		Sudo:    s.RollbackSudo,
		Timeout: s.Timeout,
		Env:     s.Env,
	}

	// Assuming conn.RunCommand is conn.Exec
	stdoutBytes, stderrBytes, runErr := conn.Exec(ctx.GoContext(), s.RollbackCmd, opts)

	if runErr != nil {
		logger.Error("Rollback command failed.", "command", s.RollbackCmd, "error", runErr, "stdout", string(stdoutBytes), "stderr", string(stderrBytes))
		// Return a CommandError if possible, or a generic error
		var cmdErr *connector.CommandError
		if errors.As(runErr, &cmdErr) {
			// Ensure CommandError is fully populated for consistent error reporting
			// cmdErr.Stdout = string(stdoutBytes)
			// cmdErr.Stderr = string(stderrBytes)
			// cmdErr.Cmd = s.RollbackCmd // Add command to the error
			return fmt.Errorf("rollback command '%s' failed for step %s on host %s: %w", s.RollbackCmd, s.Meta().Name, host.GetName(), cmdErr)
		}
		return fmt.Errorf("rollback command '%s' failed for step %s on host %s: %w. Stdout: %s, Stderr: %s", s.RollbackCmd, s.Meta().Name, host.GetName(), runErr, string(stdoutBytes), string(stderrBytes))
	}

	logger.Info("Rollback command executed successfully.", "command", s.RollbackCmd, "stdout", string(stdoutBytes), "stderr", string(stderrBytes))
	return nil
}

// Ensure CommandStep implements the step.Step interface.
var _ step.Step = (*CommandStep)(nil)
