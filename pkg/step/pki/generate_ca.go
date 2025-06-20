package pki

import (
	"fmt"
	"path/filepath"
	"strings" // For strings.ReplaceAll in subject formatting

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime" // For runtime.Logger and runtime.StepContext
	"github.com/mensylisir/kubexm/pkg/spec"    // Added for spec.StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
	// time, context no longer needed
)

// GenerateRootCAStepSpec generates a root Certificate Authority.
// This is a spec object, the actual execution is handled by an Executor.
type GenerateRootCAStepSpec struct {
	spec.StepMeta `json:",inline"` // Embed common meta fields
	CertPath      string           `json:"certPath,omitempty"`
	KeyPath       string           `json:"keyPath,omitempty"`
	CommonName   string `json:"commonName,omitempty"`
	ValidityDays int    `json:"validityDays,omitempty"`
	KeyBitSize   int    `json:"keyBitSize,omitempty"`
}

// NewGenerateRootCAStepSpec creates a new GenerateRootCAStepSpec.
func NewGenerateRootCAStepSpec(certPath, keyPath, commonName string, validityDays, keyBitSize int, stepName string) *GenerateRootCAStepSpec {
	// Determine default name and description if stepName is empty
	name := stepName
	cp := certPath
	if cp == "" {
		cp = "/etc/kubexms/pki/ca.crt (defaulted)" // temp for description if certPath is empty
	}
	if name == "" {
		name = fmt.Sprintf("Generate Root CA (Cert: %s)", cp)
	}
	description := fmt.Sprintf("Generates a root CA certificate and key: CN=%s, Cert=%s, Key=%s, Validity=%d days, Bits=%d.",
		commonName, certPath, keyPath, validityDays, keyBitSize) // Removed PEM paths from here

	s := &GenerateRootCAStepSpec{
		StepMeta: spec.StepMeta{
			Name:        name,
			Description: description, // Initial description, might be refined by populateDefaults
		},
		CertPath:     certPath,
		KeyPath:      keyPath,
		CommonName:   commonName,
		ValidityDays: validityDays,
		KeyBitSize:   keyBitSize,
	}
	// Note: populateDefaults is not called here, it's typically called by an executor using context.
	// The description might be re-evaluated after defaults are populated if it depends on them.
	return s
}

// GetName returns the step's name.
func (s *GenerateRootCAStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description.
// It might be more accurate after populateDefaults has run if it relies on defaulted paths.
func (s *GenerateRootCAStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *GenerateRootCAStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *GenerateRootCAStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

// populateDefaults populates default values for the step spec.
// This method is typically called by the executor before Precheck or Run.
func (s *GenerateRootCAStepSpec) populateDefaults(logger runtime.Logger) {
	defaultBaseDir := "/etc/kubexms/pki"
	if s.CertPath == "" {
		s.CertPath = filepath.Join(defaultBaseDir, "ca.crt")
		logger.Debug("CertPath defaulted.", "path", s.CertPath)
	}
	if s.KeyPath == "" {
		s.KeyPath = filepath.Join(defaultBaseDir, "ca.key")
		logger.Debug("KeyPath defaulted.", "path", s.KeyPath)
	}
	// Removed PEM path defaulting
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
	// After populating defaults, update StepMeta.Description if it relied on these values
	// This is a simplified update; a more robust approach might involve specific template parsing for description.
	s.StepMeta.Description = fmt.Sprintf("Generates a root CA certificate and key: CN=%s, Cert=%s, Key=%s, Validity=%d days, Bits=%d.",
		s.CommonName, s.CertPath, s.KeyPath, s.ValidityDays, s.KeyBitSize) // Removed PEM paths
	// Also update Name if it was default and CertPath got populated
	if strings.HasSuffix(s.StepMeta.Name, "(defaulted)") && s.CertPath != "" && !strings.HasSuffix(s.CertPath, "(defaulted)"){
		s.StepMeta.Name = fmt.Sprintf("Generate Root CA (Cert: %s)", s.CertPath)
	}
}

// Precheck (old method, to be adapted or used by an executor for a non-spec version)
func (s *GenerateRootCAStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger) // Ensure defaults are applied before precheck logic

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
	// Removed PEM file checks

	if certExists && keyExists {
		logger.Info("Root CA certificate and key already exist.", "certPath", s.CertPath, "keyPath", s.KeyPath)
		return true, nil
	}

	// Log specific missing files
	missingMessages := []string{}
	if !certExists {
		missingMessages = append(missingMessages, fmt.Sprintf("CertPath: %s", s.CertPath))
	}
	if !keyExists {
		missingMessages = append(missingMessages, fmt.Sprintf("KeyPath: %s", s.KeyPath))
	}

	if len(missingMessages) > 0 {
		logger.Info(fmt.Sprintf("Root CA files are missing: %s. Will attempt to regenerate.", strings.Join(missingMessages, ", ")))
	}

	return false, nil
}

// Run (old method, to be adapted or used by an executor for a non-spec version)
func (s *GenerateRootCAStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger) // Ensure defaults are applied

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

	// Store paths in ModuleCache
	const clusterRootCACertPathKey = "ClusterRootCACertPath"
	const clusterRootCAKeyPathKey = "ClusterRootCAKeyPath"
	// Removed PEM cache key constants

	ctx.ModuleCache().Set(clusterRootCACertPathKey, s.CertPath)
	ctx.ModuleCache().Set(clusterRootCAKeyPathKey, s.KeyPath)
	logger.Info("Root CA certificate and key generated and paths stored in ModuleCache.", "certPath", s.CertPath, "keyPath", s.KeyPath)

	// Removed PEM copy and chmod logic
	// Removed PEM path storage in ModuleCache

	return nil
}

// Rollback (old method, to be adapted or used by an executor for a non-spec version)
func (s *GenerateRootCAStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger) // Ensure defaults are applied

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback of step %s: %w", host.GetName(), s.GetName(), err)
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
	const clusterRootCAKeyPathKey = "ClusterRootCAKeyPath"
	// Removed PEM cache key constants

	// Removed PEM file deletion logic

	ctx.ModuleCache().Delete(clusterRootCACertPathKey)
	ctx.ModuleCache().Delete(clusterRootCAKeyPathKey)
	// Removed PEM ModuleCache deletion
	logger.Debug("Cleaned up CA cert/key paths from ModuleCache.")

	logger.Info("Rollback attempt for CA generation finished (crt/key files removed if existed, cache keys deleted).")
	return nil
}

// Ensure GenerateRootCAStepSpec implements the spec.StepSpec interface (conceptually).
// Actual interface implementation might be via an ExecutableStepSpec that wraps this data spec.
// var _ spec.StepSpec = (*GenerateRootCAStepSpec)(nil) // This line won't compile if spec.StepSpec is just GetName/GetDescription

// The following ensures that the old step.Step interface is still implemented by the new Spec object
// if this code is intended to be gradually refactored and used by an old-style executor temporarily.
// For a pure spec object, these Precheck/Run/Rollback methods would be removed from the Spec struct
// and live in a separate StepExecutor struct.
var _ step.Step = (*GenerateRootCAStepSpec)(nil)
