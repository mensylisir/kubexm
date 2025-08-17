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

type KubeadmDistributeStackedEtcdPKIStep struct {
	step.Base
	localKubeCertsDir string
	remoteKubePkiDir  string
	etcdPkiAsset      caAsset
}

type KubeadmDistributeStackedEtcdPKIStepBuilder struct {
	step.Builder[KubeadmDistributeStackedEtcdPKIStepBuilder, *KubeadmDistributeStackedEtcdPKIStep]
}

func NewKubeadmDistributeStackedEtcdPKIStepBuilder(ctx runtime.Context, instanceName string) *KubeadmDistributeStackedEtcdPKIStepBuilder {
	localCertsDir := ctx.GetKubernetesCertsDir()
	remotePkiDir := common.DefaultEtcdPKIDir

	s := &KubeadmDistributeStackedEtcdPKIStep{
		localKubeCertsDir: localCertsDir,
		remoteKubePkiDir:  remotePkiDir,
		etcdPkiAsset: caAsset{
			CertFile: "etcd/ca.crt",
			KeyFile:  "etcd/ca.key",
		},
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Distribute renewed stacked Etcd PKI files (CA bundle and key) to the master node"
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(KubeadmDistributeStackedEtcdPKIStepBuilder).Init(s)
	return b
}

func (s *KubeadmDistributeStackedEtcdPKIStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmDistributeStackedEtcdPKIStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for stacked Etcd PKI distribution...")

	if _, ok := ctx.GetTaskCache().Get(CacheKeyRemotePKIBackupPath); !ok {
		return false, fmt.Errorf("precheck failed: remote PKI backup path not found in cache. The backup step must run first")
	}

	localCert := filepath.Join(s.localKubeCertsDir, s.etcdPkiAsset.CertFile)
	localKey := filepath.Join(s.localKubeCertsDir, s.etcdPkiAsset.KeyFile)
	if _, err := os.Stat(localCert); os.IsNotExist(err) {
		return false, fmt.Errorf("precheck failed: local source file '%s' not found", localCert)
	}
	if _, err := os.Stat(localKey); os.IsNotExist(err) {
		return false, fmt.Errorf("precheck failed: local source file '%s' not found", localKey)
	}

	logger.Info("Precheck passed: backup found and local Etcd PKI source files exist.")
	return false, nil
}

func (s *KubeadmDistributeStackedEtcdPKIStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Distributing renewed stacked Etcd PKI files...")

	localCertPath := filepath.Join(s.localKubeCertsDir, s.etcdPkiAsset.CertFile)
	localKeyPath := filepath.Join(s.localKubeCertsDir, s.etcdPkiAsset.KeyFile)
	remoteCertPath := filepath.Join(s.remoteKubePkiDir, s.etcdPkiAsset.CertFile)
	remoteKeyPath := filepath.Join(s.remoteKubePkiDir, s.etcdPkiAsset.KeyFile)

	remoteEtcdDir := filepath.Dir(remoteCertPath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteEtcdDir, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote etcd pki directory '%s': %w", remoteEtcdDir, err)
	}

	log := logger.With("asset", s.etcdPkiAsset.CertFile)

	log.Infof("Uploading Etcd CA bundle to '%s'", remoteCertPath)
	if err := runner.Upload(ctx.GoContext(), conn, localCertPath, remoteCertPath, s.Sudo); err != nil {
		return fmt.Errorf("failed to upload '%s': %w", s.etcdPkiAsset.CertFile, err)
	}

	log.Infof("Uploading Etcd CA private key to '%s'", remoteKeyPath)
	if err := runner.Upload(ctx.GoContext(), conn, localKeyPath, remoteKeyPath, s.Sudo); err != nil {
		return fmt.Errorf("failed to upload '%s': %w", s.etcdPkiAsset.KeyFile, err)
	}

	logger.Info("Successfully distributed renewed stacked Etcd PKI files to the node.")
	return nil
}

func (s *KubeadmDistributeStackedEtcdPKIStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	backupPath, ok := ctx.GetTaskCache().Get(CacheKeyRemotePKIBackupPath)
	if !ok {
		logger.Error("CRITICAL: No backup path found in cache. CANNOT ROLL BACK. MANUAL INTERVENTION REQUIRED.")
		return fmt.Errorf("no backup path found for host '%s'", ctx.GetHost().GetName())
	}
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

var _ step.Step = (*KubeadmDistributeStackedEtcdPKIStep)(nil)
