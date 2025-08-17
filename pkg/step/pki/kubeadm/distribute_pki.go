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

type KubeadmDistributeK8sPKIStep struct {
	step.Base
	localKubeCertsDir string
	remoteKubePkiDir  string
	pkiFilesToSync    []caAsset
}

type KubeadmDistributeK8sPKIStepBuilder struct {
	step.Builder[KubeadmDistributeK8sPKIStepBuilder, *KubeadmDistributeK8sPKIStep]
}

func NewKubeadmDistributeK8sPKIStepBuilder(ctx runtime.Context, instanceName string) *KubeadmDistributeK8sPKIStepBuilder {
	localCertsDir := ctx.GetKubernetesCertsDir()
	remotePkiDir := common.DefaultPKIPath

	assets := []caAsset{
		{CertFile: "ca.crt", KeyFile: "ca.key"},
		{CertFile: "front-proxy-ca.crt", KeyFile: "front-proxy-ca.key"},
	}

	s := &KubeadmDistributeK8sPKIStep{
		localKubeCertsDir: localCertsDir,
		remoteKubePkiDir:  remotePkiDir,
		pkiFilesToSync:    assets,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Distribute renewed K8s PKI files (CA bundles and keys) to the master node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(KubeadmDistributeK8sPKIStepBuilder).Init(s)
	return b
}

func (s *KubeadmDistributeK8sPKIStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmDistributeK8sPKIStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	if _, ok := ctx.GetTaskCache().Get(CacheKeyRemotePKIBackupPath); !ok {
		return false, fmt.Errorf("precheck failed: remote PKI backup path not found in cache. The backup step must run first")
	}

	for _, asset := range s.pkiFilesToSync {
		localCert := filepath.Join(s.localKubeCertsDir, asset.CertFile)
		localKey := filepath.Join(s.localKubeCertsDir, asset.KeyFile)
		if _, err := os.Stat(localCert); os.IsNotExist(err) {
			return false, fmt.Errorf("precheck failed: local source file '%s' not found", localCert)
		}
		if _, err := os.Stat(localKey); os.IsNotExist(err) {
			return false, fmt.Errorf("precheck failed: local source file '%s' not found", localKey)
		}
	}

	logger.Info("Precheck passed: backup found and local source files exist.")
	return false, nil
}

func (s *KubeadmDistributeK8sPKIStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Info("Distributing renewed K8s PKI files...")
	for _, asset := range s.pkiFilesToSync {
		localCertPath := filepath.Join(s.localKubeCertsDir, asset.CertFile)
		localKeyPath := filepath.Join(s.localKubeCertsDir, asset.KeyFile)
		remoteCertPath := filepath.Join(s.remoteKubePkiDir, asset.CertFile)
		remoteKeyPath := filepath.Join(s.remoteKubePkiDir, asset.KeyFile)

		log := logger.With("ca_name", asset.CertFile)

		log.Infof("Uploading CA bundle to '%s'", remoteCertPath)
		if err := runner.Upload(ctx.GoContext(), conn, localCertPath, remoteCertPath, s.Sudo); err != nil {
			return fmt.Errorf("failed to upload '%s': %w", asset.CertFile, err)
		}

		log.Infof("Uploading CA private key to '%s'", remoteKeyPath)
		if err := runner.Upload(ctx.GoContext(), conn, localKeyPath, remoteKeyPath, s.Sudo); err != nil {
			return fmt.Errorf("failed to upload '%s': %w", asset.KeyFile, err)
		}
	}

	logger.Info("Successfully distributed renewed K8s PKI files to the node.")
	return nil
}

func (s *KubeadmDistributeK8sPKIStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	backupPath, ok := ctx.GetTaskCache().Get(CacheKeyRemotePKIBackupPath)
	if !ok {
		logger.Error("CRITICAL: No backup path found in cache. CANNOT ROLL BACK. MANUAL INTERVENTION REQUIRED.")
		return fmt.Errorf("no backup path found in cache for host '%s', cannot restore PKI", ctx.GetHost().GetName())
	}

	backupDir, ok := backupPath.(string)
	if !ok || backupDir == "" {
		logger.Error("CRITICAL: Invalid backup path in cache. CANNOT ROLL BACK. MANUAL INTERVENTION REQUIRED.")
		return fmt.Errorf("invalid backup path in cache for host '%s'", ctx.GetHost().GetName())
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Cannot connect to host for rollback, manual intervention required. Error: %v", err)
		return err
	}

	logger.Warnf("Rolling back: restoring original PKI from backup '%s'...", backupDir)

	cleanupCmd := fmt.Sprintf("rm -rf %s", s.remoteKubePkiDir)
	if _, err := runner.Run(ctx.GoContext(), conn, cleanupCmd, s.Sudo); err != nil {
		logger.Errorf("Failed to remove modified PKI directory during rollback. Manual cleanup may be needed. Error: %v", err)
	}

	restoreCmd := fmt.Sprintf("mv %s %s", backupDir, s.remoteKubePkiDir)
	if _, err := runner.Run(ctx.GoContext(), conn, restoreCmd, s.Sudo); err != nil {
		logger.Errorf("CRITICAL: Failed to restore PKI backup on host '%s'. MANUAL INTERVENTION REQUIRED. Error: %v", ctx.GetHost().GetName(), err)
		return err
	}

	ctx.GetTaskCache().Delete(CacheKeyRemotePKIBackupPath)

	logger.Info("Rollback completed: original PKI directory has been restored.")
	return nil
}

var _ step.Step = (*KubeadmDistributeK8sPKIStep)(nil)
