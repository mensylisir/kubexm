package certs

import (
	"crypto/ecdsa"
	"crypto/x509"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

type GenerateKubeCertsStep struct {
	step.Base
	KubeCertsDir string
	CertDuration time.Duration
	ClusterSpec  *v1alpha1.ClusterSpec
}

type GenerateKubeCertsStepBuilder struct {
	step.Builder[GenerateKubeCertsStepBuilder, *GenerateKubeCertsStep]
}

func NewGenerateKubeCertsStepBuilder(ctx runtime.Context, instanceName string) *GenerateKubeCertsStepBuilder {
	s := &GenerateKubeCertsStep{
		KubeCertsDir: ctx.GetKubernetesCertsDir(),
		CertDuration: common.DefaultCertificateValidityDays * 24 * time.Hour,
		ClusterSpec:  ctx.GetClusterConfig().Spec,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Kubernetes internal component certificates", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(GenerateKubeCertsStepBuilder).Init(s)
	return b
}

func (s *GenerateKubeCertsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

type certDefinition struct {
	certFile, keyFile string
	config            helpers.CertConfig
	caName            string
}

func (s *GenerateKubeCertsStep) getCertDefinitions(ctx runtime.ExecutionContext) (map[string]certDefinition, error) {
	apiServerAltNames, err := s.getAPIServerAltNames(ctx)
	if err != nil {
		return nil, err
	}

	defs := map[string]certDefinition{
		"apiserver": {
			certFile: common.APIServerCertFileName, keyFile: common.APIServerKeyFileName,
			config: helpers.CertConfig{
				CommonName: common.KubeAPIServerCN,
				AltNames:   *apiServerAltNames,
				Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
			},
			caName: "main",
		},
		"apiserver-kubelet-client": {
			certFile: common.APIServerKubeletClientCertFileName, keyFile: common.APIServerKubeletClientKeyFileName,
			config: helpers.CertConfig{
				CommonName:   common.KubeAPIServerCN,
				Organization: []string{common.DefaultCertificateOrganization},
				Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			},
			caName: "main",
		},
		"front-proxy-client": {
			certFile: common.FrontProxyClientCertFileName, keyFile: common.FrontProxyClientKeyFileName,
			config: helpers.CertConfig{
				CommonName: "front-proxy-client",
				Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			},
			caName: "front-proxy",
		},
		"admin": {
			certFile: common.AdminCertFileName, keyFile: common.AdminKeyFileName,
			config: helpers.CertConfig{
				CommonName:   "kubernetes-admin",
				Organization: []string{common.DefaultCertificateOrganization},
				Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			},
			caName: "main",
		},
		"controller-manager": {
			certFile: common.ControllerManagerCertFileName, keyFile: common.ControllerManagerKeyFileName,
			config: helpers.CertConfig{
				CommonName: common.KubeControllerManagerUser,
				Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			},
			caName: "main",
		},
		"scheduler": {
			certFile: common.SchedulerCertFileName, keyFile: common.SchedulerKeyFileName,
			config: helpers.CertConfig{
				CommonName: common.KubeSchedulerUser,
				Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
			},
			caName: "main",
		},
	}

	for name, def := range defs {
		def.config.Duration = s.CertDuration
		defs[name] = def
	}
	return defs, nil
}

func (s *GenerateKubeCertsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	defs, err := s.getCertDefinitions(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get certificate definitions for precheck: %w", err)
	}

	for name, def := range defs {
		if !helpers.FileExists(s.KubeCertsDir, def.certFile) || !helpers.FileExists(s.KubeCertsDir, def.keyFile) {
			logger.Infof("Certificate for '%s' not found. Generation is required.", name)
			return false, nil
		}
	}

	logger.Info("All required Kubernetes component certificates already exist. Step is done.")
	return true, nil
}

func (s *GenerateKubeCertsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	type caPair struct {
		Certificate *x509.Certificate
		PrivateKey  *ecdsa.PrivateKey
	}

	cas := make(map[string]caPair)
	var err error

	mainCert, mainKey, err := helpers.LoadCertificateAuthority(filepath.Join(s.KubeCertsDir, common.CACertFileName), filepath.Join(s.KubeCertsDir, common.CAKeyFileName))
	if err != nil {
		return fmt.Errorf("failed to load main kubernetes CA from %s: %w. Ensure GenerateKubeCAStep ran successfully", s.KubeCertsDir, err)
	}
	cas["main"] = caPair{Certificate: mainCert, PrivateKey: mainKey}

	fpCert, fpKey, err := helpers.LoadCertificateAuthority(filepath.Join(s.KubeCertsDir, common.FrontProxyCACertFileName), filepath.Join(s.KubeCertsDir, common.FrontProxyCAKeyFileName))
	if err != nil {
		return fmt.Errorf("failed to load front-proxy CA from %s: %w. Ensure GenerateKubeCAStep ran successfully", s.KubeCertsDir, err)
	}
	cas["front-proxy"] = caPair{Certificate: fpCert, PrivateKey: fpKey}

	defs, err := s.getCertDefinitions(ctx)
	if err != nil {
		return err
	}

	for name, def := range defs {
		logger.Infof("Generating certificate for: %s (signed by: %s CA)", name, def.caName)

		ca, ok := cas[def.caName]
		if !ok {
			return fmt.Errorf("unknown CA name '%s' for certificate '%s'", def.caName, name)
		}

		err := helpers.NewSignedCertificate(s.KubeCertsDir, def.certFile, def.keyFile, def.config, ca.Certificate, ca.PrivateKey)
		if err != nil {
			return fmt.Errorf("failed to generate certificate for %s: %w", name, err)
		}
	}

	logger.Info("All Kubernetes component certificates generated successfully.")
	return nil
}

func (s *GenerateKubeCertsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	defs, err := s.getCertDefinitions(ctx)
	if err != nil {
		logger.Errorf("Failed to get certificate definitions for rollback, skipping: %v", err)
		return nil
	}

	for name, def := range defs {
		logger.Warnf("Rolling back by deleting certificate for: %s", name)
		_ = os.Remove(filepath.Join(s.KubeCertsDir, def.certFile))
		_ = os.Remove(filepath.Join(s.KubeCertsDir, def.keyFile))
	}
	return nil
}

func (s *GenerateKubeCertsStep) getAPIServerAltNames(ctx runtime.ExecutionContext) (*helpers.AltNames, error) {
	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	//workerHosts := ctx.GetHostsByRole(common.RoleWorker)
	//etcdHosts := ctx.GetHostsByRole(common.RoleEtcd)
	//apiServerHosts := append(masterHosts, workerHosts...)
	//apiServerHosts = append(apiServerHosts, etcdHosts...)
	apiServerHosts := masterHosts
	altNames := &helpers.AltNames{
		DNSNames: []string{
			"kubernetes",
			"kubernetes.default",
			"kubernetes.default.svc",
			fmt.Sprintf("kubernetes.default.svc.%s", s.ClusterSpec.Kubernetes.DNSDomain),
			"localhost",
		},
		IPs: []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("0:0:0:0:0:0:0:1")},
	}

	_, serviceNet, err := net.ParseCIDR(s.ClusterSpec.Network.KubeServiceCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid service CIDR '%s': %w", s.ClusterSpec.Network.KubeServiceCIDR, err)
	}
	firstIP, err := helpers.GetFirstIP(serviceNet)
	if err != nil {
		return nil, err
	}
	altNames.IPs = append(altNames.IPs, firstIP)

	for _, node := range apiServerHosts {
		altNames.DNSNames = append(altNames.DNSNames, node.GetName())
		altNames.DNSNames = append(altNames.DNSNames, fmt.Sprintf("%s.cluster.local", node.GetName()))
		if ip := net.ParseIP(node.GetAddress()); ip != nil {
			altNames.IPs = append(altNames.IPs, ip)
		}
		if node.GetInternalAddress() != "" {
			if ip := net.ParseIP(node.GetInternalAddress()); ip != nil {
				altNames.IPs = append(altNames.IPs, ip)
			}
		}
	}

	endpoint := s.ClusterSpec.ControlPlaneEndpoint
	if endpoint != nil {
		if endpoint.Address != "" {
			if ip := net.ParseIP(endpoint.Address); ip != nil {
				altNames.IPs = append(altNames.IPs, ip)
			} else {
				altNames.DNSNames = append(altNames.DNSNames, endpoint.Address)
			}
		}
		if endpoint.Domain != "" {
			altNames.DNSNames = append(altNames.DNSNames, endpoint.Domain)
		}
	}

	if s.ClusterSpec.Kubernetes != nil && s.ClusterSpec.Kubernetes.APIServer.CertExtraSans != nil {
		for _, san := range s.ClusterSpec.Kubernetes.APIServer.CertExtraSans {
			if ip := net.ParseIP(san); ip != nil {
				altNames.IPs = append(altNames.IPs, ip)
			} else {
				altNames.DNSNames = append(altNames.DNSNames, san)
			}
		}
	}

	altNames.DNSNames = helpers.UniqueStrings(altNames.DNSNames)
	altNames.IPs = helpers.UniqueIPs(altNames.IPs)

	return altNames, nil
}

var _ step.Step = (*GenerateKubeCertsStep)(nil)
