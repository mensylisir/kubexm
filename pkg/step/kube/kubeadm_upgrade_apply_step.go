package kube

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	// Assuming step.StepContext will be used, which step.StepContext is an interface for.
	// No direct import of runtime needed if using the interface in methods.
)

// KubeadmUpgradeApplyStep performs 'kubeadm upgrade apply <version>' on a control-plane node.
type KubeadmUpgradeApplyStep struct {
	meta                  spec.StepMeta
	Version               string // Target Kubernetes version, e.g., "v1.23.5"
	IgnorePreflightErrors string // Comma-separated list or "all"
	CertificateRenewal    bool   // Corresponds to --certificate-renewal flag (defaults to true for kubeadm)
	EtcdUpgrade           bool   // Corresponds to --etcd-upgrade flag (defaults to true for kubeadm)
	PatchesDir            string // Optional: directory for kubeadm patch files (--patches)
	KubeadmConfigPath     string // Optional: path to a kubeadm configuration file for the upgrade (--config)
	DryRun                bool   // Corresponds to --dry-run
	Sudo                  bool
}

// NewKubeadmUpgradeApplyStep creates a new KubeadmUpgradeApplyStep.
func NewKubeadmUpgradeApplyStep(instanceName, version, ignoreErrors, patchesDir, kubeadmConfigPath string, certRenewal, etcdUpgrade, dryRun, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = fmt.Sprintf("KubeadmUpgradeApply-%s", version)
	}
	if version == "" {
		// Version is mandatory for 'upgrade apply'
		panic("version cannot be empty for KubeadmUpgradeApplyStep")
	}

	return &KubeadmUpgradeApplyStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Applies Kubernetes control plane upgrade to version %s", version),
		},
		Version:               version,
		IgnorePreflightErrors: ignoreErrors,
		CertificateRenewal:    certRenewal, // kubeadm defaults this to true
		EtcdUpgrade:           etcdUpgrade,   // kubeadm defaults this to true
		PatchesDir:            patchesDir,
		KubeadmConfigPath:     kubeadmConfigPath,
		DryRun:                dryRun,
		Sudo:                  true, // kubeadm upgrade apply usually requires sudo
	}
}

func (s *KubeadmUpgradeApplyStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *KubeadmUpgradeApplyStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	// 'kubeadm upgrade apply' is generally idempotent if the version matches.
	// A more robust precheck could involve 'kubeadm upgrade plan' and parsing its output
	// to see if an upgrade to s.Version is pending and possible.
	// For now, assume that if this step is reached, the upgrade is intended.
	// Kubeadm itself will state if it's already at the desired version.
	logger.Info("KubeadmUpgradeApplyStep Precheck: Assuming run is required to ensure specified version is applied.")
	// One simple check: ensure kubeadm command exists.
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("Precheck: failed to get connector for host %s: %w", host.GetName(), err)
	}
	if _, err := runnerSvc.LookPath(ctx.GoContext(), conn, "kubeadm"); err != nil {
		logger.Error("kubeadm command not found.", "error", err)
		return false, fmt.Errorf("kubeadm command not found on host %s for step %s: %w", host.GetName(), s.meta.Name, err)
	}
	return false, nil // Let Run proceed
}

func (s *KubeadmUpgradeApplyStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "kubeadm", "upgrade", "apply", s.Version, "--yes") // --yes for non-interactive

	// kubeadm 'upgrade apply' defaults --etcd-upgrade and --certificate-renewal to true.
	// Only add the flag if the user wants to explicitly set it to false.
	if !s.EtcdUpgrade {
		cmdArgs = append(cmdArgs, "--etcd-upgrade=false")
	}
	if !s.CertificateRenewal {
		cmdArgs = append(cmdArgs, "--certificate-renewal=false")
	}

	if s.IgnorePreflightErrors != "" {
		cmdArgs = append(cmdArgs, "--ignore-preflight-errors="+s.IgnorePreflightErrors)
	}
	if s.KubeadmConfigPath != "" {
		cmdArgs = append(cmdArgs, "--config", s.KubeadmConfigPath)
	}
	// Note: 'kubeadm upgrade apply' does not directly use --patches. Patches are typically for init/join.
	// If s.PatchesDir is set, it might be an indication that a custom config (--config) is also being used
	// which might reference these patches. Kubeadm itself will validate flags.
	if s.PatchesDir != "" {
		logger.Warn("PatchesDir is set for 'kubeadm upgrade apply', ensure your kubeadm config or process handles this.", "patchesDir", s.PatchesDir)
		// cmdArgs = append(cmdArgs, "--patches", s.PatchesDir) // Kubeadm apply doesn't accept this directly
	}
	if s.DryRun {
		cmdArgs = append(cmdArgs, "--dry-run")
	}

	cmd := strings.Join(cmdArgs, " ")

	logger.Info("Running kubeadm upgrade apply command.", "command", cmd)
	stdout, stderr, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: s.Sudo})
	if err != nil {
		logger.Error("kubeadm upgrade apply command failed.", "error", err, "stdout", string(stdout), "stderr", string(stderr))
		return fmt.Errorf("kubeadm upgrade apply command '%s' failed: %w. Stdout: %s, Stderr: %s", cmd, err, string(stdout), string(stderr))
	}

	logger.Info("kubeadm upgrade apply command completed successfully.", "stdout", string(stdout))
	return nil
}

func (s *KubeadmUpgradeApplyStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Warn("Rollback for KubeadmUpgradeApplyStep is highly complex (involves etcd restore, downgrading components) and not automatically supported. Manual intervention or etcd backup restoration is typically required.")
	return nil
}

var _ step.Step = (*KubeadmUpgradeApplyStep)(nil)
```
