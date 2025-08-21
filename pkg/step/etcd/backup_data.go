package etcd

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	remoteDataBackupSuffix = "etcd-data.bak"
)

type EtcdBackupLocalDataStep struct {
	step.Base
	remoteDataDir   string
	remoteBackupDir string
}

type EtcdBackupLocalDataStepBuilder struct {
	step.Builder[EtcdBackupLocalDataStepBuilder, *EtcdBackupLocalDataStep]
}

func NewEtcdBackupLocalDataStepBuilder(ctx runtime.Context, instanceName string) *EtcdBackupLocalDataStepBuilder {
	s := &EtcdBackupLocalDataStep{
		remoteDataDir: "/var/lib/etcd",
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Create a local backup of the etcd data directory on the node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(EtcdBackupLocalDataStepBuilder).Init(s)
	return b
}

func (s *EtcdBackupLocalDataStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *EtcdBackupLocalDataStep) getCacheKey(ctx runtime.ExecutionContext) string {
	return fmt.Sprintf(common.CacheKeyEtcdLocalBackupPath, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName())
}

func (s *EtcdBackupLocalDataStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for local etcd data backup...")

	cacheKey := s.getCacheKey(ctx)

	if _, ok := ctx.GetTaskCache().Get(cacheKey); ok {
		logger.Info("Local etcd data has already been backed up for this node. Step is done.")
		return true, nil
	}

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	checkCmd := fmt.Sprintf("[ -d %s ]", s.remoteDataDir)
	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, checkCmd, s.Sudo); err != nil {
		return false, fmt.Errorf("precheck failed: source etcd data directory '%s' not found on host '%s'", s.remoteDataDir, ctx.GetHost().GetName())
	}

	logger.Info("Precheck passed: Data directory exists and no backup has been made yet.")
	return false, nil
}

func (s *EtcdBackupLocalDataStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	s.remoteBackupDir = fmt.Sprintf("%s.%s-%d", s.remoteDataDir, remoteDataBackupSuffix, time.Now().Unix())
	cacheKey := s.getCacheKey(ctx)

	logger.Infof("Backing up remote etcd data directory from '%s' to '%s'...", s.remoteDataDir, s.remoteBackupDir)

	backupCmd := fmt.Sprintf("cp -a %s %s", s.remoteDataDir, s.remoteBackupDir)
	if _, err := runner.Run(ctx.GoContext(), conn, backupCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to back up remote etcd data directory on host '%s': %w", ctx.GetHost().GetName(), err)
	}

	ctx.GetTaskCache().Set(cacheKey, s.remoteBackupDir)
	logger.Infof("Successfully backed up etcd data directory. Backup path '%s' saved to cache with key '%s'.", s.remoteBackupDir, cacheKey)

	return nil
}

func (s *EtcdBackupLocalDataStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	cacheKey := s.getCacheKey(ctx)
	backupPath, ok := ctx.GetTaskCache().Get(cacheKey)
	if !ok {
		logger.Warn("No backup path found in cache for this node, nothing to roll back.")
		return nil
	}

	backupDir, ok := backupPath.(string)
	if !ok || backupDir == "" {
		logger.Errorf("Invalid backup path in cache for key '%s', cannot proceed with rollback.", cacheKey)
		return nil
	}

	logger.Warnf("Rolling back by removing the created data backup directory '%s'...", backupDir)
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	cleanupCmd := fmt.Sprintf("rm -rf %s", backupDir)
	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, cleanupCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to remove data backup directory during rollback. Manual cleanup may be needed on host '%s'. Error: %v", ctx.GetHost().GetName(), err)
	}

	ctx.GetTaskCache().Delete(cacheKey)

	logger.Info("Rollback for local data backup finished.")
	return nil
}

var _ step.Step = (*EtcdBackupLocalDataStep)(nil)
