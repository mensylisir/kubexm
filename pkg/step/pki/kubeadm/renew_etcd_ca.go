package kubeadm

import (
	"crypto/x509/pkix"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

type KubeadmRenewStackedEtcdCAStep struct {
	step.Base
	localNewCertsDir string
	etcdCaAsset      caAsset
	validity         time.Duration
}

type KubeadmRenewStackedEtcdCAStepBuilder struct {
	step.Builder[KubeadmRenewStackedEtcdCAStepBuilder, *KubeadmRenewStackedEtcdCAStep]
}

func NewKubeadmRenewStackedEtcdCAStepBuilder(ctx runtime.Context, instanceName string) *KubeadmRenewStackedEtcdCAStepBuilder {
	localCertsDir := ctx.GetKubernetesCertsDir()
	certsNewDir := filepath.Join(localCertsDir, "certs-new")

	s := &KubeadmRenewStackedEtcdCAStep{
		localNewCertsDir: certsNewDir,
		etcdCaAsset: caAsset{
			CertFile: "etcd/ca.crt",
			KeyFile:  "etcd/ca.key",
		},
		validity: DefaultCAValidity,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate a new stacked Etcd CA certificate using the existing private key"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(KubeadmRenewStackedEtcdCAStepBuilder).Init(s)
	return b
}

func (s *KubeadmRenewStackedEtcdCAStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmRenewStackedEtcdCAStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	keyPath := filepath.Join(s.localNewCertsDir, s.etcdCaAsset.KeyFile)
	certPath := filepath.Join(s.localNewCertsDir, s.etcdCaAsset.CertFile)

	if !helpers.IsFileExist(keyPath) {
		return false, fmt.Errorf("required Etcd CA private key '%s' for renewal is missing", keyPath)
	}

	if helpers.IsFileExist(certPath) {
		logger.Infof("New Etcd CA certificate '%s' already exists. Step is done.", certPath)
		return true, nil
	}
	return false, nil
}

func (s *KubeadmRenewStackedEtcdCAStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Infof("Starting renewal of stacked Etcd CA certificate...")

	keyPath := filepath.Join(s.localNewCertsDir, s.etcdCaAsset.KeyFile)
	certPath := filepath.Join(s.localNewCertsDir, s.etcdCaAsset.CertFile)

	subject := pkix.Name{
		CommonName: "etcd-ca",
	}

	if err := generateNewCACert(keyPath, certPath, subject, s.validity); err != nil {
		logger.Errorf("Failed to generate new Etcd CA certificate: %v", err)
		return fmt.Errorf("failed to generate new Etcd CA certificate: %w", err)
	}

	logger.Info("Successfully generated new stacked Etcd CA certificate.")
	return nil
}

func (s *KubeadmRenewStackedEtcdCAStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rolling back by deleting newly generated Etcd CA certificate from 'certs-new' directory...")

	newCertPath := filepath.Join(s.localNewCertsDir, s.etcdCaAsset.CertFile)
	_ = os.Remove(newCertPath)

	return nil
}

var _ step.Step = (*KubeadmRenewStackedEtcdCAStep)(nil)
