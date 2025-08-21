package kubeadm

import (
	"crypto/ecdsa"
	"crypto/x509"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

type KubeadmRenewLeafCertsStep struct {
	step.Base
	ClusterSpec  *v1alpha1.ClusterSpec
	CertDuration time.Duration
	caToUseDir   string
	outputDir    string
}

type KubeadmRenewLeafCertsStepBuilder struct {
	step.Builder[KubeadmRenewLeafCertsStepBuilder, *KubeadmRenewLeafCertsStep]
}

func NewKubeadmRenewLeafCertsStepBuilder(ctx runtime.Context, instanceName string) *KubeadmRenewLeafCertsStepBuilder {
	s := &KubeadmRenewLeafCertsStep{
		CertDuration: common.DefaultCertificateValidityDays * 24 * time.Hour,
		ClusterSpec:  ctx.GetClusterConfig().Spec,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate new Kubernetes leaf certificates"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 3 * time.Minute

	b := new(KubeadmRenewLeafCertsStepBuilder).Init(s)
	return b
}

func (s *KubeadmRenewLeafCertsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

type certDefinition struct {
	certFile, keyFile string
	config            helpers.CertConfig
	caName            string
}

func (s *KubeadmRenewLeafCertsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for leaf certificate renewal...")

	var caRequiresRenewal bool
	if rawVal, ok := ctx.GetModuleCache().Get(common.CacheKubeadmK8sCACertRenew); ok {
		if val, isBool := rawVal.(bool); isBool {
			caRequiresRenewal = val
		}
	}
	var leafRequiresRenewal bool
	if rawVal, ok := ctx.GetModuleCache().Get(common.CacheKubeadmK8sLeafCertRenew); ok {
		if val, isBool := rawVal.(bool); isBool {
			leafRequiresRenewal = val
		}
	}

	if !caRequiresRenewal && !leafRequiresRenewal {
		logger.Info("Neither K8s CA nor leaf certificates require renewal. Step is done.")
		return true, nil
	}

	baseCertsDir := ctx.GetKubernetesCertsDir()
	if caRequiresRenewal {
		logger.Info("CA renewal is required. Will use the new CA from 'certs-new'.")
		s.caToUseDir = filepath.Join(baseCertsDir, "certs-new")
		s.outputDir = filepath.Join(baseCertsDir, "certs-new")
	} else {
		logger.Info("Only leaf certificate renewal is required. Will use the original CA.")
		s.caToUseDir = baseCertsDir
		s.outputDir = baseCertsDir
	}

	defs, err := s.getCertDefinitions(ctx)
	if err != nil {
		return false, err
	}

	allExist := true
	for name, def := range defs {
		if !helpers.FileExists(s.outputDir, def.certFile) || !helpers.FileExists(s.outputDir, def.keyFile) {
			logger.Infof("Certificate for '%s' not found in output directory. Generation is required.", name)
			allExist = false
			break
		}
	}
	if allExist {
		logger.Info("All required Kubernetes component certificates already exist in the output directory. Step is done.")
		return true, nil
	}

	logger.Info("Precheck passed.")
	return false, nil
}

func (s *KubeadmRenewLeafCertsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	type caPair struct {
		Certificate *x509.Certificate
		PrivateKey  *ecdsa.PrivateKey
	}

	cas := make(map[string]caPair)

	logger.Infof("Loading CAs from '%s'...", s.caToUseDir)
	mainCert, mainKey, err := helpers.LoadCertificateAuthority(filepath.Join(s.caToUseDir, "ca.crt"), filepath.Join(s.caToUseDir, "ca.key"))
	if err != nil {
		return fmt.Errorf("failed to load main kubernetes CA: %w", err)
	}
	cas["main"] = caPair{Certificate: mainCert, PrivateKey: mainKey}

	fpCert, fpKey, err := helpers.LoadCertificateAuthority(filepath.Join(s.caToUseDir, "front-proxy-ca.crt"), filepath.Join(s.caToUseDir, "front-proxy-ca.key"))
	if err != nil {
		return fmt.Errorf("failed to load front-proxy CA: %w", err)
	}
	cas["front-proxy"] = caPair{Certificate: fpCert, PrivateKey: fpKey}

	defs, err := s.getCertDefinitions(ctx)
	if err != nil {
		return err
	}

	logger.Infof("Generating leaf certificates and saving to '%s'...", s.outputDir)
	for name, def := range defs {
		log := logger.With("certificate", name, "ca", def.caName)
		log.Info("Generating certificate...")

		ca, ok := cas[def.caName]
		if !ok {
			return fmt.Errorf("unknown CA name '%s' for certificate '%s'", def.caName, name)
		}

		keyPath := filepath.Join(s.outputDir, def.keyFile)
		if !helpers.IsFileExist(keyPath) {
			originalKeyPath := filepath.Join(ctx.GetKubernetesCertsDir(), def.keyFile)
			if !helpers.IsFileExist(originalKeyPath) {
				log.Warnf("Private key not found, a new one will be generated for '%s'.", name)
			} else {
				if err := helpers.CopyFile(originalKeyPath, keyPath); err != nil {
					return fmt.Errorf("failed to copy private key for '%s': %w", name, err)
				}
			}
		}

		if err := helpers.NewSignedCertificate(s.outputDir, def.certFile, def.keyFile, def.config, ca.Certificate, ca.PrivateKey); err != nil {
			return fmt.Errorf("failed to generate certificate for %s: %w", name, err)
		}
	}

	logger.Info("All Kubernetes component certificates generated successfully.")
	return nil
}

func (s *KubeadmRenewLeafCertsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	defs, err := s.getCertDefinitions(ctx)
	if err != nil {
		logger.Errorf("Failed to get certificate definitions for rollback, skipping: %v", err)
		return nil
	}

	if s.outputDir == filepath.Join(ctx.GetKubernetesCertsDir(), "certs-new") {
		logger.Warn("Rolling back by deleting newly generated assets from 'certs-new'...")
		for _, def := range defs {
			_ = os.Remove(filepath.Join(s.outputDir, def.certFile))
			_ = os.Remove(filepath.Join(s.outputDir, def.keyFile))
		}
	} else {
		logger.Warn("Rollback for in-place generation is not performed automatically.")
	}
	return nil
}

func (s *KubeadmRenewLeafCertsStep) getAPIServerAltNames(ctx runtime.ExecutionContext) (*helpers.AltNames, error) {
	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	workerHosts := ctx.GetHostsByRole(common.RoleWorker)
	etcdHosts := ctx.GetHostsByRole(common.RoleEtcd)
	apiServerHosts := append(masterHosts, workerHosts...)
	apiServerHosts = append(apiServerHosts, etcdHosts...)
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

func (s *KubeadmRenewLeafCertsStep) getCertDefinitions(ctx runtime.ExecutionContext) (map[string]certDefinition, error) {
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
	}

	for name, def := range defs {
		def.config.Duration = s.CertDuration
		defs[name] = def
	}
	return defs, nil
}

var _ step.Step = (*KubeadmRenewLeafCertsStep)(nil)
