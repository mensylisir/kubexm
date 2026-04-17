package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// RunCommandForResultStep runs a command and stores the result in context.
type RunCommandForResultStep struct {
	step.Base
	Command   string
	OutputKey string // Context key to store command result
}

type RunCommandForResultStepBuilder struct {
	step.Builder[RunCommandForResultStepBuilder, *RunCommandForResultStep]
}

func NewRunCommandForResultStepBuilder(ctx runtime.ExecutionContext, instanceName, command, outputKey string) *RunCommandForResultStepBuilder {
	s := &RunCommandForResultStep{
		Command:   command,
		OutputKey: outputKey,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Run command: %s", instanceName, command)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(RunCommandForResultStepBuilder).Init(s)
}

func (s *RunCommandForResultStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RunCommandForResultStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *RunCommandForResultStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
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

	logger.Infof("Command completed, result stored with key: %s", s.OutputKey)
	result.MarkCompleted("Command executed")
	return result, nil
}

func (s *RunCommandForResultStep) Rollback(ctx runtime.ExecutionContext) error {
	return nil
}

var _ step.Step = (*RunCommandForResultStep)(nil)
