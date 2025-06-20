package pki

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/utils" // For PathRequiresSudo
)

// GenerateCAWithPEMStepSpec generates a Certificate Authority (CA) and outputs
// the certificate and key directly in PEM format.
type GenerateCAWithPEMStepSpec struct {
	spec.StepMeta `json:",inline"`

	CertPemPath       string `json:"certPemPath,omitempty"`
	KeyPemPath        string `json:"keyPemPath,omitempty"`
	CommonName        string `json:"commonName,omitempty"`
	ValidityDays      int    `json:"validityDays,omitempty"`
	KeyBitSize        int    `json:"keyBitSize,omitempty"`
	PermissionsCertPem string `json:"permissionsCertPem,omitempty"`
	PermissionsKeyPem   string `json:"permissionsKeyPem,omitempty"`
	OutputCertPemCacheKey string `json:"outputCertPemCacheKey,omitempty"`
	OutputKeyPemCacheKey  string `json:"outputKeyPemCacheKey,omitempty"`
}

// NewGenerateCAWithPEMStepSpec creates a new GenerateCAWithPEMStepSpec.
func NewGenerateCAWithPEMStepSpec(name, description, certPemPath, keyPemPath, commonName string, validityDays, keyBitSize int) *GenerateCAWithPEMStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Generate CA (PEM format)"
	}
	finalDescription := description
	// Description will be refined in populateDefaults once paths are finalized.

	return &GenerateCAWithPEMStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		CertPemPath:       certPemPath,
		KeyPemPath:        keyPemPath,
		CommonName:        commonName,
		ValidityDays:      validityDays,
		KeyBitSize:        keyBitSize,
	}
}

// Name returns the step's name.
func (s *GenerateCAWithPEMStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *GenerateCAWithPEMStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *GenerateCAWithPEMStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *GenerateCAWithPEMStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *GenerateCAWithPEMStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *GenerateCAWithPEMStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *GenerateCAWithPEMStepSpec) populateDefaults(logger runtime.Logger) {
	defaultBaseDir := "/etc/kubexms/pki"
	if s.CertPemPath == "" {
		s.CertPemPath = filepath.Join(defaultBaseDir, "ca.pem")
		logger.Debug("CertPemPath defaulted.", "path", s.CertPemPath)
	}
	if s.KeyPemPath == "" {
		s.KeyPemPath = filepath.Join(defaultBaseDir, "ca-key.pem")
		logger.Debug("KeyPemPath defaulted.", "path", s.KeyPemPath)
	}
	if s.CommonName == "" {
		s.CommonName = "kubexms-ca-pem"
		logger.Debug("CommonName defaulted.", "cn", s.CommonName)
	}
	if s.ValidityDays == 0 {
		s.ValidityDays = 3650 // 10 years
		logger.Debug("ValidityDays defaulted.", "days", s.ValidityDays)
	}
	if s.KeyBitSize == 0 {
		s.KeyBitSize = 4096
		logger.Debug("KeyBitSize defaulted.", "bits", s.KeyBitSize)
	}
	if s.PermissionsCertPem == "" {
		s.PermissionsCertPem = "0644"
		logger.Debug("PermissionsCertPem defaulted.", "permissions", s.PermissionsCertPem)
	}
	if s.PermissionsKeyPem == "" {
		s.PermissionsKeyPem = "0600"
		logger.Debug("PermissionsKeyPem defaulted.", "permissions", s.PermissionsKeyPem)
	}

	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Generates CA certificate (%s) and key (%s) in PEM format. CN=%s, Validity=%d days, Bits=%d.",
			s.CertPemPath, s.KeyPemPath, s.CommonName, s.ValidityDays, s.KeyBitSize)
	}
}

// Precheck checks if the PEM certificate and key already exist.
func (s *GenerateCAWithPEMStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	certExists, err := conn.Exists(ctx.GoContext(), s.CertPemPath)
	if err != nil {
		logger.Warn("Failed to check PEM cert existence, assuming regeneration is needed.", "path", s.CertPemPath, "error", err)
		return false, nil
	}
	keyExists, err := conn.Exists(ctx.GoContext(), s.KeyPemPath)
	if err != nil {
		logger.Warn("Failed to check PEM key existence, assuming regeneration is needed.", "path", s.KeyPemPath, "error", err)
		return false, nil
	}

	if certExists && keyExists {
		// TODO: Optionally verify CA cert validity and match with CommonName/Key if needed.
		logger.Info("PEM CA certificate and key already exist.", "certPath", s.CertPemPath, "keyPath", s.KeyPemPath)
		return true, nil
	}

	logger.Info("PEM CA certificate and/or key do not exist. Generation needed.", "certExists", certExists, "keyExists", keyExists)
	return false, nil
}

