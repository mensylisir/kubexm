package kubeadm

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// KubeadmResetStepSpec defines parameters for running `kubeadm reset`.
type KubeadmResetStepSpec struct {
	spec.StepMeta `json:",inline"`

	KubeadmPath           string   `json:"kubeadmPath,omitempty"`
	Force                 bool     `json:"force,omitempty"`
	CertificatesDir       string   `json:"certificatesDir,omitempty"`
	CRISocketPath         string   `json:"criSocketPath,omitempty"`
	CleanupTmpDirs        bool     `json:"cleanupTmpDirs,omitempty"`
	IgnorePreflightErrors[]string `json:"ignorePreflightErrors,omitempty"`
	Sudo                  bool     `json:"sudo,omitempty"`
	KubeletKubeconfigPath string   `json:"kubeletKubeconfigPath,omitempty"` // Used for precheck
}

// NewKubeadmResetStepSpec creates a new KubeadmResetStepSpec.
func NewKubeadmResetStepSpec(name, description string) *KubeadmResetStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Kubeadm Reset"
	}
	finalDescription := description
	// Description will be refined in populateDefaults.

	return &KubeadmResetStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		Sudo:  true, // Default Sudo to true for kubeadm reset
		Force: true, // Default Force to true for kubeadm reset
	}
}

// Name returns the step's name.
func (s *KubeadmResetStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *KubeadmResetStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *KubeadmResetStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *KubeadmResetStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *KubeadmResetStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *KubeadmResetStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *KubeadmResetStepSpec) populateDefaults(logger runtime.Logger) {
	if s.KubeadmPath == "" {
		s.KubeadmPath = "kubeadm"
		logger.Debug("KubeadmPath defaulted to 'kubeadm'.")
	}
	if s.KubeletKubeconfigPath == "" {
		s.KubeletKubeconfigPath = "/etc/kubernetes/kubelet.conf"
		logger.Debug("KubeletKubeconfigPath defaulted.", "path", s.KubeletKubeconfigPath)
	}
	// Sudo and Force are defaulted in factory.

	if s.StepMeta.Description == "" {
		desc := fmt.Sprintf("Resets the node using %s", s.KubeadmPath)
		if s.Force {
			desc += " with --force"
		}
		s.StepMeta.Description = desc + "."
	}
}

// Precheck checks if kubeadm is available and if the node seems to need a reset.
func (s *KubeadmResetStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	if _, err := conn.LookPath(ctx.GoContext(), s.KubeadmPath); err != nil {
		return false, fmt.Errorf("%s command not found on host %s: %w", s.KubeadmPath, host.GetName(), err)
	}
	logger.Debug("kubeadm command found on host.")

	// If kubelet.conf (or other key K8s files like admin.conf) doesn't exist,
	// it's likely the node is already in a reset-like state or was never joined/initialized.
	// For simplicity, checking KubeletKubeconfigPath.
	exists, err := conn.Exists(ctx.GoContext(), s.KubeletKubeconfigPath)
	if err != nil {
		logger.Warn("Failed to check for kubelet.conf existence, assuming reset can proceed.", "path", s.KubeletKubeconfigPath, "error", err)
		return false, nil // Let Run attempt.
	}

	if !exists {
		logger.Info("Kubelet kubeconfig file does not exist, assuming node is already in a reset-like state or was never part of a cluster.", "path", s.KubeletKubeconfigPath)
		return true, nil // Done = true, reset is not needed.
	}

	logger.Info("Kubelet kubeconfig file exists. Node appears to be part of a cluster. Reset can proceed.", "path", s.KubeletKubeconfigPath)
	return false, nil // Not done, reset can proceed.
}

// Run executes `kubeadm reset`.
func (s *KubeadmResetStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	cmdParts := []string{}
	if s.Sudo {
		cmdParts = append(cmdParts, "sudo")
	}
	cmdParts = append(cmdParts, s.KubeadmPath, "reset")

	if s.Force {
		cmdParts = append(cmdParts, "--force")
	}
	if s.CertificatesDir != "" {
		cmdParts = append(cmdParts, fmt.Sprintf("--cert-dir=%s", s.CertificatesDir))
	}
	if s.CRISocketPath != "" {
		// Note: CRI socket path might need to be determined dynamically or passed correctly.
		// Example: "--cri-socket=unix:///var/run/containerd/containerd.sock"
		cmdParts = append(cmdParts, fmt.Sprintf("--cri-socket=%s", s.CRISocketPath))
	}
	if s.CleanupTmpDirs {
		cmdParts = append(cmdParts, "--cleanup-tmp-dirs")
	}
	if len(s.IgnorePreflightErrors) > 0 {
		cmdParts = append(cmdParts, fmt.Sprintf("--ignore-preflight-errors=%s", strings.Join(s.IgnorePreflightErrors, ",")))
	}

	fullCmd := strings.Join(cmdParts, " ")

	logger.Info("Executing kubeadm reset command.", "command", fullCmd)
	// kubeadm reset often exits 0 even if some cleanup fails or if already reset,
	// so error checking might need to be nuanced or primarily rely on stdout/stderr for issues.
	stdout, stderr, err := conn.Exec(ctx.GoContext(), fullCmd, &connector.ExecOptions{Sudo: false}) // Sudo is already in cmdParts
	if err != nil {
		// Log the error but don't necessarily fail the step, as reset is often a "best effort" cleanup.
		// Some errors (like "could not find a JWS signed token" if trying to unpublish) are non-fatal for reset's goal.
		logger.Warn("kubeadm reset command finished with an error (this might be acceptable).", "command", fullCmd, "stdout", string(stdout), "stderr", string(stderr), "error", err)
	} else {
		logger.Info("kubeadm reset executed successfully.", "stdout", string(stdout), "stderr", string(stderr))
	}

	// After reset, ensure key config files that Precheck looks for are indeed gone.
	// This makes the step more robust if called again.
	if s.KubeletKubeconfigPath != "" {
	    exists, _ := conn.Exists(ctx.GoContext(), s.KubeletKubeconfigPath)
	    if exists {
	        logger.Warn("Kubelet kubeconfig still exists after reset. Manual cleanup might be needed for full idempotency of Precheck.", "path", s.KubeletKubeconfigPath)
	    }
	}

	return nil // Typically, `kubeadm reset` failures are not fatal to a larger cleanup workflow.
}

// Rollback for kubeadm reset is not applicable.
func (s *KubeadmResetStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for kubeadm reset is not applicable.")
	return nil
}

var _ step.Step = (*KubeadmResetStepSpec)(nil)
