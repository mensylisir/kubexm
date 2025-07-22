package common

import (
	"crypto/x509"
	"fmt"
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

type GenerateCertsStep struct {
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

type GenerateCertsStepBuilder struct {
	step.Builder[GenerateCertsStepBuilder, *GenerateCertsStep]
}

func NewGenerateCertsStepBuilder(ctx runtime.Context, instanceName string) *GenerateCertsStepBuilder {
	s := &GenerateCertsStep{
		LocalCertsDir:  filepath.Join(ctx.GetGlobalWorkDir(), "certs"),
		CertDuration:   365 * 24 * time.Hour * 10,
		Permission:     "0755",
		CaCertFileName: common.EtcdCaPemFileName,
		CaKeyFileName:  common.EtcdCaKeyPemFileName,
		CommonName:     "kubexm-client",
		Organization:   []string{"system:masters"},
		Usages:         []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate certificate", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(GenerateCertsStepBuilder).Init(s)
	return b
}

func (b *GenerateCertsStepBuilder) WithCaCertFileName(name string) *GenerateCertsStepBuilder {
	b.Step.CaCertFileName = name
	return b
}

func (b *GenerateCertsStepBuilder) WithCaKeyFileName(name string) *GenerateCertsStepBuilder {
	b.Step.CaKeyFileName = name
	return b
}

func (b *GenerateCertsStepBuilder) WithCommonName(cn string) *GenerateCertsStepBuilder {
	b.Step.CommonName = cn
	return b
}

func (b *GenerateCertsStepBuilder) WithOrganization(org []string) *GenerateCertsStepBuilder {
	b.Step.Organization = org
	return b
}

func (b *GenerateCertsStepBuilder) WithUsages(usages []x509.ExtKeyUsage) *GenerateCertsStepBuilder {
	b.Step.Usages = usages
	return b
}

func (b *GenerateCertsStepBuilder) WithLocalCertsDir(path string) *GenerateCertsStepBuilder {
	b.Step.LocalCertsDir = path
	return b
}

func (b *GenerateCertsStepBuilder) WithCertDuration(duration time.Duration) *GenerateCertsStepBuilder {
	b.Step.CertDuration = duration
	return b
}

func (b *GenerateCertsStepBuilder) WithPermission(permission string) *GenerateCertsStepBuilder {
	b.Step.Permission = permission
	return b
}

func (b *GenerateCertsStepBuilder) WithHosts(hosts []string) *GenerateCertsStepBuilder {
	b.Step.Hosts = hosts
	return b
}

func (b *GenerateCertsStepBuilder) WithCert(cert string) *GenerateCertsStepBuilder {
	b.Step.Cert = cert
	return b
}

func (b *GenerateCertsStepBuilder) WithCertKey(certKey string) *GenerateCertsStepBuilder {
	b.Step.CertKey = certKey
	return b
}

func (s *GenerateCertsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateCertsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	if !fileExists(s.LocalCertsDir, s.Cert) || !fileExists(s.LocalCertsDir, s.CertKey) {
		return false, nil
	}
	return true, nil
}

func (s *GenerateCertsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")
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

func (s *GenerateCertsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

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
func fileExists(dir, file string) bool {
	_, err := os.Stat(filepath.Join(dir, file))
	return err == nil
}

func splitHosts(hosts []string) (ips []net.IP, dnsNames []string) {
	for _, host := range hosts {
		if ip := net.ParseIP(host); ip != nil {
			ips = append(ips, ip)
		} else {
			dnsNames = append(dnsNames, host)
		}
	}
	return
}

var _ step.Step = (*GenerateCertsStep)(nil)
