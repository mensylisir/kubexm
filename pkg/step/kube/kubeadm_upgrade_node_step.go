package kube

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// KubeadmUpgradeNodeStep executes 'kubeadm upgrade node' on a worker or secondary control-plane node.
type KubeadmUpgradeNodeStep struct {
	meta                  spec.StepMeta
	IgnorePreflightErrors string // Comma-separated list or "all"
	CertificatesDir       string // Path to certificates directory
	CriSocket             string // CRI socket path
	DryRun                bool   // Corresponds to --dry-run for kubeadm
	PatchesDir            string // Corresponds to --patches for kubeadm
	Sudo                  bool
}

// NewKubeadmUpgradeNodeStep creates a new KubeadmUpgradeNodeStep.
func NewKubeadmUpgradeNodeStep(instanceName, ignorePreflightErrors, certificatesDir, criSocket, patchesDir string, dryRun, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "KubeadmUpgradeNode"
	}
	return &KubeadmUpgradeNodeStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: "Upgrades the node using 'kubeadm upgrade node'.",
		},
		IgnorePreflightErrors: ignorePreflightErrors,
		CertificatesDir:       certificatesDir,
		CriSocket:             criSocket,
		DryRun:                dryRun,
		PatchesDir:            patchesDir,
		Sudo:                  true, // kubeadm upgrade node usually requires sudo
	}
}

func (s *KubeadmUpgradeNodeStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *KubeadmUpgradeNodeStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	// Precheck for 'upgrade node' is complex. It depends on the overall cluster upgrade state
	// and the current version of kubelet on this node vs the control plane version.
	// This step assumes that a higher-level Task/Module has determined this node *needs* upgrading.
	// So, precheck here might be minimal, e.g., ensure kubeadm command exists.
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	if _, err := runnerSvc.LookPath(ctx.GoContext(), conn, "kubeadm"); err != nil {
		logger.Error("kubeadm command not found.", "error", err)
		return false, fmt.Errorf("kubeadm command not found on host %s: %w", host.GetName(), err)
	}
	logger.Info("KubeadmUpgradeNodeStep Precheck: kubeadm found. Assuming run is required if this step is reached.")
	return false, nil
}

func (s *KubeadmUpgradeNodeStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubeadm", "upgrade", "node")

	if s.IgnorePreflightErrors != "" {
		cmdArgs = append(cmdArgs, "--ignore-preflight-errors="+s.IgnorePreflightErrors)
	}
	if s.CertificatesDir != "" {
		// Note: 'kubeadm upgrade node' does not typically use --certificate-dir.
		// It uses existing certs or those from --control-plane-* flags if re-joining.
		// This field might be more relevant for 'kubeadm join --control-plane'.
		// For 'upgrade node', it's usually about upgrading kubelet config to match new control plane.
		// However, if kubeadm CLI evolves, we keep it. For now, it might be unused by the command.
		logger.Warn("CertificatesDir provided for KubeadmUpgradeNodeStep, but 'kubeadm upgrade node' may not use it directly.", "certDir", s.CertificatesDir)
	}
	if s.CriSocket != "" {
		cmdArgs = append(cmdArgs, "--cri-socket", s.CriSocket)
	}
	if s.DryRun {
		cmdArgs = append(cmdArgs, "--dry-run")
	}
	if s.PatchesDir != "" {
		cmdArgs = append(cmdArgs, "--patches", s.PatchesDir)
	}

	cmd := strings.Join(cmdArgs, " ")

	logger.Info("Running kubeadm upgrade node command.", "command", cmd)
	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: s.Sudo})
	if err != nil {
		logger.Error("kubeadm upgrade node command failed.", "error", err, "stdout", string(stdout), "stderr", string(stderr))
		return fmt.Errorf("kubeadm upgrade node failed: %w. Stdout: %s, Stderr: %s", err, string(stdout), string(stderr))
	}

	logger.Info("kubeadm upgrade node completed.", "stdout", string(stdout))
	// This step is often followed by upgrading kubelet and kubectl binaries and restarting kubelet service.
	return nil
}

func (s *KubeadmUpgradeNodeStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Warn("Rollback for KubeadmUpgradeNodeStep is complex and not automatically performed (would involve downgrading kubelet and rejoining or restoring from backup).")
	return nil
}

var _ step.Step = (*KubeadmUpgradeNodeStep)(nil)
