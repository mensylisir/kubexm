package iscsi

import (
	// "github.com/kubexms/kubexms/pkg/config" // No longer used
	"github.com/mensylisir/kubexm/pkg/runtime" // For ClusterRuntime
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1" // For v1alpha1.Cluster type
	"github.com/mensylisir/kubexm/pkg/spec"
	taskISCSI "github.com/mensylisir/kubexm/pkg/task/iscsi" // Import for new task constructors
	// No direct step imports needed if tasks encapsulate all steps
)

// NewISCSIModule creates a module specification for managing iSCSI client tools and services.
func NewISCSIModule(clusterRt *runtime.ClusterRuntime) *spec.ModuleSpec {
	if clusterRt == nil || clusterRt.ClusterConfig == nil {
		return &spec.ModuleSpec{
			Name:      "iSCSI Client Management (Error: Missing Configuration)",
			IsEnabled: func(_ *runtime.ClusterRuntime) bool { return false },
			Tasks:     []*spec.TaskSpec{},
		}
	}
	cfg := clusterRt.ClusterConfig // cfg is *v1alpha1.Cluster

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
		IsEnabled: func(cr *runtime.ClusterRuntime) bool {
			// TODO: This module is currently disabled as v1alpha1.ClusterSpec
			// does not have a dedicated iSCSI client management configuration section.
			// To enable this module, add appropriate fields to v1alpha1.ClusterSpec
			// (e.g., Spec.Storage.ISCSIClient.Enabled or similar) and update this logic.
			// Also, ensure cr and cr.ClusterConfig are not nil if used.
			return false
		},
		Tasks:   allTasks,
		PreRun:  nil,
		PostRun: nil,
	}
}
