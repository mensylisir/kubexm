package iscsi

import (
	"github.com/mensylisir/kubexm/pkg/config" // For config.Cluster
	"github.com/mensylisir/kubexm/pkg/spec"
	taskISCSIFactory "github.com/mensylisir/kubexm/pkg/task/iscsi" // Alias for task spec factories
)

// NewISCSIModuleSpec creates a module specification for managing iSCSI client tools and services.
func NewISCSIModuleSpec(cfg *config.Cluster) *spec.ModuleSpec {
	if cfg == nil {
		return &spec.ModuleSpec{
			Name:        "iSCSI Client Management",
			Description: "Manages iSCSI client tools and services (Error: Missing Configuration)",
			IsEnabled:   "false",
			Tasks:       []*spec.TaskSpec{},
		}
	}

	allTasks := []*spec.TaskSpec{}

	// Assuming NewInstallAndEnableISCSITaskSpec and NewDisableAndUninstallISCSITaskSpec
	// are the new factory names and they return *spec.TaskSpec.
	// If their signatures require more parameters than just cfg, this will need adjustment.

	// Install and Enable iSCSI Client Task
	// TODO: Replace with actual factory call if NewInstallAndEnableISCSITaskSpec exists and is different
	installTaskSpec := taskISCSIFactory.NewInstallAndEnableISCSITaskSpec(cfg)
	if installTaskSpec != nil {
		allTasks = append(allTasks, installTaskSpec)
	}

	// Disable and Uninstall iSCSI Client Task
	// TODO: Replace with actual factory call if NewDisableAndUninstallISCSITaskSpec exists and is different
	uninstallTaskSpec := taskISCSIFactory.NewDisableAndUninstallISCSITaskSpec(cfg)
	if uninstallTaskSpec != nil {
		allTasks = append(allTasks, uninstallTaskSpec)
	}

	// The IsEnabled condition string. Currently, it's hardcoded to "false"
	// based on the TODO in the original code.
	// If iSCSI configuration were added to config.Cluster (e.g., cfg.Spec.Storage.ISCSI.Enabled),
	// this would be something like "cfg.Spec.Storage.ISCSI.Enabled == true".
	isEnabledCondition := "false" // Defaulting to disabled as per original TODO

	return &spec.ModuleSpec{
		Name:        "iSCSI Client Management",
		Description: "Manages iSCSI client tools and services. Currently disabled by default.",
		IsEnabled:   isEnabledCondition,
		Tasks:       allTasks,
		PreRunHook:  "",
		PostRunHook: "",
	}
}
