package kubeadm

import (
	"crypto"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

const (
	DefaultCAValidity = 10 * 365 * 24 * time.Hour
)

type KubeadmRenewK8sCAStep struct {
	step.Base
	localNewCertsDir string
	casToRenew       []caAsset
	validity         time.Duration
}

type KubeadmRenewK8sCAStepBuilder struct {
	step.Builder[KubeadmRenewK8sCAStepBuilder, *KubeadmRenewK8sCAStep]
}

func NewKubeadmRenewK8sCAStepBuilder(ctx runtime.Context, instanceName string) *KubeadmRenewK8sCAStepBuilder {
	localCertsDir := ctx.GetKubernetesCertsDir()
	certsNewDir := filepath.Join(localCertsDir, "certs-new")

	assets := []caAsset{
		{CertFile: "ca.crt", KeyFile: "ca.key"},
		{CertFile: "front-proxy-ca.crt", KeyFile: "front-proxy-ca.key"},
	}

	s := &KubeadmRenewK8sCAStep{
		localNewCertsDir: certsNewDir,
		casToRenew:       assets,
		validity:         DefaultCAValidity,
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate new Kubernetes CA certificates using existing private keys"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(KubeadmRenewK8sCAStepBuilder).Init(s)
	return b
}

func (s *KubeadmRenewK8sCAStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmRenewK8sCAStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")

	for _, asset := range s.casToRenew {
		keyPath := filepath.Join(s.localNewCertsDir, asset.KeyFile)
		certPath := filepath.Join(s.localNewCertsDir, asset.CertFile)

		if !helpers.IsFileExist(keyPath) {
			return false, fmt.Errorf("required private key '%s' for CA renewal is missing", keyPath)
		}

		if helpers.IsFileExist(certPath) {
			logger.Infof("New CA certificate '%s' already exists. Step is done.", certPath)
			return true, nil
		}
	}
	return false, nil
}

func (s *KubeadmRenewK8sCAStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	logger.Info("Starting renewal of Kubernetes CA certificates...")

	for _, asset := range s.casToRenew {
		log := logger.With("ca_name", asset.CertFile)
		log.Infof("Generating new CA certificate with %v validity...", s.validity)

		keyPath := filepath.Join(s.localNewCertsDir, asset.KeyFile)
		certPath := filepath.Join(s.localNewCertsDir, asset.CertFile)

		subject, err := getKubeadmCASubject(asset.CertFile)
		if err != nil {
			return err
		}

		if err := generateNewCACert(keyPath, certPath, subject, s.validity); err != nil {
			log.Errorf("Failed to generate new CA certificate: %v", err)
			return fmt.Errorf("failed to generate new CA '%s': %w", asset.CertFile, err)
		}
		log.Info("Successfully generated new CA certificate.")
	}

	logger.Info("All Kubernetes CA certificates have been successfully renewed.")
	return nil
}

func (s *KubeadmRenewK8sCAStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rolling back by deleting newly generated CA certificates from 'certs-new' directory...")

	for _, asset := range s.casToRenew {
		newCertPath := filepath.Join(s.localNewCertsDir, asset.CertFile)
		_ = os.Remove(newCertPath)
	}
	return nil
}

func getKubeadmCASubject(certFile string) (pkix.Name, error) {
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

func generateNewCACert(keyPath, certPath string, subject pkix.Name, validity time.Duration) error {
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
		SerialNumber:          newSerialNumber(),
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

func newSerialNumber() *big.Int {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, _ := rand.Int(rand.Reader, serialNumberLimit)
	return serialNumber
}

var _ step.Step = (*KubeadmRenewK8sCAStep)(nil)
