package backup

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	remoteConfigBackupSuffix = "k8s-config.bak"
)

type BackupRemoteConfigsStep struct {
	step.Base
	remoteConfigDir string
	remoteBackupDir string
}

type BackupRemoteConfigsStepBuilder struct {
	step.Builder[BackupRemoteConfigsStepBuilder, *BackupRemoteConfigsStep]
}

func NewBackupRemoteConfigsStepBuilder(ctx runtime.Context, instanceName string) *BackupRemoteConfigsStepBuilder {
	s := &BackupRemoteConfigsStep{
		remoteConfigDir: DefaultRemoteK8sConfigDir,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Create a local backup of the /etc/kubernetes directory on the node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(BackupRemoteConfigsStepBuilder).Init(s)
	return b
}

func (s *BackupRemoteConfigsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *BackupRemoteConfigsStep) getCacheKey(ctx runtime.ExecutionContext) string {
	return fmt.Sprintf(common.CacheKeyRemoteBackupPath, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName(), ctx.GetHost().GetName())
}

func (s *BackupRemoteConfigsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for remote Kubernetes configs backup...")

	cacheKey := s.getCacheKey(ctx)

	if _, ok := ctx.GetTaskCache().Get(cacheKey); ok {
		logger.Info("Remote Kubernetes configs have already been backed up for this node. Step is done.")
		return true, nil
	}

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	checkCmd := fmt.Sprintf("[ -d %s ]", s.remoteConfigDir)
	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, checkCmd, s.Sudo); err != nil {
		return false, fmt.Errorf("precheck failed: source config directory '%s' not found on host '%s'", s.remoteConfigDir, ctx.GetHost().GetName())
	}

	logger.Info("Precheck passed: Config directory exists and no backup has been made yet.")
	return false, nil
}

func (s *BackupRemoteConfigsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	s.remoteBackupDir = fmt.Sprintf("%s.%s-%d", s.remoteConfigDir, remoteConfigBackupSuffix, time.Now().Unix())
	cacheKey := s.getCacheKey(ctx)

	logger.Infof("Backing up remote Kubernetes config directory from '%s' to '%s'...", s.remoteConfigDir, s.remoteBackupDir)

	backupCmd := fmt.Sprintf("cp -a %s %s", s.remoteConfigDir, s.remoteBackupDir)
	if _, err := runner.Run(ctx.GoContext(), conn, backupCmd, s.Sudo); err != nil {
		return fmt.Errorf("failed to back up remote config directory on host '%s': %w", ctx.GetHost().GetName(), err)
	}

	ctx.GetTaskCache().Set(cacheKey, s.remoteBackupDir)
	logger.Infof("Successfully backed up config directory. Backup path '%s' saved to cache with key '%s'.", s.remoteBackupDir, cacheKey)

	return nil
}

func (s *BackupRemoteConfigsStep) Rollback(ctx runtime.ExecutionContext) error {
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

	logger.Warnf("Rolling back by removing the created config backup directory '%s'...", backupDir)
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	cleanupCmd := fmt.Sprintf("rm -rf %s", backupDir)
	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, cleanupCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to remove config backup directory during rollback. Manual cleanup may be needed on host '%s'. Error: %v", ctx.GetHost().GetName(), err)
	}

	ctx.GetTaskCache().Delete(cacheKey)

	logger.Info("Rollback for remote configs backup finished.")
	return nil
}

var _ step.Step = (*BackupRemoteConfigsStep)(nil)
