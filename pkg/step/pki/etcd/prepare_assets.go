package etcd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

type PrepareAssetsStep struct {
	step.Base
	localCertsDir     string
	localCAPath       string
	localCAKeyPath    string
	localOldCADir     string
	localOldCAPath    string
	localOldCAKeyPath string
	localNewCADir     string
	localNewCAPath    string
	localNewCAKeyPath string
}

func NewPrepareAssetsStep(ctx runtime.Context, instanceName string) *PrepareAssetsStep {
	localCertsDir := ctx.GetEtcdCertsDir()
	certsOldDir := filepath.Join(localCertsDir, "certs-old")
	certsNewDir := filepath.Join(localCertsDir, "certs-new")

	s := &PrepareAssetsStep{
		localCertsDir:     localCertsDir,
		localCAPath:       filepath.Join(localCertsDir, common.EtcdCaPemFileName),
		localCAKeyPath:    filepath.Join(localCertsDir, common.EtcdCaKeyPemFileName),
		localOldCADir:     certsOldDir,
		localOldCAPath:    filepath.Join(certsOldDir, common.EtcdCaPemFileName),
		localOldCAKeyPath: filepath.Join(certsOldDir, common.EtcdCaKeyPemFileName),
		localNewCADir:     certsNewDir,
		localNewCAPath:    filepath.Join(certsNewDir, common.EtcdCaPemFileName),
		localNewCAKeyPath: filepath.Join(certsNewDir, common.EtcdCaKeyPemFileName),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Prepare assets by copying the original local CA to the certs-old and certs-new directory"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return s
}

func (s *PrepareAssetsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *PrepareAssetsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	if !helpers.IsFileExist(s.localCAPath) || !helpers.IsFileExist(s.localCAKeyPath) {
		return false, fmt.Errorf("original CA files not found in '%s', cannot prepare old assets", s.localCertsDir)
	}

	if helpers.IsFileExist(s.localOldCAPath) && helpers.IsFileExist(s.localOldCAKeyPath) && helpers.IsFileExist(s.localNewCAPath) {
		sourceSum, err1 := helpers.GetFileSha256(s.localCAPath)
		stagedSum, err2 := helpers.GetFileSha256(s.localOldCAPath)
		if err1 == nil && err2 == nil && sourceSum == stagedSum {
			logger.Info("Old assets have already been prepared. Step is done.")
			return true, nil
		}
	}

	return false, nil
}

func (s *PrepareAssetsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	logger.Infof("Creating directory for old assets: '%s'", s.localOldCADir)
	if err := os.MkdirAll(s.localOldCADir, 0755); err != nil {
		return fmt.Errorf("failed to create certs-old directory: %w", err)
	}

	logger.Info("Copying original CA files to old assets directory...")
	if err := helpers.CopyFile(s.localCAPath, s.localOldCAPath); err != nil {
		return fmt.Errorf("failed to copy ca.pem to old assets directory: %w", err)
	}
	if err := helpers.CopyFile(s.localCAKeyPath, s.localOldCAKeyPath); err != nil {
		return fmt.Errorf("failed to copy ca-key.pem to old assets directory: %w", err)
	}

	logger.Infof("Creating directory for new assets: '%s'", s.localNewCADir)
	if err := os.MkdirAll(s.localNewCADir, 0755); err != nil {
		return fmt.Errorf("failed to create certs-new directory: %w", err)
	}

	logger.Info("Copying original CA files to new assets directory...")
	if err := helpers.CopyFile(s.localCAKeyPath, s.localNewCAKeyPath); err != nil {
		return fmt.Errorf("failed to copy ca-key.pem to new assets directory: %w", err)
	}

	logger.Info("Successfully prepared old assets.")
	return nil
}

func (s *PrepareAssetsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warnf("Rolling back by deleting old assets directory: %s", s.localOldCADir)

	if err := os.RemoveAll(s.localOldCADir); err != nil {
		logger.Errorf("Failed to remove old assets directory '%s' during rollback: %v", s.localOldCADir, err)
	}
	if err := os.RemoveAll(s.localNewCADir); err != nil {
		logger.Errorf("Failed to remove new assets directory '%s' during rollback: %v", s.localNewCADir, err)
	}
	return nil
}

var _ step.Step = (*PrepareAssetsStep)(nil)
