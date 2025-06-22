package pki

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net" // For parsing IP SANs
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// GenerateSignedCertStep generates a certificate signed by a CA.
type GenerateSignedCertStep struct {
	meta            spec.StepMeta
	CommonName      string
	Organizations   []string
	SANs            []string // Subject Alternative Names (DNS names and IP addresses)
	ValidityDays    int
	CACertPath      string // Path to the CA's certificate file
	CAKeyPath       string // Path to the CA's private key file
	OutputDir       string // Directory to save the new cert and key
	BaseFilename    string // Base name for the new cert and key files (e.g., "server", "etcd-client")
	IsClientCert    bool   // If true, sets clientAuth key usage
	IsServerCert    bool   // If true, sets serverAuth key usage
}

// NewGenerateSignedCertStep creates a new GenerateSignedCertStep.
// validityDays defaults to 365 (1 year) if 0.
func NewGenerateSignedCertStep(instanceName, commonName string, organizations, sans []string, validityDays int, caCertPath, caKeyPath, outputDir, baseFilename string, isClient, isServer bool) step.Step {
	metaName := instanceName
	if metaName == "" {
		metaName = fmt.Sprintf("GenerateSignedCert-%s", commonName)
	}
	if validityDays <= 0 {
		validityDays = 365 // Default to 1 year
	}

	return &GenerateSignedCertStep{
		meta: spec.StepMeta{
			Name:        metaName,
			Description: fmt.Sprintf("Generates certificate for %s signed by CA at %s", commonName, filepath.Base(caCertPath)),
		},
		CommonName:      commonName,
		Organizations:   organizations,
		SANs:            sans,
		ValidityDays:    validityDays,
		CACertPath:      caCertPath,
		CAKeyPath:       caKeyPath,
		OutputDir:       outputDir,
		BaseFilename:    baseFilename,
		IsClientCert:    isClient,
		IsServerCert:    isServer,
	}
}

func (s *GenerateSignedCertStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *GenerateSignedCertStep) certPath() string {
	return filepath.Join(s.OutputDir, fmt.Sprintf("%s.crt", s.BaseFilename))
}

func (s *GenerateSignedCertStep) keyPath() string {
	return filepath.Join(s.OutputDir, fmt.Sprintf("%s.key", s.BaseFilename))
}

func (s *GenerateSignedCertStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())

	// Check if CA files exist first, as they are prerequisites
	if _, err := os.Stat(s.CACertPath); os.IsNotExist(err) {
		logger.Errorf("CA certificate %s not found. Cannot generate signed certificate.", s.CACertPath)
		return false, fmt.Errorf("CA certificate %s not found: %w", s.CACertPath, err)
	} else if err != nil {
		return false, fmt.Errorf("failed to stat CA certificate %s: %w", s.CACertPath, err)
	}
	if _, err := os.Stat(s.CAKeyPath); os.IsNotExist(err) {
		logger.Errorf("CA key %s not found. Cannot generate signed certificate.", s.CAKeyPath)
		return false, fmt.Errorf("CA key %s not found: %w", s.CAKeyPath, err)
	} else if err != nil {
		return false, fmt.Errorf("failed to stat CA key %s: %w", s.CAKeyPath, err)
	}


	certExists := false
	if _, err := os.Stat(s.certPath()); err == nil {
		certExists = true
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to stat certificate %s: %w", s.certPath(), err)
	}

	keyExists := false
	if _, err := os.Stat(s.keyPath()); err == nil {
		keyExists = true
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to stat key %s: %w", s.keyPath(), err)
	}

	if certExists && keyExists {
		// TODO: Optionally validate existing cert (CN, SANs, Expiry, Issuer)
		logger.Info("Signed certificate and key already exist. Skipping generation.", "cert", s.certPath(), "key", s.keyPath())
		return true, nil
	}
	logger.Info("Signed certificate or key missing. Generation required.", "cert_exists", certExists, "key_exists", keyExists)
	return false, nil
}

