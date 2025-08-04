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

type GenerateCAPEMStep struct {
	step.Base
	LocalCertsDir string
	CADuration    time.Duration
	Permission    string
	CertFileName  string
	KeyFileName   string
}

type GenerateCAPEMStepBuilder struct {
	step.Builder[GenerateCAPEMStepBuilder, *GenerateCAPEMStep]
}

func NewGenerateCAPEMStepBuilder(ctx runtime.Context, instanceName string) *GenerateCAPEMStepBuilder {
	s := &GenerateCAPEMStep{
		LocalCertsDir: ctx.GetEtcdCertsDir(),
		CADuration:    10 * 365 * 24 * time.Hour,
		Permission:    "0755",
		CertFileName:  common.EtcdCaPemFileName,
		KeyFileName:   common.EtcdCaKeyPemFileName,
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

	b := new(GenerateCAPEMStepBuilder).Init(s)
	return b
}

func (b *GenerateCAPEMStepBuilder) WithCertFileName(name string) *GenerateCAPEMStepBuilder {
	b.Step.CertFileName = name
	return b
}

func (b *GenerateCAPEMStepBuilder) WithKeyFileName(name string) *GenerateCAPEMStepBuilder {
	b.Step.KeyFileName = name
	return b
}

func (b *GenerateCAPEMStepBuilder) WithLocalCertsDir(path string) *GenerateCAPEMStepBuilder {
	b.Step.LocalCertsDir = path
	return b
}

func (b *GenerateCAPEMStepBuilder) WithCADuration(duration time.Duration) *GenerateCAPEMStepBuilder {
	b.Step.CADuration = duration
	return b
}

func (b *GenerateCAPEMStepBuilder) WithPermission(permission string) *GenerateCAPEMStepBuilder {
	b.Step.Permission = permission
	return b
}

func (s *GenerateCAPEMStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateCAPEMStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

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

func (s *GenerateCAPEMStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

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

func (s *GenerateCAPEMStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

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

var _ step.Step = (*GenerateCAPEMStep)(nil)
