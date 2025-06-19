package containerd

import (
	"github.com/kubexms/kubexms/pkg/config"
	"github.com/kubexms/kubexms/pkg/spec"
	stepContainerd "github.com/kubexms/kubexms/pkg/step/containerd"
)

// NewInstallContainerdTask creates a task specification to install and configure containerd.
func NewInstallContainerdTask(cfg *config.Cluster) *spec.TaskSpec {

	containerdVersion := ""
	registryMirrors := make(map[string]string)
	insecureRegistries := []string{}
	// Default to true for Kubernetes, can be overridden by config.
	// If cfg.Spec.Containerd is nil, or if UseSystemdCgroup is not set (defaults to false for bool),
	// this factory default of 'true' will apply.
	useSystemdCgroup := true
	extraToml := ""
	configPath := "" // Step uses its own default if this is empty

	if cfg != nil {
		if cfg.Spec.ContainerRuntime != nil && cfg.Spec.ContainerRuntime.Version != "" {
		containerdVersion = cfg.Spec.ContainerRuntime.Version
		}

		if cfg.Spec.Containerd != nil { // ContainerdSpec is a pointer, so check for nil
			// If ContainerdSpec.Version is set, it overrides ContainerRuntime.Version for containerd
			if cfg.Spec.Containerd.Version != "" {
				containerdVersion = cfg.Spec.Containerd.Version
			}
			// Process RegistryMirrors: step takes map[string]string (first mirror for each registry)
			if cfg.Spec.Containerd.RegistryMirrors != nil { // Was RegistryMirrorsConfig
			for reg, mirrors := range cfg.Spec.Containerd.RegistryMirrors {
				if len(mirrors) > 0 {
				registryMirrors[reg] = mirrors[0]
				}
			}
			}
			// UseSystemdCgroup: if the ContainerdSpec section exists, its UseSystemdCgroup value
			// (which is 'false' if omitted in YAML due to bool type) will be used.
			// If we want 'true' to be the default unless explicitly set to 'false' in config:
			// This requires UseSystemdCgroup to be a *bool in config.ContainerdSpec.
			// Given current config.ContainerdSpec.UseSystemdCgroup is bool:
			useSystemdCgroup = cfg.Spec.Containerd.UseSystemdCgroup
			// If this results in `false` because it was omitted in YAML, and `true` is desired as default,
			// the logic should be:
			// if cfg.Spec.Containerd.IsSet("UseSystemdCgroup") { useSystemdCgroup = cfg.Spec.Containerd.UseSystemdCgroup } else { useSystemdCgroup = true }
			// For now, we take the value as is from config if ContainerdSpec is present.
			// The initial `useSystemdCgroup := true` above acts as a default if cfg.Spec.Containerd is nil.

			if len(cfg.Spec.Containerd.InsecureRegistries) > 0 {
				insecureRegistries = cfg.Spec.Containerd.InsecureRegistries
			}
			extraToml = cfg.Spec.Containerd.ExtraTomlConfig
			if cfg.Spec.Containerd.ConfigPath != "" {
				configPath = cfg.Spec.Containerd.ConfigPath
			}
		}
	}


	return &spec.TaskSpec{
		Name: "Install and Configure Containerd",
		RunOnRoles: []string{}, // Runs on hosts selected by the module using this task
		Steps: []spec.StepSpec{
			// Assumes containerd binaries are already downloaded and extracted by a previous task
			// (e.g., taskKubeComponents.NewFetchContainerdTask) and the path to the
			// extracted 'bin' directory is in the Task Cache under common.DefaultExtractedPathKey
			// or a more specific key like component_downloads.DefaultContainerdExtractedDirKey.
			// For this example, let's assume the FetchContainerdTask from kube_components
			// used commonstep.DefaultExtractedPathKey for the root of the extracted archive.
			// The InstallContainerdStepSpec will need to be adjusted to find binaries within that,
			// potentially looking into a 'bin' subdirectory or taking multiple SourceFileNames.
			//
			// A simpler model if InstallContainerdStepSpec is very basic (e.g. just runs 'make install'
			// from the root of extracted source):
			// &stepContainerd.InstallContainerdStepSpec{
			//	 Version: containerdVersion, // For reference or naming systemd files
			//	 SourceBuildPathSharedDataKey: "ContainerdSourceBuildPath", // If built from source
			// },
			//
			// If installing pre-compiled binaries from the extracted archive (more likely):
			// The InstallContainerdStepSpec would need to be more like InstallBinaryStep,
			// or this task should directly use multiple common.InstallBinaryStep instances.
			//
			// Given the plan, InstallContainerdStepSpec is for installing from an already extracted location.
			// It needs a key to find this location. Let's use a conceptual key that FetchContainerdTask would set.
			// In fetch_containerd.go, we used `containerdExtractedDirOutputKey := commonstep.DefaultExtractedPathKey`.
			// So, InstallContainerdStepSpec needs to read from this key.
			&stepContainerd.InstallContainerdStepSpec{
				Version:                        containerdVersion, // May be used for version-specific logic or naming
				SourceExtractedPathSharedDataKey: commonstep.DefaultExtractedPathKey, // Key where path to extracted files is stored
				// TargetBinDir, etc., would be set to defaults within InstallContainerdStepSpec or configurable.
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
