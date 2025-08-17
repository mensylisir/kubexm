package common

import (
	"crypto/x509"
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

type GenerateCertsPEMStep struct {
	step.Base
	LocalCertsDir  string
	CertDuration   time.Duration
	Permission     string
	CaCertFileName string
	CaKeyFileName  string
	Cert           string
	CertKey        string
	Hosts          []string
	CommonName     string
	Organization   []string
	Usages         []x509.ExtKeyUsage
}

type GenerateCertsPEMStepBuilder struct {
	step.Builder[GenerateCertsPEMStepBuilder, *GenerateCertsPEMStep]
}

func NewGenerateCertsPEMStepBuilder(ctx runtime.Context, instanceName string) *GenerateCertsPEMStepBuilder {
	s := &GenerateCertsPEMStep{
		LocalCertsDir:  ctx.GetEtcdCertsDir(),
		CertDuration:   365 * 24 * time.Hour * 10,
		Permission:     "0755",
		CaCertFileName: common.EtcdCaPemFileName,
		CaKeyFileName:  common.EtcdCaKeyPemFileName,
		CommonName:     "kubexm",
		Organization:   []string{"kubexm"},
		Usages:         []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	if ctx.GetClusterConfig().Spec.Certs.CertDuration != "" {
		parsedDuration, err := time.ParseDuration(ctx.GetClusterConfig().Spec.Certs.CADuration)
		if err == nil {
			s.CertDuration = parsedDuration
		} else {
			ctx.GetLogger().Warnf("Failed to parse user-provided Cert duration '%s', using default. Error: %v", ctx.GetClusterConfig().Spec.Certs.CertDuration, err)
		}
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate certificate", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(GenerateCertsPEMStepBuilder).Init(s)
	return b
}

func (b *GenerateCertsPEMStepBuilder) WithCaCertFileName(name string) *GenerateCertsPEMStepBuilder {
	b.Step.CaCertFileName = name
	return b
}

func (b *GenerateCertsPEMStepBuilder) WithCaKeyFileName(name string) *GenerateCertsPEMStepBuilder {
	b.Step.CaKeyFileName = name
	return b
}

func (b *GenerateCertsPEMStepBuilder) WithCommonName(cn string) *GenerateCertsPEMStepBuilder {
	b.Step.CommonName = cn
	return b
}

func (b *GenerateCertsPEMStepBuilder) WithOrganization(org []string) *GenerateCertsPEMStepBuilder {
	b.Step.Organization = org
	return b
}

func (b *GenerateCertsPEMStepBuilder) WithUsages(usages []x509.ExtKeyUsage) *GenerateCertsPEMStepBuilder {
	b.Step.Usages = usages
	return b
}

func (b *GenerateCertsPEMStepBuilder) WithLocalCertsDir(path string) *GenerateCertsPEMStepBuilder {
	b.Step.LocalCertsDir = path
	return b
}

func (b *GenerateCertsPEMStepBuilder) WithCertDuration(duration time.Duration) *GenerateCertsPEMStepBuilder {
	b.Step.CertDuration = duration
	return b
}

func (b *GenerateCertsPEMStepBuilder) WithPermission(permission string) *GenerateCertsPEMStepBuilder {
	b.Step.Permission = permission
	return b
}

func (b *GenerateCertsPEMStepBuilder) WithHosts(hosts []string) *GenerateCertsPEMStepBuilder {
	b.Step.Hosts = hosts
	return b
}

func (b *GenerateCertsPEMStepBuilder) WithCert(cert string) *GenerateCertsPEMStepBuilder {
	b.Step.Cert = cert
	return b
}

func (b *GenerateCertsPEMStepBuilder) WithCertKey(certKey string) *GenerateCertsPEMStepBuilder {
	b.Step.CertKey = certKey
	return b
}

func (s *GenerateCertsPEMStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateCertsPEMStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	if !fileExists(s.LocalCertsDir, s.Cert) || !fileExists(s.LocalCertsDir, s.CertKey) {
		logger.Info("Certificate or key not found. Generation is required.")
		return false, nil
	}
	logger.Info("Certificate and key already exist. Step is done.")
	return true, nil
}

func (s *GenerateCertsPEMStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Loading CA...", "caCert", s.CaCertFileName, "caKey", s.CaKeyFileName)
	caCertPath := filepath.Join(s.LocalCertsDir, s.CaCertFileName)
	caKeyPath := filepath.Join(s.LocalCertsDir, s.CaKeyFileName)
	caCert, caKey, err := helpers.LoadCertificateAuthority(caCertPath, caKeyPath)
	if err != nil {
		return fmt.Errorf("failed to load CA (cert: %s, key: %s), please ensure CA generation step ran successfully: %w", caCertPath, caKeyPath, err)
	}

	ipAddresses, dnsNames := splitHosts(s.Hosts)

	logger.Info("Generating certificate...", "cn", s.CommonName, "org", s.Organization, "hosts", s.Hosts)
	altNames := helpers.AltNames{
		DNSNames: dnsNames,
		IPs:      ipAddresses,
	}
	certCfg := helpers.CertConfig{
		CommonName:   s.CommonName,
		Organization: s.Organization,
		Usages:       s.Usages,
		Duration:     s.CertDuration,
		AltNames:     altNames,
	}

	if err := helpers.NewSignedCertificate(s.LocalCertsDir, s.Cert, s.CertKey, certCfg, caCert, caKey); err != nil {
		return fmt.Errorf("failed to generate signed certificate %s: %w", s.Cert, err)
	}

	logger.Info("Certificate generated successfully.", "cert", s.Cert, "key", s.CertKey)
	return nil
}

func (s *GenerateCertsPEMStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	filesToRemove := []string{}
	filesToRemove = append(filesToRemove, s.Cert, s.CertKey)
	for _, file := range filesToRemove {
		path := filepath.Join(s.LocalCertsDir, file)
		logger.Warnf("Rolling back by deleting: %s", path)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			logger.Error(err, "Failed to remove file during rollback", "path", path)
		}
	}

	return nil
}

var _ step.Step = (*GenerateCertsPEMStep)(nil)
