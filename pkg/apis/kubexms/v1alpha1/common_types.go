package v1alpha1

import (
	"strings"
	// "strconv" // No longer needed after removing local parseInt
	// "unicode" // No longer needed after removing local isNumericSegment, isAlphanumericHyphenSegment
	"github.com/mensylisir/kubexm/pkg/util" // Ensure util is imported
)

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
		cfg.Type = ContainerRuntimeDocker // Default to Docker
	}

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
			// Defaulting handles this
		} else {
			Validate_DockerConfig(cfg.Docker, verrs, pathPrefix+".docker")
		}
	} else if cfg.Docker != nil {
		verrs.Add("%s.docker: can only be set if type is 'docker'", pathPrefix)
	}

	if cfg.Type == ContainerRuntimeContainerd {
		if cfg.Containerd == nil {
			// Defaulting handles this
		} else {
			Validate_ContainerdConfig(cfg.Containerd, verrs, pathPrefix+".containerd")
		}
	} else if cfg.Containerd != nil {
		verrs.Add("%s.containerd: can only be set if type is 'containerd'", pathPrefix)
	}

	if cfg.Version != "" {
		if strings.TrimSpace(cfg.Version) == "" {
			verrs.Add("%s.version: cannot be only whitespace if specified", pathPrefix)
		} else if !util.IsValidRuntimeVersion(cfg.Version) { // Use util.IsValidRuntimeVersion
			verrs.Add("%s.version: '%s' is not a recognized version format", pathPrefix, cfg.Version)
		}
	}
}

// Local helper functions like isNumericSegment, isValidPort, parseInt,
// isAlphanumericHyphenSegment, and isValidRuntimeVersion are removed
// as their functionalities are expected to be covered by functions in pkg/util or standard libraries.
// Specifically, isValidRuntimeVersion, isValidPort, isValidIP, isValidDomainName, ValidateHostPortString
// are available in pkg/util/utils.go.
