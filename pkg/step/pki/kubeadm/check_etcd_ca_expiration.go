package kubeadm

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmCheckEtcdCAExpirationStep struct {
	step.Base
	localCertsDir       string
	expirationThreshold time.Duration
	etcdCaCert          string
}

type KubeadmCheckEtcdCAExpirationStepBuilder struct {
	step.Builder[KubeadmCheckEtcdCAExpirationStepBuilder, *KubeadmCheckEtcdCAExpirationStep]
}

func NewKubeadmCheckEtcdCAExpirationStepBuilder(ctx runtime.Context, instanceName string) *KubeadmCheckEtcdCAExpirationStepBuilder {
	s := &KubeadmCheckEtcdCAExpirationStep{
		localCertsDir:       filepath.Join(ctx.GetKubernetesCertsDir(), ctx.GetHost().GetName()),
		expirationThreshold: DefaultK8sCertExpirationThreshold,
		etcdCaCert:          "etcd/ca.crt",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Check the expiration of the stacked Etcd CA certificate (etcd/ca.crt)"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(KubeadmCheckEtcdCAExpirationStepBuilder).Init(s)
	return b
}

func (s *KubeadmCheckEtcdCAExpirationStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmCheckEtcdCAExpirationStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck: verifying certificate directory existence...")

	etcdCertsDir := filepath.Dir(filepath.Join(s.localCertsDir, s.etcdCaCert))
	if _, err := os.Stat(etcdCertsDir); os.IsNotExist(err) {
		logger.Errorf("Etcd certificate directory '%s' does not exist.", etcdCertsDir)
		return false, fmt.Errorf("precheck failed: etcd certificate directory '%s' does not exist", etcdCertsDir)
	}

	logger.Info("Precheck passed: etcd certificate directory found.")
	return false, nil
}

func (s *KubeadmCheckEtcdCAExpirationStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Checking stacked Etcd CA certificate expiration...")

	fullPath := filepath.Join(s.localCertsDir, s.etcdCaCert)
	log := logger.With("certificate", s.etcdCaCert, "path", fullPath)

	certData, err := os.ReadFile(fullPath)
	if err != nil {
		log.Errorf("Stacked Etcd CA certificate not found or is unreadable. Error: %v", err)
		return fmt.Errorf("failed to read stacked Etcd CA certificate file '%s': %w", fullPath, err)
	}

	pemBlock, _ := pem.Decode(certData)
	if pemBlock == nil {
		log.Error("Failed to decode PEM block from Etcd CA certificate file.")
		return fmt.Errorf("failed to decode PEM block from Etcd CA certificate file '%s'", fullPath)
	}

	cert, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		log.Errorf("Failed to parse Etcd CA certificate. Error: %v", err)
		return fmt.Errorf("failed to parse Etcd CA certificate from '%s': %w", fullPath, err)
	}

	var etcdCaRequiresRenewal bool
	remaining := time.Until(cert.NotAfter)
	remainingDays := int(remaining.Hours() / 24)
	log.Infof("Certificate valid until: %s (%d days remaining)", cert.NotAfter.Format("2006-01-02 15:04:05 MST"), remainingDays)

	if remaining <= 0 {
		log.Errorf("FATAL: Certificate has EXPIRED on %s.", cert.NotAfter.Format("2006-01-02"))
		etcdCaRequiresRenewal = true
	} else if remaining < s.expirationThreshold {
		log.Warnf("Certificate is expiring soon. Renewal is required.")
		etcdCaRequiresRenewal = true
	}

	var previousRenewalRequired bool
	if rawValue, ok := ctx.GetModuleCache().Get(common.CacheKubeadmEtcdCACertRenew); ok {
		if val, isBool := rawValue.(bool); isBool {
			previousRenewalRequired = val
		} else {
			log.Errorf("Cache corruption: expected a bool for key '%s', but got %T", common.CacheKubeadmEtcdCACertRenew, rawValue)
			return fmt.Errorf("cache corruption: value for key '%s' is not a boolean", common.CacheKubeadmEtcdCACertRenew)
		}
	}

	finalRenewalRequired := previousRenewalRequired || etcdCaRequiresRenewal

	ctx.GetTaskCache().Set(common.CacheKubeadmEtcdCACertRenew, finalRenewalRequired)
	ctx.GetModuleCache().Set(common.CacheKubeadmEtcdCACertRenew, finalRenewalRequired)
	ctx.GetPipelineCache().Set(common.CacheKubeadmEtcdCACertRenew, finalRenewalRequired)
	log.Infof("Etcd CA check complete. This CA requires renewal: %v. Cumulative CA renewal required: %v", etcdCaRequiresRenewal, finalRenewalRequired)

	return nil
}

func (s *KubeadmCheckEtcdCAExpirationStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a check-only step. Nothing to do.")
	return nil
}

var _ step.Step = (*KubeadmCheckEtcdCAExpirationStep)(nil)
