package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// RunCommandStep runs an arbitrary command on the remote host.
type RunCommandStep struct {
	step.Base
	Command   string
	OutputKey string // Context key to store command result
}

type RunCommandStepBuilder struct {
	step.Builder[RunCommandStepBuilder, *RunCommandStep]
}

func NewRunCommandStepBuilder(ctx runtime.ExecutionContext, instanceName, command string) *RunCommandStepBuilder {
	s := &RunCommandStep{
		Command: command,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Run command: %s", instanceName, command)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute
	return new(RunCommandStepBuilder).Init(s)
}

func (b *RunCommandStepBuilder) WithSudo(sudo bool) *RunCommandStepBuilder {
	b.Step.Base.Sudo = sudo
	return b
}

func (b *RunCommandStepBuilder) WithIgnoreError(ignore bool) *RunCommandStepBuilder {
	b.Step.Base.IgnoreError = ignore
	return b
}

func (b *RunCommandStepBuilder) WithTimeout(timeout time.Duration) *RunCommandStepBuilder {
	b.Step.Base.Timeout = timeout
	return b
}

func (b *RunCommandStepBuilder) WithOutputKey(key string) *RunCommandStepBuilder {
	b.Step.OutputKey = key
	return b
}

func (s *RunCommandStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RunCommandStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *RunCommandStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	logger.Infof("Running: %s", s.Command)
	runResult, err := runner.Run(ctx.GoContext(), conn, s.Command, s.Base.Sudo)
	if err != nil {
		if !s.Base.IgnoreError {
			result.MarkFailed(err, fmt.Sprintf("command failed: %s", s.Command))
			return result, err
		}
		logger.Warnf("Command failed (ignored): %v", err)
	}

	if s.OutputKey != "" {
		ctx.Export("task", s.OutputKey, runResult.Stdout)
	}

	logger.Infof("Command completed successfully")
	result.MarkCompleted("Command executed")
	return result, nil
}

func (s *RunCommandStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*RunCommandStep)(nil)
