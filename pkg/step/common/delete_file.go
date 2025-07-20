package common

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"time"

	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type DeleteFileStep struct {
	step.Base
	RemotePath string
	Recursive  bool
}

type DeleteFileStepBuilder struct {
	step.Builder[DeleteFileStepBuilder, *DeleteFileStep]
}

func NewDeleteFileStepBuilder(instanceName, remotePath string) *DeleteFileStepBuilder {
	cs := &DeleteFileStep{
		RemotePath: remotePath,
	}
	cs.Base.Meta.Name = instanceName
	cs.Base.Meta.Description = fmt.Sprintf("[%s]>>Remove [%s]", instanceName, remotePath)
	cs.Base.Sudo = false
	cs.Base.IgnoreError = false
	cs.Base.Timeout = 30 * time.Second
	return new(DeleteFileStepBuilder).Init(cs)
}

func (b *DeleteFileStepBuilder) WithRecursive(recursive bool) *DeleteFileStepBuilder {
	b.Step.Recursive = recursive
	return b
}

func (s *DeleteFileStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *DeleteFileStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.RemotePath)
	if err != nil {
		logger.Warn("Failed to check existence of remote path, assuming it might exist.", "path", s.RemotePath, "error", err)
		return false, nil
	}

	if !exists {
		logger.Info("Remote path does not exist. Step considered done.", "path", s.RemotePath)
		return true, nil
	}
	logger.Info("Remote path exists and needs removal.", "path", s.RemotePath)
	return false, nil
}

func (s *DeleteFileStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", ctx.GetHost().GetName(), err)
	}

	logger.Info("Removing remote path.", "path", s.RemotePath, "recursive", s.Recursive)
	if s.Recursive {
		rmCmd := fmt.Sprintf("rm -rf %s", s.RemotePath)
		_, errRm := runnerSvc.Run(ctx.GoContext(), conn, rmCmd, s.Sudo)
		if errRm != nil {
			return fmt.Errorf("failed to recursively remove %s: %w", s.RemotePath, errRm)
		}
	} else {
		if err := runnerSvc.Remove(ctx.GoContext(), conn, s.RemotePath, s.Sudo, s.Recursive); err != nil {
			return fmt.Errorf("failed to remove %s: %w", s.RemotePath, err)
		}
	}

	logger.Info("Remote path removed successfully.", "path", s.RemotePath)
	return nil
}

func (s *DeleteFileStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback for DeleteFileStep is not applicable (would mean restoring the file/directory).")
	return nil
}

var _ step.Step = (*DeleteFileStep)(nil)
