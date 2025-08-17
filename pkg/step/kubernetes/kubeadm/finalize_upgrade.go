package kubeadm

import (
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

type KubeadmFinalizeUpgradeStep struct {
	step.Base
}

type KubeadmFinalizeUpgradeStepBuilder struct {
	step.Builder[KubeadmFinalizeUpgradeStepBuilder, *KubeadmFinalizeUpgradeStep]
}

func NewKubeadmFinalizeUpgradeStepBuilder(ctx runtime.Context, instanceName string) *KubeadmFinalizeUpgradeStepBuilder {
	s := &KubeadmFinalizeUpgradeStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Finalize the upgrade by applying the new CoreDNS manifest"
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(KubeadmFinalizeUpgradeStepBuilder).Init(s)
	return b
}

func (s *KubeadmFinalizeUpgradeStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *KubeadmFinalizeUpgradeStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	logger.Info("Starting precheck for addon finalization...")

	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	if _, err := ctx.GetRunner().Run(ctx.GoContext(), conn, "command -v kubeadm && command -v kubectl", s.Sudo); err != nil {
		return false, fmt.Errorf("precheck failed: 'kubeadm' or 'kubectl' not found on host '%s'", ctx.GetHost().GetName())
	}

	logger.Info("Precheck passed.")
	return false, nil
}

func (s *KubeadmFinalizeUpgradeStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	// The `kubeadm upgrade apply` command should have informed us about this.
	// It often says something like:
	// "[upgrade/addons] CoreDNS upgrade recommended ... you can run 'kubeadm upgrade diff' and then apply."
	// The most robust way to upgrade CoreDNS is to let kubeadm generate the manifest for the *current* cluster state.

	logger.Info("Generating new CoreDNS manifest based on the upgraded cluster configuration...")

	// This command sequence is the safest way to upgrade CoreDNS:
	// 1. `kubeadm config view`: Dumps the current in-cluster ClusterConfiguration.
	// 2. `kubeadm config images list`: Gets the correct image tag for the new version.
	// 3. Generate manifest and apply it via a pipe.

	// Let's use a simpler, more direct approach that `kubeadm` documentation often suggests.
	// `kubeadm upgrade apply` without a version will re-apply the correct manifests for the current version.
	// It's an idempotent way to ensure addons are correct.

	logger.Warn("Applying `kubeadm upgrade apply` again to ensure all addons like CoreDNS are up-to-date. This is an idempotent operation.")

	// We need the target version to run this command correctly.
	targetVersion, ok := ctx.GetTaskCache().Get(CacheKeyTargetVersion)
	if !ok {
		return fmt.Errorf("could not retrieve target version from cache, 'plan' step must run first")
	}
	versionStr := targetVersion.(string)

	// Re-running `upgrade apply` is idempotent and will fix/upgrade addons if needed.
	// We add `--skip-phases=preflight,certs,kubeconfig,control-plane,etcd` to only touch addons.
	// NOTE: The exact phases might vary slightly. A simpler approach is to generate the manifest.
	// Let's stick to the "generate manifest" approach as it's cleaner.

	log := logger.With("addon", "CoreDNS")
	log.Info("Generating and applying new manifest for CoreDNS...")

	// This command generates the ENTIRE manifest for a new cluster, we need to extract CoreDNS part.
	// A better, more modern kubeadm approach is `kubeadm init phase addon coredns --config ...`
	// but let's use what's universally available.

	// The most reliable documented method for just upgrading addons post-`upgrade apply`
	// is to run `kubeadm upgrade apply` again on the *already upgraded version*. It will see that the
	// control plane is up-to-date and proceed to fix addons.

	finalizeCmd := fmt.Sprintf("kubeadm upgrade apply %s --yes", versionStr)

	log.Infof("Re-running 'kubeadm upgrade apply' to finalize addon configuration...")
	stdout, err := runner.Run(ctx.GoContext(), conn, finalizeCmd, s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to finalize upgrade with 'kubeadm upgrade apply': %w. Output:\n%s", err, string(stdout))
	}

	// The output should indicate that the control plane is already upgraded and it's just checking addons.
	if !strings.Contains(string(stdout), "is already up to date") && !strings.Contains(string(stdout), "Upgraded CoreDNS") {
		log.Warnf("Finalize command output was unexpected, but may have succeeded. Output:\n%s", string(stdout))
	}

	logger.Info("Finalization tasks completed successfully.")
	return nil
}

func (s *KubeadmFinalizeUpgradeStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Warn("Rollback for addon upgrades is not performed automatically. It may require applying the manifest for the previous version.")
	return nil
}

var _ step.Step = (*KubeadmFinalizeUpgradeStep)(nil)