func (s *GenerateSignedCertStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	logger.Info("Generating signed certificate and private key...")

	// Load CA certificate
	caCertPEM, err := os.ReadFile(s.CACertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate file %s: %w", s.CACertPath, err)
	}
	pemBlock, _ := pem.Decode(caCertPEM)
	if pemBlock == nil {
		return fmt.Errorf("failed to decode PEM block from CA certificate %s", s.CACertPath)
	}
	caCert, err := x509.ParseCertificate(pemBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate %s: %w", s.CACertPath, err)
	}

	// Load CA private key
	caKeyPEM, err := os.ReadFile(s.CAKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read CA private key file %s: %w", s.CAKeyPath, err)
	}
	pemBlock, _ = pem.Decode(caKeyPEM)
	if pemBlock == nil {
		return fmt.Errorf("failed to decode PEM block from CA private key %s", s.CAKeyPath)
	}
	caPrivKey, err := x509.ParsePKCS8PrivateKey(pemBlock.Bytes) // Assuming PKCS8 format for simplicity
	if err != nil {
		// Try PKCS1 as a fallback for RSA keys
		if rsaKey, rsaErr := x509.ParsePKCS1PrivateKey(pemBlock.Bytes); rsaErr == nil {
			caPrivKey = rsaKey
		} else {
			return fmt.Errorf("failed to parse CA private key %s (tried PKCS8 and PKCS1): %w / %w", s.CAKeyPath, err, rsaErr)
		}
	}


	// Generate private key for the new certificate
	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate RSA private key for certificate %s: %w", s.CommonName, err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(time.Duration(s.ValidityDays) * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number for certificate %s: %w", s.CommonName, err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   s.CommonName,
			Organization: s.Organizations,
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{},
		BasicConstraintsValid: true,
	}
	if s.IsClientCert { template.ExtKeyUsage = append(template.ExtKeyUsage, x509.ExtKeyUsageClientAuth) }
	if s.IsServerCert { template.ExtKeyUsage = append(template.ExtKeyUsage, x509.ExtKeyUsageServerAuth) }


	for _, san := range s.SANs {
		if ip := net.ParseIP(san); ip != nil {
			template.IPAddresses = append(template.IPAddresses, ip)
		} else {
			template.DNSNames = append(template.DNSNames, san)
		}
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, &privKey.PublicKey, caPrivKey)
	if err != nil {
		return fmt.Errorf("failed to create signed certificate for %s: %w", s.CommonName, err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(s.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s for signed certificate: %w", s.OutputDir, err)
	}

	// Write certificate to file
	certOut, err := os.Create(s.certPath())
	if err != nil {
		return fmt.Errorf("failed to open certificate file %s for writing: %w", s.certPath(), err)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return fmt.Errorf("failed to write certificate to %s: %w", s.certPath(), err)
	}
	logger.Info("Signed certificate saved.", "path", s.certPath())

	// Write private key to file
	keyOut, err := os.OpenFile(s.keyPath(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open private key file %s for writing: %w", s.keyPath(), err)
	}
	defer keyOut.Close()
	privBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key for %s: %w", s.CommonName, err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return fmt.Errorf("failed to write private key to %s: %w", s.keyPath(), err)
	}
	logger.Info("Private key saved.", "path", s.keyPath())

	return nil
}

func (s *GenerateSignedCertStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	logger.Info("Attempting to rollback signed certificate generation by removing certificate and key.")

	certPath := s.certPath()
	if _, err := os.Stat(certPath); err == nil {
		if errRem := os.Remove(certPath); errRem != nil {
			logger.Warnf("Failed to remove certificate during rollback: %v", "path", certPath, "error", errRem)
		} else {
			logger.Info("Certificate removed.", "path", certPath)
		}
	}

	keyPath := s.keyPath()
	if _, err := os.Stat(keyPath); err == nil {
		if errRem := os.Remove(keyPath); errRem != nil {
			logger.Warnf("Failed to remove key during rollback: %v", "path", keyPath, "error", errRem)
		} else {
			logger.Info("Key removed.", "path", keyPath)
		}
	}
	return nil
}

var _ step.Step = (*GenerateSignedCertStep)(nil)
