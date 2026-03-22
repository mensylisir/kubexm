package kubeadm

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/types"
)

type KubeadmPrepareCATransitionStep struct {
	step.Base
	localKubeCertsDir string
	localOldCertsDir  string
	localNewCertsDir  string
	casToBundle       []caAsset
}

type KubeadmPrepareCATransitionStepBuilder struct {
	step.Builder[KubeadmPrepareCATransitionStepBuilder, *KubeadmPrepareCATransitionStep]
}

func NewKubeadmPrepareCATransitionStepBuilder(ctx runtime.ExecutionContext, instanceName string) *KubeadmPrepareCATransitionStepBuilder {
	localCertsDir := ctx.GetKubernetesCertsDir()
	certsOldDir := filepath.Join(localCertsDir, "certs-old")
	certsNewDir := filepath.Join(localCertsDir, "certs-new")

	assets := []caAsset{
		{CertFile: "ca.crt"},
		{CertFile: "front-proxy-ca.crt"},
	}

	if ctx.GetClusterConfig().Spec.Etcd.Type == string(common.EtcdDeploymentTypeKubeadm) {
		assets = append(assets, caAsset{CertFile: "etcd/ca.crt"})
	}

	s := &KubeadmPrepareCATransitionStep{
		localKubeCertsDir: localCertsDir,
		localOldCertsDir:  certsOldDir,
		localNewCertsDir:  certsNewDir,
		casToBundle:       assets,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Prepare CA transition by creating and activating CA bundles"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(KubeadmPrepareCATransitionStepBuilder).Init(s)
	return b
}

func (s *KubeadmPrepareCATransitionStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmPrepareCATransitionStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	for _, asset := range s.casToBundle {
		oldCertPath := filepath.Join(s.localOldCertsDir, asset.CertFile)
		newCertPath := filepath.Join(s.localNewCertsDir, asset.CertFile)
		activeCertPath := filepath.Join(s.localKubeCertsDir, asset.CertFile)

		if !helpers.IsFileExist(oldCertPath) || !helpers.IsFileExist(newCertPath) {
			return false, fmt.Errorf("required old/new CA files for '%s' not found. Ensure previous steps ran", asset.CertFile)
		}

		expectedBundleData, err := createBundleData(oldCertPath, newCertPath)
		if err != nil {
			return false, fmt.Errorf("precheck failed to create expected bundle data: %w", err)
		}

		currentData, err := os.ReadFile(activeCertPath)
		if err != nil {
			return false, nil // Active file missing, so we're not done.
		}

		if !bytes.Equal(currentData, expectedBundleData) {
			// If even one CA is not yet a bundle, the whole step needs to run.
			return false, nil
		}
	}

	logger.Info("All active CAs are already CA bundles. Step is done.")
	return true, nil
}

func (s *KubeadmPrepareCATransitionStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Creating and activating CA bundles for smooth transition...")

	for _, asset := range s.casToBundle {
		oldCertPath := filepath.Join(s.localOldCertsDir, asset.CertFile)
		newCertPath := filepath.Join(s.localNewCertsDir, asset.CertFile)
		activeCertPath := filepath.Join(s.localKubeCertsDir, asset.CertFile)

		log := logger.With("ca_name", asset.CertFile)

		bundleData, err := createBundleData(oldCertPath, newCertPath)
		if err != nil {
			result.MarkFailed(err, "failed to create bundle data")
			return result, err
		}

		log.Infof("Activating CA bundle by overwriting '%s' with its content", activeCertPath)
		if err := os.WriteFile(activeCertPath, bundleData, 0644); err != nil {
			result.MarkFailed(err, fmt.Sprintf("failed to activate bundle for '%s'", asset.CertFile))
			return result, err
		}
	}

	logger.Info("Kubernetes CA bundles activated successfully.")
	result.MarkCompleted("CA bundles activated successfully")
	return result, nil
}

func (s *KubeadmPrepareCATransitionStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rolling back by restoring original CAs from 'certs-old' backups...")

	for _, asset := range s.casToBundle {
		oldCertPath := filepath.Join(s.localOldCertsDir, asset.CertFile)
		activeCertPath := filepath.Join(s.localKubeCertsDir, asset.CertFile)

		if !helpers.IsFileExist(oldCertPath) {
			logger.Warnf("Old CA backup '%s' not found, cannot perform a clean rollback for this CA.", oldCertPath)
			continue
		}

		if err := helpers.CopyFile(oldCertPath, activeCertPath); err != nil {
			logger.Errorf("CRITICAL: Failed to restore '%s'. Manual intervention may be required. Error: %v", activeCertPath, err)
		}
	}
	return nil
}

func createBundleData(oldCertPath, newCertPath string) ([]byte, error) {
	oldCAData, err := os.ReadFile(oldCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read old CA from '%s': %w", oldCertPath, err)
	}
	newCAData, err := os.ReadFile(newCertPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read new CA from '%s': %w", newCertPath, err)
	}

	if len(oldCAData) > 0 && oldCAData[len(oldCAData)-1] != '\n' {
		oldCAData = append(oldCAData, '\n')
	}

	return append(oldCAData, newCAData...), nil
}

var _ step.Step = (*KubeadmPrepareCATransitionStep)(nil)
