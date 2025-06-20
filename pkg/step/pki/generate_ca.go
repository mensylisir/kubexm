package pki

import (
	"context" // Required by runtime.Context
	"fmt"
	"path/filepath"
	"strings" // For strings.TrimSpace in subject formatting, if used
	"time"

	"github.com/kubexms/kubexms/pkg/connector" // For connector.ExecOptions
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step"
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
func (s *GenerateRootCAStepSpec) applyDefaults(ctxHostName string) {
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
func (e *GenerateRootCAStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	rawSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("step spec not found in context for GenerateRootCAStepExecutor Check method")
	}
	spec, ok := rawSpec.(*GenerateRootCAStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected spec type %T in context for GenerateRootCAStepExecutor Check method", rawSpec)
	}
	spec.applyDefaults(ctx.Host.Name)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()

	if ctx.Host.Runner == nil {
		return false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}

	certExists, err := ctx.Host.Runner.Exists(ctx.GoContext, spec.CertPath)
	if err != nil { return false, fmt.Errorf("failed to check CA cert %s on host %s: %w", spec.CertPath, ctx.Host.Name, err) }
	keyExists, err := ctx.Host.Runner.Exists(ctx.GoContext, spec.KeyPath)
	if err != nil { return false, fmt.Errorf("failed to check CA key %s on host %s: %w", spec.KeyPath, ctx.Host.Name, err) }

	if certExists && keyExists {
		hostCtxLogger.Infof("Root CA certificate (%s) and key (%s) already exist.", spec.CertPath, spec.KeyPath)
		// TODO: Add verification of CA properties (CN, expiry, isCA flag in cert) for true idempotency.
		return true, nil
	}
	if certExists && !keyExists { hostCtxLogger.Warnf("Root CA cert %s exists, but key %s does not. Will attempt to regenerate.", spec.CertPath, spec.KeyPath) }
    if !certExists && keyExists { hostCtxLogger.Warnf("Root CA key %s exists, but cert %s does not. Will attempt to regenerate.", spec.KeyPath, spec.CertPath) }
	if !certExists && !keyExists { hostCtxLogger.Debugf("Root CA certificate (%s) and key (%s) do not exist.", spec.CertPath, spec.KeyPath) }
	return false, nil
}

// Execute generates the root CA.
func (e *GenerateRootCAStepExecutor) Execute(ctx runtime.Context) *step.Result {
	rawSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		// Cannot get spec name if rawSpec is nil or not a StepSpec
		return step.NewResult(ctx, time.Now(), fmt.Errorf("Execute: step spec not found in context for GenerateRootCAStepExecutor"))
	}
	spec, ok := rawSpec.(*GenerateRootCAStepSpec)
	if !ok {
		// Try to get a name if rawSpec is at least a spec.StepSpec
		name := "GenerateRootCA (type error)"
		if s, isSpec := rawSpec.(spec.StepSpec); isSpec {
			name = s.GetName()
		}
		return step.NewResult(ctx, time.Now(), fmt.Errorf("Execute: unexpected spec type %T (%s) in context for GenerateRootCAStepExecutor", rawSpec, name))
	}

	spec.applyDefaults(ctx.Host.Name)
	startTime := time.Now()
	// Initialize res early, so it can be used for error reporting
	res := step.NewResult(ctx, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()


	if ctx.Host.Runner == nil {
		res.Error = fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	if _, err := ctx.Host.Runner.LookPath(ctx.GoContext, "openssl"); err != nil {
		res.Error = fmt.Errorf("openssl command not found on host %s, required to generate CA: %w", ctx.Host.Name, err)
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	certDir := filepath.Dir(spec.CertPath); keyDir := filepath.Dir(spec.KeyPath)
	// Sudo true for creating directories in /etc or similar system paths.
	if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, certDir, "0755", true); err != nil {
		res.Error = fmt.Errorf("failed to create directory %s for CA certificate on host %s: %w", certDir, ctx.Host.Name, err)
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	if certDir != keyDir { // Only create if different to avoid double logging/error
		if err := ctx.Host.Runner.Mkdirp(ctx.GoContext, keyDir, "0700", true); err != nil { // Key dir more restrictive
			res.Error = fmt.Errorf("failed to create directory %s for CA key on host %s: %w", keyDir, ctx.Host.Name, err)
			res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
		}
	}

	// Sudo true for openssl commands if writing to system paths like /etc
	genKeyCmd := fmt.Sprintf("openssl genpkey -algorithm RSA -out %s -pkeyopt rsa_keygen_bits:%d", spec.KeyPath, spec.KeyBitSize)
	hostCtxLogger.Infof("Generating CA private key: %s (this might take a moment)", spec.KeyPath)
	_, stderrKey, errKey := ctx.Host.Runner.RunWithOptions(ctx.GoContext, genKeyCmd, &connector.ExecOptions{Sudo: true})
	if errKey != nil {
		res.Error = fmt.Errorf("failed to generate CA private key %s on host %s: %w (stderr: %s)", spec.KeyPath, ctx.Host.Name, errKey, string(stderrKey))
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	if errChmodKey := ctx.Host.Runner.Chmod(ctx.GoContext, spec.KeyPath, "0600", true); errChmodKey != nil {
		hostCtxLogger.Warnf("Failed to chmod CA key %s to 0600 on host %s: %v", spec.KeyPath, ctx.Host.Name, errChmodKey)
		// Non-fatal warning for chmod failure, but key is generated.
	}

	subject := fmt.Sprintf("/CN=%s", strings.ReplaceAll(spec.CommonName, "\"", "\\\"")) // Basic CN sanitation
	genCertCmd := fmt.Sprintf("openssl req -x509 -new -nodes -key %s -sha256 -days %d -out %s -subj '%s'",
		spec.KeyPath, spec.ValidityDays, spec.CertPath, subject)
	hostCtxLogger.Infof("Generating self-signed CA certificate: %s", spec.CertPath)
	_, stderrCert, errCert := ctx.Host.Runner.RunWithOptions(ctx.GoContext, genCertCmd, &connector.ExecOptions{Sudo: true})
	if errCert != nil {
		res.Error = fmt.Errorf("failed to generate CA certificate %s on host %s: %w (stderr: %s)", spec.CertPath, ctx.Host.Name, errCert, string(stderrCert))
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	if errChmodCert := ctx.Host.Runner.Chmod(ctx.GoContext, spec.CertPath, "0644", true); errChmodCert != nil {
		hostCtxLogger.Warnf("Failed to chmod CA cert %s to 0644 on host %s: %v", spec.CertPath, ctx.Host.Name, errChmodCert)
	}

	// Store paths in Module Cache for other steps/tasks within the same module execution.
	// These keys should ideally be exported constants from a relevant package (e.g., pkg/pki/constants.go or similar)
	// For this refactoring, defining them as local consts or string literals.
	const clusterRootCACertPathKey = "ClusterRootCACertPath" // Example key for cluster-wide root CA cert
	const clusterRootCAKeyPathKey  = "ClusterRootCAKeyPath"  // Example key for cluster-wide root CA key

	// PKIPathType in spec could be used to make keys more specific if this step generates different types of CAs
	// e.g. key := fmt.Sprintf("%sRootCACertPath", spec.PKIPathType) // if PKIPathType is "Etcd" or "Kubernetes"

	ctx.Module().Set(clusterRootCACertPathKey, spec.CertPath)
	ctx.Module().Set(clusterRootCAKeyPathKey, spec.KeyPath)
	hostCtxLogger.Debugf("Stored Root CA paths in Module Cache: CertPathKey=%s (%s), KeyPathKey=%s (%s)",
		clusterRootCACertPathKey, spec.CertPath, clusterRootCAKeyPathKey, spec.KeyPath)

	res.EndTime = time.Now(); res.Status = step.StatusSucceeded
	res.Message = fmt.Sprintf("Root CA certificate and key generated successfully at %s and %s.", spec.CertPath, spec.KeyPath)
	hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	return res
}
var _ step.StepExecutor = &GenerateRootCAStepExecutor{}
