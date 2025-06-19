package preflight

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	taskPreflight "github.com/kubexms/kubexms/pkg/task/preflight"
)

// NewPreflightModule creates a new module specification for preflight checks and setup.
func NewPreflightModule(cfg *config.Cluster) *spec.ModuleSpec {
	return &spec.ModuleSpec{
		Name: "Preflight Checks and Setup",
		IsEnabled: func(clusterCfg *config.Cluster) bool { // clusterCfg is *v1alpha1.Cluster
			// Module is enabled by default.
			// It's disabled if explicitly told to skip preflight checks in global config.
			if clusterCfg != nil && clusterCfg.Spec.Global != nil && clusterCfg.Spec.Global.SkipPreflight {
				return false // SkipPreflight is true, so module is disabled.
			}
			return true // Enabled by default, if Global is nil, or if SkipPreflight is false.
		},
		Tasks: []*spec.TaskSpec{
			taskPreflight.NewSystemChecksTask(cfg), // Pass cfg to task factories
			taskPreflight.NewSetupKernelTask(cfg),   // Pass cfg to task factories
		},
		PreRun: nil,
		PostRun: nil,
	}
}
