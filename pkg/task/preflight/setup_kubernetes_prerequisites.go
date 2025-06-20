package preflight

import (
	// "fmt" // Not strictly needed for this task constructor if names are static
	"github.com/mensylisir/kubexm/pkg/config"
	"github.com/mensylisir/kubexm/pkg/spec"
	// Assuming step definitions are in package preflight, directly accessible.
	// If they were in a sub-package like "preflightstep", an alias would be used.
	// For this structure, direct access is assumed as per previous step implementations.
	"github.com/mensylisir/kubexm/pkg/step/preflight"
)

// NewSetupKubernetesPrerequisitesTask creates a task to set up common Kubernetes system prerequisites.
// The cfg *config.Cluster parameter is included for consistency and potential future use (e.g.,
// if some prerequisites become configurable via cfg).
func NewSetupKubernetesPrerequisitesTask(cfg *config.Cluster) *spec.TaskSpec {

	// Canonical list of desired kernel modules, including a placeholder for nf_conntrack.
	desiredModulesWithPlaceholder := []string{
		"br_netfilter",
		"overlay",
		"ip_vs",
		"ip_vs_rr",
		"ip_vs_wrr",
		"ip_vs_sh",
		"nf_conntrack_placeholder", // Resolved by EnsureKernelModulesPersistentStepSpec's executor
	}

	// Resolve modules for the LoadKernelModulesStepSpec.
	// LoadKernelModulesStep attempts to load all specified modules; modprobe handles non-existent ones.
	resolvedForLoading := []string{}
	for _, m := range desiredModulesWithPlaceholder {
		if m == "nf_conntrack_placeholder" {
			resolvedForLoading = append(resolvedForLoading, "nf_conntrack_ipv4", "nf_conntrack")
		} else {
			resolvedForLoading = append(resolvedForLoading, m)
		}
	}

	return &spec.TaskSpec{
		Name: "Setup Kubernetes System Prerequisites",
		// This task typically runs on all nodes or specific roles like 'k8s-node'.
		// HostFilter can be applied by the module that uses this task if needed.
		Steps: []spec.StepSpec{
			// 1. Disable Swap
			&preflight.DisableSwapStepSpec{},

			// 2. Configure SELinux
			&preflight.ConfigureSELinuxStepSpec{}, // Uses internal PopulateDefaults

			// 3. Apply Sysctl Settings
			&preflight.ApplySysctlSettingsStepSpec{}, // Uses internal PopulateDefaults

			// 4. Apply Security Limits
			&preflight.ApplySecurityLimitsStepSpec{}, // Uses internal PopulateDefaults

			// 5. Disable Common Firewalls
			&preflight.DisableFirewallStepSpec{}, // No parameters in spec

			// 6. Load Kernel Modules
			&preflight.LoadKernelModulesStepSpec{Modules: resolvedForLoading},

			// 7. Ensure Kernel Modules are Persistent
			&preflight.EnsureKernelModulesPersistentStepSpec{Modules: desiredModulesWithPlaceholder}, // Uses internal PopulateDefaults for ConfFilePath

			// 8. Update /etc/hosts file
			&preflight.UpdateHostsFileStepSpec{}, // Uses internal PopulateDefaults

			// 9. Set IPTables Alternatives to Legacy
			&preflight.SetIPTablesAlternativesStepSpec{}, // Uses internal PopulateDefaults
		},
	}
}
