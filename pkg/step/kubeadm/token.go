package kubeadm

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// KubeadmCreateTokenStepSpec defines parameters for `kubeadm token create`.
type KubeadmCreateTokenStepSpec struct {
	spec.StepMeta `json:",inline"`

	KubeadmPath           string   `json:"kubeadmPath,omitempty"`
	AdminKubeconfigPath   string   `json:"adminKubeconfigPath,omitempty"`
	TTL                   string   `json:"ttl,omitempty"`
	TokenDescription      string   `json:"tokenDescription,omitempty"`
	Groups                []string `json:"groups,omitempty"`
	Usages                []string `json:"usages,omitempty"`
	OutputTokenCacheKey   string   `json:"outputTokenCacheKey,omitempty"` // Required
	OutputTokenIDCacheKey string   `json:"outputTokenIdCacheKey,omitempty"` // Optional
	Sudo                  bool     `json:"sudo,omitempty"`
}

// NewKubeadmCreateTokenStepSpec creates a new KubeadmCreateTokenStepSpec.
func NewKubeadmCreateTokenStepSpec(name, stepDescription, tokenDescription, outputTokenCacheKey string) *KubeadmCreateTokenStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Kubeadm Create Token"
	}
	finalStepDescription := stepDescription
	// Description will be refined in populateDefaults.

	if outputTokenCacheKey == "" {
		// This is a required field for the step to be useful.
		// Consider returning an error or panicking if strict. For now, allow creation.
	}

	return &KubeadmCreateTokenStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalStepDescription,
		},
		TokenDescription:    tokenDescription,
		OutputTokenCacheKey: outputTokenCacheKey,
		// Sudo defaults to false, as kubeadm token create with a valid kubeconfig typically doesn't need it.
	}
}

// Name returns the step's name.
func (s *KubeadmCreateTokenStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *KubeadmCreateTokenStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *KubeadmCreateTokenStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *KubeadmCreateTokenStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *KubeadmCreateTokenStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *KubeadmCreateTokenStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *KubeadmCreateTokenStepSpec) populateDefaults(logger runtime.Logger) {
	if s.KubeadmPath == "" {
		s.KubeadmPath = "kubeadm"
		logger.Debug("KubeadmPath defaulted to 'kubeadm'.")
	}
	if s.AdminKubeconfigPath == "" {
		s.AdminKubeconfigPath = "/etc/kubernetes/admin.conf"
		logger.Debug("AdminKubeconfigPath defaulted.", "path", s.AdminKubeconfigPath)
	}
	if s.TTL == "" {
		s.TTL = "24h0m0s"
		logger.Debug("TTL defaulted.", "ttl", s.TTL)
	}
	if len(s.Groups) == 0 {
		s.Groups = []string{"system:bootstrappers:kubeadm:default-node-token"}
		logger.Debug("Groups defaulted.", "groups", s.Groups)
	}
	if len(s.Usages) == 0 {
		s.Usages = []string{"signing", "authentication"}
		logger.Debug("Usages defaulted.", "usages", s.Usages)
	}

	if s.StepMeta.Description == "" {
		desc := fmt.Sprintf("Creates a new bootstrap token using %s", s.KubeadmPath)
		if s.TokenDescription != "" {
			desc += fmt.Sprintf(" (Description: '%s')", s.TokenDescription)
		}
		desc += fmt.Sprintf(" with TTL %s. Output to cache key '%s'.", s.TTL, s.OutputTokenCacheKey)
		s.StepMeta.Description = desc
	}
}

