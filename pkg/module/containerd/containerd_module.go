package containerd

import (
	"github.com/mensylisir/kubexm/pkg/config" // For config.Cluster
	"github.com/mensylisir/kubexm/pkg/spec"
	taskContainerdFactory "github.com/mensylisir/kubexm/pkg/task/containerd" // Alias for task spec factories
)

// NewContainerdModuleSpec creates a module specification for installing and configuring containerd.
func NewContainerdModuleSpec(cfg *config.Cluster) *spec.ModuleSpec {
	if cfg == nil {
		// Return a spec that indicates an error or is effectively disabled
		return &spec.ModuleSpec{
			Name:        "Containerd Runtime",
			Description: "Manages the containerd container runtime (Error: Missing Configuration)",
			IsEnabled:   "false", // Always disabled if config is missing
			Tasks:       []*spec.TaskSpec{},
		}
	}

	// Default values or derive from cfg for NewInstallContainerdTaskSpec parameters
	version := ""
	if cfg.Spec.ContainerRuntime != nil {
		version = cfg.Spec.ContainerRuntime.Version
	}
	// If version is still empty, NewInstallContainerdTaskSpec might use its own default or error out.
	// For robustness, ensure critical parameters like version are set or handled.
	// If version is essential and not available, this module spec could be invalid.
	if version == "" {
		// Or handle this more gracefully, e.g. by making the module disabled with a reason
		version = "default_containerd_version" // Placeholder, actual default should be managed properly
	}

	arch := cfg.Spec.Kubernetes.Arch // Assuming this path exists
	zone := cfg.Spec.ImageStore.Zone // Assuming this path exists

	var registryMirrorsForStep map[string]string
	var insecureRegistries []string
	var useSystemdCgroup bool
	var extraTomlContent string
	var containerdConfigPath string
	var downloadDir string // Typically empty to use default path logic in task/step
	var checksum string    // Typically empty unless a specific checksum is enforced

	if cfg.Spec.Containerd != nil {
		// Convert map[string][]string to map[string]string for the step, taking the first mirror.
		if cfg.Spec.Containerd.RegistryMirrors != nil {
			registryMirrorsForStep = make(map[string]string)
			for k, v := range cfg.Spec.Containerd.RegistryMirrors {
				if len(v) > 0 {
					registryMirrorsForStep[k] = v[0]
				}
			}
		}
		insecureRegistries = cfg.Spec.Containerd.InsecureRegistries
		useSystemdCgroup = cfg.Spec.Containerd.UseSystemdCgroup
		extraTomlContent = cfg.Spec.Containerd.ExtraTomlConfig
		containerdConfigPath = cfg.Spec.Containerd.ConfigPath
		// downloadDir = cfg.Spec.Containerd.DownloadDir // If such a field existed
		// checksum = cfg.Spec.Containerd.Checksum     // If such a field existed
	}

	// runOnRoles for containerd installation usually includes all nodes or specific worker/control-plane roles.
	// This might be derived from cfg or be a standard set.
	runOnRoles := []string{"all"} // Example: install on all nodes. Adjust as needed.
	globalWorkDir := cfg.Spec.Global.WorkDir // Assuming this path exists

	installTaskSpec := taskContainerdFactory.NewInstallContainerdTaskSpec(
		version, arch, zone, downloadDir, checksum,
		registryMirrorsForStep, insecureRegistries,
		useSystemdCgroup, extraTomlContent, containerdConfigPath,
		runOnRoles, globalWorkDir,
	)

	return &spec.ModuleSpec{
		Name:        "Containerd Runtime",
		Description: "Manages the containerd container runtime.",
		// The IsEnabled condition string now refers to fields available in the 'cfg' object
		// that the Executor will have access to when evaluating this string.
		// Example: "cfg.Spec.ContainerRuntime.Type == 'containerd'"
		// For simplicity, if this module factory is called, we assume it's intended to be enabled
		// if the type is 'containerd' or if the type is empty (defaulting to containerd).
		IsEnabled:   "(cfg.Spec.ContainerRuntime == nil) || (cfg.Spec.ContainerRuntime.Type == '') || (cfg.Spec.ContainerRuntime.Type == 'containerd')",
		Tasks:       []*spec.TaskSpec{installTaskSpec},
		PreRunHook:  "", // No PreRunHook specified
		PostRunHook: "", // No PostRunHook specified
	}
}
