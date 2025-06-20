package pki

import (
	// "context" // No longer directly needed if runtime.StepContext is used
	"fmt"
	"path/filepath"
	"strings" // For strings.TrimSpace in subject formatting, if used
	"time"

	"github.com/mensylisir/kubexm/pkg/connector" // For connector.ExecOptions & connector.Connector
	"github.com/mensylisir/kubexm/pkg/runtime"   // For runtime.StepContext
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// GenerateRootCAStepSpec defines parameters for generating a root CA.
type GenerateRootCAStepSpec struct {
	CertPath     string
	KeyPath      string
	CommonName   string
	ValidityDays int
	KeyBitSize   int
	StepName     string // Optional: for a custom name for this specific step instance
	// Future fields: Organization, OU, Country for subject
}

// GetName returns the step name.
func (s *GenerateRootCAStepSpec) GetName() string {
	if s.StepName != "" { return s.StepName }
	// For a default name, we might need to know the resolved paths.
	// This GetName is on the spec, which might not have defaults applied yet if they are dynamic.
	// So, we use placeholder or rely on user setting SpecName for a fully dynamic default.
	cp := s.CertPath; if cp == "" { cp = "ca.crt (default path)" }
	kp := s.KeyPath;  if kp == "" { kp = "ca.key (default path)" }
	return fmt.Sprintf("Generate Root CA (Cert: %s, Key: %s)", cp, kp)
}

// applyDefaults sets default values for the spec if they are not provided.
// This method is called by the executor.
func (s *GenerateRootCAStepSpec) applyDefaults(logger runtime.Logger, hostName string) { // Added logger for consistency if needed
	// If paths are empty, default to a common, non-host-specific path for a global CA,
	// or make it host-specific if that's the design for this CA type.
	// For a root CA, it's often global, so not using ctxHostName in path by default.
	// If a per-host CA was intended, path would include ctxHostName.
	defaultBaseDir := "/etc/kubexms/pki"
	if s.CertPath == "" {
		s.CertPath = filepath.Join(defaultBaseDir, "ca.crt")
	}
	if s.KeyPath == "" {
		s.KeyPath = filepath.Join(defaultBaseDir, "ca.key")
	}
	if s.CommonName == "" { s.CommonName = "kubexms-root-ca" }
	if s.ValidityDays == 0 { s.ValidityDays = 365 * 10 } // 10 years
	if s.KeyBitSize == 0 { s.KeyBitSize = 4096 }
}
var _ spec.StepSpec = &GenerateRootCAStepSpec{}

// GenerateRootCAStepExecutor implements logic for GenerateRootCAStepSpec.
type GenerateRootCAStepExecutor struct{}

func init() {
	step.Register(step.GetSpecTypeName(&GenerateRootCAStepSpec{}), &GenerateRootCAStepExecutor{})
}

// Check determines if the root CA cert and key already exist.
func (e *GenerateRootCAStepExecutor) Check(ctx runtime.StepContext) (isDone bool, err error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost() // This step might be host-specific for where openssl runs or where files are checked
	goCtx := ctx.GoContext()

	hostNameForDefaults := "localhost" // Default if host is nil (e.g. control-plane step)
	if currentHost != nil {
		logger = logger.With("host", currentHost.GetName())
		hostNameForDefaults = currentHost.GetName()
	}

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		return false, fmt.Errorf("step spec not found in context for GenerateRootCAStepExecutor Check method")
	}
	spec, ok := rawSpec.(*GenerateRootCAStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		return false, fmt.Errorf("unexpected spec type %T in context for GenerateRootCAStepExecutor Check method", rawSpec)
	}
	spec.applyDefaults(logger, hostNameForDefaults) // Pass logger and hostname
	logger = logger.With("step", spec.GetName())


	if currentHost == nil { // If no specific host, this check might be on the control node or is conceptual.
		logger.Debug("No specific host in context, cannot check for existing CA files on a remote host. Assuming not done if paths are default.")
		// This check becomes tricky. If paths are default, it implies a shared CA.
		// If this step ALWAYS runs on a specific node (e.g. first master), then currentHost should be set.
		// For now, if no host, we can't check remote files.
		return false, fmt.Errorf("currentHost is nil, cannot perform file checks for GenerateRootCA")
	}

	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		return false, fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err)
	}

	certExists, err := conn.Exists(goCtx, spec.CertPath) // Use connector
	if err != nil {
		logger.Error("Failed to check CA cert existence", "path", spec.CertPath, "error", err)
		return false, fmt.Errorf("failed to check CA cert %s on host %s: %w", spec.CertPath, currentHost.GetName(), err)
	}
	keyExists, err := conn.Exists(goCtx, spec.KeyPath) // Use connector
	if err != nil {
		logger.Error("Failed to check CA key existence", "path", spec.KeyPath, "error", err)
		return false, fmt.Errorf("failed to check CA key %s on host %s: %w", spec.KeyPath, currentHost.GetName(), err)
	}

	if certExists && keyExists {
		logger.Info("Root CA certificate and key already exist.", "certPath", spec.CertPath, "keyPath", spec.KeyPath)
		return true, nil
	}
	if certExists && !keyExists { logger.Warn("Root CA cert exists, but key does not. Will attempt to regenerate.", "certPath", spec.CertPath, "keyPath", spec.KeyPath) }
    if !certExists && keyExists { logger.Warn("Root CA key exists, but cert does not. Will attempt to regenerate.", "keyPath", spec.KeyPath, "certPath", spec.CertPath) }
	if !certExists && !keyExists { logger.Debug("Root CA certificate and key do not exist.", "certPath", spec.CertPath, "keyPath", spec.KeyPath) }
	return false, nil
}