// Precheck ensures kubeadm is available and admin kubeconfig exists.
func (s *KubeadmCreateTokenStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.OutputTokenCacheKey == "" {
		return false, fmt.Errorf("OutputTokenCacheKey must be specified for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	if _, err := conn.LookPath(ctx.GoContext(), s.KubeadmPath); err != nil {
		return false, fmt.Errorf("%s command not found on host %s: %w", s.KubeadmPath, host.GetName(), err)
	}
	logger.Debug("kubeadm command found on host.")

	adminConfigExists, err := conn.Exists(ctx.GoContext(), s.AdminKubeconfigPath)
	if err != nil {
		logger.Error("Failed to check admin kubeconfig existence.", "path", s.AdminKubeconfigPath, "error", err)
		return false, fmt.Errorf("failed to check existence of admin kubeconfig %s on host %s: %w", s.AdminKubeconfigPath, host.GetName(), err)
	}
	if !adminConfigExists {
		return false, fmt.Errorf("admin kubeconfig %s does not exist on host %s, required for token creation", s.AdminKubeconfigPath, host.GetName())
	}
	logger.Debug("Admin kubeconfig file exists.", "path", s.AdminKubeconfigPath)

	// This step always runs to create a new token as requested.
	// Idempotency for "a token with these exact properties exists" is complex and not typical for `token create`.
	return false, nil
}

// Run executes `kubeadm token create`.
func (s *KubeadmCreateTokenStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.OutputTokenCacheKey == "" {
		return fmt.Errorf("OutputTokenCacheKey must be specified for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	cmdParts := []string{}
	if s.Sudo {
		cmdParts = append(cmdParts, "sudo")
	}
	cmdParts = append(cmdParts, s.KubeadmPath)
	cmdParts = append(cmdParts, fmt.Sprintf("--kubeconfig=%s", s.AdminKubeconfigPath))
	cmdParts = append(cmdParts, "token", "create")

	if s.TTL != "" {
		cmdParts = append(cmdParts, fmt.Sprintf("--ttl=%s", s.TTL))
	}
	if s.TokenDescription != "" {
		cmdParts = append(cmdParts, fmt.Sprintf("--description=%s", s.TokenDescription))
	}
	if len(s.Groups) > 0 {
		cmdParts = append(cmdParts, fmt.Sprintf("--groups=%s", strings.Join(s.Groups, ",")))
	}
	if len(s.Usages) > 0 {
		cmdParts = append(cmdParts, fmt.Sprintf("--usages=%s", strings.Join(s.Usages, ",")))
	}

	fullCmd := strings.Join(cmdParts, " ")

	logger.Info("Executing kubeadm token create command.", "command", fullCmd)
	stdout, stderr, err := conn.Exec(ctx.GoContext(), fullCmd, &connector.ExecOptions{Sudo: false}) // Sudo is already in cmdParts
	if err != nil {
		return fmt.Errorf("failed to execute kubeadm token create (stdout: %s, stderr: %s): %w", string(stdout), string(stderr), err)
	}

	tokenString := strings.TrimSpace(string(stdout))
	if tokenString == "" {
		return fmt.Errorf("kubeadm token create returned empty token string (stderr: %s)", string(stderr))
	}

	ctx.StepCache().Set(s.OutputTokenCacheKey, tokenString)
	logger.Info("Kubeadm token created successfully and stored in cache.", "key", s.OutputTokenCacheKey)

	if s.OutputTokenIDCacheKey != "" {
		// Token format is typically <id>.<secret>
		parts := strings.Split(tokenString, ".")
		if len(parts) == 2 && len(parts[0]) == 6 { // Basic validation for a typical token ID
			tokenID := parts[0]
			ctx.StepCache().Set(s.OutputTokenIDCacheKey, tokenID)
			logger.Debug("Stored token ID in cache.", "key", s.OutputTokenIDCacheKey, "id", tokenID)
		} else {
			logger.Warn("Could not parse token ID from generated token string.", "token", tokenString)
		}
	}
	return nil
}

// Rollback attempts to delete the created token if it was cached.
func (s *KubeadmCreateTokenStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger)

	if s.OutputTokenCacheKey == "" {
		logger.Info("OutputTokenCacheKey is not set, cannot determine token to delete for rollback.")
		return nil
	}

	cachedToken, found := ctx.StepCache().Get(s.OutputTokenCacheKey)
	if !found {
		logger.Info("No token found in cache to delete for rollback.", "key", s.OutputTokenCacheKey)
		return nil
	}
	tokenString, ok := cachedToken.(string)
	if !ok || tokenString == "" {
		logger.Warn("Cached token is not a valid string or is empty, cannot delete.", "key", s.OutputTokenCacheKey, "value", cachedToken)
		return nil
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}

	cmdParts := []string{}
	if s.Sudo {
		cmdParts = append(cmdParts, "sudo")
	}
	cmdParts = append(cmdParts, s.KubeadmPath)
	cmdParts = append(cmdParts, fmt.Sprintf("--kubeconfig=%s", s.AdminKubeconfigPath))
	cmdParts = append(cmdParts, "token", "delete", tokenString)

	fullCmd := strings.Join(cmdParts, " ")

	logger.Info("Attempting to delete kubeadm token for rollback.", "command", fullCmd)
	stdout, stderr, err := conn.Exec(ctx.GoContext(), fullCmd, &connector.ExecOptions{Sudo: false})
	if err != nil {
		// Log error but do not fail the rollback itself, as token might have already expired or been deleted.
		logger.Error("kubeadm token delete command failed during rollback (best effort).", "token", tokenString, "stdout", string(stdout), "stderr", string(stderr), "error", err)
	} else {
		logger.Info("Kubeadm token deleted successfully for rollback.", "token", tokenString, "stdout", string(stdout))
	}

	// Clean up cache keys
	ctx.StepCache().Delete(s.OutputTokenCacheKey)
	if s.OutputTokenIDCacheKey != "" {
		ctx.StepCache().Delete(s.OutputTokenIDCacheKey)
	}
	logger.Debug("Cleaned up token cache keys for rollback.")
	return nil
}

var _ step.Step = (*KubeadmCreateTokenStepSpec)(nil)
