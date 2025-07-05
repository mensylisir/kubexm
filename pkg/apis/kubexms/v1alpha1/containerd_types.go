package v1alpha1

import (
	"net/url"
	"strings"
	"github.com/mensylisir/kubexm/pkg/util" // Import the util package
)

// ContainerdConfig defines specific settings for the Containerd runtime.
// Corresponds to `kubernetes.containerRuntime.containerd` in YAML.
type ContainerdConfig struct {
	// Version of Containerd to install or manage.
	// This can be different from ContainerRuntimeConfig.Version if user wants to specify explicitly here.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`

	// RegistryMirrors maps registry hosts to their mirror URLs.
	// Example: {"docker.io": ["https://mirror.example.com"]}
	// Corresponds to `registryMirrors` in YAML.
	RegistryMirrors map[string][]string `json:"registryMirrors,omitempty" yaml:"registryMirrors,omitempty"`

	// InsecureRegistries is a list of registries that should be treated as insecure.
	// Corresponds to `insecureRegistries` in YAML.
	InsecureRegistries []string `json:"insecureRegistries,omitempty" yaml:"insecureRegistries,omitempty"`

	// UseSystemdCgroup specifies whether to configure containerd to use systemd cgroup driver.
	// Defaults to true.
	// No direct YAML field, typically a best practice applied by the tool.
	UseSystemdCgroup *bool `json:"useSystemdCgroup,omitempty" yaml:"useSystemdCgroup,omitempty"`

	// ExtraTomlConfig allows appending custom TOML configuration to containerd's config.toml.
	// Corresponds to `extraTomlConfig` in YAML.
	ExtraTomlConfig string `json:"extraTomlConfig,omitempty" yaml:"extraTomlConfig,omitempty"`

	// ConfigPath is the path to the main containerd configuration file.
	// Defaults to "/etc/containerd/config.toml".
	ConfigPath *string `json:"configPath,omitempty" yaml:"configPath,omitempty"`

	// DisabledPlugins is a list of plugins to disable in containerd.
	// Example: ["cri", "diff", "events"]
	DisabledPlugins []string `json:"disabledPlugins,omitempty" yaml:"disabledPlugins,omitempty"`

	// RequiredPlugins is a list of plugins that must be enabled. Useful for validation.
	// Example: ["io.containerd.grpc.v1.cri"]
	RequiredPlugins []string `json:"requiredPlugins,omitempty" yaml:"requiredPlugins,omitempty"`

	// Imports are additional .toml files to import into the main config.
	Imports []string `json:"imports,omitempty" yaml:"imports,omitempty"`
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
		cfg.UseSystemdCgroup = boolPtr(true)
	}
	if cfg.ConfigPath == nil {
		cfg.ConfigPath = stringPtr("/etc/containerd/config.toml")
	}
	if cfg.DisabledPlugins == nil {
		cfg.DisabledPlugins = []string{}
	}
	if cfg.RequiredPlugins == nil {
		// CRI plugin is essential for Kubernetes integration.
		cfg.RequiredPlugins = []string{"io.containerd.grpc.v1.cri"}
	}
	if cfg.Imports == nil {
		cfg.Imports = []string{}
	}
	// Version: No default here; should be inherited from ContainerRuntimeConfig.Version if empty,
	// or explicitly set by user. The installer logic will handle this.
}

// Validate_ContainerdConfig validates ContainerdConfig.
func Validate_ContainerdConfig(cfg *ContainerdConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.Version != "" {
		if strings.TrimSpace(cfg.Version) == "" {
			verrs.Add("%s.version: cannot be only whitespace if specified", pathPrefix)
		} else if !util.IsValidRuntimeVersion(cfg.Version) { // Use util.IsValidRuntimeVersion
			verrs.Add("%s.version: '%s' is not a recognized version format", pathPrefix, cfg.Version)
		}
	}

	for reg, mirrors := range cfg.RegistryMirrors {
		if strings.TrimSpace(reg) == "" {
			verrs.Add("%s.registryMirrors: registry host key cannot be empty", pathPrefix)
		}
		if len(mirrors) == 0 {
			verrs.Add("%s.registryMirrors[\"%s\"]: must contain at least one mirror URL", pathPrefix, reg)
		}
		for i, mirrorURL := range mirrors {
			if strings.TrimSpace(mirrorURL) == "" {
				verrs.Add("%s.registryMirrors[\"%s\"][%d]: mirror URL cannot be empty", pathPrefix, reg, i)
			} else {
				u, err := url.ParseRequestURI(mirrorURL)
				if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
					verrs.Add("%s.registryMirrors[\"%s\"][%d]: invalid URL format for mirror '%s' (must be http or https)", pathPrefix, reg, i, mirrorURL)
				}
			}
		}
	}
	for i, insecureReg := range cfg.InsecureRegistries {
		if strings.TrimSpace(insecureReg) == "" {
			verrs.Add("%s.insecureRegistries[%d]: registry host cannot be empty", pathPrefix, i)
		} else if !util.ValidateHostPortString(insecureReg) { // Use util.ValidateHostPortString
			verrs.Add("%s.insecureRegistries[%d]: invalid host:port format for insecure registry '%s'", pathPrefix, i, insecureReg)
		}
	}
	if cfg.ConfigPath != nil && strings.TrimSpace(*cfg.ConfigPath) == "" {
		verrs.Add("%s.configPath: cannot be empty if specified", pathPrefix)
	}
	for i, plug := range cfg.DisabledPlugins {
		if strings.TrimSpace(plug) == "" {
			verrs.Add("%s.disabledPlugins[%d]: plugin name cannot be empty", pathPrefix, i)
		}
	}
	for i, plug := range cfg.RequiredPlugins {
		if strings.TrimSpace(plug) == "" {
			verrs.Add("%s.requiredPlugins[%d]: plugin name cannot be empty", pathPrefix, i)
		}
	}
	for i, imp := range cfg.Imports {
		if strings.TrimSpace(imp) == "" {
			verrs.Add("%s.imports[%d]: import path cannot be empty", pathPrefix, i)
		}
	}
	// ExtraTomlConfig is a string, specific TOML validation is complex and usually skipped here.
}
