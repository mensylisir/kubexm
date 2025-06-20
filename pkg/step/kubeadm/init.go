package kubeadm

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// KubeadmInitStepSpec defines parameters for running `kubeadm init`.
type KubeadmInitStepSpec struct {
	spec.StepMeta `json:",inline"`

	ConfigPath             string   `json:"configPath,omitempty"`
	KubeadmPath            string   `json:"kubeadmPath,omitempty"` // Path to kubeadm binary
	UploadCerts            bool     `json:"uploadCerts,omitempty"`
	CertificateKey         string   `json:"certificateKey,omitempty"`
	PatchesDir             string   `json:"patchesDir,omitempty"`
	DryRun                 bool     `json:"dryRun,omitempty"`
	IgnorePreflightErrors  []string `json:"ignorePreflightErrors,omitempty"`
	AdminKubeconfigPath    string   `json:"adminKubeconfigPath,omitempty"`
	OutputKubeconfigCacheKey string `json:"outputKubeconfigCacheKey,omitempty"`
	Sudo                   bool     `json:"sudo,omitempty"`
}

// NewKubeadmInitStepSpec creates a new KubeadmInitStepSpec.
func NewKubeadmInitStepSpec(name, description, configPath string) *KubeadmInitStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Kubeadm Init"
	}
	finalDescription := description
	// Description will be refined in populateDefaults.

	return &KubeadmInitStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		ConfigPath: configPath,
		Sudo:       true, // Default Sudo to true for kubeadm init
	}
}

// Name returns the step's name.
func (s *KubeadmInitStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *KubeadmInitStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *KubeadmInitStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *KubeadmInitStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *KubeadmInitStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *KubeadmInitStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *KubeadmInitStepSpec) populateDefaults(logger runtime.Logger) {
	if s.AdminKubeconfigPath == "" {
		s.AdminKubeconfigPath = "/etc/kubernetes/admin.conf"
		logger.Debug("AdminKubeconfigPath defaulted.", "path", s.AdminKubeconfigPath)
	}
	if s.KubeadmPath == "" {
		s.KubeadmPath = "kubeadm" // Assumes kubeadm is in PATH
		logger.Debug("KubeadmPath defaulted to 'kubeadm'.")
	}
	// Sudo is defaulted to true in factory.

	if s.StepMeta.Description == "" {
		desc := fmt.Sprintf("Initializes Kubernetes control plane using %s", s.KubeadmPath)
		if s.ConfigPath != "" {
			desc += fmt.Sprintf(" with config %s", s.ConfigPath)
		}
		if s.UploadCerts {
			desc += " and uploads certificates"
		}
		if s.DryRun {
			desc += " (dry run)"
		}
		s.StepMeta.Description = desc + "."
	}
}

// Precheck checks if the admin kubeconfig file already exists.
func (s *KubeadmInitStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Check if kubeadm command exists
	if _, err := conn.LookPath(ctx.GoContext(), s.KubeadmPath); err != nil {
		return false, fmt.Errorf("%s command not found on host %s: %w", s.KubeadmPath, host.GetName(), err)
	}

	exists, err := conn.Exists(ctx.GoContext(), s.AdminKubeconfigPath)
	if err != nil {
		logger.Warn("Failed to check for admin.conf existence, assuming cluster not initialized.", "path", s.AdminKubeconfigPath, "error", err)
		return false, nil // Let Run attempt.
	}

	if exists {
		logger.Info("Admin kubeconfig file already exists, assuming cluster is initialized.", "path", s.AdminKubeconfigPath)
		return true, nil
	}

	logger.Info("Admin kubeconfig file does not exist. Kubeadm init required.", "path", s.AdminKubeconfigPath)
	return false, nil
}

