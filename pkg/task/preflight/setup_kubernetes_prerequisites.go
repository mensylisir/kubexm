package preflight

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	preflightstep "github.com/kubexms/kubexms/pkg/step/preflight" // Alias for step package
)

// NewSetupKubernetesPrerequisitesTask creates a task to set up common Kubernetes system prerequisites.
func NewSetupKubernetesPrerequisitesTask(cfg *config.Cluster) *spec.TaskSpec {

	// Canonical list of desired kernel modules, including a placeholder for nf_conntrack.
	desiredModulesWithPlaceholder := []string{
		"br_netfilter",
		"overlay",
		"ip_vs",
		"ip_vs_rr",
		"ip_vs_wrr",
		"ip_vs_sh",
		"nf_conntrack_placeholder", // Will be resolved by EnsureKernelModulesPersistentStepSpec's executor
	}

	// Resolve modules for the LoadKernelModulesStepSpec (tries both nf_conntrack variants).
	// This logic is kept in the task definition as it's specific to how these modules are loaded.
	resolvedForLoading := []string{}
	for _, m := range desiredModulesWithPlaceholder {
		if m == "nf_conntrack_placeholder" {
			// LoadKernelModulesStep will attempt to load both; modprobe handles non-existent modules gracefully (logs errors).
			resolvedForLoading = append(resolvedForLoading, "nf_conntrack_ipv4", "nf_conntrack")
		} else {
			resolvedForLoading = append(resolvedForLoading, m)
		}
	}

	return &spec.TaskSpec{
		Name: "Setup Kubernetes System Prerequisites",
		// This task typically runs on all nodes or specific roles like 'k8s-node'.
		// HostFilter can be applied by the module if needed.
		Steps: []spec.StepSpec{
			// 1. Disable Swap (original preflight step)
			&preflightstep.DisableSwapStepSpec{},

			// 2. Configure SELinux (from sysprep, now in preflight)
			&preflightstep.ConfigureSELinuxStepSpec{},

			// 3. Apply Sysctl Settings (from sysprep, now in preflight)
			&preflightstep.ApplySysctlSettingsStepSpec{},

			// 4. Apply Security Limits (from sysprep, now in preflight)
			&preflightstep.ApplySecurityLimitsStepSpec{},

			// 5. Disable Common Firewalls (from sysprep, now in preflight)
			&preflightstep.DisableFirewallStepSpec{},

			// 6. Load Kernel Modules (original preflight step, configured with resolved list)
			&preflightstep.LoadKernelModulesStepSpec{Modules: resolvedForLoading},

			// 7. Ensure Kernel Modules are Persistent (from sysprep, now in preflight, uses placeholder list)
			&preflightstep.EnsureKernelModulesPersistentStepSpec{Modules: desiredModulesWithPlaceholder},

			// 8. Update /etc/hosts file (from sysprep, now in preflight)
			&preflightstep.UpdateHostsFileStepSpec{},

			// 9. Set IPTables Alternatives to Legacy (from sysprep, now in preflight)
			&preflightstep.SetIPTablesAlternativesStepSpec{},
		},
	}
}
