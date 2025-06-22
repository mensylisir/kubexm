package pki

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// GenerateCACertStep generates a self-signed CA certificate and key.
type GenerateCACertStep struct {
	meta          spec.StepMeta
	CommonName    string
	Organizations []string
	ValidityDays  int
	OutputDir     string // Directory where ca.crt and ca.key will be saved
	BaseFilename  string // Base name for cert and key files, e.g., "ca" or "etcd-ca"
}

// NewGenerateCACertStep creates a new GenerateCACertStep.
// instanceName is optional. validityDays defaults to 3650 (10 years) if 0.
// baseFilename defaults to "ca" if empty.
func NewGenerateCACertStep(instanceName, commonName string, organizations []string, validityDays int, outputDir, baseFilename string) step.Step {
	metaName := instanceName
	if metaName == "" {
		metaName = fmt.Sprintf("GenerateCA-%s", commonName)
	}
	if validityDays <= 0 {
		validityDays = 3650 // Default to 10 years
	}
	fname := baseFilename
	if fname == "" {
		fname = "ca"
	}

	return &GenerateCACertStep{
		meta: spec.StepMeta{
			Name:        metaName,
			Description: fmt.Sprintf("Generates a self-signed CA certificate for %s", commonName),
		},
		CommonName:    commonName,
		Organizations: organizations,
		ValidityDays:  validityDays,
		OutputDir:     outputDir,
		BaseFilename:  fname,
	}
}

func (s *GenerateCACertStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *GenerateCACertStep) certPath() string {
	return filepath.Join(s.OutputDir, fmt.Sprintf("%s.crt", s.BaseFilename))
}

func (s *GenerateCACertStep) keyPath() string {
	return filepath.Join(s.OutputDir, fmt.Sprintf("%s.key", s.BaseFilename))
}

func (s *GenerateCACertStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	certExists := false
	if _, err := os.Stat(s.certPath()); err == nil {
		certExists = true
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to stat CA certificate %s: %w", s.certPath(), err)
	}

	keyExists := false
	if _, err := os.Stat(s.keyPath()); err == nil {
		keyExists = true
	} else if !os.IsNotExist(err) {
		return false, fmt.Errorf("failed to stat CA key %s: %w", s.keyPath(), err)
	}

	if certExists && keyExists {
		// TODO: Optionally validate existing CA cert (e.g., expiry, CN)
		logger.Info("CA certificate and key already exist. Skipping generation.", "cert", s.certPath(), "key", s.keyPath())
		return true, nil
	}
	logger.Info("CA certificate or key missing. Generation required.", "cert_exists", certExists, "key_exists", keyExists)
	return false, nil
}

func (s *GenerateCACertStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	logger.Info("Generating CA certificate and private key...")

	privKey, err := rsa.GenerateKey(rand.Reader, 2048) // Using 2048 bits for RSA key
	if err != nil {
		return fmt.Errorf("failed to generate RSA private key for CA: %w", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.Add(time.Duration(s.ValidityDays) * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("failed to generate serial number for CA: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   s.CommonName,
			Organization: s.Organizations,
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	if err != nil {
		return fmt.Errorf("failed to create CA certificate: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(s.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s for CA: %w", s.OutputDir, err)
	}

	// Write CA certificate to file
	certOut, err := os.Create(s.certPath())
	if err != nil {
		return fmt.Errorf("failed to open CA certificate file %s for writing: %w", s.certPath(), err)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return fmt.Errorf("failed to write CA certificate to %s: %w", s.certPath(), err)
	}
	logger.Info("CA certificate saved.", "path", s.certPath())

	// Write CA private key to file
	keyOut, err := os.OpenFile(s.keyPath(), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open CA private key file %s for writing: %w", s.keyPath(), err)
	}
	defer keyOut.Close()
	privBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("failed to marshal CA private key: %w", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return fmt.Errorf("failed to write CA private key to %s: %w", s.keyPath(), err)
	}
	logger.Info("CA private key saved.", "path", s.keyPath())

	return nil
}

func (s *GenerateCACertStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())
	logger.Info("Attempting to rollback CA generation by removing certificate and key.")

	certPath := s.certPath()
	if _, err := os.Stat(certPath); err == nil {
		if errRem := os.Remove(certPath); errRem != nil {
			logger.Warnf("Failed to remove CA certificate during rollback: %v", "path", certPath, "error", errRem)
			// Continue to attempt key removal
		} else {
			logger.Info("CA certificate removed.", "path", certPath)
		}
	}

	keyPath := s.keyPath()
	if _, err := os.Stat(keyPath); err == nil {
		if errRem := os.Remove(keyPath); errRem != nil {
			logger.Warnf("Failed to remove CA key during rollback: %v", "path", keyPath, "error", errRem)
		} else {
			logger.Info("CA key removed.", "path", keyPath)
		}
	}
	// Note: This doesn't remove the OutputDir itself, only the cert/key.
	return nil
}

var _ step.Step = (*GenerateCACertStep)(nil)
