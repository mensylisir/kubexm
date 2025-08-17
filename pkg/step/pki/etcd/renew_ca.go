package etcd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

type ResignCAStep struct {
	step.Base
	localNewCertsDir string
	certDuration     time.Duration
}

type ResignCAStepBuilder struct {
	step.Builder[ResignCAStepBuilder, *ResignCAStep]
}

func NewResignCAStepBuilder(ctx runtime.Context, instanceName string) *ResignCAStepBuilder {
	localCertsDir := ctx.GetEtcdCertsDir()
	s := &ResignCAStep{
		localNewCertsDir: filepath.Join(localCertsDir, "certs-new"),
		certDuration:     10 * 365 * 24 * time.Hour,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Re-sign the CA certificate using the existing key in the certs-new directory"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	b := new(ResignCAStepBuilder).Init(s)
	return b
}

func (s *ResignCAStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ResignCAStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	newCaKeyPath := filepath.Join(s.localNewCertsDir, "ca-key.pem")
	newCaCertPath := filepath.Join(s.localNewCertsDir, "ca.pem")

	if !helpers.IsFileExist(newCaKeyPath) || !helpers.IsFileExist(newCaCertPath) {
		return false, fmt.Errorf("CA files not found in '%s'. Ensure PrepareAssetsStep ran successfully", s.localNewCertsDir)
	}

	cert, err := helpers.LoadCertificate(newCaCertPath)
	if err == nil {
		if time.Until(cert.NotAfter) > (s.certDuration - 24*time.Hour) {
			logger.Info("CA certificate in 'certs-new' directory has already been re-signed. Step is done.")
			return true, nil
		}
	}

	return false, nil
}

func (s *ResignCAStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	logger.Info("Re-signing CA certificate using key and cert from 'certs-new' directory...")
	_, _, err := helpers.ResignCertificateAuthority(s.localNewCertsDir, "ca.pem", "ca-key.pem", s.certDuration)
	if err != nil {
		return fmt.Errorf("failed to re-sign CA certificate: %w", err)
	}

	logger.Info("Successfully re-signed CA certificate and saved to 'certs-new' directory.")
	return nil
}

func (s *ResignCAStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for ResignCAStep is a no-op to preserve asset directories for potential manual recovery.")
	return nil
}

var _ step.Step = (*ResignCAStep)(nil)
