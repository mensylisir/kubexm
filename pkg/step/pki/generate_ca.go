package pki

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/kubexms/kubexms/pkg/runner"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
)

// GenerateRootCAStep generates a self-signed root CA certificate and private key.
// This step is typically run on a single control-plane node.
// The generated CA certificate and key can be stored in SharedData or a well-known
// location on the host for other steps to use.
type GenerateRootCAStep struct {
	// CertPath is the desired path on the host to store the CA certificate.
	// If empty, a default host-specific path like "/etc/kubexms/pki/<hostname>/ca.crt" will be used.
	CertPath string
	// KeyPath is the desired path on the host to store the CA private key.
	// If empty, a default host-specific path like "/etc/kubexms/pki/<hostname>/ca.key" will be used.
	KeyPath string
	// CommonName for the CA certificate.
	CommonName string
	// ValidityDays is the number of days the CA certificate will be valid.
	ValidityDays int
	// KeyBitSize is the bit size for the private key (e.g., 2048, 4096).
	KeyBitSize int

	// TODO: Consider Organization, OU, Country, etc. fields for subject.
}

// Name returns a human-readable name for the step.
func (s *GenerateRootCAStep) Name() string {
	// Use effective paths in name after defaults are applied, if possible,
	// but Name() might be called before Check/Run where defaults are applied.
	// For now, use potentially empty CertPath/KeyPath or rely on user setting them before Name() is critical.
	// Or, make defaultPathsIfNeeded callable from Name() if ctx is available, but it's not.
	// So, the name might be generic if paths aren't pre-set.
	nameCertPath := s.CertPath
	nameKeyPath := s.KeyPath
	if nameCertPath == "" { nameCertPath = "default_ca.crt" }
	if nameKeyPath == "" { nameKeyPath = "default_ca.key" }
	return fmt.Sprintf("Generate Root CA (Cert: %s, Key: %s)", nameCertPath, nameKeyPath)
}

// defaultPathsIfNeeded sets default values for paths and parameters if they are not provided.
// It uses the host's name to create a host-specific PKI directory path if paths are empty.
func (s *GenerateRootCAStep) defaultPathsIfNeeded(hostName string) {
	// Base PKI directory, potentially made host-specific if paths are not absolute.
	// If CertPath/KeyPath are absolute, they are used as is.
	// If they are relative, they could be relative to a global PKI dir or host-specific.
	// For simplicity, if empty, we define a default absolute path.
	defaultBaseDir := "/etc/kubexms/pki" // A general base
	if hostName != "" { // Make it host-specific if paths are not fully qualified by user
		// This logic might need refinement based on how shared vs. per-host CAs are handled.
		// If this step is for a truly GLOBAL CA, hostName shouldn't be in the path unless it's the "CA master".
		// Assuming for now, if paths are empty, they default to a common, non-host-specific path.
		// defaultBaseDir = filepath.Join("/etc/kubexms/pki", hostName)
	}

	if s.CertPath == "" {
		s.CertPath = filepath.Join(defaultBaseDir, "ca.crt")
	}
	if s.KeyPath == "" {
		s.KeyPath = filepath.Join(defaultBaseDir, "ca.key")
	}
	if s.CommonName == "" {
		s.CommonName = "kubexms-root-ca"
	}
	if s.ValidityDays == 0 {
		s.ValidityDays = 365 * 10 // 10 years
	}
	if s.KeyBitSize == 0 {
		s.KeyBitSize = 4096
	}
}

