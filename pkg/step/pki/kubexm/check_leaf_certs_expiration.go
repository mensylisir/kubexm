package kubexm

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

const (
	DefaultK8sLeafCertExpirationThreshold = 90 * 24 * time.Hour
)

type KubexmCheckLeafCertsExpirationStep struct {
	step.Base
	remoteCertsDir      string
	expirationThreshold time.Duration
	leafCerts           []string
}

type KubexmCheckLeafCertsExpirationStepBuilder struct {
	step.Builder[KubexmCheckLeafCertsExpirationStepBuilder, *KubexmCheckLeafCertsExpirationStep]
}

func NewKubexmCheckLeafCertsExpirationStepBuilder(ctx runtime.Context, instanceName string) *KubexmCheckLeafCertsExpirationStepBuilder {
	certsToCheck := []string{
		"apiserver.crt",
		"apiserver-kubelet-client.crt",
		"front-proxy-client.crt",
	}

	if ctx.GetClusterConfig().Spec.Etcd.Type == string(common.EtcdDeploymentTypeKubeadm) {
		certsToCheck = append(certsToCheck, "apiserver-etcd-client.crt")
	}

	s := &KubexmCheckLeafCertsExpirationStep{
		remoteCertsDir:      common.DefaultPKIPath,
		expirationThreshold: DefaultK8sLeafCertExpirationThreshold,
		leafCerts:           certsToCheck,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Check the expiration of Kubernetes leaf certificates on the node"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(KubexmCheckLeafCertsExpirationStepBuilder).Init(s)
	return b
}

func (s *KubexmCheckLeafCertsExpirationStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubexmCheckLeafCertsExpirationStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for Kubernetes leaf certificate expiration on remote node.")
	return false, nil
}

func (s *KubexmCheckLeafCertsExpirationStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	logger.Infof("Checking expiration for %d Kubernetes leaf certificates in '%s'...", len(s.leafCerts), s.remoteCertsDir)

	var anyLeafRequiresRenewal bool

	for _, certFile := range s.leafCerts {
		remotePath := filepath.Join(s.remoteCertsDir, certFile)
		log := logger.With("certificate", certFile, "path", remotePath)

		certData, err := runner.ReadFile(ctx.GoContext(), conn, remotePath)
		if err != nil {
			log.Errorf("Critical leaf certificate not found or is unreadable. Error: %v", err)
			return fmt.Errorf("failed to read critical leaf certificate '%s': %w", remotePath, err)
		}

		pemBlock, _ := pem.Decode(certData)
		if pemBlock == nil {
			log.Error("Failed to decode PEM block from critical leaf certificate.")
			return fmt.Errorf("failed to decode PEM block from '%s'", remotePath)
		}

		cert, err := x509.ParseCertificate(pemBlock.Bytes)
		if err != nil {
			log.Errorf("Failed to parse critical leaf certificate. Error: %v", err)
			return fmt.Errorf("failed to parse critical leaf certificate from '%s': %w", remotePath, err)
		}

		remaining := time.Until(cert.NotAfter)
		log.Infof("Certificate valid until: %s", cert.NotAfter.Format("2006-01-02 15:04:05 MST"))

		if remaining < s.expirationThreshold {
			remainingDays := int(remaining.Hours() / 24)
			if remaining <= 0 {
				log.Errorf("FATAL: Leaf certificate has EXPIRED on %s!", cert.NotAfter.Format("2006-01-02"))
			} else {
				log.Warnf("Leaf certificate is expiring soon (in %d days). Renewal is required.", remainingDays)
			}
			anyLeafRequiresRenewal = true
		}
	}

	cacheKey := fmt.Sprintf(common.CacheKubexmK8sLeafCertRenew, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName())
	ctx.GetTaskCache().Set(cacheKey, anyLeafRequiresRenewal)
	ctx.GetModuleCache().Set(cacheKey, anyLeafRequiresRenewal)
	ctx.GetPipelineCache().Set(cacheKey, anyLeafRequiresRenewal)

	if anyLeafRequiresRenewal {
		logger.Warnf("One or more Kubernetes leaf certificates on this node require renewal. Result saved to cache ('%s': true).", cacheKey)
	} else {
		logger.Info("All Kubernetes leaf certificates on this node are valid. Result saved to cache ('%s': false).", cacheKey)
	}

	return nil
}

func (s *KubexmCheckLeafCertsExpirationStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Rollback is not applicable for a check-only step. Nothing to do.")
	return nil
}

var _ step.Step = (*KubexmCheckLeafCertsExpirationStep)(nil)
