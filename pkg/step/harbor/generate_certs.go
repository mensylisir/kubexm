package harbor

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

type GenerateHarborCertsStep struct {
	step.Base
	CertsDir string
}

type GenerateHarborCertsStepBuilder struct {
	step.Builder[GenerateHarborCertsStepBuilder, *GenerateHarborCertsStep]
}

func NewGenerateHarborCertsStepBuilder(ctx runtime.Context, instanceName string) *GenerateHarborCertsStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentHarbor, representativeArch)

	if err != nil || binaryInfo == nil {
		return nil
	}

	s := &GenerateHarborCertsStep{
		CertsDir: filepath.Join(ctx.GetGlobalWorkDir(), "certs", "harbor"),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate self-signed TLS certificates for Harbor", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(GenerateHarborCertsStepBuilder).Init(s)
	return b
}

func (s *GenerateHarborCertsStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateHarborCertsStep) CertFiles() map[string]string {
	return map[string]string{
		"caCert":     "ca.crt",
		"caKey":      "ca.key",
		"serverCert": "harbor.crt",
		"serverKey":  "harbor.key",
	}
}

func (s *GenerateHarborCertsStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")
	if err := os.MkdirAll(s.CertsDir, 0755); err != nil {
		return false, fmt.Errorf("failed to create certs directory %s: %w", s.CertsDir, err)
	}

	for _, certFile := range s.CertFiles() {
		p := filepath.Join(s.CertsDir, certFile)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			logger.Infof("Certificate file '%s' does not exist. Generation is required.", p)
			return false, nil
		}
	}

	logger.Info("All required Harbor certificates already exist. Step is done.")
	return true, nil
}

func (s *GenerateHarborCertsStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")
	cfg := ctx.GetClusterConfig().Spec

	if cfg.Registry == nil || cfg.Registry.MirroringAndRewriting == nil || cfg.Registry.MirroringAndRewriting.PrivateRegistry == "" {
		return fmt.Errorf("`registry.mirroringAndRewriting.privateRegistry` must be set to generate Harbor certificate")
	}

	domain := cfg.Registry.MirroringAndRewriting.PrivateRegistry
	if u, err := url.Parse("scheme://" + domain); err == nil {
		domain = u.Host
	}

	registryHosts := ctx.GetHostsByRole(common.RoleRegistry)
	if len(registryHosts) == 0 {
		return fmt.Errorf("no host with role '%s' found", common.RoleRegistry)
	}
	registryHost := registryHosts[0]

	logger.Info("Creating/Loading Harbor Certificate Authority...")
	caSubject := pkix.Name{
		CommonName:   fmt.Sprintf("%s-ca", domain),
		Organization: []string{"KubeXM Harbor CA"},
	}
	certFiles := s.CertFiles()

	caCert, caKey, err := helpers.NewCertificateAuthorityWithSubject(s.CertsDir, certFiles["caCert"], certFiles["caKey"], caSubject, 10*365*24*time.Hour)
	if err != nil {
		return fmt.Errorf("failed to create/load Harbor CA: %w", err)
	}
	logger.Info("Harbor Certificate Authority is ready.")

	logger.Info("Generating Harbor server certificate...")
	serverCertConfig := helpers.CertConfig{
		CommonName:   domain,
		Organization: []string{"KubeXM"},
		Usages:       []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		Duration:     5 * 365 * 24 * time.Hour,
		AltNames: helpers.AltNames{
			DNSNames: []string{
				domain,
				registryHost.GetName(),
				"localhost",
			},
			IPs: []net.IP{
				net.ParseIP("127.0.0.1"),
				net.ParseIP(registryHost.GetAddress()),
			},
		},
	}

	validIPs := []net.IP{}
	for _, ip := range serverCertConfig.AltNames.IPs {
		if ip != nil {
			validIPs = append(validIPs, ip)
		}
	}
	serverCertConfig.AltNames.IPs = validIPs

	if err := helpers.NewSignedCertificate(s.CertsDir, certFiles["serverCert"], certFiles["serverKey"], serverCertConfig, caCert, caKey); err != nil {
		return fmt.Errorf("failed to generate Harbor server certificate: %w", err)
	}

	logger.Info("Successfully generated all Harbor certificates.")
	return nil
}

func (s *GenerateHarborCertsStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	logger.Warnf("Rolling back by deleting generated certificates directory: %s", s.CertsDir)
	if err := os.RemoveAll(s.CertsDir); err != nil {
		logger.Errorf("Failed to delete directory '%s' during rollback: %v", s.CertsDir, err)
	}

	return nil
}

var _ step.Step = (*GenerateHarborCertsStep)(nil)