// Check determines if the root CA certificate and key already exist at the specified paths.
func (s *GenerateRootCAStep) Check(ctx *runtime.Context) (isDone bool, err error) {
	if ctx.Host.Runner == nil {
		return false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	s.defaultPathsIfNeeded(ctx.Host.Name) // Ensure paths are set using host context if needed for defaults
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", s.Name()).Sugar()


	certExists, err := ctx.Host.Runner.Exists(ctx.GoContext, s.CertPath)
	if err != nil {
		return false, fmt.Errorf("failed to check existence of CA cert %s on host %s: %w", s.CertPath, ctx.Host.Name, err)
	}
	keyExists, err := ctx.Host.Runner.Exists(ctx.GoContext, s.KeyPath)
	if err != nil {
		return false, fmt.Errorf("failed to check existence of CA key %s on host %s: %w", s.KeyPath, ctx.Host.Name, err)
	}

	if certExists && keyExists {
		hostCtxLogger.Infof("Root CA certificate (%s) and key (%s) already exist.", s.CertPath, s.KeyPath)
		// TODO: Add verification of CA properties (e.g., common name, expiry, is-ca) for true idempotency.
		return true, nil
	}
	if certExists && !keyExists {
	    hostCtxLogger.Warnf("Root CA cert %s exists, but key %s does not. Will attempt to regenerate.", s.CertPath, s.KeyPath)
	}
    if !certExists && keyExists {
	    hostCtxLogger.Warnf("Root CA key %s exists, but cert %s does not. Will attempt to regenerate.", s.KeyPath, s.CertPath)
	}
	if !certExists && !keyExists {
		hostCtxLogger.Debugf("Root CA certificate (%s) and key (%s) do not exist.", s.CertPath, s.KeyPath)
	}

	return false, nil // Not "done" if either is missing
}

// Run generates the root CA certificate and private key using openssl commands.
func (s *GenerateRootCAStep) Run(ctx *runtime.Context) *step.Result {
	s.defaultPathsIfNeeded(ctx.Host.Name)
	startTime := time.Now()
	res := step.NewResult(s.Name(), ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", s.Name()).Sugar()


	if ctx.Host.Runner == nil {
		res.Error = fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
		res.Status = "Failed"; res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}

	if _, err := ctx.Host.Runner.LookPath(ctx.GoContext, "openssl"); err != nil {
		res.Error = fmt.Errorf("openssl command not found on host %s, required to generate CA: %w", ctx.Host.Name, err)
		res.Status = "Failed"; res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}

	certDir := filepath.Dir(s.CertPath)
	keyDir := filepath.Dir(s.KeyPath)
	// Sudo true for creating directories in /etc or similar system paths.
	if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, certDir, "0755", true); err != nil {
		res.Error = fmt.Errorf("failed to create directory %s for CA certificate on host %s: %w", certDir, ctx.Host.Name, err)
		res.Status = "Failed"; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	if certDir != keyDir {
		if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, keyDir, "0700", true); err != nil {
			res.Error = fmt.Errorf("failed to create directory %s for CA key on host %s: %w", keyDir, ctx.Host.Name, err)
			res.Status = "Failed"; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
		}
	}

	genKeyCmd := fmt.Sprintf("openssl genpkey -algorithm RSA -out %s -pkeyopt rsa_keygen_bits:%d", s.KeyPath, s.KeyBitSize)
	hostCtxLogger.Infof("Generating CA private key: %s (this might take a moment)", s.KeyPath)
	_, stderrKey, errKey := ctx.Host.Runner.Run(ctx.GoContext, genKeyCmd, true)
	if errKey != nil {
		res.Error = fmt.Errorf("failed to generate CA private key %s on host %s: %w (stderr: %s)", s.KeyPath, ctx.Host.Name, errKey, string(stderrKey))
		res.Status = "Failed"; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	if errChmodKey := ctx.Host.Runner.Chmod(ctx.GoContext, s.KeyPath, "0600", true); errChmodKey != nil {
		hostCtxLogger.Warnf("Failed to chmod CA key %s to 0600 on host %s: %v", s.KeyPath, ctx.Host.Name, errChmodKey)
	}

	subject := fmt.Sprintf("/CN=%s", s.CommonName)
	// Example with more fields: /C=US/ST=California/L=City/O=MyOrg/OU=MyOU/CN=MyCA
	// if s.Organization != "" { subject += fmt.Sprintf("/O=%s", s.Organization) } // etc.
	genCertCmd := fmt.Sprintf("openssl req -x509 -new -nodes -key %s -sha256 -days %d -out %s -subj '%s'",
		s.KeyPath, s.ValidityDays, s.CertPath, subject) // Quote subject
	hostCtxLogger.Infof("Generating self-signed CA certificate: %s", s.CertPath)
	_, stderrCert, errCert := ctx.Host.Runner.Run(ctx.GoContext, genCertCmd, true)
	if errCert != nil {
		res.Error = fmt.Errorf("failed to generate CA certificate %s on host %s: %w (stderr: %s)", s.CertPath, ctx.Host.Name, errCert, string(stderrCert))
		res.Status = "Failed"; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	if errChmodCert := ctx.Host.Runner.Chmod(ctx.GoContext, s.CertPath, "0644", true); errChmodCert != nil {
		hostCtxLogger.Warnf("Failed to chmod CA cert %s to 0644 on host %s: %v", s.CertPath, ctx.Host.Name, errChmodCert)
	}

	// Store paths in SharedData for other steps to potentially use.
	// Key names should be well-defined constants or exported variables if used across packages.
	if ctx.SharedData != nil {
		ctx.SharedData.Store(fmt.Sprintf("pki.ca.%s.certPath", ctx.Host.Name), s.CertPath)
		ctx.SharedData.Store(fmt.Sprintf("pki.ca.%s.keyPath", ctx.Host.Name), s.KeyPath)
		// Storing actual content might be too large for SharedData, paths are usually better.
	}

	res.EndTime = time.Now()
	res.Status = "Succeeded"
	res.Message = fmt.Sprintf("Root CA certificate and key generated successfully at %s and %s on host %s.", s.CertPath, s.KeyPath, ctx.Host.Name)
	hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	return res
}

var _ step.Step = &GenerateRootCAStep{}
