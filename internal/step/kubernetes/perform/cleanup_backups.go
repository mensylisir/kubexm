package perform

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/types"
)

type CleanupBackupsStep struct {
	step.Base
}

type CleanupBackupsStepBuilder struct {
	step.Builder[CleanupBackupsStepBuilder, *CleanupBackupsStep]
}

func NewCleanupBackupsStepBuilder(ctx runtime.ExecutionContext, instanceName string) *CleanupBackupsStepBuilder {
	s := &CleanupBackupsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Clean up temporary, node-local config backups"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(CleanupBackupsStepBuilder).Init(s)
	return b
}

func (s *CleanupBackupsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanupBackupsStep) getCacheKey(ctx runtime.ExecutionContext) string {
	return fmt.Sprintf(common.CacheKeyRemoteBackupPath, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName(), ctx.GetHost().GetName())
}

func (s *CleanupBackupsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *CleanupBackupsStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	cacheKey := s.getCacheKey(ctx)
	backupPath, ok := ctx.GetTaskCache().Get(cacheKey)
	if !ok {
		logger.Info("No remote backup path found in cache for this node. Nothing to clean up.")
		result.MarkCompleted("nothing to clean up")
		return result, nil
	}

	backupDir, ok := backupPath.(string)
	if !ok || backupDir == "" {
		logger.Warnf("Invalid remote backup path in cache. Skipping cleanup.")
		result.MarkCompleted("invalid backup path, skipping")
		return result, nil
	}

	logger.Warnf("Cleaning up temporary config backup directory on remote node: '%s'", backupDir)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector, cannot perform cleanup: %v", err)
		result.MarkCompleted("failed to get connector, skipping")
		return result, nil
	}

	cleanupCmd := fmt.Sprintf("rm -rf %s", backupDir)
	if _, err := runner.Run(ctx.GoContext(), conn, cleanupCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to remove remote backup directory '%s'. Manual cleanup may be required. Error: %v", backupDir, err)
	}

	ctx.GetTaskCache().Delete(cacheKey)
	logger.Info("Successfully cleaned up temporary config backup.")
	result.MarkCompleted("cleanup completed")
	return result, nil
}

func (s *CleanupBackupsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a cleanup step.")
	return nil
}

var _ step.Step = (*CleanupBackupsStep)(nil)
