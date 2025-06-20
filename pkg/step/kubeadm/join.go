package kubeadm

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// KubeadmJoinStepSpec defines parameters for running `kubeadm join`.
type KubeadmJoinStepSpec struct {
	spec.StepMeta `json:",inline"`

	KubeadmPath                 string   `json:"kubeadmPath,omitempty"`
	ConfigPath                string   `json:"configPath,omitempty"`
	DiscoveryTokenCACertHashes  []string `json:"discoveryTokenCaCertHashes,omitempty"`
	Token                       string   `json:"token,omitempty"`
	ControlPlane              bool     `json:"controlPlane,omitempty"`
	CertificateKey            string   `json:"certificateKey,omitempty"`
	NodeName                  string   `json:"nodeName,omitempty"`
	PatchesDir                string   `json:"patchesDir,omitempty"`
	IgnorePreflightErrors     []string `json:"ignorePreflightErrors,omitempty"`
	APIServerAdvertiseAddress string   `json:"apiServerAdvertiseAddress,omitempty"`
	APIServerBindPort         int32    `json:"apiServerBindPort,omitempty"`
	ControlPlaneEndpoint      string   `json:"controlPlaneEndpoint,omitempty"` // Positional argument
	Sudo                      bool     `json:"sudo,omitempty"`
	KubeletKubeconfigPath     string   `json:"kubeletKubeconfigPath,omitempty"`
}

// NewKubeadmJoinStepSpec creates a new KubeadmJoinStepSpec.
func NewKubeadmJoinStepSpec(name, description, token, controlPlaneEndpoint string) *KubeadmJoinStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Kubeadm Join"
	}
	finalDescription := description
	// Description will be refined in populateDefaults.

	return &KubeadmJoinStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		Token:                token,
		ControlPlaneEndpoint: controlPlaneEndpoint,
		Sudo:                 true, // Default Sudo to true for kubeadm join
	}
}

// Name returns the step's name.
func (s *KubeadmJoinStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *KubeadmJoinStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *KubeadmJoinStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *KubeadmJoinStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *KubeadmJoinStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *KubeadmJoinStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *KubeadmJoinStepSpec) populateDefaults(logger runtime.Logger) {
	if s.KubeadmPath == "" {
		s.KubeadmPath = "kubeadm"
		logger.Debug("KubeadmPath defaulted to 'kubeadm'.")
	}
	if s.KubeletKubeconfigPath == "" {
		s.KubeletKubeconfigPath = "/etc/kubernetes/kubelet.conf"
		logger.Debug("KubeletKubeconfigPath defaulted.", "path", s.KubeletKubeconfigPath)
	}
	// Sudo is defaulted in factory.

	if s.StepMeta.Description == "" {
		desc := fmt.Sprintf("Joins node to Kubernetes cluster using %s and endpoint %s", s.KubeadmPath, s.ControlPlaneEndpoint)
		if s.ControlPlane {
			desc += " as a control plane node"
		}
		if s.ConfigPath != "" {
			desc += fmt.Sprintf(" with config %s", s.ConfigPath)
		}
		s.StepMeta.Description = desc + "."
	}
}

// Precheck checks if kubeadm is available and if the node seems already joined.
func (s *KubeadmJoinStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.Token == "" || s.ControlPlaneEndpoint == "" {
		return false, fmt.Errorf("Token and ControlPlaneEndpoint must be specified for %s", s.GetName())
	}
	if s.ControlPlane && s.CertificateKey == "" {
		// CertificateKey is required only when joining a control plane node if not using --config with this info.
		// If --config is used, it might contain the cert key. This precheck is basic.
		logger.Warn("Joining as control plane, CertificateKey is not set. This might be an issue if not using a config file with this information.")
	}


	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	if _, err := conn.LookPath(ctx.GoContext(), s.KubeadmPath); err != nil {
		return false, fmt.Errorf("%s command not found on host %s: %w", s.KubeadmPath, host.GetName(), err)
	}
	logger.Debug("kubeadm command found on host.")

	exists, err := conn.Exists(ctx.GoContext(), s.KubeletKubeconfigPath)
	if err != nil {
		logger.Warn("Failed to check for kubelet.conf existence, assuming node not joined.", "path", s.KubeletKubeconfigPath, "error", err)
		return false, nil
	}

	if exists {
		logger.Info("Kubelet kubeconfig file already exists, assuming node is already part of a cluster.", "path", s.KubeletKubeconfigPath)
		return true, nil
	}

	logger.Info("Kubelet kubeconfig file does not exist. Kubeadm join required.", "path", s.KubeletKubeconfigPath)
	return false, nil
}

