package pki

import (
	"fmt"
	"path/filepath"
	"strings" // For strings.ReplaceAll in subject formatting

	"github.com/mensylisir/kubexm/pkg/connector"
	// "github.com/mensylisir/kubexm/pkg/engine" // Removed
	"github.com/mensylisir/kubexm/pkg/logger" // For logger.Logger type
	"github.com/mensylisir/kubexm/pkg/spec"   // Added for StepMeta
	"github.com/mensylisir/kubexm/pkg/step"   // For step.StepContext
)

// GenerateRootCAStep generates a root Certificate Authority.
type GenerateRootCAStep struct {
	meta         spec.StepMeta
	CertPath     string
	KeyPath      string
	CommonName   string
	ValidityDays int
	KeyBitSize   int
	Sudo         bool // Whether to use sudo for file operations and openssl commands
}

// NewGenerateRootCAStep creates a new GenerateRootCAStep.
func NewGenerateRootCAStep(instanceName, certPath, keyPath, commonName string, validityDays, keyBitSize int, sudo bool) step.Step {
	s := &GenerateRootCAStep{
		// Meta will be populated/updated by populateDefaults
		CertPath:     certPath,
		KeyPath:      keyPath,
		CommonName:   commonName,
		ValidityDays: validityDays,
		KeyBitSize:   keyBitSize,
		Sudo:         sudo,
	}
	// Set initial meta name
	if instanceName == "" {
		instanceName = "GenerateRootCA"
	}
	s.meta.Name = instanceName
	// Description will be set more accurately after defaults are populated.
	s.meta.Description = "Generates a root CA certificate and key."
	return s
}

func (s *GenerateRootCAStep) populateDefaults(logger logger.Logger) { // Changed to logger.Logger
	defaultBaseDir := "/etc/kubexms/pki" // This could be configurable via ClusterSpec
	if s.CertPath == "" {
		s.CertPath = filepath.Join(defaultBaseDir, "ca.crt")
		logger.Debug("CertPath defaulted.", "path", s.CertPath)
	}
	if s.KeyPath == "" {
		s.KeyPath = filepath.Join(defaultBaseDir, "ca.key")
		logger.Debug("KeyPath defaulted.", "path", s.KeyPath)
	}
	if s.CommonName == "" {
		s.CommonName = "kubexms-root-ca"
		logger.Debug("CommonName defaulted.", "cn", s.CommonName)
	}
	if s.ValidityDays == 0 {
		s.ValidityDays = 365 * 10 // 10 years
		logger.Debug("ValidityDays defaulted.", "days", s.ValidityDays)
	}
	if s.KeyBitSize == 0 {
		s.KeyBitSize = 4096 // Standard default
		logger.Debug("KeyBitSize defaulted.", "bits", s.KeyBitSize)
	}
	// Update description with actual values
	s.meta.Description = fmt.Sprintf("Generates a root CA certificate and key: CN=%s, Cert=%s, Key=%s, Validity=%d days, Bits=%d.",
		s.CommonName, s.CertPath, s.KeyPath, s.ValidityDays, s.KeyBitSize)
	if s.meta.Name == "GenerateRootCA" { // If default name was used, make it more specific
		s.meta.Name = fmt.Sprintf("GenerateRootCA-%s", s.CommonName)
	}
}

// Meta returns the step's metadata.
func (s *GenerateRootCAStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *GenerateRootCAStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) { // Changed to step.StepContext
	// populateDefaults needs to be called first to ensure paths are set for check
	// and logger uses the final step name.
	s.populateDefaults(ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "populateDefaults.Precheck"))
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	certExists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.CertPath) // Sudo not needed for Exists
	if err != nil {
		logger.Warn("Failed to check CA cert existence, assuming regeneration is needed.", "path", s.CertPath, "error", err)
		return false, nil
	}
	keyExists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.KeyPath) // Sudo not needed for Exists
	if err != nil {
		logger.Warn("Failed to check CA key existence, assuming regeneration is needed.", "path", s.KeyPath, "error", err)
		return false, nil
	}

	if certExists && keyExists {
		logger.Info("Root CA certificate and key already exist.", "certPath", s.CertPath, "keyPath", s.KeyPath)
		return true, nil
	}

	if certExists && !keyExists {
		logger.Warn("Root CA cert exists, but key does not. Will attempt to regenerate.", "certPath", s.CertPath, "keyPath", s.KeyPath)
	}
	if !certExists && keyExists {
		logger.Warn("Root CA key exists, but cert does not. Will attempt to regenerate.", "keyPath", s.KeyPath, "certPath", s.CertPath)
	}
	if !certExists && !keyExists {
		logger.Debug("Root CA certificate and/or key do not exist.", "certPath", s.CertPath, "keyPath", s.KeyPath)
	}
	return false, nil
}

