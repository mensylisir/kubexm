package etcd

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type EtcdCleanupBackupsStep struct {
	step.Base
}

type EtcdCleanupBackupsStepBuilder struct {
	step.Builder[EtcdCleanupBackupsStepBuilder, *EtcdCleanupBackupsStep]
}

func NewEtcdCleanupBackupsStepBuilder(ctx runtime.Context, instanceName string) *EtcdCleanupBackupsStepBuilder {
	s := &EtcdCleanupBackupsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Clean up temporary, node-local etcd data backups"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(EtcdCleanupBackupsStepBuilder).Init(s)
	return b
}

func (s *EtcdCleanupBackupsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EtcdCleanupBackupsStep) getCacheKey(ctx runtime.ExecutionContext) string {
	return fmt.Sprintf(CacheKeyEtcdLocalBackupPath, ctx.GetHost().GetName())
}

func (s *EtcdCleanupBackupsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *EtcdCleanupBackupsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	cacheKey := s.getCacheKey(ctx)
	backupPath, ok := ctx.GetTaskCache().Get(cacheKey)
	if !ok {
		logger.Info("No local backup path found in cache for this node. Nothing to clean up.")
		return nil
	}

	backupDir, ok := backupPath.(string)
	if !ok || backupDir == "" {
		logger.Warnf("Invalid local backup path found in cache for key '%s'. Skipping cleanup.", cacheKey)
		return nil
	}

	logger.Warnf("Cleaning up temporary etcd data backup directory on remote node: '%s'", backupDir)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for host, cannot perform cleanup. Error: %v", err)
		return nil
	}

	cleanupCmd := fmt.Sprintf("rm -rf %s", backupDir)
	if _, err := runner.Run(ctx.GoContext(), conn, cleanupCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to remove remote backup directory '%s'. Manual cleanup may be required. Error: %v", backupDir, err)
		return nil
	}

	ctx.GetTaskCache().Delete(cacheKey)

	logger.Info("Successfully cleaned up temporary etcd data backup.")
	return nil
}

func (s *EtcdCleanupBackupsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a cleanup step.")
	return nil
}

var _ step.Step = (*EtcdCleanupBackupsStep)(nil)
