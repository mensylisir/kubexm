package pki

import (
	"fmt"
	"path/filepath"
	"strings" // For strings.ReplaceAll in subject formatting

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime" // For runtime.Logger and runtime.StepContext
	"github.com/mensylisir/kubexm/pkg/step"
	// time, spec, context no longer needed
)

// GenerateRootCAStep generates a root Certificate Authority.
type GenerateRootCAStep struct {
	CertPath     string
	KeyPath      string
	CommonName   string
	ValidityDays int
	KeyBitSize   int
	StepName     string
}

// NewGenerateRootCAStep creates a new GenerateRootCAStep.
func NewGenerateRootCAStep(certPath, keyPath, commonName string, validityDays, keyBitSize int, stepName string) step.Step {
	s := &GenerateRootCAStep{
		CertPath:     certPath,
		KeyPath:      keyPath,
		CommonName:   commonName,
		ValidityDays: validityDays,
		KeyBitSize:   keyBitSize,
		StepName:     stepName,
	}
	// Defaults are applied in populateDefaults, called by Precheck/Run via their context logger
	return s
}

func (s *GenerateRootCAStep) populateDefaults(logger runtime.Logger) {
	defaultBaseDir := "/etc/kubexms/pki"
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
		s.KeyBitSize = 4096
		logger.Debug("KeyBitSize defaulted.", "bits", s.KeyBitSize)
	}
}

func (s *GenerateRootCAStep) Name() string {
	if s.StepName != "" {
		return s.StepName
	}
	// Use a temporary CertPath for naming if s.CertPath is not yet populated by defaults.
	// This ensures a somewhat meaningful default name even if populateDefaults hasn't run.
	cp := s.CertPath
	if cp == "" {
	    // This is a fallback if Name() is called before populateDefaults (e.g. by external logging before Precheck)
	    // In normal flow, populateDefaults in Precheck/Run will set s.CertPath before this might be an issue.
	    cp = filepath.Join("/etc/kubexms/pki", "ca.crt") + " (default path)"
	}
	return fmt.Sprintf("Generate Root CA (Cert: %s)", cp)
}

func (s *GenerateRootCAStep) Description() string {
	// At description time, defaults might not have been applied yet if called early.
	// For a more accurate description, ensure populateDefaults has run or use potentially empty values.
	// For now, use current values.
	return fmt.Sprintf("Generates a root CA certificate and key: CN=%s, Cert=%s, Key=%s, Validity=%d days, Bits=%d.",
		s.CommonName, s.CertPath, s.KeyPath, s.ValidityDays, s.KeyBitSize)
}

func (s *GenerateRootCAStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.Name(), err)
	}

	certExists, err := conn.Exists(ctx.GoContext(), s.CertPath)
	if err != nil {
		// If error checking, assume not done to be safe and let Run attempt generation.
		logger.Warn("Failed to check CA cert existence, assuming regeneration is needed.", "path", s.CertPath, "error", err)
		return false, nil
	}
	keyExists, err := conn.Exists(ctx.GoContext(), s.KeyPath)
	if err != nil {
		logger.Warn("Failed to check CA key existence, assuming regeneration is needed.", "path", s.KeyPath, "error", err)
		return false, nil
	}

	if certExists && keyExists {
		logger.Info("Root CA certificate and key already exist.", "certPath", s.CertPath, "keyPath", s.KeyPath)
		// Optionally, verify CA cert validity here if needed.
		// For now, existence is enough for Precheck.
		return true, nil
	}

	if certExists && !keyExists { logger.Warn("Root CA cert exists, but key does not. Will attempt to regenerate.", "certPath", s.CertPath, "keyPath", s.KeyPath) }
	if !certExists && keyExists { logger.Warn("Root CA key exists, but cert does not. Will attempt to regenerate.", "keyPath", s.KeyPath, "certPath", s.CertPath) }
	if !certExists && !keyExists { logger.Debug("Root CA certificate and/or key do not exist.", "certPath", s.CertPath, "keyPath", s.KeyPath) }
	return false, nil
}

