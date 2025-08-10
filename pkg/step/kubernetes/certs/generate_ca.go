package certs

import (
	"crypto/x509/pkix"
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

type GenerateKubeCAStep struct {
	step.Base
	LocalCertsDir string
	CADuration    time.Duration
}

type GenerateKubeCAStepBuilder struct {
	step.Builder[GenerateKubeCAStepBuilder, *GenerateKubeCAStep]
}

func NewGenerateKubeCAStepBuilder(ctx runtime.Context, instanceName string) *GenerateKubeCAStepBuilder {
	s := &GenerateKubeCAStep{
		LocalCertsDir: ctx.GetKubernetesCertsDir(),
		CADuration:    common.TenYears * 24 * time.Hour,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate or load Kubernetes root CAs and service account keys", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(GenerateKubeCAStepBuilder).Init(s)
	return b
}

func (b *GenerateKubeCAStepBuilder) WithLocalCertsDir(path string) *GenerateKubeCAStepBuilder {
	b.Step.LocalCertsDir = path
	return b
}

func (b *GenerateKubeCAStepBuilder) WithCADuration(duration time.Duration) *GenerateKubeCAStepBuilder {
	b.Step.CADuration = duration
	return b
}

func (s *GenerateKubeCAStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateKubeCAStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	requiredFiles := []string{
		common.CACertFileName, common.CAKeyFileName,
		common.FrontProxyCACertFileName, common.FrontProxyCAKeyFileName,
		common.ServiceAccountPublicKeyFileName, common.ServiceAccountPrivateKeyFileName,
	}

	for _, file := range requiredFiles {
		if !helpers.FileExists(s.LocalCertsDir, file) {
			logger.Infof("Required PKI file not found: %s. Generation is required.", file)
			return false, nil
		}
	}

	logger.Info("All required Kubernetes root CAs and service account keys already exist. Step is done.")
	return true, nil
}

func (s *GenerateKubeCAStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	if err := os.MkdirAll(s.LocalCertsDir, common.DefaultCertificateDirPermission); err != nil {
		return fmt.Errorf("failed to create local certs directory %s: %w", s.LocalCertsDir, err)
	}

	logger.Info("Ensuring main Kubernetes CA exists...")
	mainCASubject := pkix.Name{CommonName: "kubernetes"}
	_, _, err := helpers.NewCertificateAuthorityWithSubject(s.LocalCertsDir, common.CACertFileName, common.CAKeyFileName, mainCASubject, s.CADuration)
	if err != nil {
		return fmt.Errorf("failed to setup main Kubernetes CA: %w", err)
	}

	logger.Info("Ensuring front-proxy CA exists...")
	fpCASubject := pkix.Name{CommonName: "front-proxy-ca"}
	_, _, err = helpers.NewCertificateAuthorityWithSubject(s.LocalCertsDir, common.FrontProxyCACertFileName, common.FrontProxyCAKeyFileName, fpCASubject, s.CADuration)
	if err != nil {
		return fmt.Errorf("failed to setup front-proxy CA: %w", err)
	}

	logger.Info("Ensuring service account key pair exists...")
	if err := helpers.NewServiceAccountKeyPair(s.LocalCertsDir, common.ServiceAccountPublicKeyFileName, common.ServiceAccountPrivateKeyFileName); err != nil {
		return fmt.Errorf("failed to generate service account key pair: %w", err)
	}

	logger.Info("All Kubernetes root CAs and service account keys are ready.")
	return nil
}

func (s *GenerateKubeCAStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	filesToDelete := []string{
		common.CACertFileName, common.CAKeyFileName,
		common.FrontProxyCACertFileName, common.FrontProxyCAKeyFileName,
		common.ServiceAccountPublicKeyFileName, common.ServiceAccountPrivateKeyFileName,
	}

	for _, file := range filesToDelete {
		path := filepath.Join(s.LocalCertsDir, file)
		logger.Warnf("Rolling back by deleting file: %s", path)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			logger.Errorf("Failed to remove file during rollback: %v", err)
		}
	}
	return nil
}

var _ step.Step = (*GenerateKubeCAStep)(nil)
