package containerd

import (
	"github.com/mensylisir/kubexm/pkg/config"
	"github.com/mensylisir/kubexm/pkg/spec"
	commonstep "github.com/mensylisir/kubexm/pkg/step/common" // Added import
	stepContainerd "github.com/mensylisir/kubexm/pkg/step/containerd"
)

// NewInstallContainerdTask creates a task specification to install and configure containerd.
func NewInstallContainerdTask(cfg *config.Cluster) *spec.TaskSpec {

	var containerdVersion string // Corrected declaration
	registryMirrors := make(map[string]string)
	insecureRegistries := []string{}
	useSystemdCgroup := true // Default if no config is found, but SetDefaults should handle this.
	extraToml := ""
	configPath := ""

	if cfg != nil && cfg.Spec.ContainerRuntime != nil { // Ensure ContainerRuntime exists
		if cfg.Spec.ContainerRuntime.Version != "" {
			containerdVersion = cfg.Spec.ContainerRuntime.Version
		}
	}

	if cfg != nil && cfg.Spec.Containerd != nil { // Ensure Containerd config exists
		// If ContainerdConfig.Version is set, it overrides ContainerRuntime.Version for containerd
		if cfg.Spec.Containerd.Version != "" {
			containerdVersion = cfg.Spec.Containerd.Version
		}

		// Process RegistryMirrors
		if cfg.Spec.Containerd.RegistryMirrors != nil {
			for reg, mirrors := range cfg.Spec.Containerd.RegistryMirrors {
				if len(mirrors) > 0 {
					registryMirrors[reg] = mirrors[0]
				}
			}
		}

		// UseSystemdCgroup should be defaulted to true by SetDefaults_ContainerdConfig
		// if the Containerd section is present in YAML (even if empty or field omitted).
		// If Containerd section is entirely absent, SetDefaults_Cluster creates ContainerdConfig,
		// and SetDefaults_ContainerdConfig should ensure UseSystemdCgroup is true.
		useSystemdCgroup = cfg.Spec.Containerd.UseSystemdCgroup

		if len(cfg.Spec.Containerd.InsecureRegistries) > 0 {
			insecureRegistries = cfg.Spec.Containerd.InsecureRegistries
		}
		extraToml = cfg.Spec.Containerd.ExtraTomlConfig
		if cfg.Spec.Containerd.ConfigPath != "" {
			configPath = cfg.Spec.Containerd.ConfigPath
		}
	} else if cfg != nil {
		// cfg.Spec.Containerd is nil, but cfg itself and cfg.Spec might not be.
		// This implies that SetDefaults_Cluster should have initialized cfg.Spec.Containerd.
		// If it's still nil here, it's an unexpected state if the module is enabled.
		// However, for safety, retain the local default for useSystemdCgroup.
		// The version would remain empty or from ContainerRuntime if that was non-nil.
		// This block might indicate that the IsEnabled logic for the module needs to be robust.
		// For now, useSystemdCgroup remains true (its initial value).
	}


	return &spec.TaskSpec{
		Name: "Install and Configure Containerd",
		RunOnRoles: []string{}, // Assuming it runs on all hosts passed to it, or roles are filtered by module.
		Steps: []spec.StepSpec{
			// Note: A Download/Fetch step for containerd of 'containerdVersion' would typically precede this.
			// InstallContainerdStepSpec expects an already extracted archive path.
			&stepContainerd.InstallContainerdStepSpec{
				// Version field removed as it's not part of InstallContainerdStepSpec
				SourceExtractedPathSharedDataKey: commonstep.DefaultExtractedPathKey,
			},
			&stepContainerd.ConfigureContainerdStepSpec{
				RegistryMirrors:    registryMirrors,
				UseSystemdCgroup:   useSystemdCgroup,
				InsecureRegistries: insecureRegistries,
				ExtraTomlContent:   extraToml,
				ConfigFilePath:     configPath,
			},
			&stepContainerd.EnableAndStartContainerdStepSpec{},
		},
		Concurrency: 10,
		IgnoreError: false,
	}
}
