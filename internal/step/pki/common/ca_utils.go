package common

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"
)

// DefaultCAValidity is the default validity period for CA certificates (10 years).
const DefaultCAValidity = 10 * 365 * 24 * time.Hour

// GetKubeadmCASubject returns the pkix.Name subject for kubeadm-managed CA certificates.
func GetKubeadmCASubject(certFile string) (pkix.Name, error) {
	switch certFile {
	case "ca.crt":
		return pkix.Name{
			CommonName:   "kubernetes",
			Organization: []string{"system:masters"},
		}, nil
	case "front-proxy-ca.crt":
		return pkix.Name{
			CommonName: "front-proxy-ca",
		}, nil
	default:
		return pkix.Name{}, fmt.Errorf("unknown kubeadm CA file: %s", certFile)
	}
}

// GenerateNewCACert generates a new CA certificate using the existing private key.
// It writes the new certificate to certPath using the key at keyPath.
func GenerateNewCACert(keyPath, certPath string, subject pkix.Name, validity time.Duration) error {
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read private key from %s: %w", keyPath, err)
	}
	pemBlock, _ := pem.Decode(keyPEM)
	if pemBlock == nil {
		return fmt.Errorf("failed to decode PEM block from key file %s", keyPath)
	}
	privateKey, err := x509.ParsePKCS8PrivateKey(pemBlock.Bytes)
	if err != nil {
		privateKey, err = x509.ParsePKCS1PrivateKey(pemBlock.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse private key from %s: %w", keyPath, err)
		}
	}

	publicKey := privateKey.(crypto.Signer).Public()

	template := &x509.Certificate{
		SerialNumber:          NewSerialNumber(),
		Subject:               subject,
		NotBefore:             time.Now().Add(-5 * time.Minute).UTC(),
		NotAfter:              time.Now().UTC().Add(validity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, template, publicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	certFile, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to create certificate file at %s: %w", certPath, err)
	}
	defer certFile.Close()

	if err := pem.Encode(certFile, &pem.Block{Type: "CERTIFICATE", Bytes: certBytes}); err != nil {
		return fmt.Errorf("failed to write certificate to %s: %w", certPath, err)
	}

	return nil
}

// NewSerialNumber generates a cryptographically random serial number.
func NewSerialNumber() *big.Int {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, _ := rand.Int(rand.Reader, serialNumberLimit)
	return serialNumber
}
