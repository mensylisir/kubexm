package kube_proxy

import (
	"crypto/x509"
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

type GenerateKubeProxyCertsStep struct {
	step.Base
	CertsDir     string
	CertDuration time.Duration
}

type GenerateKubeProxyCertsStepBuilder struct {
	step.Builder[GenerateKubeProxyCertsStepBuilder, *GenerateKubeProxyCertsStep]
}

func NewGenerateKubeProxyCertsStepBuilder(ctx runtime.Context, instanceName string) *GenerateKubeProxyCertsStepBuilder {
	s := &GenerateKubeProxyCertsStep{
		CertsDir:     ctx.GetKubernetesCertsDir(),
		CertDuration: common.DefaultCertificateValidityDays * 24 * time.Hour,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate client certificate for kube-proxy", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(GenerateKubeProxyCertsStepBuilder).Init(s)
	return b
}

func (s *GenerateKubeProxyCertsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateKubeProxyCertsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	if !helpers.FileExists(s.CertsDir, common.KubeProxyClientCertFileName) || !helpers.FileExists(s.CertsDir, common.KubeProxyClientKeyFileName) {
		return false, nil
	}
	ctx.GetLogger().Info("Kube-proxy client certificate already exists. Step is done.")
	return true, nil
}

func (s *GenerateKubeProxyCertsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	logger.Info("Loading main Kubernetes CA...")
	mainCA, mainKey, err := helpers.LoadCertificateAuthority(filepath.Join(s.CertsDir, common.CACertFileName), filepath.Join(s.CertsDir, common.CAKeyFileName))
	if err != nil {
		return fmt.Errorf("failed to load main kubernetes CA: %w", err)
	}

	logger.Info("Generating kube-proxy client certificate...")
	kubeProxyClientCfg := helpers.CertConfig{
		CommonName:   common.KubeProxyUser,
		Organization: []string{common.SystemNodeProxierOrganization},
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		Duration:     s.CertDuration,
	}

	if err := helpers.NewSignedCertificate(s.CertsDir, common.KubeProxyClientCertFileName, common.KubeProxyClientKeyFileName, kubeProxyClientCfg, mainCA, mainKey); err != nil {
		return fmt.Errorf("failed to generate kube-proxy client certificate: %w", err)
	}

	logger.Info("Kube-proxy client certificate generated successfully.")
	return nil
}

func (s *GenerateKubeProxyCertsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	logger.Warnf("Rolling back by deleting kube-proxy certificate and key...")
	_ = os.Remove(filepath.Join(s.CertsDir, common.KubeProxyClientCertFileName))
	_ = os.Remove(filepath.Join(s.CertsDir, common.KubeProxyClientKeyFileName))

	return nil
}

var _ step.Step = (*GenerateKubeProxyCertsStep)(nil)
