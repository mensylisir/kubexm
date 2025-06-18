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
			if clusterCfg == nil || clusterCfg.Spec.ContainerRuntime == nil {
				// If no ContainerRuntime spec, it implies defaults are used.
				// SetDefaults sets ContainerRuntime.Type to "containerd" if it's empty.
				// So, if this section is missing entirely, it will default to containerd and be enabled.
				// However, if the user *explicitly* sets a different runtime type, this module should be disabled.
				// This function relies on SetDefaults having run on clusterCfg.
				return true // Default to enabled if no explicit runtime type is set (defaults will make it containerd)
			}
			// If ContainerRuntime.Type is explicitly set, enable only if it's "containerd".
			return clusterCfg.Spec.ContainerRuntime.Type == "containerd"
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
