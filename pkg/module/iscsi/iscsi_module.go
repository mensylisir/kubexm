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
		IsEnabled: func(clusterCfg *config.Cluster) bool { // clusterCfg is *v1alpha1.Cluster
			// TODO: This module is currently disabled as v1alpha1.ClusterSpec
			// does not have a dedicated iSCSI client management configuration section.
			// To enable this module, add appropriate fields to v1alpha1.ClusterSpec
			// (e.g., Spec.Storage.ISCSIClient.Enabled or similar) and update this logic.
			return false
		},
		Tasks:   allTasks,
		PreRun:  nil,
		PostRun: nil,
	}
}
