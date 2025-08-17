package kubexm

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

type caAsset struct {
	CertFile string
	KeyFile  string
}

type KubexmPrepareCAAssetsStep struct {
	step.Base
	localKubeCertsDir string
	localOldCertsDir  string
	localNewCertsDir  string
	caAssetsToBackup  []caAsset
}

type KubexmPrepareCAAssetsStepBuilder struct {
	step.Builder[KubexmPrepareCAAssetsStepBuilder, *KubexmPrepareCAAssetsStep]
}

func NewKubexmPrepareCAAssetsStepBuilder(ctx runtime.Context, instanceName string) *KubexmPrepareCAAssetsStepBuilder {
	localCertsDir := ctx.GetKubernetesCertsDir()
	certsOldDir := filepath.Join(localCertsDir, "certs-old")
	certsNewDir := filepath.Join(localCertsDir, "certs-new")

	assets := []caAsset{
		{CertFile: "ca.crt", KeyFile: "ca.key"},
		{CertFile: "front-proxy-ca.crt", KeyFile: "front-proxy-ca.key"},
	}

	s := &KubexmPrepareCAAssetsStep{
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

	b := new(KubexmPrepareCAAssetsStepBuilder).Init(s)
	return b
}

func (s *KubexmPrepareCAAssetsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubexmPrepareCAAssetsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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

func (s *KubexmPrepareCAAssetsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	logger.Info("Ensuring clean state by removing previous backup/working directories...")
	_ = os.RemoveAll(s.localOldCertsDir)
	_ = os.RemoveAll(s.localNewCertsDir)

	logger.Infof("Creating backup directory for old CAs: '%s'", s.localOldCertsDir)
	if err := os.MkdirAll(s.localOldCertsDir, 0755); err != nil {
		return fmt.Errorf("failed to create certs-old directory: %w", err)
	}

	logger.Infof("Creating working directory for new CAs: '%s'", s.localNewCertsDir)
	if err := os.MkdirAll(s.localNewCertsDir, 0755); err != nil {
		return fmt.Errorf("failed to create certs-new directory: %w", err)
	}

	for _, asset := range s.caAssetsToBackup {
		sourceCert := filepath.Join(s.localKubeCertsDir, asset.CertFile)
		sourceKey := filepath.Join(s.localKubeCertsDir, asset.KeyFile)
		destOldCert := filepath.Join(s.localOldCertsDir, asset.CertFile)
		destOldKey := filepath.Join(s.localOldCertsDir, asset.KeyFile)
		destNewKey := filepath.Join(s.localNewCertsDir, asset.KeyFile)

		if err := os.MkdirAll(filepath.Dir(destOldCert), 0755); err != nil {
			return fmt.Errorf("failed to create subdirectory for backup: %w", err)
		}

		log := logger.With("asset", asset.CertFile)
		log.Info("Backing up original certificate and key to 'certs-old'...")
		if err := helpers.CopyFile(sourceCert, destOldCert); err != nil {
			return fmt.Errorf("failed to copy '%s' to backup directory: %w", asset.CertFile, err)
		}
		if err := helpers.CopyFile(sourceKey, destOldKey); err != nil {
			return fmt.Errorf("failed to copy '%s' to backup directory: %w", asset.KeyFile, err)
		}

		log.Info("Staging private key into 'certs-new' for renewal step...")
		if err := os.MkdirAll(filepath.Dir(destNewKey), 0755); err != nil {
			return fmt.Errorf("failed to create subdirectory for backup: %w", err)
		}
		if err := helpers.CopyFile(sourceKey, destNewKey); err != nil {
			return fmt.Errorf("failed to copy '%s' to backup directory: %w", asset.KeyFile, err)
		}
	}

	logger.Info("Successfully prepared and backed up CA assets.")
	return nil
}

func (s *KubexmPrepareCAAssetsStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*KubexmPrepareCAAssetsStep)(nil)
