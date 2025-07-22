package etcd

import (
	"fmt"
	"io/fs" // 引入 fs 包
	"os"
	"path/filepath"
	"strconv" // 引入 strconv 包
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers" // 假设 pki 在 helpers 中
)

type GenerateEtcdCAStep struct {
	step.Base
	LocalCertsDir string
	CADuration    time.Duration
	Permission    string
}

type GenerateEtcdCAStepBuilder struct {
	step.Builder[GenerateEtcdCAStepBuilder, *GenerateEtcdCAStep]
}

func NewGenerateEtcdCAStepBuilder(ctx runtime.Context, instanceName string) *GenerateEtcdCAStepBuilder {
	s := &GenerateEtcdCAStep{
		LocalCertsDir: filepath.Join(ctx.GetGlobalWorkDir(), "certs", "etcd"),
		CADuration:    10 * 365 * 24 * time.Hour,
		Permission:    "0755",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate or load etcd CA for the cluster", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(GenerateEtcdCAStepBuilder).Init(s)
	return b
}

func (b *GenerateEtcdCAStepBuilder) WithLocalCertsDir(path string) *GenerateEtcdCAStepBuilder {
	b.Step.LocalCertsDir = path
	return b
}

func (b *GenerateEtcdCAStepBuilder) WithCADuration(duration time.Duration) *GenerateEtcdCAStepBuilder {
	b.Step.CADuration = duration
	return b
}

func (b *GenerateEtcdCAStepBuilder) WithPermission(permission string) *GenerateEtcdCAStepBuilder {
	b.Step.Permission = permission
	return b
}

func (s *GenerateEtcdCAStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateEtcdCAStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	caCertPath := filepath.Join(s.LocalCertsDir, common.EtcdCaPemFileName)
	caKeyPath := filepath.Join(s.LocalCertsDir, common.EtcdCaKeyPemFileName)

	if _, err := os.Stat(caCertPath); os.IsNotExist(err) {
		logger.Info("ETCD CA certificate not found. Generation is required.")
		return false, nil
	}
	if _, err := os.Stat(caKeyPath); os.IsNotExist(err) {
		logger.Info("ETCD CA private key not found. Generation is required.")
		return false, nil
	}

	logger.Info("ETCD CA certificate and key already exist. Step is done.")
	return true, nil
}

func (s *GenerateEtcdCAStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	perm, err := strconv.ParseUint(s.Permission, 8, 32)
	if err != nil {
		return fmt.Errorf("invalid permission format '%s': %w", s.Permission, err)
	}
	if err := os.MkdirAll(s.LocalCertsDir, fs.FileMode(perm)); err != nil {
		return fmt.Errorf("failed to create local certs directory %s: %w", s.LocalCertsDir, err)
	}

	logger.Info("Ensuring ETCD CA certificate and key exist...")
	_, _, err = helpers.NewCertificateAuthority(s.LocalCertsDir, common.EtcdCaPemFileName, common.EtcdCaKeyPemFileName, s.CADuration)
	if err != nil {
		return fmt.Errorf("failed to setup ETCD CA: %w", err)
	}

	logger.Info("ETCD CA is ready.")
	return nil
}

func (s *GenerateEtcdCAStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	caCertPath := filepath.Join(s.LocalCertsDir, common.EtcdCaPemFileName)
	caKeyPath := filepath.Join(s.LocalCertsDir, common.EtcdCaKeyPemFileName)

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

var _ step.Step = (*GenerateEtcdCAStep)(nil)
