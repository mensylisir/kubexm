package kubeadm

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmDistributeKubeconfigsStep struct {
	step.Base
	localCertsDir       string
	remoteKubeconfigDir string
	configsToDistribute []string
}

// KubeadmDistributeKubeconfigsStepBuilder for KubeadmDistributeKubeconfigsStep
type KubeadmDistributeKubeconfigsStepBuilder struct {
	step.Builder[KubeadmDistributeKubeconfigsStepBuilder, *KubeadmDistributeKubeconfigsStep]
}

func NewKubeadmDistributeKubeconfigsStepBuilder(ctx runtime.Context, instanceName string) *KubeadmDistributeKubeconfigsStepBuilder {
	s := &KubeadmDistributeKubeconfigsStep{
		remoteKubeconfigDir: common.KubernetesConfigDir,
		configsToDistribute: []string{
			"admin.conf",
			"controller-manager.conf",
			"scheduler.conf",
		},
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Distribute renewed kubeconfig files to the master node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(KubeadmDistributeKubeconfigsStepBuilder).Init(s)
	return b
}

func (s *KubeadmDistributeKubeconfigsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmDistributeKubeconfigsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	baseCertsDir := ctx.GetKubernetesCertsDir()
	certsNewDir := baseCertsDir
	if _, err := os.Stat(certsNewDir); err == nil {
		s.localCertsDir = certsNewDir
	} else {
		s.localCertsDir = baseCertsDir
	}

	cacheKey := fmt.Sprintf(common.CacheKubeconfigsBackupPath, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName())
	if _, ok := ctx.GetTaskCache().Get(cacheKey); !ok {
		logger.Warn("Remote kubeconfig backup path not found in cache. Rollback might not be possible.")
	}

	for _, confFile := range s.configsToDistribute {
		localPath := filepath.Join(s.localCertsDir, confFile)
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			return false, fmt.Errorf("precheck failed: local source kubeconfig file '%s' not found", localPath)
		}
	}

	logger.Info("Precheck passed: local source kubeconfig files exist.")
	return false, nil
}

func (s *KubeadmDistributeKubeconfigsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Infof("Distributing %d renewed kubeconfig files to '%s'...", len(s.configsToDistribute), s.remoteKubeconfigDir)

	if err := runner.Mkdirp(ctx.GoContext(), conn, s.remoteKubeconfigDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote kubeconfig directory '%s': %w", s.remoteKubeconfigDir, err)
	}

	for _, confFile := range s.configsToDistribute {
		localPath := filepath.Join(s.localCertsDir, confFile)
		remotePath := filepath.Join(s.remoteKubeconfigDir, confFile)
		log := logger.With("source", localPath, "destination", remotePath)

		log.Info("Uploading kubeconfig file...")
		if err := runner.Upload(ctx.GoContext(), conn, localPath, remotePath, s.Sudo); err != nil {
			return fmt.Errorf("failed to upload kubeconfig '%s': %w", confFile, err)
		}
	}

	logger.Info("Successfully distributed renewed kubeconfig files to the node.")
	return nil
}

func (s *KubeadmDistributeKubeconfigsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	cacheKey := fmt.Sprintf(common.CacheKubeconfigsBackupPath, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName())
	backupPath, ok := ctx.GetTaskCache().Get(cacheKey)
	if !ok {
		logger.Error("CRITICAL: No kubeconfig backup path found in cache. CANNOT ROLL BACK. MANUAL INTERVENTION REQUIRED.")
		return fmt.Errorf("no backup path found in cache for host '%s', cannot restore /etc/kubernetes", ctx.GetHost().GetName())
	}

	backupDir, ok := backupPath.(string)
	if !ok || backupDir == "" {
		logger.Error("CRITICAL: Invalid backup path in cache (not a non-empty string). CANNOT ROLL BACK. MANUAL INTERVENTION REQUIRED.")
		return fmt.Errorf("invalid backup path in cache for host '%s', value: %v", ctx.GetHost().GetName(), backupPath)
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("CRITICAL: Cannot connect to host '%s' for rollback. MANUAL INTERVENTION REQUIRED. Error: %v", ctx.GetHost().GetName(), err)
		return err
	}

	logger.Warnf("Rolling back: restoring original '/etc/kubernetes' from backup '%s'...", backupDir)

	cleanupCmd := fmt.Sprintf("rm -rf %s", s.remoteKubeconfigDir)
	if _, err := runner.Run(ctx.GoContext(), conn, cleanupCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to remove modified directory '%s' during rollback. Continuing with restore attempt. Error: %v", s.remoteKubeconfigDir, err)
	}

	restoreCmd := fmt.Sprintf("mv %s %s", backupDir, s.remoteKubeconfigDir)
	if _, err := runner.Run(ctx.GoContext(), conn, restoreCmd, s.Sudo); err != nil {
		logger.Errorf("CRITICAL: Failed to restore backup on host '%s'. The '/etc/kubernetes' directory is in an inconsistent state. MANUAL INTERVENTION REQUIRED. Error: %v", ctx.GetHost().GetName(), err)
		return fmt.Errorf("failed to restore backup '%s' to '%s' on host '%-s': %w", backupDir, s.remoteKubeconfigDir, ctx.GetHost().GetName(), err)
	}

	ctx.GetTaskCache().Delete(cacheKey)
	logger.Info("Rollback completed: original '/etc/kubernetes' directory has been restored from backup.")
	return nil
}

var _ step.Step = (*KubeadmDistributeKubeconfigsStep)(nil)
