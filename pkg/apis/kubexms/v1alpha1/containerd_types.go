package v1alpha1

import "strings"

const (
	ContainerRuntimeContainerd = "containerd"
	ContainerRuntimeDocker     = "docker"
	// Add other supported runtimes if any
)

// ContainerRuntimeConfig specifies the container runtime settings for the cluster.
type ContainerRuntimeConfig struct {
	// Type of container runtime. Supported values: "containerd", "docker".
	// Defaults to "containerd".
	Type string `json:"type,omitempty"`

	// Version of the container runtime.
	Version string `json:"version,omitempty"`

	// Docker holds Docker-specific configurations.
	// Only applicable if Type is "docker".
	Docker *DockerConfig `json:"docker,omitempty"`
}

// ContainerdConfig defines specific settings for the Containerd runtime.
// These settings are only applicable if ContainerRuntimeConfig.Type is "containerd".
type ContainerdConfig struct {
	// Version of Containerd to install or manage.
	// If ContainerRuntimeConfig.Version is set and this is empty, this might inherit from there.
	Version string `json:"version,omitempty"`

	// RegistryMirrors maps registry hosts to their mirror URLs.
	// Example: {"docker.io": ["https://mirror.example.com"]}
	RegistryMirrors map[string][]string `json:"registryMirrors,omitempty"`

	// InsecureRegistries is a list of registries that should be treated as insecure.
	InsecureRegistries []string `json:"insecureRegistries,omitempty"`

	// UseSystemdCgroup specifies whether to configure containerd to use systemd cgroup driver.
	// Defaults to true.
	UseSystemdCgroup *bool `json:"useSystemdCgroup,omitempty"`

	// ExtraTomlConfig allows appending custom TOML configuration to containerd's config.toml.
	ExtraTomlConfig string `json:"extraTomlConfig,omitempty"`

	// ConfigPath is the path to the main containerd configuration file.
	// Defaults to "/etc/containerd/config.toml".
	ConfigPath *string `json:"configPath,omitempty"`
	// DisabledPlugins is a list of plugins to disable in containerd.
	// Example: ["cri", "diff", "events"]
	DisabledPlugins []string `json:"disabledPlugins,omitempty"`
	// RequiredPlugins is a list of plugins that must be enabled. Useful for validation.
	// Example: ["io.containerd.grpc.v1.cri"]
	RequiredPlugins []string `json:"requiredPlugins,omitempty"`
	// Imports are additional .toml files to import into the main config.
	Imports []string `json:"imports,omitempty"`
}

// SetDefaults_ContainerRuntimeConfig sets default values for ContainerRuntimeConfig.
func SetDefaults_ContainerRuntimeConfig(cfg *ContainerRuntimeConfig) {
	if cfg == nil {
		return
	}
	if cfg.Type == "" {
		cfg.Type = ContainerRuntimeContainerd
	}

	if cfg.Type == ContainerRuntimeDocker {
		if cfg.Docker == nil {
			cfg.Docker = &DockerConfig{}
		}
		SetDefaults_DockerConfig(cfg.Docker)
	}
}

// Validate_ContainerRuntimeConfig validates ContainerRuntimeConfig.
func Validate_ContainerRuntimeConfig(cfg *ContainerRuntimeConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	validTypes := []string{ContainerRuntimeContainerd, ContainerRuntimeDocker}
	isValidType := false
	for _, vt := range validTypes {
		if cfg.Type == vt {
			isValidType = true; break
		}
	}
	if !isValidType {
		verrs.Add("%s.type: invalid type '%s', must be one of %v", pathPrefix, cfg.Type, validTypes)
	}
	// Version validation can be added if specific formats or ranges are required.

	if cfg.Type == ContainerRuntimeDocker {
		if cfg.Docker == nil {
			verrs.Add("%s.docker: docker configuration section cannot be nil if containerRuntime.type is '%s'", pathPrefix, ContainerRuntimeDocker)
		} else {
			Validate_DockerConfig(cfg.Docker, verrs, pathPrefix+".docker")
		}
	}
}

// SetDefaults_ContainerdConfig sets default values for ContainerdConfig.
func SetDefaults_ContainerdConfig(cfg *ContainerdConfig) {
	if cfg == nil {
		return
	}
	if cfg.RegistryMirrors == nil {
		cfg.RegistryMirrors = make(map[string][]string)
	}
	if cfg.InsecureRegistries == nil {
		cfg.InsecureRegistries = []string{}
	}
	if cfg.UseSystemdCgroup == nil {
		defaultUseSystemdCgroup := true
		cfg.UseSystemdCgroup = &defaultUseSystemdCgroup
	}
	if cfg.ConfigPath == nil {
	   defaultPath := "/etc/containerd/config.toml"
	   cfg.ConfigPath = &defaultPath
	}
	if cfg.DisabledPlugins == nil { cfg.DisabledPlugins = []string{} }
	if cfg.RequiredPlugins == nil { cfg.RequiredPlugins = []string{"io.containerd.grpc.v1.cri"} } // CRI plugin is typically required
	if cfg.Imports == nil { cfg.Imports = []string{} }
}

// Validate_ContainerdConfig validates ContainerdConfig.
func Validate_ContainerdConfig(cfg *ContainerdConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	// Version validation can be added.
	// For RegistryMirrors, ensure URLs are valid if specified.
	for reg, mirrors := range cfg.RegistryMirrors {
	   if strings.TrimSpace(reg) == "" {
		   verrs.Add("%s.registryMirrors: registry host key cannot be empty", pathPrefix)
	   }
	   if len(mirrors) == 0 {
		   verrs.Add("%s.registryMirrors[\"%s\"]: must contain at least one mirror URL", pathPrefix, reg)
	   }
	   for i, mirrorURL := range mirrors {
		   if strings.TrimSpace(mirrorURL) == "" { // Basic check, URL validation can be more complex
			   verrs.Add("%s.registryMirrors[\"%s\"][%d]: mirror URL cannot be empty", pathPrefix, reg, i)
		   }
	   }
	}
	for i, insecureReg := range cfg.InsecureRegistries {
	   if strings.TrimSpace(insecureReg) == "" {
		   verrs.Add("%s.insecureRegistries[%d]: registry host cannot be empty", pathPrefix, i)
	   }
	}
	if cfg.ConfigPath != nil && strings.TrimSpace(*cfg.ConfigPath) == "" {
	   verrs.Add("%s.configPath: cannot be empty if specified", pathPrefix)
	}
	for i, plug := range cfg.DisabledPlugins {
		if strings.TrimSpace(plug) == "" { verrs.Add("%s.disabledPlugins[%d]: plugin name cannot be empty", pathPrefix, i)}
	}
	for i, plug := range cfg.RequiredPlugins {
		if strings.TrimSpace(plug) == "" { verrs.Add("%s.requiredPlugins[%d]: plugin name cannot be empty", pathPrefix, i)}
	}
	for i, imp := range cfg.Imports {
		if strings.TrimSpace(imp) == "" { verrs.Add("%s.imports[%d]: import path cannot be empty", pathPrefix, i)}
		// Could add path validation, e.g., ensure it's an absolute path or ends with .toml
	}
}
