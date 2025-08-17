package kubeadm

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

type KubeadmPrepareStackedEtcdCAAssetsStep struct {
	step.Base
	localKubeCertsDir string
	localOldCertsDir  string
	localNewCertsDir  string
	etcdCaAsset       caAsset
}

type KubeadmPrepareStackedEtcdCAAssetsStepBuilder struct {
	step.Builder[KubeadmPrepareStackedEtcdCAAssetsStepBuilder, *KubeadmPrepareStackedEtcdCAAssetsStep]
}

func NewKubeadmPrepareStackedEtcdCAAssetsStepBuilder(ctx runtime.Context, instanceName string) *KubeadmPrepareStackedEtcdCAAssetsStepBuilder {
	localCertsDir := ctx.GetKubernetesCertsDir()
	certsOldDir := filepath.Join(localCertsDir, "certs-old")
	certsNewDir := filepath.Join(localCertsDir, "certs-new")

	s := &KubeadmPrepareStackedEtcdCAAssetsStep{
		localKubeCertsDir: localCertsDir,
		localOldCertsDir:  certsOldDir,
		localNewCertsDir:  certsNewDir,
		etcdCaAsset: caAsset{
			CertFile: "etcd/ca.crt",
			KeyFile:  "etcd/ca.key",
		},
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Prepare assets by backing up the original stacked Etcd CA to a 'certs-old' directory"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(KubeadmPrepareStackedEtcdCAAssetsStepBuilder).Init(s)
	return b
}

func (s *KubeadmPrepareStackedEtcdCAAssetsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmPrepareStackedEtcdCAAssetsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	originalCertPath := filepath.Join(s.localKubeCertsDir, s.etcdCaAsset.CertFile)
	originalKeyPath := filepath.Join(s.localKubeCertsDir, s.etcdCaAsset.KeyFile)
	if !helpers.IsFileExist(originalCertPath) || !helpers.IsFileExist(originalKeyPath) {
		return false, fmt.Errorf("original stacked Etcd CA file '%s' or '%s' not found, cannot prepare assets", originalCertPath, originalKeyPath)
	}

	backupCertPath := filepath.Join(s.localOldCertsDir, s.etcdCaAsset.CertFile)
	if helpers.IsFileExist(backupCertPath) {
		sourceSum, err1 := helpers.GetFileSha256(originalCertPath)
		backupSum, err2 := helpers.GetFileSha256(backupCertPath)
		if err1 == nil && err2 == nil && sourceSum == backupSum {
			logger.Info("Stacked Etcd CA assets have already been prepared and backed up. Step is done.")
			return true, nil
		}
	}

	return false, nil
}

func (s *KubeadmPrepareStackedEtcdCAAssetsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	sourceCert := filepath.Join(s.localKubeCertsDir, s.etcdCaAsset.CertFile)
	sourceKey := filepath.Join(s.localKubeCertsDir, s.etcdCaAsset.KeyFile)
	destOldCert := filepath.Join(s.localOldCertsDir, s.etcdCaAsset.CertFile)
	destOldKey := filepath.Join(s.localOldCertsDir, s.etcdCaAsset.KeyFile)
	destNewKey := filepath.Join(s.localNewCertsDir, s.etcdCaAsset.KeyFile)

	if err := os.MkdirAll(filepath.Dir(destOldCert), 0755); err != nil {
		return fmt.Errorf("failed to create 'certs-old/etcd' subdirectory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(destNewKey), 0755); err != nil {
		return fmt.Errorf("failed to create 'certs-new/etcd' subdirectory: %w", err)
	}

	log := logger.With("asset", s.etcdCaAsset.CertFile)

	log.Info("Backing up stacked Etcd CA certificate and key...")
	if err := helpers.CopyFile(sourceCert, destOldCert); err != nil {
		return fmt.Errorf("failed to copy '%s' to backup directory: %w", s.etcdCaAsset.CertFile, err)
	}
	if err := helpers.CopyFile(sourceKey, destOldKey); err != nil {
		return fmt.Errorf("failed to copy '%s' to backup directory: %w", s.etcdCaAsset.KeyFile, err)
	}

	log.Info("Staging stacked Etcd CA private key for renewal step...")
	if err := helpers.CopyFile(sourceKey, destNewKey); err != nil {
		return fmt.Errorf("failed to stage key '%s' to new assets directory: %w", s.etcdCaAsset.KeyFile, err)
	}

	logger.Info("Successfully prepared stacked Etcd CA assets.")
	return nil
}

func (s *KubeadmPrepareStackedEtcdCAAssetsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	
	etcdOldDir := filepath.Join(s.localOldCertsDir, "etcd")
	etcdNewDir := filepath.Join(s.localNewCertsDir, "etcd")

	logger.Warnf("Rolling back by removing stacked Etcd CA asset directories: '%s' and '%s'", etcdOldDir, etcdNewDir)
	_ = os.RemoveAll(etcdOldDir)
	_ = os.RemoveAll(etcdNewDir)

	return nil
}

var _ step.Step = (*KubeadmPrepareStackedEtcdCAAssetsStep)(nil)
