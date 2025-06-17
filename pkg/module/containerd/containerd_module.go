package containerd

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/module"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/task"
	taskContainerd "github.com/kubexms/kubexms/pkg/task/containerd" // Import the actual containerd tasks
	// taskPreflight "github.com/kubexms/kubexms/pkg/task/preflight" // Might depend on some preflight tasks being done
)

// NewContainerdModule creates a module for installing and configuring containerd.
func NewContainerdModule(cfg *config.Cluster) *module.Module {
	return &module.Module{
		Name: "Containerd Runtime",
		IsEnabled: func(clusterCfg *config.Cluster) bool {
			// Example: Enable if containerd is the chosen runtime in config.
			// This requires a config structure like:
			// cfg.Spec.ContainerRuntime.Type == "containerd"
			// For now, assume enabled if this module is included in a pipeline.
			if clusterCfg != nil && clusterCfg.Spec.ContainerRuntime != nil { // Assuming ContainerRuntime field in ClusterSpec
				// Ensure Type field exists and is checked.
				// This is a placeholder for actual config structure.
				// Example: return clusterCfg.Spec.ContainerRuntime.Type == "containerd"
				return true // For now, assume if ContainerRuntime section exists, containerd is implied or default
			}
			// Default to true if not specified, or false if it must be explicit.
			// Let's default to true for this example if the specific config isn't there,
			// implying containerd is the default or only runtime.
			return true
		},
		Tasks: []*task.Task{
			// It's good practice to ensure preflight checks passed before installing components.
			// This dependency is usually handled by pipeline ordering.

			taskContainerd.NewInstallContainerdTask(cfg),
		},
		PreRun: func(cluster *runtime.ClusterRuntime) error {
			if cluster != nil && cluster.Logger != nil {
				cluster.Logger.Infof("Preparing to install and configure containerd...")
			}
			return nil
		},
		PostRun: func(cluster *runtime.ClusterRuntime, moduleErr error) error {
			if cluster != nil && cluster.Logger != nil {
				if moduleErr != nil {
					cluster.Logger.Errorf("Containerd module finished with error: %v", moduleErr)
				} else {
					cluster.Logger.Successf("Containerd module completed successfully.")
				}
			}
			return nil
		},
	}
}

// Placeholder for config structure assumed by NewContainerdModule's IsEnabled
// This should eventually live in pkg/config/config.go
/*
// In pkg/config/config.go eventually:

// Assuming ClusterSpec is already defined
// type ClusterSpec struct {
// 	// ... other fields ...
// 	ContainerRuntime *ContainerRuntimeSpec `yaml:"containerRuntime,omitempty"`
//	Containerd       *ContainerdSpec       `yaml:"containerd,omitempty"` // This was from task example
// }

// Example of how ContainerRuntimeSpec might look
type ContainerRuntimeSpec struct {
    Type string `yaml:"type,omitempty"` // "containerd", "docker", etc.
	// Other common runtime settings like socket path, etc.
	// Specific settings for the chosen type might be in a sub-struct or a generic map.
	Options map[string]interface{} `yaml:"options,omitempty"`
}

// ContainerdSpec (as used in task/containerd/install.go example)
// This might be part of ContainerRuntimeSpec.Options or a direct field if containerd is special.
type ContainerdSpec struct {
	Version            string              `yaml:"version,omitempty"`
	RegistryMirrors    map[string][]string `yaml:"registryMirrors,omitempty"`
	InsecureRegistries []string            `yaml:"insecureRegistries,omitempty"`
	UseSystemdCgroup   *bool               `yaml:"useSystemdCgroup,omitempty"`
	ExtraTomlConfig    string              `yaml:"extraTomlConfig,omitempty"`
	ConfigPath         string              `yaml:"configPath,omitempty"`
}

// A more integrated approach in ClusterSpec:
// type ClusterSpec struct {
//     Hosts  []HostSpec
//     Global GlobalSpec
//     ContainerRuntime struct { // Could be a pointer if optional
//         Type string `yaml:"type"` // e.g., "containerd"
//         Version string `yaml:"version,omitempty"` // Common version field
//         // Containerd-specific settings if type is "containerd"
//         RegistryMirrors    map[string][]string `yaml:"registryMirrors,omitempty"`
//         InsecureRegistries []string            `yaml:"insecureRegistries,omitempty"`
//         UseSystemdCgroup   *bool               `yaml:"useSystemdCgroup,omitempty"` // Use pointer for explicit true/false
//         ExtraTomlConfig    string              `yaml:"extraTomlConfig,omitempty"`
//     } `yaml:"containerRuntime"`
//     // ... other specs like Etcd, Kubernetes ...
// }

*/
