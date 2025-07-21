package command

import (
	"errors"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"time"
)

type CommandStep struct {
	step.Base
	Cmd                   string
	Env                   []string
	ExpectedExitCode      int
	CheckCmd              string
	CheckSudo             bool
	CheckExpectedExitCode int
	RollbackCmd           string
	RollbackSudo          bool
}

type CommandStepBuilder struct {
	step.Builder[CommandStepBuilder, *CommandStep]
}

func NewCommandStepBuilder(ctx runtime.ExecutionContext, instanceName, cmd string) *CommandStepBuilder {
	cs := &CommandStep{
		Cmd:              cmd,
		ExpectedExitCode: 0,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Execute [%s]", instanceName, cmd)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 30 * time.Second
	return new(CommandStepBuilder).Init(cs)
}

func (b *CommandStepBuilder) WithEnv(env []string) *CommandStepBuilder {
	b.Step.Env = env
	return b
}

func (b *CommandStepBuilder) WithCheck(cmd string, sudo bool, expectedExitCode int) *CommandStepBuilder {
	b.Step.CheckCmd = cmd
	b.Step.CheckSudo = sudo
	b.Step.CheckExpectedExitCode = expectedExitCode
	return b
}

func (b *CommandStepBuilder) WithRollback(cmd string, sudo bool) *CommandStepBuilder {
	b.Step.RollbackCmd = cmd
	b.Step.RollbackSudo = sudo
	return b
}

func (s *CommandStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CommandStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	if s.CheckCmd == "" {
		logger.Debug("No CheckCmd defined, main command will run")
		return false, nil
	}

	logger.Debug("Executing CheckCmd", "command", s.CheckCmd)

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", ctx.GetHost().GetName(), s.Meta().Name, err)
	}

	opts := &connector.ExecOptions{
		Sudo:    s.CheckSudo,
		Timeout: s.Timeout,
		Env:     s.Env,
	}
	_, stderrBytes, runErr := conn.Exec(ctx.GoContext(), s.CheckCmd, opts)
	checkCmdStderr := string(stderrBytes)
	if runErr != nil {
		var cmdErr *connector.CommandError
		if errors.As(runErr, &cmdErr) {
			if cmdErr.ExitCode == s.CheckExpectedExitCode {
				logger.Debug("CheckCmd completed with expected exit code. Main command will be skipped.", "exitCode", cmdErr.ExitCode)
				return true, nil
			}
			logger.Debug("CheckCmd failed with different exit code than expected for 'done' state. Main command will run.", "exitCode", cmdErr.ExitCode, "expected", s.CheckExpectedExitCode, "stderr", checkCmdStderr)
			return false, nil
		}
		logger.Error("Failed to execute CheckCmd", "error", runErr, "stderr", checkCmdStderr)
		return false, fmt.Errorf("check command '%s' execution failed for step %s on host %s: %w. Stderr: %s", s.CheckCmd, s.Meta().Name, ctx.GetHost().GetName(), runErr, checkCmdStderr)
	}

	if s.CheckExpectedExitCode == 0 {
		logger.Debug("CheckCmd completed successfully (exit 0). Main command will be skipped.")
		return true, nil
	}

	logger.Debug("CheckCmd completed with exit code 0, but expected different for 'done' state. Main command will run.", "expected", s.CheckExpectedExitCode)
	return false, nil
}

func (s *CommandStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", ctx.GetHost().GetName(), s.Meta().Name, err)
	}

	opts := &connector.ExecOptions{
		Sudo:    s.Sudo,
		Timeout: s.Timeout,
		Env:     s.Env,
	}

	logger.Info("Running command", "command", s.Cmd, "sudo", s.Sudo)
	stdoutBytes, stderrBytes, runErr := conn.Exec(ctx.GoContext(), s.Cmd, opts)
	if runErr != nil {
		var cmdErr *connector.CommandError
		if errors.As(runErr, &cmdErr) {
			if s.IgnoreError {
				logger.Warn("Command exited with error, but error is ignored.", "command", s.Cmd, "exitCode", cmdErr.ExitCode, "stdout", string(stdoutBytes), "stderr", string(stderrBytes))
				return nil
			}
			if cmdErr.ExitCode == s.ExpectedExitCode {
				logger.Info("Command completed with expected (non-zero) exit code.", "exitCode", cmdErr.ExitCode, "stdout", string(stdoutBytes), "stderr", string(stderrBytes))
				return nil
			}
			logger.Error("Command failed with unexpected exit code.", "exitCode", cmdErr.ExitCode, "expected", s.ExpectedExitCode, "stdout", string(stdoutBytes), "stderr", string(stderrBytes))
			return cmdErr
		}
		logger.Error("Failed to execute command (non-CommandError).", "error", runErr, "stdout", string(stdoutBytes), "stderr", string(stderrBytes))
		return fmt.Errorf("command '%s' failed for step %s on host %s (non-CommandError): %w. Stdout: %s, Stderr: %s", s.Cmd, s.Meta().Name, ctx.GetHost().GetName(), runErr, string(stdoutBytes), string(stderrBytes))
	}

	if s.ExpectedExitCode == 0 {
		logger.Info("Command completed successfully (exit 0).", "stdout", string(stdoutBytes), "stderr", string(stderrBytes))
		return nil
	}

	errMsg := fmt.Sprintf("command '%s' exited 0 (stdout: %s, stderr: %s), but expected exit code %d for step %s on host %s", s.Cmd, string(stdoutBytes), string(stderrBytes), s.ExpectedExitCode, s.Meta().Name, ctx.GetHost().GetName())
	logger.Error(errMsg)
	return &connector.CommandError{
		Cmd:        s.Cmd,
		Underlying: errors.New(errMsg),
		ExitCode:   0,
		Stdout:     string(stdoutBytes),
		Stderr:     string(stderrBytes),
	}
}

func (s *CommandStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	if s.RollbackCmd == "" {
		logger.Debug("No RollbackCmd defined for this command step.")
		return nil
	}

	logger.Info("Attempting to run rollback command", "command", s.RollbackCmd, "sudo", s.RollbackSudo)

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error("Failed to get connector for host during rollback", "error", err)
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", ctx.GetHost().GetName(), s.Meta().Name, err)
	}

	opts := &connector.ExecOptions{
		Sudo:    s.RollbackSudo,
		Timeout: s.Timeout,
		Env:     s.Env,
	}

	stdoutBytes, stderrBytes, runErr := conn.Exec(ctx.GoContext(), s.RollbackCmd, opts)

	if runErr != nil {
		logger.Error("Rollback command failed.", "command", s.RollbackCmd, "error", runErr, "stdout", string(stdoutBytes), "stderr", string(stderrBytes))
		var cmdErr *connector.CommandError
		if errors.As(runErr, &cmdErr) {
			return fmt.Errorf("rollback command '%s' failed for step %s on host %s: %w", s.RollbackCmd, s.Meta().Name, ctx.GetHost().GetName(), cmdErr)
		}
		return fmt.Errorf("rollback command '%s' failed for step %s on host %s: %w. Stdout: %s, Stderr: %s", s.RollbackCmd, s.Meta().Name, ctx.GetHost().GetName(), runErr, string(stdoutBytes), string(stderrBytes))
	}

	logger.Info("Rollback command executed successfully.", "command", s.RollbackCmd, "stdout", string(stdoutBytes), "stderr", string(stderrBytes))
	return nil
}

var _ step.Step = (*CommandStep)(nil)
