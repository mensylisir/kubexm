package containerd

import (
	// "github.com/kubexms/kubexms/pkg/config" // No longer used directly
	"github.com/mensylisir/kubexm/pkg/runtime" // For ClusterRuntime
	"github.com/mensylisir/kubexm/pkg/spec"
	taskContainerd "github.com/mensylisir/kubexm/pkg/task/containerd"
)

// NewContainerdModule creates a module specification for installing and configuring containerd.
func NewContainerdModule(clusterRt *runtime.ClusterRuntime) *spec.ModuleSpec {
	if clusterRt == nil || clusterRt.ClusterConfig == nil {
		// This case indicates an issue with pipeline setup or runtime initialization.
		// Return a disabled module or log an error.
		return &spec.ModuleSpec{
			Name:      "Containerd Runtime (Error: Missing Configuration)",
			IsEnabled: func(_ *runtime.ClusterRuntime) bool { return false },
			Tasks:     []*spec.TaskSpec{},
		}
	}
	cfg := clusterRt.ClusterConfig // cfg is *v1alpha1.Cluster

	return &spec.ModuleSpec{
		Name: "Containerd Runtime",
		IsEnabled: func(cr *runtime.ClusterRuntime) bool {
			if cr == nil || cr.ClusterConfig == nil || cr.ClusterConfig.Spec.ContainerRuntime == nil {
				// If ContainerRuntime section is entirely absent from YAML,
				// SetDefaults_Cluster creates it, and SetDefaults_ContainerRuntimeConfig
				// would set its Type to "containerd". So this module should run.
				return true
			}
			// If ContainerRuntime section exists, its Type field would have been defaulted to "containerd"
			// if it was initially empty. So, we just check if it's "containerd".
			return cr.ClusterConfig.Spec.ContainerRuntime.Type == "containerd"
		},
		Tasks: []*spec.TaskSpec{
			taskContainerd.NewInstallContainerdTask(cfg),
		},
		PreRun: nil,
		PostRun: nil,
	}
}

// Placeholder for config structure assumed by NewContainerdModule's IsEnabled
// This should eventually live in pkg/config/config.go
/*
// In pkg/config/config.go eventually:

type ClusterSpec struct {
	// ... other fields ...
	ContainerRuntime *ContainerRuntimeSpec `yaml:"containerRuntime,omitempty"`
}

type ContainerRuntimeSpec struct {
    Type string `yaml:"type,omitempty"` // "containerd", "docker", etc.
}
*/
