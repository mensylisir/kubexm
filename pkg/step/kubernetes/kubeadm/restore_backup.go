package kubeadm

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type RestoreKubeadmBackupStep struct {
	step.Base
	remoteKubeDir string
}

type RestoreKubeadmBackupStepBuilder struct {
	step.Builder[RestoreKubeadmBackupStepBuilder, *RestoreKubeadmBackupStep]
}

func NewRestoreKubeadmBackupStepBuilder(ctx runtime.Context, instanceName string) *RestoreKubeadmBackupStepBuilder {
	s := &RestoreKubeadmBackupStep{
		remoteKubeDir: "/etc/kubernetes",
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Restore Kubernetes configuration from a kubeadm-generated backup"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 10 * time.Minute

	b := new(RestoreKubeadmBackupStepBuilder).Init(s)
	return b
}

func (s *RestoreKubeadmBackupStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *RestoreKubeadmBackupStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for restoring kubeadm backup...")

	cacheKey := fmt.Sprintf(CacheKeyKubeadmBackupPath, ctx.GetHost().GetName())
	backupPath, ok := ctx.GetTaskCache().Get(cacheKey)
	if !ok {
		return false, fmt.Errorf("precheck failed: kubeadm backup path not found in cache for key '%s'. Cannot proceed with restore", cacheKey)
	}

	backupDir, ok := backupPath.(string)
	if !ok || backupDir == "" {
		return false, fmt.Errorf("precheck failed: invalid backup path in cache for key '%s'", cacheKey)
	}

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	checkCmd := fmt.Sprintf("[ -d %s ]", backupDir)
	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, checkCmd, s.Sudo); err != nil {
		return false, fmt.Errorf("precheck failed: kubeadm backup directory '%s' not found on host '%s'", backupDir, ctx.GetHost().GetName())
	}

	logger.Infof("Precheck passed: Found valid backup directory at '%s'.", backupDir)
	return false, nil
}

func (s *RestoreKubeadmBackupStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	cacheKey := fmt.Sprintf(CacheKeyKubeadmBackupPath, ctx.GetHost().GetName())
	backupPath, _ := ctx.GetTaskCache().Get(cacheKey)
	backupDir := backupPath.(string)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Warnf("!!! EXECUTING CRITICAL RESTORE OPERATION !!!")
	logger.Warnf("Restoring Kubernetes configuration from backup: '%s'...", backupDir)

	logger.Infof("Removing current configuration directory: '%s'...", s.remoteKubeDir)
	cleanupCmd := fmt.Sprintf("rm -rf %s", s.remoteKubeDir)
	if _, err := runner.Run(ctx.GoContext(), conn, cleanupCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to remove current config directory. Will attempt to restore anyway. Error: %v", err)
	}

	logger.Infof("Moving backup '%s' to '%s'...", backupDir, s.remoteKubeDir)
	restoreCmd := fmt.Sprintf("mv %s %s", backupDir, s.remoteKubeDir)
	if _, err := runner.Run(ctx.GoContext(), conn, restoreCmd, s.Sudo); err != nil {
		logger.Errorf("CRITICAL: FAILED TO RESTORE BACKUP. Host '%s' is in a broken state. MANUAL INTERVENTION IS REQUIRED.", ctx.GetHost().GetName())
		return fmt.Errorf("failed to move backup directory on host '%s': %w", ctx.GetHost().GetName(), err)
	}

	logger.Info("Restarting kubelet to apply restored configuration...")
	restartCmd := "systemctl restart kubelet"
	if _, err := runner.Run(ctx.GoContext(), conn, restartCmd, s.Sudo); err != nil {
		logger.Errorf("CRITICAL: FAILED TO RESTART KUBELET after restore. Host '%s' might not recover. MANUAL INTERVENTION IS REQUIRED. Error: %v", ctx.GetHost().GetName(), err)
		return fmt.Errorf("failed to restart kubelet after restore on host '%s': %w", ctx.GetHost().GetName(), err)
	}

	ctx.GetTaskCache().Delete(cacheKey)

	logger.Info("Restore operation completed. The node is attempting to revert to its pre-upgrade state. Further health checks are required.")
	return nil
}

func (s *RestoreKubeadmBackupStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Error("Rollback is not possible for a restore operation. If this step was triggered as part of a larger rollback, the node is likely in a pre-upgrade state.")
	return nil
}

var _ step.Step = (*RestoreKubeadmBackupStep)(nil)
