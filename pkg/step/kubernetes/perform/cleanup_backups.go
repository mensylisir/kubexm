package perform

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CleanupBackupsStep struct {
	step.Base
}

type CleanupBackupsStepBuilder struct {
	step.Builder[CleanupBackupsStepBuilder, *CleanupBackupsStep]
}

func NewCleanupBackupsStepBuilder(ctx runtime.Context, instanceName string) *CleanupBackupsStepBuilder {
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
	return fmt.Sprintf(common.CacheKeyRemoteBackupPath, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName())
}

func (s *CleanupBackupsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *CleanupBackupsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	cacheKey := s.getCacheKey(ctx)
	backupPath, ok := ctx.GetTaskCache().Get(cacheKey)
	if !ok {
		logger.Info("No remote backup path found in cache for this node. Nothing to clean up.")
		return nil
	}

	backupDir, ok := backupPath.(string)
	if !ok || backupDir == "" {
		logger.Warnf("Invalid remote backup path in cache. Skipping cleanup.")
		return nil
	}

	logger.Warnf("Cleaning up temporary config backup directory on remote node: '%s'", backupDir)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector, cannot perform cleanup: %v", err)
		return nil
	}

	cleanupCmd := fmt.Sprintf("rm -rf %s", backupDir)
	if _, err := runner.Run(ctx.GoContext(), conn, cleanupCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to remove remote backup directory '%s'. Manual cleanup may be required. Error: %v", backupDir, err)
	}

	ctx.GetTaskCache().Delete(cacheKey)
	logger.Info("Successfully cleaned up temporary config backup.")
	return nil
}

func (s *CleanupBackupsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a cleanup step.")
	return nil
}

var _ step.Step = (*CleanupBackupsStep)(nil)
