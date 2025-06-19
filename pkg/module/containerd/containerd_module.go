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
		IsEnabled: func(clusterCfg *config.Cluster) bool { // clusterCfg is *v1alpha1.Cluster
			if clusterCfg == nil {
				return true // Should not happen if called by runtime with valid cfg
			}
			if clusterCfg.Spec.ContainerRuntime == nil {
				// If ContainerRuntime section is entirely absent from YAML,
				// SetDefaults_Cluster creates it, and SetDefaults_ContainerRuntimeConfig
				// would set its Type to "containerd". So this module should run.
				return true
			}
			// If ContainerRuntime section exists, its Type field would have been defaulted to "containerd"
			// if it was initially empty. So, we just check if it's "containerd".
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