// Run executes `kubeadm init`.
func (s *KubeadmInitStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	var initArgs []string
	if s.ConfigPath != "" {
		initArgs = append(initArgs, fmt.Sprintf("--config=%s", s.ConfigPath))
	}
	if s.UploadCerts {
		initArgs = append(initArgs, "--upload-certs")
	}
	if s.CertificateKey != "" {
		initArgs = append(initArgs, fmt.Sprintf("--certificate-key=%s", s.CertificateKey))
	}
	if s.PatchesDir != "" {
		initArgs = append(initArgs, fmt.Sprintf("--patches=%s", s.PatchesDir))
	}
	if s.DryRun {
		initArgs = append(initArgs, "--dry-run")
	}
	if len(s.IgnorePreflightErrors) > 0 {
		initArgs = append(initArgs, fmt.Sprintf("--ignore-preflight-errors=%s", strings.Join(s.IgnorePreflightErrors, ",")))
	}

	// Use the generic RunKubeadmStepSpec to execute the command
	// The factory for RunKubeadmStepSpec: NewRunKubeadmStepSpec(name, description, subCommand string, subCommandArgs []string, globalArgs []string)
	// For `kubeadm init`, there are no "global args" before "init", they are all part of the subcommand args or covered by --config.
	kubeadmExecutorStep := NewRunKubeadmStepSpec(
		fmt.Sprintf("%s actual execution", s.GetName()),
		fmt.Sprintf("Internal execution of %s init", s.KubeadmPath),
		"init", // SubCommand
		initArgs, // SubCommandArgs for "init"
		nil,      // GlobalArgs for kubeadm itself (e.g. --kubeconfig, --v=X) - not typically used with `init` directly this way
	)
	kubeadmExecutorStep.Sudo = s.Sudo // Propagate sudo setting
	// KubeadmPath is handled by RunKubeadmStepSpec if it's not "kubeadm" (not directly supported by RunKubeadmStepSpec yet, assumes kubeadm is in PATH)
	// If KubeadmPath is custom, RunKubeadmStepSpec would need to be enhanced or command constructed manually.
	// For now, assuming RunKubeadmStepSpec uses "kubeadm" from PATH. If s.KubeadmPath is different, this needs adjustment.
	// Let's adjust RunKubeadmStepSpec to accept KubeadmPath or construct command directly here.
	// For now, let's construct the command directly here to use s.KubeadmPath.

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	cmdParts := []string{}
	if s.Sudo {
		cmdParts = append(cmdParts, "sudo")
	}
	cmdParts = append(cmdParts, s.KubeadmPath, "init")
	cmdParts = append(cmdParts, initArgs...)
	fullCmd := strings.Join(cmdParts, " ")

	logger.Info("Executing kubeadm init command.", "command", fullCmd)
	stdout, stderr, err := conn.Exec(ctx.GoContext(), fullCmd, &connector.ExecOptions{Sudo: false}) // Sudo is already in cmdParts
	if err != nil {
		return fmt.Errorf("failed to execute kubeadm init (stdout: %s, stderr: %s): %w", string(stdout), string(stderr), err)
	}
	logger.Info("Kubeadm init executed successfully.", "stdout", string(stdout))


	if s.OutputKubeconfigCacheKey != "" && s.AdminKubeconfigPath != "" {
		logger.Info("Reading admin kubeconfig content to cache.", "path", s.AdminKubeconfigPath)
		// Kubeconfig is typically root-owned, so sudo for cat.
		catCmd := fmt.Sprintf("cat %s", s.AdminKubeconfigPath)
		kubeconfigContentBytes, stderrCat, errCat := conn.Exec(ctx.GoContext(), catCmd, &connector.ExecOptions{Sudo: true})
		if errCat != nil {
			// Log as warning because init might have succeeded but reading kubeconfig failed.
			logger.Warn("Failed to read admin kubeconfig for caching.", "path", s.AdminKubeconfigPath, "stderr", string(stderrCat), "error", errCat)
		} else {
			ctx.StepCache().Set(s.OutputKubeconfigCacheKey, string(kubeconfigContentBytes))
			logger.Debug("Stored admin kubeconfig content in StepCache.", "key", s.OutputKubeconfigCacheKey)
		}
	}
	return nil
}

// Rollback executes `kubeadm reset`.
func (s *KubeadmInitStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger) // Ensure KubeadmPath is set

	resetArgs := []string{"--force"} // Common argument for reset

	// Use the generic RunKubeadmStepSpec to execute the reset command
	kubeadmResetExecutorStep := NewRunKubeadmStepSpec(
		fmt.Sprintf("Rollback for %s: kubeadm reset", s.GetName()),
		fmt.Sprintf("Internal execution of %s reset to roll back init", s.KubeadmPath),
		"reset",
		resetArgs,
		nil, // No global args for reset typically
	)
	kubeadmResetExecutorStep.Sudo = s.Sudo // Propagate sudo setting

	logger.Info("Executing kubeadm reset for rollback.")
	// Similar to Run, if KubeadmPath is custom, direct command construction is safer for now.
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}

	cmdParts := []string{}
	if s.Sudo {
		cmdParts = append(cmdParts, "sudo")
	}
	cmdParts = append(cmdParts, s.KubeadmPath, "reset")
	cmdParts = append(cmdParts, resetArgs...)
	fullCmd := strings.Join(cmdParts, " ")

	stdout, stderr, err := conn.Exec(ctx.GoContext(), fullCmd, &connector.ExecOptions{Sudo: false})
	if err != nil {
		// Log error but do not fail the rollback itself, as reset can also fail if node is already reset.
		logger.Error("kubeadm reset command failed during rollback (best effort).", "command", fullCmd, "stdout", string(stdout), "stderr", string(stderr), "error", err)
	} else {
		logger.Info("kubeadm reset executed successfully for rollback.", "stdout", string(stdout))
	}
	return nil
}

var _ step.Step = (*KubeadmInitStepSpec)(nil)
