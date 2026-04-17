package common

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

// CopyBinaryStep copies a binary to a remote path.
type CopyBinaryStep struct {
	step.Base
	SourcePath string
	TargetPath string
	Mode       string
}

type CopyBinaryStepBuilder struct {
	step.Builder[CopyBinaryStepBuilder, *CopyBinaryStep]
}

func NewCopyBinaryStepBuilder(ctx runtime.ExecutionContext, instanceName, sourcePath, targetPath, mode string) *CopyBinaryStepBuilder {
	s := &CopyBinaryStep{
		SourcePath: sourcePath,
		TargetPath: targetPath,
		Mode:       mode,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Copy binary from %s to %s", instanceName, sourcePath, targetPath)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute
	return new(CopyBinaryStepBuilder).Init(s)
}

func (s *CopyBinaryStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CopyBinaryStep) Precheck(ctx runtime.ExecutionContext) (bool, error) {
	return false, nil
}

func (s *CopyBinaryStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get connector")
		return result, err
	}

	logger.Infof("Copying binary from %s to %s:%s", s.SourcePath, ctx.GetHost().GetName(), s.TargetPath)
	if err := runner.CopyFile(ctx.GoContext(), conn, s.SourcePath, s.TargetPath, false, s.Base.Sudo); err != nil {
		result.MarkFailed(err, "failed to copy binary")
		return result, err
	}

	logger.Infof("Binary copied successfully to %s", s.TargetPath)
	result.MarkCompleted("Binary copied")
	return result, nil
}

func (s *CopyBinaryStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName())
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing %s", s.TargetPath)
	runner.Remove(ctx.GoContext(), conn, s.TargetPath, true, false)
	return nil
}

var _ step.Step = (*CopyBinaryStep)(nil)