// Run executes `kubeadm join`.
func (s *KubeadmJoinStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.Token == "" || s.ControlPlaneEndpoint == "" {
		return fmt.Errorf("Token and ControlPlaneEndpoint must be specified for %s", s.GetName())
	}
	if s.ControlPlane && s.CertificateKey == "" && s.ConfigPath == "" {
		// If joining as CP and no config file providing the cert key, it's an error.
		return fmt.Errorf("CertificateKey is required when joining as a control plane node and no external config is provided for %s", s.GetName())
	}


	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	cmdParts := []string{}
	if s.Sudo {
		cmdParts = append(cmdParts, "sudo")
	}
	cmdParts = append(cmdParts, s.KubeadmPath, "join", s.ControlPlaneEndpoint)

	if s.ConfigPath != "" {
		cmdParts = append(cmdParts, fmt.Sprintf("--config=%s", s.ConfigPath))
	} else { // These flags are ignored if --config is used
		cmdParts = append(cmdParts, fmt.Sprintf("--token=%s", s.Token))
		if len(s.DiscoveryTokenCACertHashes) > 0 {
			cmdParts = append(cmdParts, fmt.Sprintf("--discovery-token-ca-cert-hash=%s", strings.Join(s.DiscoveryTokenCACertHashes, ",")))
		}
		// Add other flags only if not using --config
		if s.ControlPlane {
			cmdParts = append(cmdParts, "--control-plane")
			if s.CertificateKey != "" {
				cmdParts = append(cmdParts, fmt.Sprintf("--certificate-key=%s", s.CertificateKey))
			}
			if s.APIServerAdvertiseAddress != "" {
				cmdParts = append(cmdParts, fmt.Sprintf("--apiserver-advertise-address=%s", s.APIServerAdvertiseAddress))
			}
			if s.APIServerBindPort > 0 {
				cmdParts = append(cmdParts, fmt.Sprintf("--apiserver-bind-port=%d", s.APIServerBindPort))
			}
		}
		if s.NodeName != "" {
			cmdParts = append(cmdParts, fmt.Sprintf("--node-name=%s", s.NodeName))
		}
	}

	// These flags can be used with or without --config
	if s.PatchesDir != "" {
		cmdParts = append(cmdParts, fmt.Sprintf("--patches=%s", s.PatchesDir))
	}
	if len(s.IgnorePreflightErrors) > 0 {
		cmdParts = append(cmdParts, fmt.Sprintf("--ignore-preflight-errors=%s", strings.Join(s.IgnorePreflightErrors, ",")))
	}

	fullCmd := strings.Join(cmdParts, " ")

	logger.Info("Executing kubeadm join command.", "command", fullCmd)
	stdout, stderr, err := conn.Exec(ctx.GoContext(), fullCmd, &connector.ExecOptions{Sudo: false}) // Sudo is already in cmdParts
	if err != nil {
		return fmt.Errorf("failed to execute kubeadm join (stdout: %s, stderr: %s): %w", string(stdout), string(stderr), err)
	}

	logger.Info("Kubeadm join executed successfully.", "stdout", string(stdout))
	return nil
}

// Rollback executes `kubeadm reset`.
func (s *KubeadmJoinStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	s.populateDefaults(logger) // Ensure KubeadmPath is set

	resetArgs := []string{"--force"}

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

	logger.Info("Executing kubeadm reset for rollback.", "command", fullCmd)
	stdout, stderr, err := conn.Exec(ctx.GoContext(), fullCmd, &connector.ExecOptions{Sudo: false})
	if err != nil {
		logger.Error("kubeadm reset command failed during rollback (best effort).", "command", fullCmd, "stdout", string(stdout), "stderr", string(stderr), "error", err)
	} else {
		logger.Info("kubeadm reset executed successfully for rollback.", "stdout", string(stdout))
	}
	return nil // Rollback is best-effort for reset
}

var _ step.Step = (*KubeadmJoinStepSpec)(nil)
