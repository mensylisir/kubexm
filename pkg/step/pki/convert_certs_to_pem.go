package pki

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step" // For step.Step interface
)

const (
	DefaultCertPemCacheKey = "ConvertedCertPemPath"
	DefaultKeyPemCacheKey  = "ConvertedKeyPemPath"
)

// ConvertCertsToPemStepSpec defines the parameters for converting existing certificate
// and key files to PEM format by copying them and setting appropriate permissions.
type ConvertCertsToPemStepSpec struct {
	spec.StepMeta `json:",inline"` // Embed common meta fields

	SourceCertPath         string `json:"sourceCertPath,omitempty"`
	SourceKeyPath          string `json:"sourceKeyPath,omitempty"`
	TargetCertPemPath      string `json:"targetCertPemPath,omitempty"`
	TargetKeyPemPath       string `json:"targetKeyPemPath,omitempty"`
	SourceCertPathCacheKey string `json:"sourceCertPathCacheKey,omitempty"` // For StepCache
	SourceKeyPathCacheKey  string `json:"sourceKeyPathCacheKey,omitempty"`  // For StepCache
	SourceCertPathTaskCacheKey string `json:"sourceCertPathTaskCacheKey,omitempty"` // For TaskCache
	SourceKeyPathTaskCacheKey  string `json:"sourceKeyPathTaskCacheKey,omitempty"`  // For TaskCache
	SourceCertPathModuleCacheKey string `json:"sourceCertPathModuleCacheKey,omitempty"` // For ModuleCache
	SourceKeyPathModuleCacheKey  string `json:"sourceKeyPathModuleCacheKey,omitempty"`  // For ModuleCache
	CertPemCacheKey        string `json:"certPemCacheKey,omitempty"` // For StepCache output
	KeyPemCacheKey         string `json:"keyPemCacheKey,omitempty"` // For StepCache output
	PermissionsCertPem     string `json:"permissionsCertPem,omitempty"`
	PermissionsKeyPem      string `json:"permissionsKeyPem,omitempty"`
}

// NewConvertCertsToPemStepSpec creates a new ConvertCertsToPemStepSpec.
func NewConvertCertsToPemStepSpec(name, description, sourceCertPath, sourceKeyPath, targetCertPemPath, targetKeyPemPath string) *ConvertCertsToPemStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Convert Certificates to PEM"
	}
	finalDescription := description
	if finalDescription == "" {
		finalDescription = fmt.Sprintf("Converts source cert %s and key %s to PEM format at %s and %s",
			sourceCertPath, sourceKeyPath, targetCertPemPath, targetKeyPemPath)
	}

	return &ConvertCertsToPemStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		SourceCertPath:    sourceCertPath,
		SourceKeyPath:     sourceKeyPath,
		TargetCertPemPath: targetCertPemPath,
		TargetKeyPemPath:  targetKeyPemPath,
	}
}

