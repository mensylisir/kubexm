package kubeadm

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmCleanupBackupsStep struct {
	step.Base
}

type KubeadmCleanupBackupsStepBuilder struct {
	step.Builder[KubeadmCleanupBackupsStepBuilder, *KubeadmCleanupBackupsStep]
}

func NewKubeadmCleanupBackupsStepBuilder(ctx runtime.Context, instanceName string) *KubeadmCleanupBackupsStepBuilder {
	s := &KubeadmCleanupBackupsStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Clean up temporary backups created by 'kubeadm upgrade' on the node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(KubeadmCleanupBackupsStepBuilder).Init(s)
	return b
}

func (s *KubeadmCleanupBackupsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmCleanupBackupsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	return false, nil
}

func (s *KubeadmCleanupBackupsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	cacheKey := fmt.Sprintf(common.CacheKeyKubeadmBackupPath, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName())
	backupPath, ok := ctx.GetTaskCache().Get(cacheKey)
	if !ok {
		logger.Info("No kubeadm backup path found in cache for this node. Nothing to clean up.")
		return nil
	}

	backupDir, ok := backupPath.(string)
	if !ok || backupDir == "" {
		logger.Warnf("Invalid kubeadm backup path in cache for key '%s'. Skipping cleanup.", cacheKey)
		return nil
	}

	logger.Warnf("Cleaning up 'kubeadm upgrade' backup directory on remote node: '%s'", backupDir)

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
	logger.Info("Successfully cleaned up remote 'kubeadm upgrade' backup.")
	return nil
}

func (s *KubeadmCleanupBackupsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a cleanup step.")
	return nil
}

var _ step.Step = (*KubeadmCleanupBackupsStep)(nil)
