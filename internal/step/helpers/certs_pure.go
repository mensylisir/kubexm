package helpers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"time"
)

// EncodeCertPEM encodes a certificate to PEM format.
func EncodeCertPEM(derBytes []byte) ([]byte, error) {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	}), nil
}

// EncodeKeyPEM encodes an ECDSA private key to PEM format.
func EncodeKeyPEM(key *ecdsa.PrivateKey) ([]byte, error) {
	derBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal EC private key: %w", err)
	}
	return pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: derBytes,
	}), nil
}

// EncodeRSAPrivateKeyPEM encodes an RSA private key to PEM format.
func EncodeRSAPrivateKeyPEM(key *rsa.PrivateKey) ([]byte, error) {
	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}), nil
}

// DecodeCertPEM decodes a certificate from PEM format.
func DecodeCertPEM(pemData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("unsupported type: %s", block.Type)
	}
	return x509.ParseCertificate(block.Bytes)
}

// DecodeKeyPEM decodes an ECDSA private key from PEM format.
func DecodeKeyPEM(pemData []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	if !isECPrivateKey(block.Type) {
		return nil, fmt.Errorf("unsupported key type: %s", block.Type)
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

// DecodeRSAPrivateKeyPEM decodes an RSA private key from PEM format.
func DecodeRSAPrivateKeyPEM(pemData []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found")
	}
	if block.Type != "RSA PRIVATE KEY" {
		return nil, fmt.Errorf("unsupported key type: %s", block.Type)
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

// GenerateCA generates a new CA certificate and key.
func GenerateCA(subject pkix.Name, duration time.Duration) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate CA private key: %w", err)
	}
	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))

	if duration == 0 {
		duration = 10 * 365 * 24 * time.Hour
	}
	notAfter := time.Now().Add(duration)

	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               subject,
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create CA certificate: %w", err)
	}

	caCert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse created CA certificate: %w", err)
	}

	return caCert, privKey, nil
}

// GenerateSignedCert generates a signed certificate using the given CA.
func GenerateSignedCert(cfg CertConfig, caCert *x509.Certificate, caKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key for %s: %w", cfg.CommonName, err)
	}

	duration := cfg.Duration
	if duration == 0 {
		duration = 365 * 24 * time.Hour
	}
	notAfter := time.Now().Add(duration)

	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{CommonName: cfg.CommonName, Organization: cfg.Organization},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           cfg.Usages,
		BasicConstraintsValid: true,
		DNSNames:              cfg.AltNames.DNSNames,
		IPAddresses:           cfg.AltNames.IPs,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &privKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to sign certificate for %s: %w", cfg.CommonName, err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse certificate: %w", err)
	}

	return cert, privKey, nil
}

// GenerateServiceAccountKey generates a service account key pair.
func GenerateServiceAccountKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// HashCA generates a SHA256 hash of a CA certificate for identification.
func HashCA(caCert *x509.Certificate) string {
	sum := sha256.Sum256(caCert.Raw)
	return fmt.Sprintf("%x", sum)
}

// IsCertificateBundlePEM checks if PEM data contains multiple certificates.
func IsCertificateBundlePEM(pemData []byte) bool {
	var certCount int
	rest := pemData

	for {
		block, remainingData := pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			certCount++
		}
		if len(remainingData) == 0 || len(rest) == len(remainingData) {
			break
		}
		rest = remainingData
	}

	return certCount > 1
}
