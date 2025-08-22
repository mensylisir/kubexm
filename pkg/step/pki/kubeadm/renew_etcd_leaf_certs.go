package kubeadm

import (
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

type KubeadmRenewStackedEtcdLeafCertsStep struct {
	step.Base
	CertDuration time.Duration
	etcdNodes    []connector.Host
	caToUseDir   string
	outputDir    string
}

type KubeadmRenewStackedEtcdLeafCertsStepBuilder struct {
	step.Builder[KubeadmRenewStackedEtcdLeafCertsStepBuilder, *KubeadmRenewStackedEtcdLeafCertsStep]
}

func NewKubeadmRenewStackedEtcdLeafCertsStepBuilder(ctx runtime.Context, instanceName string) *KubeadmRenewStackedEtcdLeafCertsStepBuilder {
	s := &KubeadmRenewStackedEtcdLeafCertsStep{
		CertDuration: 10 * 365 * 24 * time.Hour,
		etcdNodes:    ctx.GetHostsByRole(common.RoleEtcd),
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate new stacked Etcd leaf certificates"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(KubeadmRenewStackedEtcdLeafCertsStepBuilder).Init(s)
	return b
}

func (s *KubeadmRenewStackedEtcdLeafCertsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmRenewStackedEtcdLeafCertsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	caCacheKey := fmt.Sprintf(common.CacheKubeadmEtcdCACertRenew, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName())
	caRequiresRenewal1, _ := ctx.GetPipelineCache().Get(caCacheKey)
	caRequiresRenewal := caRequiresRenewal1.(bool)
	anyLeafRequiresRenewal := false
	for _, node := range s.etcdNodes {
		cacheKey := fmt.Sprintf(common.CacheKubeadmEtcdLeafCertRenew, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName(), node.GetName())
		nodeRequiresRenewal, _ := ctx.GetTaskCache().Get(cacheKey)
		if nodeRequiresRenewal.(bool) {
			anyLeafRequiresRenewal = true
			break
		}
	}
	if !caRequiresRenewal && !anyLeafRequiresRenewal {
		logger.Info("Neither CA nor any leaf certificates require renewal. Step is done.")
		return true, nil
	}

	baseCertsDir := ctx.GetKubernetesCertsDir()
	if caRequiresRenewal {
		logger.Info("CA renewal is in progress. Will use the new CA to generate new leaf certificates.")
		s.caToUseDir = filepath.Join(baseCertsDir, "certs-new")
		s.outputDir = filepath.Join(baseCertsDir, "certs-new")
	} else {
		logger.Info("Only leaf certificate renewal is required. Will use the original CA.")
		s.caToUseDir = baseCertsDir
		s.outputDir = baseCertsDir
	}
	s.outputDir = filepath.Join(s.outputDir, "etcd")
	if !helpers.IsFileExist(filepath.Join(s.caToUseDir, "etcd/ca.crt")) {
		return false, fmt.Errorf("precheck failed: new Etcd CA not found in '%s/etcd'", s.caToUseDir)
	}

	logger.Info("Precheck passed. Ready to generate Etcd leaf certificates.")
	return false, nil
}

func (s *KubeadmRenewStackedEtcdLeafCertsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	if err := os.MkdirAll(s.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", s.outputDir, err)
	}

	logger.Infof("Loading CA from '%s' to generate leaf certificates...", s.caToUseDir)
	caCertPath := filepath.Join(s.caToUseDir, "etcd/ca.crt")
	caKeyPath := filepath.Join(s.caToUseDir, "etcd/ca.key")

	caCert, caKey, err := helpers.LoadCertificateAuthority(caCertPath, caKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load CA: %w", err)
	}

	altNames := helpers.AltNames{
		DNSNames: []string{"localhost", "etcd", "etcd.kube-system", "etcd.kube-system.svc",
			fmt.Sprintf("etcd.kube-system.svc.%s", ctx.GetClusterConfig().Spec.Kubernetes.ClusterName, ctx.GetClusterConfig().Spec.Kubernetes.DNSDomain),
			ctx.GetClusterConfig().Spec.ControlPlaneEndpoint.Domain},
		IPs: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	for _, node := range s.etcdNodes {
		altNames.DNSNames = append(altNames.DNSNames, node.GetName())
		if ip := net.ParseIP(node.GetAddress()); ip != nil {
			altNames.IPs = append(altNames.IPs, ip)
		}
		if node.GetInternalAddress() != "" && node.GetInternalAddress() != node.GetAddress() {
			if ip := net.ParseIP(node.GetInternalAddress()); ip != nil {
				altNames.IPs = append(altNames.IPs, ip)
			}
		}
	}
	logger.Infof("Generating new leaf certificates and saving to '%s'...", s.outputDir)
	logger.Debugf("Generating certificates for etcd node ")
	serverCfg := helpers.CertConfig{
		CommonName:   "kubexm-kubeadm-etcd",
		Organization: []string{"etcd"},
		AltNames:     altNames,
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		Duration:     s.CertDuration,
	}
	serverCertFile := "server.crt"
	serverKeyFile := "server.key"
	if err := helpers.NewSignedCertificate(s.outputDir, serverCertFile, serverKeyFile, serverCfg, caCert, caKey); err != nil {
		return fmt.Errorf("failed to generate etcd server certificate for node: %w", err)
	}

	peerCfg := helpers.CertConfig{
		CommonName:   "kubexm-kubeadm-etcd",
		Organization: []string{"etcd"},
		AltNames:     altNames,
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		Duration:     s.CertDuration,
	}
	peerCertFile := "peer.crt"
	peerKeyFile := "peer.key"
	if err := helpers.NewSignedCertificate(s.outputDir, peerCertFile, peerKeyFile, peerCfg, caCert, caKey); err != nil {
		return fmt.Errorf("failed to generate etcd peer certificate for node: %w", err)
	}

	healthCheckClientCfg := helpers.CertConfig{
		CommonName:   "kube-etcd-healthcheck-client",
		Organization: []string{"system:masters"},
		AltNames:     altNames,
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		Duration:     s.CertDuration,
	}
	healthCheckClientCertFile := "healthcheck-client.crt"
	healthCheckClientKeyFile := "healthcheck-client.key"
	if err := helpers.NewSignedCertificate(s.outputDir, healthCheckClientCertFile, healthCheckClientKeyFile, healthCheckClientCfg, caCert, caKey); err != nil {
		return fmt.Errorf("failed to generate etcd healthcheck-client certificate for node: %w", err)
	}

	logger.Info("All stacked Etcd leaf certificates generated successfully.")
	return nil
}

func (s *KubeadmRenewStackedEtcdLeafCertsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rolling back by deleting newly generated Etcd leaf certificates from 'certs-new/etcd'...")

	if s.outputDir == "" {
		logger.Info("No certificates were defined or output directory is empty, nothing to roll back.")
		return nil
	}

	outputEtcdDir := filepath.Join(s.outputDir, "etcd")
	if outputEtcdDir != "" {
		logger.Warnf("Rolling back by deleting output directory: %s", s.outputDir)
		if err := os.RemoveAll(s.outputDir); err != nil {
			logger.Errorf("Failed to remove output directory '%s' during rollback: %v", s.outputDir, err)
		}
	} else {
		logger.Warn("Rolling back in-place certificate generation is not performed automatically. Please restore from backups if needed.")
	}

	logger.Info("Rollback of Etcd leaf certificate generation finished.")
	return nil
}

var _ step.Step = (*KubeadmRenewStackedEtcdLeafCertsStep)(nil)
