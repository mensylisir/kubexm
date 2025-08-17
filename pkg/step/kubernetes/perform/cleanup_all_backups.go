package perform

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type CleanupAllBackupsStep struct {
	step.Base
}

type CleanupAllBackupsStepBuilder struct {
	step.Builder[CleanupAllBackupsStepBuilder, *CleanupAllBackupsStep]
}

func NewCleanupAllBackupsStepBuilder(ctx runtime.Context, instanceName string) *CleanupAllBackupsStepBuilder {
	s := &CleanupAllBackupsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Clean up all temporary files and backups"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(CleanupAllBackupsStepBuilder).Init(s)
	return b
}

func (s *CleanupAllBackupsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CleanupAllBackupsStep) LocalCleanup(ctx runtime.Context) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "LocalCleanup")
	logger.Info("Starting local workspace cleanup...")

	remoteConfigsDir := filepath.Join(ctx.GetGlobalWorkDir(), "remote-configs")
	if _, err := os.Stat(remoteConfigsDir); err == nil {
		logger.Infof("Removing local directory with fetched remote configs: '%s'", remoteConfigsDir)
		if err := os.RemoveAll(remoteConfigsDir); err != nil {
			logger.Errorf("Failed to remove local remote-configs directory: %v", err)
		}
	}

	baseCertsDir := ctx.GetKubernetesCertsDir()
	certsNewDir := filepath.Join(baseCertsDir, "certs-new")
	certsOldDir := filepath.Join(baseCertsDir, "certs-old")
	if _, err := os.Stat(certsNewDir); err == nil {
		logger.Infof("Removing 'certs-new' directory...")
		if err := os.RemoveAll(certsNewDir); err != nil {
			logger.Errorf("Failed to remove 'certs-new': %v", err)
		}
	}
	if _, err := os.Stat(certsOldDir); err == nil {
		logger.Infof("Removing 'certs-old' directory...")
		if err := os.RemoveAll(certsOldDir); err != nil {
			logger.Errorf("Failed to remove 'certs-old': %v", err)
		}
	}

	logger.Info("Local workspace cleanup finished.")
}

func (s *CleanupAllBackupsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *CleanupAllBackupsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Starting remote node cleanup...")

	cacheKey := fmt.Sprintf(CacheKeyRemoteBackupPath, ctx.GetHost().GetName())
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
		logger.Errorf("Failed to connect, cannot perform remote cleanup: %v", err)
		return nil
	}

	cleanupCmd := fmt.Sprintf("rm -rf %s", backupDir)
	if _, err := runner.Run(ctx.GoContext(), conn, cleanupCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to remove remote backup directory '%s'. Manual cleanup may be required. Error: %v", backupDir, err)
	}

	ctx.GetTaskCache().Delete(cacheKey)
	logger.Info("Remote node cleanup finished.")
	return nil
}

func (s *CleanupAllBackupsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a cleanup step.")
	return nil
}

var _ step.Step = (*CleanupAllBackupsStep)(nil)
