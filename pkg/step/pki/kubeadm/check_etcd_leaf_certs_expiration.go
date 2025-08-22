package kubeadm

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmCheckStackedEtcdLeafCertsExpirationStep struct {
	step.Base
	remoteKubePkiDir    string
	expirationThreshold time.Duration
	etcdLeafCerts       []string
}

type KubeadmCheckStackedEtcdLeafCertsExpirationStepBuilder struct {
	step.Builder[KubeadmCheckStackedEtcdLeafCertsExpirationStepBuilder, *KubeadmCheckStackedEtcdLeafCertsExpirationStep]
}

func NewKubeadmCheckStackedEtcdLeafCertsExpirationStepBuilder(ctx runtime.Context, instanceName string) *KubeadmCheckStackedEtcdLeafCertsExpirationStepBuilder {
	s := &KubeadmCheckStackedEtcdLeafCertsExpirationStep{
		remoteKubePkiDir:    common.DefaultPKIPath,
		expirationThreshold: DefaultK8sLeafCertExpirationThreshold,
		etcdLeafCerts: []string{
			"etcd/server.crt",
			"etcd/peer.crt",
			"etcd/healthcheck-client.crt",
		},
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Check expiration of Kubeadm-managed (stacked) etcd leaf certificates"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(KubeadmCheckStackedEtcdLeafCertsExpirationStepBuilder).Init(s)
	return b
}

func (s *KubeadmCheckStackedEtcdLeafCertsExpirationStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmCheckStackedEtcdLeafCertsExpirationStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck: verifying remote stacked etcd PKI directory existence...")

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	remoteEtcdPkiDir := filepath.Join(s.remoteKubePkiDir, "etcd")
	checkCmd := fmt.Sprintf("[ -d %s ]", remoteEtcdPkiDir)
	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, checkCmd, s.Sudo); err != nil {
		logger.Errorf("Stacked etcd PKI directory '%s' does not exist on remote host.", remoteEtcdPkiDir)
		return false, fmt.Errorf("precheck failed: stacked etcd PKI directory '%s' not found on host '%s'", remoteEtcdPkiDir, ctx.GetHost().GetName())
	}

	logger.Info("Precheck passed: stacked etcd PKI directory found.")
	return false, nil
}

func (s *KubeadmCheckStackedEtcdLeafCertsExpirationStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Infof("Checking expiration for %d stacked etcd leaf certificates in '%s'...", len(s.etcdLeafCerts), s.remoteKubePkiDir)

	var anyCertRequiresRenewal bool

	for _, certFile := range s.etcdLeafCerts {
		remotePath := filepath.Join(s.remoteKubePkiDir, certFile)
		log := logger.With("certificate", certFile, "path", remotePath)

		certData, err := runner.ReadFile(ctx.GoContext(), conn, remotePath)
		if err != nil {
			log.Errorf("Critical stacked etcd leaf certificate not found or is unreadable. Error: %v", err)
			return fmt.Errorf("failed to read critical stacked etcd leaf certificate '%s': %w", remotePath, err)
		}

		pemBlock, _ := pem.Decode(certData)
		if pemBlock == nil {
			log.Error("Failed to decode PEM block from critical stacked etcd leaf certificate.")
			return fmt.Errorf("failed to decode PEM block from '%s'", remotePath)
		}

		cert, err := x509.ParseCertificate(pemBlock.Bytes)
		if err != nil {
			log.Errorf("Failed to parse critical stacked etcd leaf certificate. Error: %v", err)
			return fmt.Errorf("failed to parse critical stacked etcd leaf certificate from '%s': %w", remotePath, err)
		}

		remaining := time.Until(cert.NotAfter)
		log.Infof("Certificate valid until: %s", cert.NotAfter.Format("2006-01-02 15:04:05 MST"))

		if remaining < s.expirationThreshold {
			remainingDays := int(remaining.Hours() / 24)
			if remaining <= 0 {
				log.Errorf("FATAL: Certificate has EXPIRED on %s!", cert.NotAfter.Format("2006-01-02"))
			} else {
				log.Warnf("Certificate is expiring soon (in %d days). Renewal is required.", remainingDays)
			}
			anyCertRequiresRenewal = true
		}
	}

	ctx.GetTaskCache().Set(fmt.Sprintf(common.CacheKubeadmEtcdLeafCertRenew, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName()), anyCertRequiresRenewal)
	ctx.GetModuleCache().Set(fmt.Sprintf(common.CacheKubeadmEtcdLeafCertRenew, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName()), anyCertRequiresRenewal)
	ctx.GetPipelineCache().Set(fmt.Sprintf(common.CacheKubeadmEtcdLeafCertRenew, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName()), anyCertRequiresRenewal)

	cacheKey := fmt.Sprintf(common.CacheKubeadmEtcdLeafCertRenew, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName())
	if anyCertRequiresRenewal {
		logger.Warnf("One or more stacked etcd leaf certificates require renewal. Result saved to cache ('%s': true).", cacheKey)
	} else {
		logger.Info("All stacked etcd leaf certificates are valid. Result saved to cache ('%s': false).", cacheKey)
	}

	return nil
}

func (s *KubeadmCheckStackedEtcdLeafCertsExpirationStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a check-only step. Nothing to do.")
	return nil
}

var _ step.Step = (*KubeadmCheckStackedEtcdLeafCertsExpirationStep)(nil)