func (s *GenerateRootCAStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for step %s: %w", host.GetName(), s.Name(), err)
	}

	if _, err := conn.LookPath(ctx.GoContext(), "openssl"); err != nil {
		return fmt.Errorf("openssl command not found on host %s for step %s: %w", host.GetName(), s.Name(), err)
	}

	certDir := filepath.Dir(s.CertPath)
	keyDir := filepath.Dir(s.KeyPath)

	execOptsSudo := &connector.ExecOptions{Sudo: true} // Assuming these operations need sudo

	// Using Exec to ensure directory creation with sudo if needed.
	mkdirCertCmd := fmt.Sprintf("mkdir -p %s", certDir)
	_, stderrMkdirCert, errMkdirCert := conn.Exec(ctx.GoContext(), mkdirCertCmd, execOptsSudo)
	if errMkdirCert != nil {
		return fmt.Errorf("failed to create directory %s for CA certificate (stderr: %s) on host %s: %w", certDir, string(stderrMkdirCert), host.GetName(), errMkdirCert)
	}
	chmodCertDirCmd := fmt.Sprintf("chmod 0755 %s", certDir) // Typical permissions for PKI dirs
	if _, _, errChmodDir := conn.Exec(ctx.GoContext(), chmodCertDirCmd, execOptsSudo); errChmodDir != nil {
		logger.Warn("Failed to chmod CA cert directory.", "path", certDir, "error", errChmodDir)
	}


	if certDir != keyDir {
		mkdirKeyCmd := fmt.Sprintf("mkdir -p %s", keyDir)
		_, stderrMkdirKey, errMkdirKey := conn.Exec(ctx.GoContext(), mkdirKeyCmd, execOptsSudo)
		if errMkdirKey != nil {
			return fmt.Errorf("failed to create directory %s for CA key (stderr: %s) on host %s: %w", keyDir, string(stderrMkdirKey), host.GetName(), errMkdirKey)
		}
		chmodKeyDirCmd := fmt.Sprintf("chmod 0700 %s", keyDir) // Key dir more restrictive
		if _, _, errChmodDir := conn.Exec(ctx.GoContext(), chmodKeyDirCmd, execOptsSudo); errChmodDir != nil {
			logger.Warn("Failed to chmod CA key directory.", "path", keyDir, "error", errChmodDir)
		}
	} else { // If same dir, ensure it has appropriate restrictive permissions for the key
	    chmodKeyDirCmd := fmt.Sprintf("chmod 0700 %s", keyDir)
		if _, _, errChmodDir := conn.Exec(ctx.GoContext(), chmodKeyDirCmd, execOptsSudo); errChmodDir != nil {
			logger.Warn("Failed to chmod CA key directory (shared with cert).", "path", keyDir, "error", errChmodDir)
		}
	}


	genKeyCmd := fmt.Sprintf("openssl genpkey -algorithm RSA -out %s -pkeyopt rsa_keygen_bits:%d", s.KeyPath, s.KeyBitSize)
	logger.Info("Generating CA private key.", "keyPath", s.KeyPath)
	_, stderrKey, errKey := conn.Exec(ctx.GoContext(), genKeyCmd, execOptsSudo)
	if errKey != nil {
		return fmt.Errorf("failed to generate CA private key %s on host %s (stderr: %s): %w", s.KeyPath, host.GetName(), string(stderrKey), errKey)
	}
	chmodKeyCmd := fmt.Sprintf("chmod 0600 %s", s.KeyPath)
	if _, _, errChmodKey := conn.Exec(ctx.GoContext(), chmodKeyCmd, execOptsSudo); errChmodKey != nil {
		logger.Warn("Failed to chmod CA key.", "keyPath", s.KeyPath, "error", errChmodKey)
	}

	// Basic escaping for CN in subj string. Complex CNs might need more robust escaping.
	subject := fmt.Sprintf("/CN=%s", strings.ReplaceAll(s.CommonName, "'", `'\''`))
	genCertCmd := fmt.Sprintf("openssl req -x509 -new -nodes -key %s -sha256 -days %d -out %s -subj '%s'",
		s.KeyPath, s.ValidityDays, s.CertPath, subject)
	logger.Info("Generating self-signed CA certificate.", "certPath", s.CertPath)
	_, stderrCert, errCert := conn.Exec(ctx.GoContext(), genCertCmd, execOptsSudo)
	if errCert != nil {
		return fmt.Errorf("failed to generate CA certificate %s on host %s (stderr: %s): %w", s.CertPath, host.GetName(), string(stderrCert), errCert)
	}
	chmodCertCmd := fmt.Sprintf("chmod 0644 %s", s.CertPath)
	if _, _, errChmodCert := conn.Exec(ctx.GoContext(), chmodCertCmd, execOptsSudo); errChmodCert != nil {
		logger.Warn("Failed to chmod CA cert.", "certPath", s.CertPath, "error", errChmodCert)
	}

	// Store paths in ModuleCache as per original logic
	const clusterRootCACertPathKey = "ClusterRootCACertPath"
	const clusterRootCAKeyPathKey  = "ClusterRootCAKeyPath"
	ctx.ModuleCache().Set(clusterRootCACertPathKey, s.CertPath)
	ctx.ModuleCache().Set(clusterRootCAKeyPathKey, s.KeyPath)
	logger.Info("Root CA certificate and key generated and paths stored in ModuleCache.", "certPath", s.CertPath, "keyPath", s.KeyPath)
	return nil
}

func (s *GenerateRootCAStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", host.GetName(), s.Name(), err)
	}

	execOptsSudo := &connector.ExecOptions{Sudo: true}

	logger.Info("Attempting to remove CA certificate for rollback.", "path", s.CertPath)
	rmCertCmd := fmt.Sprintf("rm -f %s", s.CertPath)
	if _, stderrRmCert, errRmCert := conn.Exec(ctx.GoContext(), rmCertCmd, execOptsSudo); errRmCert != nil {
		logger.Error("Failed to remove CA certificate during rollback.", "path", s.CertPath, "stderr", string(stderrRmCert), "error", errRmCert)
	}

	logger.Info("Attempting to remove CA key for rollback.", "path", s.KeyPath)
	rmKeyCmd := fmt.Sprintf("rm -f %s", s.KeyPath)
	if _, stderrRmKey, errRmKey := conn.Exec(ctx.GoContext(), rmKeyCmd, execOptsSudo); errRmKey != nil {
		logger.Error("Failed to remove CA key during rollback.", "path", s.KeyPath, "stderr", string(stderrRmKey), "error", errRmKey)
	}

	const clusterRootCACertPathKey = "ClusterRootCACertPath"
	const clusterRootCAKeyPathKey  = "ClusterRootCAKeyPath"
    ctx.ModuleCache().Delete(clusterRootCACertPathKey)
    ctx.ModuleCache().Delete(clusterRootCAKeyPathKey)
	logger.Debug("Cleaned up CA paths from ModuleCache.")

	logger.Info("Rollback attempt for CA generation finished (files removed if existed, cache keys deleted).")
	return nil
}

// Ensure GenerateRootCAStep implements the step.Step interface.
var _ step.Step = (*GenerateRootCAStep)(nil)
