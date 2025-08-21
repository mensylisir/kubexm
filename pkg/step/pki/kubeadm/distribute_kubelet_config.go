// in step/pki/kubeadm/distribute_kubelet_config.go

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

type KubeadmDistributeKubeletConfigStep struct {
	step.Base
	localCertsDir        string
	remotePKIDir         string
	remoteKubeletConfDir string
}

type KubeadmDistributeKubeletConfigStepBuilder struct {
	step.Builder[KubeadmDistributeKubeletConfigStepBuilder, *KubeadmDistributeKubeletConfigStep]
}

func NewKubeadmDistributeKubeletConfigStepBuilder(ctx runtime.Context, instanceName string) *KubeadmDistributeKubeletConfigStepBuilder {
	s := &KubeadmDistributeKubeletConfigStep{
		remotePKIDir:         common.DefaultPKIPath,
		remoteKubeletConfDir: common.KubeletPKIDirTarget,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Distribute ca.crt and the node-specific kubelet.conf to the node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(KubeadmDistributeKubeletConfigStepBuilder).Init(s)
	return b
}

func (s *KubeadmDistributeKubeletConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmDistributeKubeletConfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	baseCertsDir := ctx.GetKubernetesCertsDir()

	s.localCertsDir = baseCertsDir

	localCaPath := filepath.Join(s.localCertsDir, "ca.crt")
	if _, err := os.Stat(localCaPath); os.IsNotExist(err) {
		return false, fmt.Errorf("precheck failed: local source file '%s' not found", localCaPath)
	}

	nodeName := ctx.GetHost().GetName()
	localKubeletConfPath := filepath.Join(s.localCertsDir, fmt.Sprintf("kubelet-%s.conf", nodeName))
	if _, err := os.Stat(localKubeletConfPath); os.IsNotExist(err) {
		return false, fmt.Errorf("precheck failed: local source file '%s' not found", localKubeletConfPath)
	}

	logger.Info("Precheck passed: local source files (ca.crt, kubelet.conf) exist.")
	return false, nil
}

func (s *KubeadmDistributeKubeletConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	nodeName := ctx.GetHost().GetName()
	logger.Infof("Distributing new certificates and kubelet config for node %s...", nodeName)

	localCaPath := filepath.Join(s.localCertsDir, "ca.crt")
	remoteCaPath := filepath.Join(s.remotePKIDir, "ca.crt")
	logger.Infof("Uploading new ca.crt to '%s'", remoteCaPath)
	if err := runner.Upload(ctx.GoContext(), conn, localCaPath, remoteCaPath, s.Sudo); err != nil {
		return fmt.Errorf("failed to upload ca.crt for node %s: %w", nodeName, err)
	}

	localKubeletConfPath := filepath.Join(s.localCertsDir, fmt.Sprintf("kubelet-%s.conf", nodeName))
	remoteKubeletConfPath := filepath.Join(s.remoteKubeletConfDir, "kubelet.conf")
	logger.Infof("Uploading new kubelet.conf to '%s'", remoteKubeletConfPath)
	if err := runner.Upload(ctx.GoContext(), conn, localKubeletConfPath, remoteKubeletConfPath, s.Sudo); err != nil {
		return fmt.Errorf("failed to upload kubelet.conf for node %s: %w", nodeName, err)
	}

	logger.Info("Successfully distributed new ca.crt and kubelet.conf.")
	return nil
}

func (s *KubeadmDistributeKubeletConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	cacheKey := fmt.Sprintf(common.CacheKubeconfigsBackupPath, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName())
	backupPath, ok := ctx.GetTaskCache().Get(cacheKey)
	if !ok {
		logger.Error("CRITICAL: No backup path found in cache for '/etc/kubernetes'. CANNOT ROLL BACK. MANUAL INTERVENTION REQUIRED.")
		return fmt.Errorf("no backup path found for host '%s'", ctx.GetHost().GetName())
	}

	backupDir, ok := backupPath.(string)
	if !ok || backupDir == "" {
		logger.Error("CRITICAL: Invalid backup path in cache (not a non-empty string). CANNOT ROLL BACK. MANUAL INTERVENTION REQUIRED.")
		return fmt.Errorf("invalid backup path in cache for host '%s', value: %v", ctx.GetHost().GetName(), backupPath)
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("CRITICAL: Cannot connect to host '%s' for rollback. The '/etc/kubernetes' directory is likely in an inconsistent state. MANUAL INTERVENTION REQUIRED. Error: %v", ctx.GetHost().GetName(), err)
		return err
	}

	logger.Warnf("Rolling back: restoring original '/etc/kubernetes' from backup '%s'...", backupDir)

	targetDir := s.remoteKubeletConfDir

	cleanupCmd := fmt.Sprintf("rm -rf %s", targetDir)
	if _, err := runner.Run(ctx.GoContext(), conn, cleanupCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to remove modified directory '%s' during rollback. Continuing with restore attempt. Error: %v", targetDir, err)
	}

	restoreCmd := fmt.Sprintf("mv %s %s", backupDir, targetDir)
	if _, err := runner.Run(ctx.GoContext(), conn, restoreCmd, s.Sudo); err != nil {
		logger.Errorf("CRITICAL: Failed to restore backup on host '%s'. The '/etc/kubernetes' directory is in an inconsistent state. MANUAL INTERVENTION REQUIRED. Error: %v", ctx.GetHost().GetName(), err)
		return fmt.Errorf("failed to restore backup '%s' to '%s' on host '%s': %w", backupDir, targetDir, ctx.GetHost().GetName(), err)
	}

	ctx.GetTaskCache().Delete(cacheKey)

	logger.Info("Rollback completed: original '/etc/kubernetes' directory has been restored from backup.")
	return nil
}

var _ step.Step = (*KubeadmDistributeKubeletConfigStep)(nil)
