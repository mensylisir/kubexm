package containerd

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	stepContainerd "github.com/kubexms/kubexms/pkg/step/containerd" // Import containerd step specs
	// "github.com/kubexms/kubexms/pkg/task" // No longer needed
)

// NewInstallContainerdTask creates a task specification to install and configure containerd.
// cfg can be used to specify containerd version, mirror settings, etc.
func NewInstallContainerdTask(cfg *config.Cluster) *spec.TaskSpec {

	// Default values, to be potentially overridden by cfg
	containerdVersion := "" // Default: install latest available version
	registryMirrors := map[string]string{}
	insecureRegistries := []string{}
	useSystemdCgroup := true // Common best practice for Kubernetes
	extraToml := ""
	configPath := "" // Use default in step if empty

	// Example of reading from a hypothetical config structure (adjust to actual config.go structure)
	// if cfg != nil && cfg.Spec.ContainerRuntime != nil && cfg.Spec.ContainerRuntime.Type == "containerd" {
	//    if cfg.Spec.ContainerRuntime.Version != "" {
	//        containerdVersion = cfg.Spec.ContainerRuntime.Version
	//    }
	//    if cfg.Spec.ContainerRuntime.Containerd != nil { // Assuming a nested ContainerdSpec
	//        if len(cfg.Spec.ContainerRuntime.Containerd.RegistryMirrors) > 0 {
	//             // Logic to convert map[string][]string to map[string]string (taking first mirror)
	//             for reg, mirrors := range cfg.Spec.ContainerRuntime.Containerd.RegistryMirrors {
	//                 if len(mirrors) > 0 {
	//                     registryMirrors[reg] = mirrors[0]
	//                 }
	//             }
	//        }
	//        if len(cfg.Spec.ContainerRuntime.Containerd.InsecureRegistries) > 0 {
	//            insecureRegistries = cfg.Spec.ContainerRuntime.Containerd.InsecureRegistries
	//        }
	//        if cfg.Spec.ContainerRuntime.Containerd.UseSystemdCgroup != nil {
	//            useSystemdCgroup = *cfg.Spec.ContainerRuntime.Containerd.UseSystemdCgroup
	//        }
	//        extraToml = cfg.Spec.ContainerRuntime.Containerd.ExtraTomlConfig
	//        configPath = cfg.Spec.ContainerRuntime.Containerd.ConfigPath
	//    }
	// }


	return &spec.TaskSpec{
		Name: "Install and Configure Containerd",
		RunOnRoles: []string{}, // Typically all nodes needing a runtime
		Steps: []spec.StepSpec{
			&stepContainerd.InstallContainerdStepSpec{
				Version: containerdVersion,
			},
			// Assuming ConfigureContainerdMirrorStepSpec was renamed/generalized to ConfigureContainerdStepSpec
			&stepContainerd.ConfigureContainerdStepSpec{
				RegistryMirrors:    registryMirrors,
				InsecureRegistries: insecureRegistries,
				UseSystemdCgroup:   useSystemdCgroup,
				ExtraTomlContent:   extraToml,
				ConfigFilePath:     configPath,
			},
			&stepContainerd.EnableAndStartContainerdStepSpec{},
		},
		Concurrency: 10, // Default concurrency for this task
		IgnoreError: false, // Containerd installation is usually critical
	}
}

// Note: The actual config structure (e.g., cfg.Spec.ContainerRuntime.Containerd) needs to be
// defined in pkg/config/config.go for the commented-out configuration loading examples to work.
// The step spec struct stepContainerd.ConfigureContainerdStepSpec must match the name
// used here (previously it might have been ConfigureContainerdMirrorStepSpec).