func (s *GenerateRootCAStep) Run(ctx step.StepContext, host connector.Host) error { // Changed to step.StepContext
	s.populateDefaults(ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "populateDefaults.Run"))
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	if _, err := runnerSvc.LookPath(ctx.GoContext(), conn, "openssl"); err != nil {
		return fmt.Errorf("openssl command not found on host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}

	certDir := filepath.Dir(s.CertPath)
	keyDir := filepath.Dir(s.KeyPath)

	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, certDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create directory %s for CA certificate on host %s: %w", certDir, host.GetName(), err)
	}
	// No separate chmod for dir needed if Mkdirp handles it or perms are sufficient by default.

	if certDir != keyDir {
		if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, keyDir, "0700", s.Sudo); err != nil { // Key dir more restrictive
			return fmt.Errorf("failed to create directory %s for CA key on host %s: %w", keyDir, host.GetName(), err)
		}
	} else { // If same dir, ensure it has appropriate restrictive permissions for the key
		if err := runnerSvc.Chmod(ctx.GoContext(), conn, keyDir, "0700", s.Sudo); err != nil {
			logger.Warn("Failed to chmod CA key directory (shared with cert).", "path", keyDir, "error", err)
		}
	}

	genKeyCmd := fmt.Sprintf("openssl genpkey -algorithm RSA -out %s -pkeyopt rsa_keygen_bits:%d", s.KeyPath, s.KeyBitSize)
	logger.Info("Generating CA private key.", "keyPath", s.KeyPath)
	if _, errKey := runnerSvc.Run(ctx.GoContext(), conn, genKeyCmd, s.Sudo); errKey != nil {
		return fmt.Errorf("failed to generate CA private key %s on host %s: %w", s.KeyPath, host.GetName(), errKey)
	}
	if errChmodKey := runnerSvc.Chmod(ctx.GoContext(), conn, s.KeyPath, "0600", s.Sudo); errChmodKey != nil {
		logger.Warn("Failed to chmod CA key.", "keyPath", s.KeyPath, "error", errChmodKey)
	}

	subject := fmt.Sprintf("/CN=%s", strings.ReplaceAll(s.CommonName, "'", `'\''`)) // Basic escaping
	genCertCmd := fmt.Sprintf("openssl req -x509 -new -nodes -key %s -sha256 -days %d -out %s -subj '%s'",
		s.KeyPath, s.ValidityDays, s.CertPath, subject)
	logger.Info("Generating self-signed CA certificate.", "certPath", s.CertPath)
	if _, errCert := runnerSvc.Run(ctx.GoContext(), conn, genCertCmd, s.Sudo); errCert != nil {
		return fmt.Errorf("failed to generate CA certificate %s on host %s: %w", s.CertPath, host.GetName(), errCert)
	}
	if errChmodCert := runnerSvc.Chmod(ctx.GoContext(), conn, s.CertPath, "0644", s.Sudo); errChmodCert != nil {
		logger.Warn("Failed to chmod CA cert.", "certPath", s.CertPath, "error", errChmodCert)
	}

	const clusterRootCACertPathKey = "ClusterRootCACertPath" // Consider moving to pkg/common
	const clusterRootCAKeyPathKey = "ClusterRootCAKeyPath"
	ctx.ModuleCache().Set(clusterRootCACertPathKey, s.CertPath)
	ctx.ModuleCache().Set(clusterRootCAKeyPathKey, s.KeyPath)
	logger.Info("Root CA certificate and key generated and paths stored in ModuleCache.", "certPath", s.CertPath, "keyPath", s.KeyPath)
	return nil
}

func (s *GenerateRootCAStep) Rollback(ctx step.StepContext, host connector.Host) error { // Changed to step.StepContext
	s.populateDefaults(ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "populateDefaults.Rollback"))
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", host.GetName(), s.meta.Name, err)
	}

	logger.Info("Attempting to remove CA certificate for rollback.", "path", s.CertPath)
	if errRmCert := runnerSvc.Remove(ctx.GoContext(), conn, s.CertPath, s.Sudo); errRmCert != nil {
		logger.Error("Failed to remove CA certificate during rollback.", "path", s.CertPath, "error", errRmCert)
	}

	logger.Info("Attempting to remove CA key for rollback.", "path", s.KeyPath)
	if errRmKey := runnerSvc.Remove(ctx.GoContext(), conn, s.KeyPath, s.Sudo); errRmKey != nil {
		logger.Error("Failed to remove CA key during rollback.", "path", s.KeyPath, "error", errRmKey)
	}

	const clusterRootCACertPathKey = "ClusterRootCACertPath"
	const clusterRootCAKeyPathKey = "ClusterRootCAKeyPath"
	ctx.ModuleCache().Delete(clusterRootCACertPathKey)
	ctx.ModuleCache().Delete(clusterRootCAKeyPathKey)
	logger.Debug("Cleaned up CA paths from ModuleCache.")

	logger.Info("Rollback attempt for CA generation finished (files removed if existed, cache keys deleted).")
	return nil
}

// Ensure GenerateRootCAStep implements the step.Step interface.
var _ step.Step = (*GenerateRootCAStep)(nil)
