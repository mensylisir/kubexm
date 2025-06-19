package iscsi

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	taskISCSI "github.com/kubexms/kubexms/pkg/task/iscsi" // Import for new task constructors
	// No direct step imports needed if tasks encapsulate all steps
)

// NewISCSIModule creates a module specification for managing iSCSI client tools and services.
func NewISCSIModule(cfg *config.Cluster) *spec.ModuleSpec {

	allTasks := []*spec.TaskSpec{}

	// Install and Enable iSCSI Client Task
	installTask := taskISCSI.NewInstallAndEnableISCSITask(cfg)
	if installTask != nil {
		allTasks = append(allTasks, installTask)
	}

	// Disable and Uninstall iSCSI Client Task
	uninstallTask := taskISCSI.NewDisableAndUninstallISCSITask(cfg)
	if uninstallTask != nil {
		allTasks = append(allTasks, uninstallTask)
	}

	return &spec.ModuleSpec{
		Name: "iSCSI Client Management",
		IsEnabled: func(clusterCfg *config.Cluster) bool {
			// This function checks if iSCSI client management is enabled.
			// It should ideally check a specific field in the cluster configuration.
			// Example:
			// if clusterCfg != nil &&
			//    clusterCfg.Spec.Features != nil &&
			//    clusterCfg.Spec.Features.ISCSIClient != nil &&
			//    clusterCfg.Spec.Features.ISCSIClient.Managed {
			//     return true
			// }
			// return false
			// For now, returning true to assume it's managed if module is included,
			// or until a proper config field is established.
			// A more realistic default might be based on whether any iSCSI-dependent features are enabled.
			// Let's use a hypothetical config path for now as per the original plan.
			if clusterCfg != nil &&
				clusterCfg.Spec.ISCSIClient != nil && // Assumes config.ISCSIClientSpec exists
				clusterCfg.Spec.ISCSIClient.Managed {
				return true
			}
			return false // Default to false if not explicitly managed
		},
		Tasks:   allTasks,
		PreRun:  nil,
		PostRun: nil,
	}
}
