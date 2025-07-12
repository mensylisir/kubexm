package kube

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// KubeadmResetStep executes 'kubeadm reset' on a node.
type KubeadmResetStep struct {
	meta                  spec.StepMeta
	Force                 bool
	IgnorePreflightErrors string // Comma-separated list or "all"
	CertificatesDir       string // Path to certificates directory for cleanup
	CriSocket             string // CRI socket path
	Sudo                  bool
}

// NewKubeadmResetStep creates a new KubeadmResetStep.
func NewKubeadmResetStep(instanceName string, force bool, ignorePreflightErrors, certificatesDir, criSocket string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "KubeadmReset"
	}
	return &KubeadmResetStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: "Resets the node using 'kubeadm reset'.",
		},
		Force:                 force,
		IgnorePreflightErrors: ignorePreflightErrors,
		CertificatesDir:       certificatesDir,
		CriSocket:             criSocket,
		Sudo:                  true, // kubeadm reset usually requires sudo
	}
}

func (s *KubeadmResetStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *KubeadmResetStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	// Precheck for reset is tricky. If the node is not part of a cluster, reset might not do much or error.
	// If it is part of a cluster, reset is a destructive action.
	// This step is usually called when we *know* we want to reset.
	// A simple precheck could be if /etc/kubernetes/manifests is empty or kubelet service is not running.
	// For now, assume if this step is planned, it's intended to run.
	logger.Info("KubeadmResetStep Precheck: Assuming run is required if this step is reached to ensure clean state.")
	return false, nil
}

func (s *KubeadmResetStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubeadm", "reset")
	if s.Force {
		cmdArgs = append(cmdArgs, "--force")
	}
	if s.IgnorePreflightErrors != "" {
		cmdArgs = append(cmdArgs, "--ignore-preflight-errors="+s.IgnorePreflightErrors)
	}
	if s.CertificatesDir != "" {
		cmdArgs = append(cmdArgs, "--cert-dir", s.CertificatesDir)
	}
	if s.CriSocket != "" {
		cmdArgs = append(cmdArgs, "--cri-socket", s.CriSocket)
	}
	cmd := strings.Join(cmdArgs, " ")

	logger.Info("Running kubeadm reset command.", "command", cmd)
	// kubeadm reset can take some time.
	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: s.Sudo})
	if err != nil {
		// kubeadm reset might return errors if node was already clean, which might be acceptable.
		// Check stderr for specific messages if needed.
		logger.Error("kubeadm reset command finished with an error (this may be acceptable if node was already clean).", "error", err, "stdout", string(stdout), "stderr", string(stderr))
		// Decide if this should be a hard failure or a warning.
		// For a general reset step, an error means it didn't complete as expected by kubeadm.
		return fmt.Errorf("kubeadm reset failed: %w. Stdout: %s, Stderr: %s", err, string(stdout), string(stderr))
	}

	logger.Info("kubeadm reset completed.", "stdout", string(stdout))
	// Additional cleanup steps (e.g., removing CNI configs, /var/lib/etcd, etc.) should be separate steps
	// planned by the Task, as `kubeadm reset` doesn't always clean everything.
	return nil
}

func (s *KubeadmResetStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Warn("Rollback for KubeadmResetStep is not applicable (would mean re-initializing the node).")
	return nil
}

var _ step.Step = (*KubeadmResetStep)(nil)
