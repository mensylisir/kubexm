package iscsi

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step/iscsi"
)

// NewISCSIModule creates a module specification for managing iSCSI client tools and services.
func NewISCSIModule(cfg *config.Cluster) *spec.ModuleSpec {

	installAndEnableTask := &spec.TaskSpec{
		Name: "Install and Enable iSCSI Client",
		Steps: []spec.StepSpec{
			&iscsi.InstallISCSIClientPackagesStepSpec{},
			&iscsi.EnableISCSIClientServiceStepSpec{},
		},
	}

	disableAndUninstallTask := &spec.TaskSpec{
		Name: "Disable and Uninstall iSCSI Client",
		Steps: []spec.StepSpec{
			&iscsi.DisableISCSIClientServiceStepSpec{},
			&iscsi.UninstallISCSIClientPackagesStepSpec{},
		},
	}

	// Assemble the tasks for this module.
	iscsiTaskSpecs := []*spec.TaskSpec{
		installAndEnableTask,
		disableAndUninstallTask,
	}

	return &spec.ModuleSpec{
		Name: "iSCSI Client Management",
		IsEnabled: func(clusterCfg *config.Cluster) bool {
			// This function checks if the iSCSI client management is enabled in the cluster configuration.
			// It assumes a structure like:
			// clusterCfg.Spec.ISCSIClient.Managed
			//
			// The actual config.ISCSIClientSpec struct would be defined in pkg/config/config.go
			// type ClusterSpec struct {
			//    ...
			//    ISCSIClient *ISCSIClientSpec `yaml:"iscsiClient,omitempty"`
			//    ...
			// }
			// type ISCSIClientSpec struct {
			//    Managed bool `yaml:"managed,omitempty"`
			// }
			if clusterCfg != nil &&
				clusterCfg.Spec.Other != nil { // TODO: Replace Other with ISCSIClient once defined in config.go
				// This is a temporary placeholder for the actual config check.
				// Replace 'clusterCfg.Spec.Other != nil' with:
				// clusterCfg.Spec.ISCSIClient != nil && clusterCfg.Spec.ISCSIClient.Managed
				// For now, to make it compilable without the actual config change,
				// we'll check a generic existing field or just return false/true.
				// Let's assume for now it's disabled by default until config is ready.
				// To actually test, one might temporarily return true or check another existing boolean field.
				// For the purpose of this step, we are coding against the future config structure.
				// if clusterCfg.Spec.ISCSIClient != nil && clusterCfg.Spec.ISCSIClient.Managed {
				// return true
				// }
			}
			// Default to false if the specific config path is not yet available or not true.
			return false // Placeholder: change this once config.ISCSIClientSpec is implemented
		},
		Tasks:   iscsiTaskSpecs,
		PreRun:  nil,
		PostRun: nil,
	}
}