// GetName returns the step's name.
func (s *ConvertCertsToPemStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description.
func (s *ConvertCertsToPemStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *ConvertCertsToPemStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ConvertCertsToPemStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *ConvertCertsToPemStepSpec) populateDefaults(logger runtime.Logger) {
	if s.TargetCertPemPath == "" && s.SourceCertPath != "" {
		s.TargetCertPemPath = strings.Replace(s.SourceCertPath, ".crt", ".pem", 1)
		logger.Debug("TargetCertPemPath defaulted based on SourceCertPath.", "path", s.TargetCertPemPath)
	}
	if s.TargetKeyPemPath == "" && s.SourceKeyPath != "" {
		s.TargetKeyPemPath = strings.Replace(s.SourceKeyPath, ".key", "-key.pem", 1)
		logger.Debug("TargetKeyPemPath defaulted based on SourceKeyPath.", "path", s.TargetKeyPemPath)
	}

	if s.PermissionsCertPem == "" {
		s.PermissionsCertPem = "0644"
		logger.Debug("PermissionsCertPem defaulted.", "permissions", s.PermissionsCertPem)
	}
	if s.PermissionsKeyPem == "" {
		s.PermissionsKeyPem = "0600"
		logger.Debug("PermissionsKeyPem defaulted.", "permissions", s.PermissionsKeyPem)
	}

	// Update description if it was default and paths got populated
	if s.StepMeta.Description == "Converts source cert  and key  to PEM format at  and " ||
		strings.HasPrefix(s.StepMeta.Description, "Converts source cert %s and key %s to PEM format") { // Check for initial default-like desc
		s.StepMeta.Description = fmt.Sprintf("Converts source cert %s and key %s to PEM format at %s and %s",
			s.SourceCertPath, s.SourceKeyPath, s.TargetCertPemPath, s.TargetKeyPemPath)
	}
}

func (s *ConvertCertsToPemStepSpec) getEffectivePaths(ctx runtime.StepContext) (certPath, keyPath string, err error) {
	certPath = s.SourceCertPath
	if certPath == "" && s.SourceCertPathCacheKey != "" { // Check StepCache first
		cachedVal, found := ctx.StepCache().Get(s.SourceCertPathCacheKey)
		if found {
			pathStr, ok := cachedVal.(string)
			if !ok {
				return "", "", fmt.Errorf("cached source cert path from StepCache (key %s) is not a string", s.SourceCertPathCacheKey)
			}
			certPath = pathStr
		}
	}
	if certPath == "" && s.SourceCertPathModuleCacheKey != "" { // Then check ModuleCache
		cachedVal, found := ctx.ModuleCache().Get(s.SourceCertPathModuleCacheKey)
		if found {
			pathStr, ok := cachedVal.(string)
			if !ok {
				return "", "", fmt.Errorf("cached source cert path from ModuleCache (key %s) is not a string", s.SourceCertPathModuleCacheKey)
			}
			certPath = pathStr
		}
	}
	if certPath == "" && s.SourceCertPathTaskCacheKey != "" { // Then check TaskCache
		cachedVal, found := ctx.TaskCache().Get(s.SourceCertPathTaskCacheKey)
		if found {
			pathStr, ok := cachedVal.(string)
			if !ok {
				return "", "", fmt.Errorf("cached source cert path from TaskCache (key %s) is not a string", s.SourceCertPathTaskCacheKey)
			}
			certPath = pathStr
		}
	}

	keyPath = s.SourceKeyPath
	if keyPath == "" && s.SourceKeyPathCacheKey != "" { // Check StepCache first
		cachedVal, found := ctx.StepCache().Get(s.SourceKeyPathCacheKey)
		if found {
			pathStr, ok := cachedVal.(string)
			if !ok {
				return "", "", fmt.Errorf("cached source key path from StepCache (key %s) is not a string", s.SourceKeyPathCacheKey)
			}
			keyPath = pathStr
		}
	}
	if keyPath == "" && s.SourceKeyPathModuleCacheKey != "" { // Then check ModuleCache
		cachedVal, found := ctx.ModuleCache().Get(s.SourceKeyPathModuleCacheKey)
		if found {
			pathStr, ok := cachedVal.(string)
			if !ok {
				return "", "", fmt.Errorf("cached source key path from ModuleCache (key %s) is not a string", s.SourceKeyPathModuleCacheKey)
			}
			keyPath = pathStr
		}
	}
	if keyPath == "" && s.SourceKeyPathTaskCacheKey != "" { // Then check TaskCache
		cachedVal, found := ctx.TaskCache().Get(s.SourceKeyPathTaskCacheKey)
		if found {
			pathStr, ok := cachedVal.(string)
			if !ok {
				return "", "", fmt.Errorf("cached source key path from TaskCache (key %s) is not a string", s.SourceKeyPathTaskCacheKey)
			}
			keyPath = pathStr
		}
	}

	if certPath == "" {
		return "", "", fmt.Errorf("effective source certificate path could not be determined (direct, StepCache key '%s', TaskCache key '%s', ModuleCache key '%s')",
			s.SourceCertPathCacheKey, s.SourceCertPathTaskCacheKey, s.SourceCertPathModuleCacheKey)
	}
	if keyPath == "" {
		return "", "", fmt.Errorf("effective source key path could not be determined (direct, StepCache key '%s', TaskCache key '%s', ModuleCache key '%s')",
			s.SourceKeyPathCacheKey, s.SourceKeyPathTaskCacheKey, s.SourceKeyPathModuleCacheKey)
	}

	return certPath, keyPath, nil
}

// Precheck checks if the conversion is already done or if source files exist.
func (s *ConvertCertsToPemStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	sourceCert, sourceKey, err := s.getEffectivePaths(ctx)
	if err != nil {
		logger.Error("Failed to determine source paths for cert/key", "error", err)
		return false, nil // Cannot proceed, but don't error out precheck, let Run fail.
	}
	if sourceCert == "" || sourceKey == "" {
		logger.Error("Source certificate path or key path is empty and not found in cache.")
		return false, nil // Let Run handle this as a fatal error.
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	sourceCertExists, err := conn.Exists(ctx.GoContext(), sourceCert)
	if err != nil {
		logger.Warn("Failed to check source cert existence, assuming it might not be there.", "path", sourceCert, "error", err)
		return false, nil
	}
	if !sourceCertExists {
		logger.Error("Source certificate does not exist.", "path", sourceCert)
		return false, fmt.Errorf("source certificate %s does not exist on host %s", sourceCert, host.GetName())
	}

	sourceKeyExists, err := conn.Exists(ctx.GoContext(), sourceKey)
	if err != nil {
		logger.Warn("Failed to check source key existence, assuming it might not be there.", "path", sourceKey, "error", err)
		return false, nil
	}
	if !sourceKeyExists {
		logger.Error("Source key does not exist.", "path", sourceKey)
		return false, fmt.Errorf("source key %s does not exist on host %s", sourceKey, host.GetName())
	}

	targetCertPemExists, err := conn.Exists(ctx.GoContext(), s.TargetCertPemPath)
	if err != nil {
		logger.Warn("Failed to check target cert PEM existence.", "path", s.TargetCertPemPath, "error", err)
		return false, nil // Let Run attempt.
	}
	targetKeyPemExists, err := conn.Exists(ctx.GoContext(), s.TargetKeyPemPath)
	if err != nil {
		logger.Warn("Failed to check target key PEM existence.", "path", s.TargetKeyPemPath, "error", err)
		return false, nil // Let Run attempt.
	}

	if targetCertPemExists && targetKeyPemExists {
		logger.Info("Target PEM certificate and key already exist.", "certPem", s.TargetCertPemPath, "keyPem", s.TargetKeyPemPath)
		return true, nil
	}
	if targetCertPemExists {
		logger.Info("Target PEM certificate exists, but key PEM does not. Will attempt conversion.", "certPem", s.TargetCertPemPath, "keyPem", s.TargetKeyPemPath)
	}
	if targetKeyPemExists {
		logger.Info("Target PEM key exists, but certificate PEM does not. Will attempt conversion.", "keyPem", s.TargetKeyPemPath, "certPem", s.TargetCertPemPath)
	}

	logger.Info("Target PEM files do not exist or are incomplete. Conversion needed.")
	return false, nil
}

// Run executes the certificate to PEM conversion.
func (s *ConvertCertsToPemStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	sourceCert, sourceKey, err := s.getEffectivePaths(ctx)
	if err != nil {
		return fmt.Errorf("run: failed to determine source paths for cert/key on host %s: %w", host.GetName(), err)
	}
	if sourceCert == "" || sourceKey == "" {
		return fmt.Errorf("run: source certificate path or key path is empty for step %s on host %s", s.GetName(), host.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	execOptsSudo := &connector.ExecOptions{Sudo: true} // Assume sudo for /etc or similar paths

	// Ensure target directories exist
	for _, targetPath := range []string{s.TargetCertPemPath, s.TargetKeyPemPath} {
		targetDir := filepath.Dir(targetPath)
		mkdirCmd := fmt.Sprintf("mkdir -p %s", targetDir)
		_, stderrMkdir, errMkdir := conn.Exec(ctx.GoContext(), mkdirCmd, execOptsSudo)
		if errMkdir != nil {
			return fmt.Errorf("failed to create directory %s (stderr: %s) on host %s: %w", targetDir, string(stderrMkdir), host.GetName(), errMkdir)
		}
	}

	// Copy cert to PEM path
	cpCertCmd := fmt.Sprintf("cp -f %s %s", sourceCert, s.TargetCertPemPath)
	logger.Info("Copying source certificate to target PEM location.", "source", sourceCert, "destination", s.TargetCertPemPath)
	_, stderrCpCert, errCpCert := conn.Exec(ctx.GoContext(), cpCertCmd, execOptsSudo)
	if errCpCert != nil {
		return fmt.Errorf("failed to copy source certificate to %s (stderr: %s) on host %s: %w", s.TargetCertPemPath, string(stderrCpCert), host.GetName(), errCpCert)
	}
	chmodCertCmd := fmt.Sprintf("chmod %s %s", s.PermissionsCertPem, s.TargetCertPemPath)
	_, stderrChmodCert, errChmodCert := conn.Exec(ctx.GoContext(), chmodCertCmd, execOptsSudo)
	if errChmodCert != nil {
		return fmt.Errorf("failed to set permissions on %s (stderr: %s) on host %s: %w", s.TargetCertPemPath, string(stderrChmodCert), host.GetName(), errChmodCert)
	}
	logger.Info("Target PEM certificate created and permissions set.", "path", s.TargetCertPemPath, "permissions", s.PermissionsCertPem)

	// Copy key to PEM path
	cpKeyCmd := fmt.Sprintf("cp -f %s %s", sourceKey, s.TargetKeyPemPath)
	logger.Info("Copying source key to target PEM location.", "source", sourceKey, "destination", s.TargetKeyPemPath)
	_, stderrCpKey, errCpKey := conn.Exec(ctx.GoContext(), cpKeyCmd, execOptsSudo)
	if errCpKey != nil {
		return fmt.Errorf("failed to copy source key to %s (stderr: %s) on host %s: %w", s.TargetKeyPemPath, string(stderrCpKey), host.GetName(), errCpKey)
	}
	chmodKeyCmd := fmt.Sprintf("chmod %s %s", s.PermissionsKeyPem, s.TargetKeyPemPath)
	_, stderrChmodKey, errChmodKey := conn.Exec(ctx.GoContext(), chmodKeyCmd, execOptsSudo)
	if errChmodKey != nil {
		return fmt.Errorf("failed to set permissions on %s (stderr: %s) on host %s: %w", s.TargetKeyPemPath, string(stderrChmodKey), host.GetName(), errChmodKey)
	}
	logger.Info("Target PEM key created and permissions set.", "path", s.TargetKeyPemPath, "permissions", s.PermissionsKeyPem)

	if s.CertPemCacheKey != "" {
		ctx.StepCache().Set(s.CertPemCacheKey, s.TargetCertPemPath)
		logger.Debug("Stored target cert PEM path in cache.", "key", s.CertPemCacheKey, "path", s.TargetCertPemPath)
	}
	if s.KeyPemCacheKey != "" {
		ctx.StepCache().Set(s.KeyPemCacheKey, s.TargetKeyPemPath)
		logger.Debug("Stored target key PEM path in cache.", "key", s.KeyPemCacheKey, "path", s.TargetKeyPemPath)
	}

	logger.Info("Successfully converted certificates to PEM format.")
	return nil
}

// Rollback attempts to remove the created PEM files.
func (s *ConvertCertsToPemStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger) // To ensure Target paths are determined if they were dynamic

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}
	execOptsSudo := &connector.ExecOptions{Sudo: true}

	if s.TargetCertPemPath != "" {
		logger.Info("Attempting to remove target PEM certificate for rollback.", "path", s.TargetCertPemPath)
		rmCmd := fmt.Sprintf("rm -f %s", s.TargetCertPemPath)
		if _, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, execOptsSudo); errRm != nil {
			logger.Error("Failed to remove target PEM certificate during rollback (best effort).", "path", s.TargetCertPemPath, "stderr", string(stderrRm), "error", errRm)
		}
	}
	if s.TargetKeyPemPath != "" {
		logger.Info("Attempting to remove target PEM key for rollback.", "path", s.TargetKeyPemPath)
		rmCmd := fmt.Sprintf("rm -f %s", s.TargetKeyPemPath)
		if _, stderrRm, errRm := conn.Exec(ctx.GoContext(), rmCmd, execOptsSudo); errRm != nil {
			logger.Error("Failed to remove target PEM key during rollback (best effort).", "path", s.TargetKeyPemPath, "stderr", string(stderrRm), "error", errRm)
		}
	}

	if s.CertPemCacheKey != "" {
		ctx.StepCache().Delete(s.CertPemCacheKey)
		logger.Debug("Removed target cert PEM path from cache.", "key", s.CertPemCacheKey)
	}
	if s.KeyPemCacheKey != "" {
		ctx.StepCache().Delete(s.KeyPemCacheKey)
		logger.Debug("Removed target key PEM path from cache.", "key", s.KeyPemCacheKey)
	}

	logger.Info("Rollback attempt for PEM conversion finished.")
	return nil
}

// Ensure ConvertCertsToPemStepSpec implements the step.Step interface.
var _ step.Step = (*ConvertCertsToPemStepSpec)(nil)
