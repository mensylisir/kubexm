package containerd

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	taskContainerd "github.com/kubexms/kubexms/pkg/task/containerd"
)

// NewContainerdModule creates a module specification for installing and configuring containerd.
func NewContainerdModule(cfg *config.Cluster) *spec.ModuleSpec {
	return &spec.ModuleSpec{
		Name: "Containerd Runtime",
		IsEnabled: func(clusterCfg *config.Cluster) bool {
			// Enable if containerd is the chosen runtime, or if no runtime is specified (defaulting to containerd).
			if clusterCfg == nil || clusterCfg.Spec.ContainerRuntime == nil {
				// If ContainerRuntime spec itself is missing, assume default (containerd) is desired.
				return true
			}
			// If ContainerRuntime spec exists, check its Type.
			// Empty Type also implies default (containerd).
			return clusterCfg.Spec.ContainerRuntime.Type == "containerd" || clusterCfg.Spec.ContainerRuntime.Type == ""
		},
		Tasks: []*spec.TaskSpec{
			taskContainerd.NewInstallContainerdTask(cfg), // Pass cfg
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
