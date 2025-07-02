package v1alpha1

import "strings"

// ContainerRuntimeType defines the type of container runtime.
type ContainerRuntimeType string

const (
	ContainerRuntimeDocker     ContainerRuntimeType = "docker"
	ContainerRuntimeContainerd ContainerRuntimeType = "containerd"
	// Add other runtimes like cri-o, isula if supported by YAML
)

// ContainerRuntimeConfig is a wrapper for specific container runtime configurations.
// Corresponds to `kubernetes.containerRuntime` in YAML.
type ContainerRuntimeConfig struct {
	// Type specifies the container runtime to use (e.g., "docker", "containerd").
	Type ContainerRuntimeType `json:"type,omitempty" yaml:"type,omitempty"`
	// Version of the container runtime.
	Version string `json:"version,omitempty" yaml:"version,omitempty"`

	// Docker holds Docker-specific configurations.
	// Only applicable if Type is "docker".
	Docker *DockerConfig `json:"docker,omitempty" yaml:"docker,omitempty"`
	// Containerd holds Containerd-specific configurations.
	// Only applicable if Type is "containerd".
	Containerd *ContainerdConfig `json:"containerd,omitempty" yaml:"containerd,omitempty"`
}

// SetDefaults_ContainerRuntimeConfig sets default values for ContainerRuntimeConfig.
func SetDefaults_ContainerRuntimeConfig(cfg *ContainerRuntimeConfig) {
	if cfg == nil {
		return
	}
	if cfg.Type == "" {
		cfg.Type = ContainerRuntimeDocker // Default to Docker as per original KubeKey behavior, can be changed.
	}
	// Version might be defaulted based on chosen type or a global default.
	// For now, assume version is explicitly set or handled by higher-level logic.

	if cfg.Type == ContainerRuntimeDocker {
		if cfg.Docker == nil {
			cfg.Docker = &DockerConfig{}
		}
		SetDefaults_DockerConfig(cfg.Docker)
	}

	if cfg.Type == ContainerRuntimeContainerd {
		if cfg.Containerd == nil {
			cfg.Containerd = &ContainerdConfig{}
		}
		SetDefaults_ContainerdConfig(cfg.Containerd)
	}
}

// Validate_ContainerRuntimeConfig validates ContainerRuntimeConfig.
func Validate_ContainerRuntimeConfig(cfg *ContainerRuntimeConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		verrs.Add("%s: section cannot be nil", pathPrefix)
		return
	}
	validTypes := []ContainerRuntimeType{ContainerRuntimeDocker, ContainerRuntimeContainerd, ""} // Allow empty for default
	isValid := false
	for _, vt := range validTypes {
		if cfg.Type == vt || (cfg.Type == "" && vt == ContainerRuntimeDocker) { // Defaulting "" to Docker
			isValid = true
			break
		}
	}
	if !isValid {
		verrs.Add("%s.type: invalid container runtime type '%s'", pathPrefix, cfg.Type)
	}

	if cfg.Type == ContainerRuntimeDocker {
		if cfg.Docker == nil {
			// This case should be handled by defaulting, but good for completeness
			// verrs.Add("%s.docker: must be defined if type is 'docker'", pathPrefix)
		} else {
			Validate_DockerConfig(cfg.Docker, verrs, pathPrefix+".docker")
		}
	} else if cfg.Docker != nil {
		verrs.Add("%s.docker: can only be set if type is 'docker'", pathPrefix)
	}

	if cfg.Type == ContainerRuntimeContainerd {
		if cfg.Containerd == nil {
			// verrs.Add("%s.containerd: must be defined if type is 'containerd'", pathPrefix)
		} else {
			Validate_ContainerdConfig(cfg.Containerd, verrs, pathPrefix+".containerd")
		}
	} else if cfg.Containerd != nil {
		verrs.Add("%s.containerd: can only be set if type is 'containerd'", pathPrefix)
	}

	// Validate Version if Type is set
	if cfg.Type != "" && cfg.Version == "" {
		// verrs.Add("%s.version: cannot be empty if container runtime type ('%s') is specified", pathPrefix, cfg.Type)
		// Allowing empty version for now, as it might be defaulted by the installer based on Kubernetes version or other logic.
		// A warning could be logged by the controller if it's empty and no default can be determined.
	}
	if cfg.Version != "" && strings.TrimSpace(cfg.Version) == "" {
		verrs.Add("%s.version: cannot be only whitespace if specified", pathPrefix)
	}
}
