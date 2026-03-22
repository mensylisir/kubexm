package kubeadm

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/types"
)

type caAsset struct {
	CertFile string
	KeyFile  string
}

type KubeadmPrepareCAAssetsStep struct {
	step.Base
	localKubeCertsDir string
	localOldCertsDir  string
	localNewCertsDir  string
	caAssetsToBackup  []caAsset
}

type KubeadmPrepareCAAssetsStepBuilder struct {
	step.Builder[KubeadmPrepareCAAssetsStepBuilder, *KubeadmPrepareCAAssetsStep]
}

func NewKubeadmPrepareCAAssetsStepBuilder(ctx runtime.ExecutionContext, instanceName string) *KubeadmPrepareCAAssetsStepBuilder {
	localCertsDir := ctx.GetKubernetesCertsDir()
	certsOldDir := filepath.Join(localCertsDir, "certs-old")
	certsNewDir := filepath.Join(localCertsDir, "certs-new")

	assets := []caAsset{
		{CertFile: "ca.crt", KeyFile: "ca.key"},
		{CertFile: "front-proxy-ca.crt", KeyFile: "front-proxy-ca.key"},
	}

	s := &KubeadmPrepareCAAssetsStep{
		localKubeCertsDir: localCertsDir,
		localOldCertsDir:  certsOldDir,
		localNewCertsDir:  certsNewDir,
		caAssetsToBackup:  assets,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Prepare assets by backing up original Kubeadm CAs to a 'certs-old' directory"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(KubeadmPrepareCAAssetsStepBuilder).Init(s)
	return b
}

func (s *KubeadmPrepareCAAssetsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmPrepareCAAssetsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	for _, asset := range s.caAssetsToBackup {
		originalCertPath := filepath.Join(s.localKubeCertsDir, asset.CertFile)
		originalKeyPath := filepath.Join(s.localKubeCertsDir, asset.KeyFile)
		if !helpers.IsFileExist(originalCertPath) || !helpers.IsFileExist(originalKeyPath) {
			return false, fmt.Errorf("original CA file '%s' or '%s' not found, cannot prepare assets", originalCertPath, originalKeyPath)
		}
	}

	primaryCaCert := filepath.Join(s.localKubeCertsDir, "ca.crt")
	backupCaCert := filepath.Join(s.localOldCertsDir, "ca.crt")

	if helpers.IsFileExist(backupCaCert) {
		sourceSum, err1 := helpers.GetFileSha256(primaryCaCert)
		backupSum, err2 := helpers.GetFileSha256(backupCaCert)
		if err1 == nil && err2 == nil && sourceSum == backupSum {
			logger.Info("CA assets have already been prepared and backed up. Step is done.")
			return true, nil
		}
	}

	return false, nil
}

func (s *KubeadmPrepareCAAssetsStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	logger.Info("Ensuring clean state by removing previous backup/working directories...")
	_ = os.RemoveAll(s.localOldCertsDir)
	_ = os.RemoveAll(s.localNewCertsDir)

	logger.Infof("Creating backup directory for old CAs: '%s'", s.localOldCertsDir)
	if err := os.MkdirAll(s.localOldCertsDir, 0755); err != nil {
		result.MarkFailed(err, "failed to create certs-old directory")
		return result, err
	}

	logger.Infof("Creating working directory for new CAs: '%s'", s.localNewCertsDir)
	if err := os.MkdirAll(s.localNewCertsDir, 0755); err != nil {
		result.MarkFailed(err, "failed to create certs-new directory")
		return result, err
	}

	for _, asset := range s.caAssetsToBackup {
		sourceCert := filepath.Join(s.localKubeCertsDir, asset.CertFile)
		sourceKey := filepath.Join(s.localKubeCertsDir, asset.KeyFile)
		destOldCert := filepath.Join(s.localOldCertsDir, asset.CertFile)
		destOldKey := filepath.Join(s.localOldCertsDir, asset.KeyFile)
		destNewKey := filepath.Join(s.localNewCertsDir, asset.KeyFile)

		if err := os.MkdirAll(filepath.Dir(destOldCert), 0755); err != nil {
			result.MarkFailed(err, "failed to create subdirectory for backup")
			return result, err
		}

		log := logger.With("asset", asset.CertFile)
		log.Info("Backing up original certificate and key to 'certs-old'...")
		if err := helpers.CopyFile(sourceCert, destOldCert); err != nil {
			result.MarkFailed(err, fmt.Sprintf("failed to copy '%s' to backup directory", asset.CertFile))
			return result, err
		}
		if err := helpers.CopyFile(sourceKey, destOldKey); err != nil {
			result.MarkFailed(err, fmt.Sprintf("failed to copy '%s' to backup directory", asset.KeyFile))
			return result, err
		}

		log.Info("Staging private key into 'certs-new' for renewal step...")
		if err := os.MkdirAll(filepath.Dir(destNewKey), 0755); err != nil {
			result.MarkFailed(err, "failed to create subdirectory for certs-new")
			return result, err
		}
		if err := helpers.CopyFile(sourceKey, destNewKey); err != nil {
			result.MarkFailed(err, fmt.Sprintf("failed to copy '%s' to certs-new directory", asset.KeyFile))
			return result, err
		}
	}

	logger.Info("Successfully prepared and backed up CA assets.")
	result.MarkCompleted("CA assets prepared successfully")
	return result, nil
}

func (s *KubeadmPrepareCAAssetsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warnf("Rolling back by deleting backup and working directories...")

	if err := os.RemoveAll(s.localOldCertsDir); err != nil {
		logger.Errorf("Failed to remove 'certs-old' directory '%s' during rollback: %v", s.localOldCertsDir, err)
	}
	if err := os.RemoveAll(s.localNewCertsDir); err != nil {
		logger.Errorf("Failed to remove 'certs-new' directory '%s' during rollback: %v", s.localNewCertsDir, err)
	}
	return nil
}

var _ step.Step = (*KubeadmPrepareCAAssetsStep)(nil)
