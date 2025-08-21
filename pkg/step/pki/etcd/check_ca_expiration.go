package etcd

import (
	"crypto/x509"
	"encoding/pem"
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

const (
	DefaultCertExpirationThreshold = 180 * 24 * time.Hour
)

type CheckCAExpirationStep struct {
	step.Base
	localCaCertPath     string
	ExpirationThreshold time.Duration
}

type CheckCAExpirationStepBuilder struct {
	step.Builder[CheckCAExpirationStepBuilder, *CheckCAExpirationStep]
}

func NewCheckCAExpirationStepBuilder(ctx runtime.Context, instanceName string) *CheckCAExpirationStepBuilder {
	localCertsDir := ctx.GetEtcdCertsDir()
	s := &CheckCAExpirationStep{
		localCaCertPath:     filepath.Join(localCertsDir, common.EtcdCaPemFileName),
		ExpirationThreshold: DefaultCertExpirationThreshold,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Check the expiration of the etcd CA certificate in the local workspace"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute
	b := new(CheckCAExpirationStepBuilder).Init(s)
	return b
}

func (s *CheckCAExpirationStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CheckCAExpirationStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Debugf("Checking CA expiration")
	if !helpers.IsFileExist(s.localCaCertPath) {
		logger.Warnf("CA certificate not found. Assuming that it requires renewal.")
		return false, fmt.Errorf("etcd CA certificate not found at '%s'", s.localCaCertPath)
	}
	return false, nil
}

func (s *CheckCAExpirationStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	logger.Infof("Checking expiration for CA certificate: %s", s.localCaCertPath)

	certData, err := os.ReadFile(s.localCaCertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate file: %w", err)
	}

	pemBlock, _ := pem.Decode(certData)
	if pemBlock == nil {
		return fmt.Errorf("failed to decode PEM block from CA certificate file")
	}

	cert, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	remaining := time.Until(cert.NotAfter)
	remainingDays := int(remaining.Hours() / 24)

	logger.Infof("CA certificate expires on: %s (in %d days)", cert.NotAfter.Format("2006-01-02"), remainingDays)

	requiresRenewal := false

	if remaining <= 0 {
		errMsg := fmt.Sprintf("FATAL: ETCD CA certificate has already expired on %s", cert.NotAfter.Format("2006-01-02"))
		logger.Error(nil, errMsg)
	} else if remaining < s.ExpirationThreshold {
		logger.Warnf("CA certificate is expiring soon (in %d days). Renewal is required.", remainingDays)
		requiresRenewal = true
	} else {
		logger.Info("CA certificate validity is sufficient. No renewal required.")
	}
	ctx.GetTaskCache().Set(common.CacheKubexmEtcdCACertRenew, requiresRenewal)
	ctx.GetModuleCache().Set(common.CacheKubexmEtcdCACertRenew, requiresRenewal)
	ctx.GetPipelineCache().Set(common.CacheKubexmEtcdCACertRenew, requiresRenewal)
	logger.Infof("Result 'ca_requires_renewal' (%v) has been saved to the pipeline cache.", requiresRenewal)

	return nil
}

func (s *CheckCAExpirationStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Debugf("No rollback required for CheckCAExpirationStep")
	return nil
}

var _ step.Step = (*CheckCAExpirationStep)(nil)
