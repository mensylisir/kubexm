package common

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

type GenerateCAStep struct {
	step.Base
	LocalCertsDir string
	CADuration    time.Duration
	Permission    string
	CertFileName  string
	KeyFileName   string
}

type GenerateCAStepBuilder struct {
	step.Builder[GenerateCAStepBuilder, *GenerateCAStep]
}

func NewGenerateCAStepBuilder(ctx runtime.Context, instanceName string) *GenerateCAStepBuilder {
	s := &GenerateCAStep{
		LocalCertsDir: ctx.GetKubernetesCertsDir(),
		CADuration:    10 * 365 * 24 * time.Hour,
		Permission:    "0755",
		CertFileName:  common.CACertFileName,
		KeyFileName:   common.CAKeyFileName,
	}
	if ctx.GetClusterConfig().Spec.Certs.CADuration != "" {
		parsedDuration, err := time.ParseDuration(ctx.GetClusterConfig().Spec.Certs.CADuration)
		if err == nil {
			s.CADuration = parsedDuration
		} else {
			ctx.GetLogger().Warnf("Failed to parse user-provided CA duration '%s', using default. Error: %v", ctx.GetClusterConfig().Spec.Certs.CADuration, err)
		}
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate or load etcd CA for the cluster", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(GenerateCAStepBuilder).Init(s)
	return b
}

func (b *GenerateCAStepBuilder) WithCertFileName(name string) *GenerateCAStepBuilder {
	b.Step.CertFileName = name
	return b
}

func (b *GenerateCAStepBuilder) WithKeyFileName(name string) *GenerateCAStepBuilder {
	b.Step.KeyFileName = name
	return b
}

func (b *GenerateCAStepBuilder) WithLocalCertsDir(path string) *GenerateCAStepBuilder {
	b.Step.LocalCertsDir = path
	return b
}

func (b *GenerateCAStepBuilder) WithCADuration(duration time.Duration) *GenerateCAStepBuilder {
	b.Step.CADuration = duration
	return b
}

func (b *GenerateCAStepBuilder) WithPermission(permission string) *GenerateCAStepBuilder {
	b.Step.Permission = permission
	return b
}

func (s *GenerateCAStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateCAStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	caCertPath := filepath.Join(s.LocalCertsDir, s.CertFileName)
	caKeyPath := filepath.Join(s.LocalCertsDir, s.KeyFileName)

	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		logger.Info("CA certificate not found. Generation is required.")
		return false, nil
	}
	if _, err := os.Stat(caKeyPath); os.IsNotExist(err) {
		logger.Info("CA private key not found. Generation is required.")
		return false, nil
	}

	logger.Info("CA certificate and key already exist. Step is done.")
	return true, nil
}

func (s *GenerateCAStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	perm, err := strconv.ParseUint(s.Permission, 8, 32)
	if err != nil {
		return fmt.Errorf("invalid permission format '%s': %w", s.Permission, err)
	}
	if err := os.MkdirAll(s.LocalCertsDir, fs.FileMode(perm)); err != nil {
		return fmt.Errorf("failed to create local certs directory %s: %w", s.LocalCertsDir, err)
	}

	logger.Info("Ensuring CA certificate and key exist...")
	_, _, err = helpers.NewCertificateAuthority(s.LocalCertsDir, s.CertFileName, s.KeyFileName, s.CADuration)
	if err != nil {
		return fmt.Errorf("failed to setup ETCD CA: %w", err)
	}

	logger.Info("CA is ready.")
	return nil
}

func (s *GenerateCAStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	caCertPath := filepath.Join(s.LocalCertsDir, s.CertFileName)
	caKeyPath := filepath.Join(s.LocalCertsDir, s.KeyFileName)

	logger.Warnf("Rolling back by deleting CA certificate: %s", caCertPath)
	if err := os.Remove(caCertPath); err != nil && !os.IsNotExist(err) {
		logger.Errorf("Failed to remove CA certificate during rollback: %v", err)
	}

	logger.Warnf("Rolling back by deleting CA private key: %s", caKeyPath)
	if err := os.Remove(caKeyPath); err != nil && !os.IsNotExist(err) {
		logger.Errorf("Failed to remove CA private key during rollback: %v", err)
	}

	return nil
}

var _ step.Step = (*GenerateCAStep)(nil)
