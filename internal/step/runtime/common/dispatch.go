package common

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// DispatchBinaryStep dispatches a binary file to remote host.
type DispatchBinaryStep struct {
	step.Base
	SourcePath string
	TargetPath string
	Mode       string
}

type DispatchBinaryStepBuilder struct {
	step.Builder[DispatchBinaryStepBuilder, *DispatchBinaryStep]
}

func NewDispatchBinaryStepBuilder(ctx runtime.ExecutionContext, instanceName, sourcePath, targetPath, mode string) *DispatchBinaryStepBuilder {
	s := &DispatchBinaryStep{
		SourcePath: sourcePath,
		TargetPath: targetPath,
		Mode:       mode,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Dispatch binary %s to %s", instanceName, sourcePath, targetPath)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute
	return new(DispatchBinaryStepBuilder).Init(s)
}

func (s *DispatchBinaryStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DispatchBinaryStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.TargetPath)
	if err != nil {
		return false, err
	}

	if exists {
		return true, nil
	}

	return false, nil
}

func (s *DispatchBinaryStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	targetDir := filepath.Dir(s.TargetPath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, targetDir, "0755", true); err != nil {
		result.MarkFailed(err, "failed to create target directory")
		return result, err
	}

	logger.Infof("Copying binary from %s to %s:%s", s.SourcePath, ctx.GetHost().GetName(), s.TargetPath)
	if err := runner.CopyFile(ctx.GoContext(), conn, s.SourcePath, s.TargetPath, false, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to copy binary")
		return result, err
	}

	logger.Infof("Binary dispatched successfully to %s", s.TargetPath)
	result.MarkCompleted(fmt.Sprintf("Binary dispatched to %s", s.TargetPath))
	return result, nil
}

func (s *DispatchBinaryStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing %s", s.TargetPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.TargetPath, true, false); err != nil {
		logger.Errorf("Failed to remove %s during rollback: %v", s.TargetPath, err)
	}
	return nil
}

var _ step.Step = (*DispatchBinaryStep)(nil)
