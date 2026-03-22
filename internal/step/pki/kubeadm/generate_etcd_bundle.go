package kubeadm

import (
	"bytes"
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

type KubeadmPrepareStackedEtcdCATransitionStep struct {
	step.Base
	localKubeCertsDir string
	localOldCertsDir  string
	localNewCertsDir  string
	etcdCaAsset       caAsset
}

type KubeadmPrepareStackedEtcdCATransitionStepBuilder struct {
	step.Builder[KubeadmPrepareStackedEtcdCATransitionStepBuilder, *KubeadmPrepareStackedEtcdCATransitionStep]
}

func NewKubeadmPrepareStackedEtcdCATransitionStepBuilder(ctx runtime.ExecutionContext, instanceName string) *KubeadmPrepareStackedEtcdCATransitionStepBuilder {
	localCertsDir := ctx.GetKubernetesCertsDir()
	certsOldDir := filepath.Join(localCertsDir, "certs-old")
	certsNewDir := filepath.Join(localCertsDir, "certs-new")

	s := &KubeadmPrepareStackedEtcdCATransitionStep{
		localKubeCertsDir: localCertsDir,
		localOldCertsDir:  certsOldDir,
		localNewCertsDir:  certsNewDir,
		etcdCaAsset:       caAsset{CertFile: "etcd/ca.crt"},
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Prepare stacked Etcd CA transition by creating and activating its CA bundle"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(KubeadmPrepareStackedEtcdCATransitionStepBuilder).Init(s)
	return b
}

func (s *KubeadmPrepareStackedEtcdCATransitionStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmPrepareStackedEtcdCATransitionStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	oldCertPath := filepath.Join(s.localOldCertsDir, s.etcdCaAsset.CertFile)
	newCertPath := filepath.Join(s.localNewCertsDir, s.etcdCaAsset.CertFile)
	activeCertPath := filepath.Join(s.localKubeCertsDir, s.etcdCaAsset.CertFile)

	if !helpers.IsFileExist(oldCertPath) || !helpers.IsFileExist(newCertPath) {
		return false, fmt.Errorf("required old/new Etcd CA files for '%s' not found. Ensure previous steps ran", s.etcdCaAsset.CertFile)
	}

	expectedBundleData, err := createBundleData(oldCertPath, newCertPath)
	if err != nil {
		return false, fmt.Errorf("precheck failed to create expected bundle data for Etcd CA: %w", err)
	}

	currentData, err := os.ReadFile(activeCertPath)
	if err != nil {
		return false, nil
	}

	if bytes.Equal(currentData, expectedBundleData) {
		logger.Info("Active Etcd CA is already a CA bundle. Step is done.")
		return true, nil
	}

	return false, nil
}

func (s *KubeadmPrepareStackedEtcdCATransitionStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Creating and activating stacked Etcd CA bundle...")

	oldCertPath := filepath.Join(s.localOldCertsDir, s.etcdCaAsset.CertFile)
	newCertPath := filepath.Join(s.localNewCertsDir, s.etcdCaAsset.CertFile)
	activeCertPath := filepath.Join(s.localKubeCertsDir, s.etcdCaAsset.CertFile)

	bundleData, err := createBundleData(oldCertPath, newCertPath)
	if err != nil {
		result.MarkFailed(err, "failed to create Etcd CA bundle data")
		return result, err
	}

	log := logger.With("ca_name", s.etcdCaAsset.CertFile)
	log.Infof("Activating Etcd CA bundle by overwriting '%s'", activeCertPath)
	if err := os.WriteFile(activeCertPath, bundleData, 0644); err != nil {
		result.MarkFailed(err, "failed to activate bundle for Etcd CA")
		return result, err
	}

	logger.Info("Stacked Etcd CA bundle activated successfully.")
	result.MarkCompleted("Etcd CA bundle activated successfully")
	return result, nil
}

func (s *KubeadmPrepareStackedEtcdCATransitionStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rolling back by restoring original stacked Etcd CA from 'certs-old' backup...")

	oldCertPath := filepath.Join(s.localOldCertsDir, s.etcdCaAsset.CertFile)
	activeCertPath := filepath.Join(s.localKubeCertsDir, s.etcdCaAsset.CertFile)

	if !helpers.IsFileExist(oldCertPath) {
		logger.Warnf("Old Etcd CA backup '%s' not found, cannot perform a clean rollback.", oldCertPath)
		return nil
	}

	if err := helpers.CopyFile(oldCertPath, activeCertPath); err != nil {
		logger.Errorf("CRITICAL: Failed to restore '%s'. Manual intervention may be required. Error: %v", activeCertPath, err)
	}

	return nil
}

var _ step.Step = (*KubeadmPrepareStackedEtcdCATransitionStep)(nil)
