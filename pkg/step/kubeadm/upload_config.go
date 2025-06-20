package kubeadm

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// KubeadmUploadConfigStepSpec defines parameters for `kubeadm init phase upload-config`.
type KubeadmUploadConfigStepSpec struct {
	spec.StepMeta `json:",inline"`

	KubeadmPath         string `json:"kubeadmPath,omitempty"`
	ConfigPath          string `json:"configPath,omitempty"` // Path to the kubeadm config YAML to upload
	AdminKubeconfigPath string `json:"adminKubeconfigPath,omitempty"`
	PhaseName           string `json:"phaseName,omitempty"` // e.g., "all", "kubelet", "kubeadm"
	Sudo                bool   `json:"sudo,omitempty"`
}

// NewKubeadmUploadConfigStepSpec creates a new KubeadmUploadConfigStepSpec.
func NewKubeadmUploadConfigStepSpec(name, description, configPath string) *KubeadmUploadConfigStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Kubeadm Upload Config"
	}
	finalDescription := description
	// Description will be refined in populateDefaults.

	return &KubeadmUploadConfigStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		ConfigPath: configPath,
		// Sudo defaults to false, as kubeadm upload-config with a valid kubeconfig typically doesn't need it.
	}
}

// Name returns the step's name.
func (s *KubeadmUploadConfigStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *KubeadmUploadConfigStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *KubeadmUploadConfigStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *KubeadmUploadConfigStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *KubeadmUploadConfigStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *KubeadmUploadConfigStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *KubeadmUploadConfigStepSpec) populateDefaults(logger runtime.Logger) {
	if s.KubeadmPath == "" {
		s.KubeadmPath = "kubeadm"
		logger.Debug("KubeadmPath defaulted to 'kubeadm'.")
	}
	if s.AdminKubeconfigPath == "" {
		s.AdminKubeconfigPath = "/etc/kubernetes/admin.conf"
		logger.Debug("AdminKubeconfigPath defaulted.", "path", s.AdminKubeconfigPath)
	}
	if s.PhaseName == "" {
		s.PhaseName = "all"
		logger.Debug("PhaseName defaulted to 'all'.")
	}

	if s.StepMeta.Description == "" {
		desc := fmt.Sprintf("Uploads kubeadm config phase '%s' using %s", s.PhaseName, s.KubeadmPath)
		if s.ConfigPath != "" {
			desc += fmt.Sprintf(" from master config %s", s.ConfigPath)
		}
		if s.AdminKubeconfigPath != "" {
			desc += fmt.Sprintf(" (auth via %s)", s.AdminKubeconfigPath)
		}
		s.StepMeta.Description = desc + "."
	}
}

// Precheck ensures kubeadm is available and required config files exist.
func (s *KubeadmUploadConfigStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.ConfigPath == "" {
		return false, fmt.Errorf("ConfigPath must be specified for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	if _, err := conn.LookPath(ctx.GoContext(), s.KubeadmPath); err != nil {
		return false, fmt.Errorf("%s command not found on host %s: %w", s.KubeadmPath, host.GetName(), err)
	}
	logger.Debug("kubeadm command found on host.")

	// Check for admin kubeconfig needed for authentication
	adminConfigExists, err := conn.Exists(ctx.GoContext(), s.AdminKubeconfigPath)
	if err != nil {
		logger.Error("Failed to check admin kubeconfig existence.", "path", s.AdminKubeconfigPath, "error", err)
		return false, fmt.Errorf("failed to check existence of admin kubeconfig %s on host %s: %w", s.AdminKubeconfigPath, host.GetName(), err)
	}
	if !adminConfigExists {
		return false, fmt.Errorf("admin kubeconfig %s does not exist on host %s, required for upload-config", s.AdminKubeconfigPath, host.GetName())
	}
	logger.Debug("Admin kubeconfig file exists.", "path", s.AdminKubeconfigPath)

	// Check for the main kubeadm config file to be uploaded
	kubeadmConfigExists, err := conn.Exists(ctx.GoContext(), s.ConfigPath)
	if err != nil {
		logger.Error("Failed to check kubeadm config file existence.", "path", s.ConfigPath, "error", err)
		return false, fmt.Errorf("failed to check existence of kubeadm config %s on host %s: %w", s.ConfigPath, host.GetName(), err)
	}
	if !kubeadmConfigExists {
		return false, fmt.Errorf("kubeadm config file %s does not exist on host %s for upload", s.ConfigPath, host.GetName())
	}
	logger.Debug("Kubeadm config file to upload exists.", "path", s.ConfigPath)

	// Idempotency for upload-config is difficult to check without querying Kubernetes API for ConfigMaps.
	// Kubeadm itself might handle some idempotency (e.g., not re-uploading identical configs).
	// For this step, if all inputs are present, we assume it needs to run.
	logger.Info("Precheck complete, required files exist. Upload will proceed if invoked.")
	return false, nil
}

// Run executes `kubeadm init phase upload-config`.
func (s *KubeadmUploadConfigStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.ConfigPath == "" {
		return fmt.Errorf("ConfigPath must be specified for %s", s.GetName())
	}
	if s.AdminKubeconfigPath == "" {
		return fmt.Errorf("AdminKubeconfigPath must be specified for %s (used for --kubeconfig global flag)", s.GetName())
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
	// Global --kubeconfig flag for authentication
	cmdParts = append(cmdParts, fmt.Sprintf("--kubeconfig=%s", s.AdminKubeconfigPath))

	cmdParts = append(cmdParts, "init", "phase", "upload-config", s.PhaseName)
	cmdParts = append(cmdParts, fmt.Sprintf("--config=%s", s.ConfigPath))

	fullCmd := strings.Join(cmdParts, " ")

	logger.Info("Executing kubeadm upload-config command.", "command", fullCmd)
	// Sudo is prepended to cmdParts if s.Sudo is true.
	// Thus, ExecOptions Sudo should be false.
	stdout, stderr, err := conn.Exec(ctx.GoContext(), fullCmd, &connector.ExecOptions{Sudo: false})
	if err != nil {
		return fmt.Errorf("failed to execute kubeadm upload-config (stdout: %s, stderr: %s): %w", string(stdout), string(stderr), err)
	}

	logger.Info("Kubeadm upload-config executed successfully.", "stdout", string(stdout))
	return nil
}

// Rollback for kubeadm upload-config is not straightforward.
func (s *KubeadmUploadConfigStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Warn("Rollback for kubeadm upload-config is not implemented. Manual ConfigMap (e.g., kubeadm-config, kubelet-config) deletion or modification may be required if issues arise.")
	return nil
}

var _ step.Step = (*KubeadmUploadConfigStepSpec)(nil)
