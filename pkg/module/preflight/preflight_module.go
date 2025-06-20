package preflight

import (
	// "github.com/kubexms/kubexms/pkg/config" // No longer used
	"github.com/kubexms/kubexms/pkg/runtime" // For ClusterRuntime
	"github.com/kubexms/kubexms/pkg/apis/kubexms/v1alpha1" // For v1alpha1.Cluster type
	"github.com/kubexms/kubexms/pkg/spec"
	taskPreflight "github.com/kubexms/kubexms/pkg/task/preflight"
)

// NewPreflightModule creates a new module specification for preflight checks and setup.
func NewPreflightModule(clusterRt *runtime.ClusterRuntime) *spec.ModuleSpec {
	if clusterRt == nil || clusterRt.ClusterConfig == nil {
		return &spec.ModuleSpec{
			Name:      "Preflight Checks and Setup (Error: Missing Configuration)",
			IsEnabled: func(_ *runtime.ClusterRuntime) bool { return false },
			Tasks:     []*spec.TaskSpec{},
		}
	}
	cfg := clusterRt.ClusterConfig // cfg is *v1alpha1.Cluster

	return &spec.ModuleSpec{
		Name: "Preflight Checks and Setup",
		IsEnabled: func(cr *runtime.ClusterRuntime) bool {
			if cr == nil || cr.ClusterConfig == nil || cr.ClusterConfig.Spec.Global == nil {
				// If Global spec is missing, SkipPreflight is effectively false (module enabled).
				// SetDefaults_Cluster ensures Global is initialized, so Global should not be nil here.
				return true
			}
			// Module is enabled by default.
			// It's disabled if explicitly told to skip preflight checks in global config.
			return !cr.ClusterConfig.Spec.Global.SkipPreflight
		},
		Tasks: []*spec.TaskSpec{
			taskPreflight.NewSystemChecksTask(cfg), // Pass cfg to task factories
			taskPreflight.NewSetupKernelTask(cfg),   // Pass cfg to task factories
		},
		PreRun: nil,
		PostRun: nil,
	}
}
