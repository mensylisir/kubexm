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

type KubeadmDistributeStackedEtcdLeafCertsStep struct {
	step.Base
	localCertsDir    string
	remoteKubePkiDir string
}

type KubeadmDistributeStackedEtcdLeafCertsStepBuilder struct {
	step.Builder[KubeadmDistributeStackedEtcdLeafCertsStepBuilder, *KubeadmDistributeStackedEtcdLeafCertsStep]
}

func NewKubeadmDistributeStackedEtcdLeafCertsStepBuilder(ctx runtime.Context, instanceName string) *KubeadmDistributeStackedEtcdLeafCertsStepBuilder {
	s := &KubeadmDistributeStackedEtcdLeafCertsStep{
		remoteKubePkiDir: common.DefaultEtcdPKIDir,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Distribute renewed stacked Etcd leaf certificates and keys to the master node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(KubeadmDistributeStackedEtcdLeafCertsStepBuilder).Init(s)
	return b
}

func (s *KubeadmDistributeStackedEtcdLeafCertsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmDistributeStackedEtcdLeafCertsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	baseCertsDir := ctx.GetKubernetesCertsDir()
	certsNewDir := filepath.Join(baseCertsDir, "certs-new")
	if _, err := os.Stat(certsNewDir); err == nil {
		s.localCertsDir = certsNewDir
	} else {
		s.localCertsDir = baseCertsDir
	}

	cacheKey := fmt.Sprintf(common.CacheKubeCertsBackupPath, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName())
	if _, ok := ctx.GetTaskCache().Get(cacheKey); !ok {
		return false, fmt.Errorf("precheck failed: remote PKI backup path not found in cache")
	}

	nodeName := ctx.GetHost().GetName()
	filesToCheck := map[string]string{
		"server":      "server-%s",
		"peer":        "peer-%s",
		"healthcheck": "healthcheck-client",
	}

	for _, pattern := range filesToCheck {
		baseName := fmt.Sprintf(pattern, nodeName)
		localCert := filepath.Join(s.localCertsDir, "etcd", fmt.Sprintf("%s.crt", baseName))
		if _, err := os.Stat(localCert); os.IsNotExist(err) {
			return false, fmt.Errorf("precheck failed: local source file '%s' not found", localCert)
		}
	}

	logger.Info("Precheck passed: backup found and local Etcd leaf certs exist.")
	return false, nil
}

func (s *KubeadmDistributeStackedEtcdLeafCertsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	baseCertsDir := ctx.GetKubernetesCertsDir()
	certsNewDir := filepath.Join(baseCertsDir, "certs-new")
	if _, err := os.Stat(certsNewDir); err == nil {
		s.localCertsDir = certsNewDir
	} else {
		s.localCertsDir = baseCertsDir
	}

	nodeName := ctx.GetHost().GetName()
	logger.Infof("Distributing renewed stacked Etcd leaf certificates for node %s...", nodeName)

	filesToDistribute := map[string]string{
		fmt.Sprintf("server-%s.crt", nodeName): "server.crt",
		fmt.Sprintf("server-%s.key", nodeName): "server.key",
		fmt.Sprintf("peer-%s.crt", nodeName):   "peer.crt",
		fmt.Sprintf("peer-%s.key", nodeName):   "peer.key",
		"healthcheck-client.crt":               "healthcheck-client.crt",
		"healthcheck-client.key":               "healthcheck-client.key",
	}

	for localFileName, remoteFileName := range filesToDistribute {
		localPath := filepath.Join(s.localCertsDir, "etcd", localFileName)
		remotePath := filepath.Join(s.remoteKubePkiDir, "etcd", remoteFileName)
		log := logger.With("source", localPath, "destination", remotePath)

		log.Info("Uploading file...")
		if err := runner.Upload(ctx.GoContext(), conn, localPath, remotePath, s.Sudo); err != nil {
			return fmt.Errorf("failed to upload '%s': %w", localFileName, err)
		}
	}

	logger.Info("Successfully distributed renewed stacked Etcd leaf certificates.")
	return nil
}

func (s *KubeadmDistributeStackedEtcdLeafCertsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	cacheKey := fmt.Sprintf(common.CacheKubeCertsBackupPath, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName())
	backupPath, _ := ctx.GetTaskCache().Get(cacheKey)
	backupDir := backupPath.(string)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Warnf("Rolling back: restoring entire PKI from backup '%s'...", backupDir)

	cleanupCmd := fmt.Sprintf("rm -rf %s", s.remoteKubePkiDir)
	_, _ = runner.Run(ctx.GoContext(), conn, cleanupCmd, s.Sudo)

	restoreCmd := fmt.Sprintf("mv %s %s", backupDir, s.remoteKubePkiDir)
	if _, err := runner.Run(ctx.GoContext(), conn, restoreCmd, s.Sudo); err != nil {
		logger.Errorf("CRITICAL: Failed to restore PKI backup on host '%s'. MANUAL INTERVENTION REQUIRED. Error: %v", ctx.GetHost().GetName(), err)
		return err
	}

	logger.Info("Rollback completed: original PKI directory has been restored.")
	return nil
}

var _ step.Step = (*KubeadmDistributeStackedEtcdLeafCertsStep)(nil)
