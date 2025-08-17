package kubeadm

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	CacheKeyK8sCARequiresRenewal      = "kubeadm_ca_requires_renewal"
	DefaultK8sCertExpirationThreshold = 180 * 24 * time.Hour
)

type KubeadmCheckK8sCAExpirationStep struct {
	step.Base
	localCertsDir       string
	expirationThreshold time.Duration
	k8sCaCerts          []string
}

type KubeadmCheckK8sCAExpirationStepBuilder struct {
	step.Builder[KubeadmCheckK8sCAExpirationStepBuilder, *KubeadmCheckK8sCAExpirationStep]
}

func NewKubeadmCheckK8sCAExpirationStepBuilder(ctx runtime.Context, instanceName string) *KubeadmCheckK8sCAExpirationStepBuilder {
	s := &KubeadmCheckK8sCAExpirationStep{
		localCertsDir:       filepath.Join(ctx.GetKubernetesCertsDir(), ctx.GetHost().GetName()),
		expirationThreshold: DefaultK8sCertExpirationThreshold,
		k8sCaCerts: []string{
			"ca.crt",
			"front-proxy-ca.crt",
		},
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Check the expiration of core Kubernetes CA certificates (ca.crt, front-proxy-ca.crt)"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(KubeadmCheckK8sCAExpirationStepBuilder).Init(s)
	return b
}

func (s *KubeadmCheckK8sCAExpirationStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmCheckK8sCAExpirationStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck: verifying certificate directory existence...")

	if _, err := os.Stat(s.localCertsDir); os.IsNotExist(err) {
		logger.Errorf("Certificate directory '%s' does not exist.", s.localCertsDir)
		return false, fmt.Errorf("precheck failed: certificate directory '%s' does not exist", s.localCertsDir)
	}

	logger.Info("Precheck passed: certificate directory found.")
	return false, nil
}

func (s *KubeadmCheckK8sCAExpirationStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Checking core Kubernetes CA certificates expiration...")

	var anyCARequiresRenewal bool

	for _, certFile := range s.k8sCaCerts {
		fullPath := filepath.Join(s.localCertsDir, certFile)
		log := logger.With("certificate", certFile, "path", fullPath)

		certData, err := os.ReadFile(fullPath)
		if err != nil {
			log.Errorf("Critical CA certificate not found or is unreadable. Error: %v", err)
			return fmt.Errorf("failed to read critical CA certificate file '%s': %w", fullPath, err)
		}

		pemBlock, _ := pem.Decode(certData)
		if pemBlock == nil {
			log.Error("Failed to decode PEM block from critical CA certificate file.")
			return fmt.Errorf("failed to decode PEM block from critical CA certificate file '%s'", fullPath)
		}

		cert, err := x509.ParseCertificate(pemBlock.Bytes)
		if err != nil {
			log.Errorf("Failed to parse critical CA certificate. Error: %v", err)
			return fmt.Errorf("failed to parse critical CA certificate from '%s': %w", fullPath, err)
		}

		remaining := time.Until(cert.NotAfter)
		remainingDays := int(remaining.Hours() / 24)
		log.Infof("Certificate valid until: %s (%d days remaining)", cert.NotAfter.Format("2006-01-02 15:04:05 MST"), remainingDays)

		if remaining <= 0 {
			log.Errorf("FATAL: Certificate has EXPIRED on %s.", cert.NotAfter.Format("2006-01-02"))
			anyCARequiresRenewal = true
		} else if remaining < s.expirationThreshold {
			log.Warnf("Certificate is expiring soon. Renewal is required.")
			anyCARequiresRenewal = true
		}
	}

	ctx.GetModuleCache().Set(CacheKeyK8sCARequiresRenewal, anyCARequiresRenewal)
	logger.Infof("Core K8s CA check complete. Renewal required: %v", anyCARequiresRenewal)

	return nil
}

func (s *KubeadmCheckK8sCAExpirationStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a check-only step. Nothing to do.")
	return nil
}

var _ step.Step = (*KubeadmCheckK8sCAExpirationStep)(nil)
