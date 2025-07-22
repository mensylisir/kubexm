package helpers

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

type CertConfig struct {
	CommonName   string
	Organization []string
	AltNames     AltNames
	Usages       []x509.ExtKeyUsage
	Duration     time.Duration
}

type AltNames struct {
	DNSNames []string
	IPs      []net.IP
}

func NewCertificateAuthority(pkiPath string, caCertFileName, caKeyFileName string, duration time.Duration) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	caCertPath := filepath.Join(pkiPath, caCertFileName)
	caKeyPath := filepath.Join(pkiPath, caKeyFileName)

	if _, err := os.Stat(caCertPath); err == nil {
		if _, errKey := os.Stat(caKeyPath); errKey == nil {
			return LoadCertificateAuthority(caCertPath, caKeyPath)
		}
	}

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
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   "kubexm-etcd-ca",
			Organization: []string{"KubeXM"},
		},
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

	if err := WriteCert(pkiPath, caCertFileName, certDER); err != nil {
		return nil, nil, err
	}
	if err := WriteKey(pkiPath, caKeyFileName, privKey); err != nil {
		return nil, nil, err
	}

	return caCert, privKey, nil
}

func NewSignedCertificate(pkiPath, certFileName, keyFileName string, cfg CertConfig, caCert *x509.Certificate, caKey *ecdsa.PrivateKey) error {
	certPath := filepath.Join(pkiPath, certFileName)
	keyPath := filepath.Join(pkiPath, keyFileName)

	if _, err := os.Stat(certPath); err == nil {
		if _, errKey := os.Stat(keyPath); errKey == nil {
			return nil
		}
	}

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate private key for %s: %w", cfg.CommonName, err)
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
		return fmt.Errorf("failed to sign certificate for %s: %w", cfg.CommonName, err)
	}

	if err := WriteCert(pkiPath, certFileName, certDER); err != nil {
		return err
	}
	return WriteKey(pkiPath, keyFileName, privKey)
}

func LoadCertificateAuthority(certPath, keyPath string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CA certificate file %s: %w", certPath, err)
	}
	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		return nil, nil, fmt.Errorf("failed to decode PEM block containing the certificate from %s", certPath)
	}
	caCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate from %s: %w", certPath, err)
	}

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read CA private key file %s: %w", keyPath, err)
	}
	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil || !isECPrivateKey(keyBlock.Type) {
		return nil, nil, fmt.Errorf("failed to decode PEM block containing the private key from %s (type was %s)", keyPath, keyBlock.Type)
	}
	caKey, err := x509.ParseECPrivateKey(keyBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA private key from %s: %w", keyPath, err)
	}

	return caCert, caKey, nil
}

func WriteCert(pkiPath, fileName string, derBytes []byte) error {
	certOut, err := os.Create(filepath.Join(pkiPath, fileName))
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", fileName, err)
	}
	defer certOut.Close()
	return pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
}

func WriteKey(pkiPath, fileName string, key *ecdsa.PrivateKey) error {
	keyOut, err := os.OpenFile(filepath.Join(pkiPath, fileName), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", fileName, err)
	}
	defer keyOut.Close()

	derBytes, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to marshal EC private key: %w", err)
	}

	return pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: derBytes})
}

func isECPrivateKey(blockType string) bool {
	return blockType == "EC PRIVATE KEY" || blockType == "PRIVATE KEY"
}
