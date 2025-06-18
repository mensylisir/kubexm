package containerd

import (
	"github.com/kubexms/kubexms/pkg/config"
	// "github.com/kubexms/kubexms/pkg/runtime" // No longer needed for PreRun/PostRun func signatures
	"github.com/kubexms/kubexms/pkg/spec"
	taskContainerd "github.com/kubexms/kubexms/pkg/task/containerd"
	// "github.com/kubexms/kubexms/pkg/module" // No longer needed
	// commandStepSpec "github.com/kubexms/kubexms/pkg/step/command" // If hooks were simple commands
)

// NewContainerdModule creates a module specification for installing and configuring containerd.
func NewContainerdModule(cfg *config.Cluster) *spec.ModuleSpec {
	return &spec.ModuleSpec{
		Name: "Containerd Runtime",
		IsEnabled: func(clusterCfg *config.Cluster) bool {
			// Example of how IsEnabled could depend on config:
			// This assumes a structure like cfg.Spec.ContainerRuntime.Type
			// The actual structure needs to be defined in pkg/config/config.go
			// if clusterCfg != nil && clusterCfg.Spec.ContainerRuntime != nil {
			// 	return clusterCfg.Spec.ContainerRuntime.Type == "containerd"
			// }
			// Defaulting to true if no specific config is found, implying containerd is default/always installed by this module.
			return true
		},
		Tasks: []*spec.TaskSpec{
			taskContainerd.NewInstallContainerdTask(cfg),
			// Other containerd related tasks can be added here.
			// For example, a task to preload images:
			// taskContainerd.NewPreloadImagesTask(cfg),
		},
		// PreRun and PostRun hooks are now spec.StepSpec types.
		// If they were simple logging as before, they'd be removed or converted to CommandStepSpec.
		// For this refactor, setting to nil.
		PreRun:  nil,
		PostRun: nil,
	}
}

// Placeholder for config structure that might be used by IsEnabled or tasks.
// This would ultimately reside in pkg/config.
/*
type ClusterSpec struct {
	// ... other fields ...
	ContainerRuntime *ContainerRuntimeSpec `yaml:"containerRuntime,omitempty"`
}

type ContainerRuntimeSpec struct {
    Type string `yaml:"type,omitempty"` // e.g., "containerd", "docker"
    // ... other common runtime settings ...
}
*/
