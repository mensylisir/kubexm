package preflight

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step/preflight" // Direct import for step specs
	taskPreflight "github.com/kubexms/kubexms/pkg/task/preflight"
)

// NewPreflightModule creates a new module specification for preflight checks and setup.
func NewPreflightModule(cfg *config.Cluster) *spec.ModuleSpec {

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
	resolvedForLoading := []string{}
	for _, m := range desiredModulesWithPlaceholder {
		if m == "nf_conntrack_placeholder" {
			resolvedForLoading = append(resolvedForLoading, "nf_conntrack_ipv4", "nf_conntrack")
		} else {
			resolvedForLoading = append(resolvedForLoading, m)
		}
	}

	// Task for setting up common Kubernetes prerequisites.
	// Each step spec within this task relies on its internal PopulateDefaults()
	// to set its specific parameters based on the system or common defaults.
	setupKubernetesPrerequisitesTask := &spec.TaskSpec{
		Name: "Setup Kubernetes System Prerequisites", // Renamed for clarity
		Steps: []spec.StepSpec{
			// 1. Disable Swap (this is an existing step in preflight, not the one from sysprep)
			&preflight.DisableSwapStepSpec{},

			// 2. Configure SELinux
			&preflight.ConfigureSELinuxStepSpec{},

			// 3. Apply Sysctl Settings
			&preflight.ApplySysctlSettingsStepSpec{},

			// 4. Apply Security Limits
			&preflight.ApplySecurityLimitsStepSpec{},

			// 5. Disable Common Firewalls
			&preflight.DisableFirewallStepSpec{},

			// 6. Load Kernel Modules (attempts to load resolved names)
			&preflight.LoadKernelModulesStepSpec{Modules: resolvedForLoading},

			// 7. Ensure Kernel Modules are Persistent (will resolve placeholder internally)
			&preflight.EnsureKernelModulesPersistentStepSpec{Modules: desiredModulesWithPlaceholder},

			// 8. Update /etc/hosts file
			&preflight.UpdateHostsFileStepSpec{},

			// 9. Set IPTables Alternatives to Legacy
			&preflight.SetIPTablesAlternativesStepSpec{},
		},
	}

	// Start with existing tasks (if any, like system checks from task factories)
	tasks := []*spec.TaskSpec{
		// These factory functions might create tasks with steps that are now covered by
		// setupKubernetesPrerequisitesTask. This might lead to redundancy.
		// For this refactoring, we are primarily focused on integrating the new task.
		// A later review could consolidate steps from these factory-generated tasks
		// if they overlap with SetupKubernetesPrerequisitesTask.
		taskPreflight.NewSystemChecksTask(cfg), // Example: checks CPU, memory
		taskPreflight.NewSetupKernelTask(cfg),   // Example: might do some kernel setup, potentially overlapping module loading
	}

	// Conditionally add the new comprehensive prerequisites task.
	// TODO: Implement cfg.Spec.Preflight.EnableKubernetesPrerequisites (or similar) in config.ClusterSpec
	//       and use it here to make this task's inclusion truly conditional.
	//       For example:
	//       enableK8sPrerequisites := false
	//       if cfg.Spec.Preflight != nil && cfg.Spec.Preflight.EnableKubernetesPrerequisites {
	//           enableK8sPrerequisites = true
	//       }
	//       For now, unconditionally adding the task for testing/development.
	enableK8sPrerequisites := true // Placeholder: Default to true for now
	// if cfg.Spec.Preflight != nil && cfg.Spec.Preflight.EnableKubernetesPrerequisites { // Example of actual check
	// 	enableK8sPrerequisites = true
	// } else if cfg.Spec.Preflight == nil { // if Preflight spec part is missing, maybe default to true or false based on desired behavior
	//    enableK8sPrerequisites = true // Defaulting to true if spec section is absent
	// }


	if enableK8sPrerequisites {
		tasks = append(tasks, setupKubernetesPrerequisitesTask)
	}

	return &spec.ModuleSpec{
		Name: "Preflight Checks and Setup",
		IsEnabled: func(clusterCfg *config.Cluster) bool {
			// Module is enabled by default.
			// It's disabled if explicitly told to skip preflight checks in global config.
			if clusterCfg != nil && clusterCfg.Spec.Global != nil && clusterCfg.Spec.Global.SkipPreflight {
				return false // SkipPreflight is true, so module is disabled.
			}
			return true // Enabled by default or if SkipPreflight is false.
		},
		Tasks:   tasks,
		PreRun:  nil,
		PostRun: nil,
	}
}
