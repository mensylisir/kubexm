package etcd

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

type GenerateNewLeafCertsStep struct {
	step.Base
	etcdNodes    []connector.Host
	certDuration time.Duration
	caToUseDir   string
	outputDir    string
}

type GenerateNewLeafCertsStepBuilder struct {
	step.Builder[GenerateNewLeafCertsStepBuilder, *GenerateNewLeafCertsStep]
}

func NewGenerateNewLeafCertsStepBuilder(ctx runtime.Context, instanceName string) *GenerateNewLeafCertsStepBuilder {
	s := &GenerateNewLeafCertsStep{
		etcdNodes:    ctx.GetHostsByRole(common.RoleEtcd),
		certDuration: 10 * 365 * 24 * time.Hour,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate new etcd leaf certificates on the control plane"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute
	b := new(GenerateNewLeafCertsStepBuilder).Init(s)
	return b
}

func (s *GenerateNewLeafCertsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateNewLeafCertsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	caCacheKey := fmt.Sprintf(common.CacheKubexmEtcdCACertRenew, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName())
	caRequiresRenewal, _ := ctx.GetTaskCache().GetBool(caCacheKey)
	anyLeafRequiresRenewal := false
	for _, node := range s.etcdNodes {
		cacheKey := fmt.Sprintf(common.CacheKubexmEtcdLeafCertRenew, ctx.GetRunID(), ctx.GetPipelineName(), ctx.GetModuleName(), ctx.GetTaskName(), node.GetName())
		nodeRequiresRenewal, _ := ctx.GetTaskCache().GetBool(cacheKey)
		if nodeRequiresRenewal {
			anyLeafRequiresRenewal = true
			break
		}
	}

	if !caRequiresRenewal && !anyLeafRequiresRenewal {
		logger.Info("Neither CA nor any leaf certificates require renewal. Step is done.")
		return true, nil
	}

	baseCertsDir := ctx.GetEtcdCertsDir()
	if caRequiresRenewal {
		logger.Info("CA renewal is in progress. Will use the new CA to generate new leaf certificates.")
		s.caToUseDir = filepath.Join(baseCertsDir, "certs-new")
		s.outputDir = filepath.Join(baseCertsDir, "certs-new")
	} else {
		logger.Info("Only leaf certificate renewal is required. Will use the original CA.")
		s.caToUseDir = baseCertsDir
		s.outputDir = baseCertsDir
	}

	caCertPath := filepath.Join(s.caToUseDir, common.EtcdCaPemFileName)
	caKeyPath := filepath.Join(s.caToUseDir, common.EtcdCaKeyPemFileName)
	if !helpers.IsFileExist(caCertPath) || !helpers.IsFileExist(caKeyPath) {
		return false, fmt.Errorf("the required CA certificate/key for signing was not found in '%s'", s.caToUseDir)
	}

	return false, nil
}

func (s *GenerateNewLeafCertsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	if err := os.MkdirAll(s.outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory '%s': %w", s.outputDir, err)
	}

	logger.Infof("Loading CA from '%s' to generate leaf certificates...", s.caToUseDir)
	caCertPath := filepath.Join(s.caToUseDir, common.EtcdCaPemFileName)
	caKeyPath := filepath.Join(s.caToUseDir, common.EtcdCaKeyPemFileName)
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
	for _, node := range s.etcdNodes {
		nodeName := node.GetName()
		logger.Debugf("Generating certificates for etcd node '%s'", nodeName)

		adminCfg := helpers.CertConfig{
			CommonName:   fmt.Sprintf("%s-admin", nodeName),
			Organization: []string{"kubexm-admin"},
			AltNames:     altNames,
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			Duration:     s.certDuration,
		}
		adminCertFile := fmt.Sprintf(common.EtcdAdminCertFileNamePattern, nodeName)
		adminKeyFile := fmt.Sprintf(common.EtcdAdminKeyFileNamePattern, nodeName)
		if err := helpers.NewSignedCertificate(s.outputDir, adminCertFile, adminKeyFile, adminCfg, caCert, caKey); err != nil {
			return fmt.Errorf("failed to generate etcd admin certificate for node %s: %w", nodeName, err)
		}

		nodeCfg := helpers.CertConfig{
			CommonName:   fmt.Sprintf("%s-node", nodeName),
			Organization: []string{"kubexm-node"},
			AltNames:     altNames,
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			Duration:     s.certDuration,
		}
		nodeCertFile := fmt.Sprintf(common.EtcdNodeCertFileNamePattern, nodeName)
		nodeKeyFile := fmt.Sprintf(common.EtcdNodeKeyFileNamePattern, nodeName)
		if err := helpers.NewSignedCertificate(s.outputDir, nodeCertFile, nodeKeyFile, nodeCfg, caCert, caKey); err != nil {
			return fmt.Errorf("failed to generate etcd node certificate for node %s: %w", nodeName, err)
		}

		memberCfg := helpers.CertConfig{
			CommonName:   fmt.Sprintf("%s-member", nodeName),
			Organization: []string{"kubexm-etcd"},
			AltNames:     altNames,
			Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			Duration:     s.certDuration,
		}
		memberCertFile := fmt.Sprintf(common.EtcdMemberCertFileNamePattern, nodeName)
		memberKeyFile := fmt.Sprintf(common.EtcdMemberKeyFileNamePattern, nodeName)
		if err := helpers.NewSignedCertificate(s.outputDir, memberCertFile, memberKeyFile, memberCfg, caCert, caKey); err != nil {
			return fmt.Errorf("failed to generate etcd member certificate for node %s: %w", nodeName, err)
		}
	}

	logger.Info("All new leaf certificates generated successfully.")
	return nil
}

func (s *GenerateNewLeafCertsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	if s.outputDir != "" && s.outputDir != ctx.GetEtcdCertsDir() {
		logger.Warnf("Rolling back by deleting output directory: %s", s.outputDir)
		if err := os.RemoveAll(s.outputDir); err != nil {
			logger.Errorf("Failed to remove output directory '%s' during rollback: %v", s.outputDir, err)
		}
	} else {
		logger.Warn("Rolling back in-place certificate generation is not performed automatically. Please restore from backups if needed.")
	}

	return nil
}

var _ step.Step = (*GenerateNewLeafCertsStep)(nil)
