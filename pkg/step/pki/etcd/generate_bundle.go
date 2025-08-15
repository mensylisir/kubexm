package etcd

import (
	"bytes"
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

type PrepareCATransitionStep struct {
	step.Base
	localCertsDir   string
	localCAPath     string
	localOldCAPath  string
	localNewCAPath  string
	localBundlePath string
}

func NewPrepareCATransitionStep(ctx runtime.Context, instanceName string) *PrepareCATransitionStep {
	localCertsDir := ctx.GetEtcdCertsDir()
	s := &PrepareCATransitionStep{
		localCertsDir:   localCertsDir,
		localCAPath:     filepath.Join(localCertsDir, common.EtcdCaPemFileName),
		localOldCAPath:  filepath.Join(localCertsDir, "certs-old", "ca.pem"),
		localNewCAPath:  filepath.Join(localCertsDir, "certs-new", "ca.pem"),
		localBundlePath: filepath.Join(localCertsDir, "ca-bundle.pem"),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Prepare CA files for transition by creating and activating a CA bundle"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	return s
}

func (s *PrepareCATransitionStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *PrepareCATransitionStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	if !helpers.IsFileExist(s.localOldCAPath) {
		return false, fmt.Errorf("source of old CA '%s' not found. Ensure PrepareAssetsStep ran", s.localOldCAPath)
	}
	if !helpers.IsFileExist(s.localNewCAPath) {
		return false, fmt.Errorf("source of new CA '%s' not found. Ensure ResignCAStep ran", s.localNewCAPath)
	}

	oldCAData, err := os.ReadFile(s.localOldCAPath)
	if err != nil {
		return false, fmt.Errorf("precheck failed to read old CA data: %w", err)
	}
	newCAData, err := os.ReadFile(s.localNewCAPath)
	if err != nil {
		return false, fmt.Errorf("precheck failed to read new CA data: %w", err)
	}

	if len(oldCAData) > 0 && oldCAData[len(oldCAData)-1] != '\n' {
		oldCAData = append(oldCAData, '\n')
	}
	expectedBundleData := append(oldCAData, newCAData...)

	currentCAData, err := os.ReadFile(s.localCAPath)
	if err == nil && bytes.Equal(currentCAData, expectedBundleData) {
		logger.Info("Current ca.pem is already the CA bundle. Step is done.")
		if !helpers.IsFileExist(s.localBundlePath) {
			_ = os.WriteFile(s.localBundlePath, expectedBundleData, 0644)
		}
		return true, nil
	}

	return false, nil
}

func (s *PrepareCATransitionStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	oldCAData, err := os.ReadFile(s.localOldCAPath)
	if err != nil {
		return fmt.Errorf("failed to read ca-old.pem: %w", err)
	}
	newCAData, err := os.ReadFile(s.localNewCAPath)
	if err != nil {
		return fmt.Errorf("failed to read ca-new.pem: %w", err)
	}

	if len(oldCAData) > 0 && oldCAData[len(oldCAData)-1] != '\n' {
		oldCAData = append(oldCAData, '\n')
	}
	bundleData := append(oldCAData, newCAData...)

	logger.Infof("Creating CA bundle file at '%s'", s.localBundlePath)
	if err := os.WriteFile(s.localBundlePath, bundleData, 0644); err != nil {
		return fmt.Errorf("failed to write ca-bundle.pem: %w", err)
	}

	logger.Infof("Activating CA bundle by writing its content to '%s'", s.localCAPath)
	if err := os.WriteFile(s.localCAPath, bundleData, 0644); err != nil {
		return fmt.Errorf("failed to write bundle content to ca.pem: %w", err)
	}

	logger.Info("CA bundle created and activated successfully.")
	return nil
}

func (s *PrepareCATransitionStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	if !helpers.IsFileExist(s.localOldCAPath) {
		logger.Warnf("Old CA backup '%s' not found, cannot perform a clean rollback.", s.localOldCAPath)
		return nil
	}

	logger.Warnf("Rolling back by restoring '%s' from '%s'", s.localCAPath, s.localOldCAPath)

	if err := helpers.CopyFile(s.localOldCAPath, s.localCAPath); err != nil {
		logger.Errorf("CRITICAL: Failed to restore ca.pem from '%s'. Manual intervention may be required. Error: %v", s.localOldCAPath, err)
	}

	if err := os.Remove(s.localBundlePath); err != nil && !os.IsNotExist(err) {
		logger.Warnf("Failed to remove ca-bundle.pem during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*PrepareCATransitionStep)(nil)