// Execute generates the root CA.
func (e *GenerateRootCAStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.Now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost() // Assuming CA generation happens on a specific host or control node
	goCtx := ctx.GoContext()

	res := step.NewResult(ctx, currentHost, startTime, nil) // currentHost can be nil if this is a control-plane local step

	hostNameForDefaults := "localhost"
	if currentHost != nil {
		logger = logger.With("host", currentHost.GetName())
		hostNameForDefaults = currentHost.GetName()
	}

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		res.Error = fmt.Errorf("Execute: step spec not found in context for GenerateRootCAStepExecutor"); res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*GenerateRootCAStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		name := "GenerateRootCA (type error)"; if s, isSpec := rawSpec.(spec.StepSpec); isSpec { name = s.GetName() }
		res.Error = fmt.Errorf("Execute: unexpected spec type %T (%s) in context for GenerateRootCAStepExecutor", rawSpec, name); res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	spec.applyDefaults(logger, hostNameForDefaults) // Pass logger and hostname
	logger = logger.With("step", spec.GetName())

	if currentHost == nil {
		// This implies the step is running locally on the control plane node, not via a connector to a remote host.
		// File operations would be local. This needs a different execution path or clear expectation for `GetConnectorForHost(nil)`.
		// For now, assume a connector for localhost or that openssl commands run locally without a typical connector.
		// This part of the logic might need refinement based on how "local" steps are handled.
		// Let's assume for now it requires a host, and if not, it's an error for this step as designed.
		logger.Error("Current host is nil. This step requires a host context to run openssl commands.")
		res.Error = fmt.Errorf("current host is nil, GenerateRootCA requires a host context"); res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	conn, err := ctx.GetConnectorForHost(currentHost)
	if err != nil {
		logger.Error("Failed to get connector for host", "error", err)
		res.Error = fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), err); res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	// Check for openssl using LookPath on the connector
	if _, err := conn.LookPath(goCtx, "openssl"); err != nil {
		logger.Error("openssl command not found on host, required to generate CA.", "error", err)
		res.Error = fmt.Errorf("openssl command not found on host %s: %w", currentHost.GetName(), err); res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	certDir := filepath.Dir(spec.CertPath); keyDir := filepath.Dir(spec.KeyPath)
	if err := conn.Mkdir(goCtx, certDir, "0755"); err != nil { // Sudo handled by connector
		logger.Error("Failed to create directory for CA certificate.", "path", certDir, "error", err)
		res.Error = fmt.Errorf("failed to create directory %s for CA certificate on host %s: %w", certDir, currentHost.GetName(), err); res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	if certDir != keyDir {
		if err := conn.Mkdir(goCtx, keyDir, "0700"); err != nil { // Key dir more restrictive
			logger.Error("Failed to create directory for CA key.", "path", keyDir, "error", err)
			res.Error = fmt.Errorf("failed to create directory %s for CA key on host %s: %w", keyDir, currentHost.GetName(), err); res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
		}
	}

	genKeyCmd := fmt.Sprintf("openssl genpkey -algorithm RSA -out %s -pkeyopt rsa_keygen_bits:%d", spec.KeyPath, spec.KeyBitSize)
	logger.Info("Generating CA private key (this might take a moment).", "keyPath", spec.KeyPath)
	_, stderrKey, errKey := conn.RunCommand(goCtx, genKeyCmd, &connector.ExecOptions{Sudo: true})
	if errKey != nil {
		logger.Error("Failed to generate CA private key.", "keyPath", spec.KeyPath, "stderr", string(stderrKey), "error", errKey)
		res.Error = fmt.Errorf("failed to generate CA private key %s on host %s: %w (stderr: %s)", spec.KeyPath, currentHost.GetName(), errKey, string(stderrKey)); res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	chmodKeyCmd := fmt.Sprintf("chmod 0600 %s", spec.KeyPath)
	if _, _, errChmodKey := conn.RunCommand(goCtx, chmodKeyCmd, &connector.ExecOptions{Sudo: true}); errChmodKey != nil {
		logger.Warn("Failed to chmod CA key.", "keyPath", spec.KeyPath, "error", errChmodKey)
	}

	subject := fmt.Sprintf("/CN=%s", strings.ReplaceAll(spec.CommonName, "\"", "\\\""))
	genCertCmd := fmt.Sprintf("openssl req -x509 -new -nodes -key %s -sha256 -days %d -out %s -subj '%s'",
		spec.KeyPath, spec.ValidityDays, spec.CertPath, subject)
	logger.Info("Generating self-signed CA certificate.", "certPath", spec.CertPath)
	_, stderrCert, errCert := conn.RunCommand(goCtx, genCertCmd, &connector.ExecOptions{Sudo: true})
	if errCert != nil {
		logger.Error("Failed to generate CA certificate.", "certPath", spec.CertPath, "stderr", string(stderrCert), "error", errCert)
		res.Error = fmt.Errorf("failed to generate CA certificate %s on host %s: %w (stderr: %s)", spec.CertPath, currentHost.GetName(), errCert, string(stderrCert)); res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	chmodCertCmd := fmt.Sprintf("chmod 0644 %s", spec.CertPath)
	if _, _, errChmodCert := conn.RunCommand(goCtx, chmodCertCmd, &connector.ExecOptions{Sudo: true}); errChmodCert != nil {
		logger.Warn("Failed to chmod CA cert.", "certPath", spec.CertPath, "error", errChmodCert)
	}

	const clusterRootCACertPathKey = "ClusterRootCACertPath"
	const clusterRootCAKeyPathKey  = "ClusterRootCAKeyPath"

	ctx.ModuleCache().Set(clusterRootCACertPathKey, spec.CertPath) // Use ModuleCache
	ctx.ModuleCache().Set(clusterRootCAKeyPathKey, spec.KeyPath)
	logger.Debug("Stored Root CA paths in Module Cache.", "CertPathKey", clusterRootCACertPathKey, "CertPath", spec.CertPath, "KeyPathKey", clusterRootCAKeyPathKey, "KeyPath", spec.KeyPath)

	res.EndTime = time.Now(); res.Status = step.StatusSucceeded
	res.Message = fmt.Sprintf("Root CA certificate and key generated successfully at %s and %s.", spec.CertPath, spec.KeyPath)
	logger.Info("Step succeeded.", "message", res.Message)
	return res
}
var _ step.StepExecutor = &GenerateRootCAStepExecutor{}