// Run executes the OpenSSL commands to generate PEM certificate and key.
func (s *GenerateCAWithPEMStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	if _, err := conn.LookPath(ctx.GoContext(), "openssl"); err != nil {
		return fmt.Errorf("openssl command not found on host %s: %w", host.GetName(), err)
	}

	// Ensure target directories exist
	for _, targetPath := range []string{s.CertPemPath, s.KeyPemPath} {
		targetDir := filepath.Dir(targetPath)
		// Assume sudo might be needed if paths are in /etc or similar
		execOptsMkdir := &connector.ExecOptions{Sudo: utils.PathRequiresSudo(targetDir)}
		mkdirCmd := fmt.Sprintf("mkdir -p %s", targetDir)
		_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, execOptsMkdir)
		if errMkdir != nil {
			return fmt.Errorf("failed to create directory %s (stderr: %s) on host %s: %w", targetDir, string(stderrMkdir), host.GetName(), errMkdir)
		}
	}

	execOptsSudo := &connector.ExecOptions{Sudo: true} // Default to sudo for openssl and chmod commands on PKI files

	// Generate Private Key (PEM format by default for genpkey)
	genKeyCmd := fmt.Sprintf("openssl genpkey -algorithm RSA -outform PEM -out %s -pkeyopt rsa_keygen_bits:%d",
		s.KeyPemPath, s.KeyBitSize)
	logger.Info("Generating CA private key (PEM).", "path", s.KeyPemPath)
	_, stderrKey, errKey := conn.Exec(ctx.GoContext(), genKeyCmd, execOptsSudo)
	if errKey != nil {
		return fmt.Errorf("failed to generate CA private key %s (stderr: %s): %w", s.KeyPemPath, string(stderrKey), errKey)
	}
	chmodKeyCmd := fmt.Sprintf("chmod %s %s", s.PermissionsKeyPem, s.KeyPemPath)
	if _, _, errChmodKey := conn.Exec(ctx.GoContext(), chmodKeyCmd, execOptsSudo); errChmodKey != nil {
		logger.Warn("Failed to set permissions on CA PEM key.", "path", s.KeyPemPath, "permissions", s.PermissionsKeyPem, "error", errChmodKey)
	}

	// Generate Self-Signed Certificate (PEM format by default for -out with .pem)
	subject := fmt.Sprintf("/CN=%s", strings.ReplaceAll(s.CommonName, "'", `'\''`)) // Basic CN escaping
	genCertCmd := fmt.Sprintf("openssl req -x509 -new -nodes -key %s -sha256 -days %d -outform PEM -out %s -subj '%s'",
		s.KeyPemPath, s.ValidityDays, s.CertPemPath, subject)
	logger.Info("Generating self-signed CA certificate (PEM).", "path", s.CertPemPath)
	_, stderrCert, errCert := conn.Exec(ctx.GoContext(), genCertCmd, execOptsSudo)
	if errCert != nil {
		return fmt.Errorf("failed to generate CA certificate %s (stderr: %s): %w", s.CertPemPath, string(stderrCert), errCert)
	}
	chmodCertCmd := fmt.Sprintf("chmod %s %s", s.PermissionsCertPem, s.CertPemPath)
	if _, _, errChmodCert := conn.Exec(ctx.GoContext(), chmodCertCmd, execOptsSudo); errChmodCert != nil {
		logger.Warn("Failed to set permissions on CA PEM certificate.", "path", s.CertPemPath, "permissions", s.PermissionsCertPem, "error", errChmodCert)
	}

	if s.OutputCertPemCacheKey != "" {
		ctx.StepCache().Set(s.OutputCertPemCacheKey, s.CertPemPath)
		logger.Debug("Stored CA PEM certificate path in cache.", "key", s.OutputCertPemCacheKey, "path", s.CertPemPath)
	}
	if s.OutputKeyPemCacheKey != "" {
		ctx.StepCache().Set(s.OutputKeyPemCacheKey, s.KeyPemPath)
		logger.Debug("Stored CA PEM key path in cache.", "key", s.OutputKeyPemCacheKey, "path", s.KeyPemPath)
	}

	logger.Info("CA certificate and key (PEM format) generated successfully.")
	return nil
}

// Rollback removes the generated PEM certificate and key.
func (s *GenerateCAWithPEMStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger) // Ensure paths are set for removal

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}
	execOptsSudo := &connector.ExecOptions{Sudo: true}

	for _, path := range []string{s.CertPemPath, s.KeyPemPath} {
		if path == "" { continue }
		logger.Info("Attempting to remove file for rollback.", "path", path)
		rmCmd := fmt.Sprintf("rm -f %s", path)
		_, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, execOptsSudo)
		if errRm != nil {
			logger.Error("Failed to remove file during rollback (best effort).", "path", path, "stderr", string(stderrRm), "error", errRm)
		} else {
			logger.Info("File removed successfully.", "path", path)
		}
	}

	if s.OutputCertPemCacheKey != "" {
		ctx.StepCache().Delete(s.OutputCertPemCacheKey)
	}
	if s.OutputKeyPemCacheKey != "" {
		ctx.StepCache().Delete(s.OutputKeyPemCacheKey)
	}
	logger.Debug("Cleaned up cache keys for PEM paths.")
	logger.Info("Rollback attempt for PEM CA generation finished.")
	return nil
}

var _ step.Step = (*GenerateCAWithPEMStepSpec)(nil)
