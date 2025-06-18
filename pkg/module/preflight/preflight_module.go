package preflight

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step/preflight" // Direct import for step specs
	taskPreflight "github.com/kubexms/kubexms/pkg/task/preflight"
)

// NewPreflightModule creates a new module specification for preflight checks and setup.
func NewPreflightModule(cfg *config.Cluster) *spec.ModuleSpec {
	// Define the list of kernel modules to be loaded by preflight.LoadKernelModulesStepSpec.
	defaultModulesForLoading := []string{
		"br_netfilter",
		"overlay",
		"ip_vs",
		"ip_vs_rr",
		"ip_vs_wrr",
		"ip_vs_sh",
		"nf_conntrack_ipv4", // Attempt this first
		"nf_conntrack",      // Fallback
	}

	// New comprehensive task for Kubernetes prerequisites setup
	setupKubernetesPrerequisitesTask := &spec.TaskSpec{
		Name: "Setup Kubernetes Prerequisites",
		Steps: []spec.StepSpec{
			// 1. Disable Swap
			&preflight.DisableSwapStepSpec{},

			// 2. Configure SELinux
			&preflight.ConfigureSELinuxStepSpec{}, // Relies on its own PopulateDefaults

			// 3. Apply Sysctl Settings
			&preflight.ApplySysctlSettingsStepSpec{}, // Relies on its own PopulateDefaults

			// 4. Apply Security Limits
			&preflight.ApplySecurityLimitsStepSpec{}, // Relies on its own PopulateDefaults

			// 5. Disable Common Firewalls
			&preflight.DisableFirewallStepSpec{}, // Parameter-less

			// 6. Load Kernel Modules
			&preflight.LoadKernelModulesStepSpec{Modules: defaultModulesForLoading},

			// 7. Ensure Kernel Modules are Persistent
			// This step uses its internal PopulateDefaults to determine the module list,
			// including resolving "nf_conntrack_placeholder".
			&preflight.EnsureKernelModulesPersistentStepSpec{},

			// 8. Update /etc/hosts file
			&preflight.UpdateHostsFileStepSpec{}, // Relies on its own PopulateDefaults

			// 9. Set IPTables Alternatives to Legacy
			&preflight.SetIPTablesAlternativesStepSpec{}, // Relies on its own PopulateDefaults
		},
	}

	// Existing tasks (if any)
	existingTasks := []*spec.TaskSpec{
		taskPreflight.NewSystemChecksTask(cfg), // Pass cfg to task factories
		taskPreflight.NewSetupKernelTask(cfg),   // Pass cfg to task factories
	}

	allTasks := append(existingTasks, setupKubernetesPrerequisitesTask)

	return &spec.ModuleSpec{
		Name: "Preflight Checks and Setup",
		IsEnabled: func(clusterCfg *config.Cluster) bool {
			// Module is enabled by default.
			// It's disabled if explicitly told to skip preflight checks in global config.
			if clusterCfg != nil && clusterCfg.Spec.Global.SkipPreflight {
				return false // SkipPreflight is true, so module is disabled.
			}
			return true // Enabled by default or if SkipPreflight is false.
		},
		Tasks:   allTasks,
		PreRun:  nil,
		PostRun: nil,
	}
}
