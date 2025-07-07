package v1alpha1

import (
	"strings"
	// "strconv" // No longer needed after removing local parseInt
	// "unicode" // No longer needed after removing local isNumericSegment, isAlphanumericHyphenSegment
	"github.com/mensylisir/kubexm/pkg/util" // Ensure util is imported
	"github.com/mensylisir/kubexm/pkg/util/validation" // Import validation
	"fmt" // Import fmt
	"github.com/mensylisir/kubexm/pkg/common" // Moved import here
)

// ContainerRuntimeType defines the type of container runtime.
type ContainerRuntimeType string

// Ensure common is imported if not already
// import "github.com/mensylisir/kubexm/pkg/common" // Removed from here

const (
	ContainerRuntimeDocker     ContainerRuntimeType = common.RuntimeDocker
	ContainerRuntimeContainerd ContainerRuntimeType = common.RuntimeContainerd
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
		cfg.Type = ContainerRuntimeContainerd // Default to Containerd, which uses common.RuntimeContainerd
	}

	if cfg.Type == common.RuntimeDocker { // Use common constant
		if cfg.Docker == nil {
			cfg.Docker = &DockerConfig{}
		}
		SetDefaults_DockerConfig(cfg.Docker)
	}

	if cfg.Type == common.RuntimeContainerd { // Use common constant
		if cfg.Containerd == nil {
			cfg.Containerd = &ContainerdConfig{}
		}
		SetDefaults_ContainerdConfig(cfg.Containerd)
	}
}

// Validate_ContainerRuntimeConfig validates ContainerRuntimeConfig.
func Validate_ContainerRuntimeConfig(cfg *ContainerRuntimeConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		verrs.Add(pathPrefix, "section cannot be nil")
		return
	}
	// Type should be defaulted by SetDefaults_ContainerRuntimeConfig before validation.
	// So, it should be either Docker or Containerd.
	if cfg.Type != common.RuntimeDocker && cfg.Type != common.RuntimeContainerd { // Use common constants
		verrs.Add(pathPrefix+".type", fmt.Sprintf("invalid container runtime type '%s', must be '%s' or '%s'", cfg.Type, common.RuntimeDocker, common.RuntimeContainerd))
	}

	if cfg.Type == common.RuntimeDocker { // Use common constant
		if cfg.Docker == nil {
			// Defaulting handles this
		} else {
			Validate_DockerConfig(cfg.Docker, verrs, pathPrefix+".docker")
		}
	} else if cfg.Docker != nil {
		verrs.Add(pathPrefix+".docker", "can only be set if type is 'docker'")
	}

	if cfg.Type == common.RuntimeContainerd { // Use common constant
		if cfg.Containerd == nil {
			// Defaulting handles this
		} else {
			Validate_ContainerdConfig(cfg.Containerd, verrs, pathPrefix+".containerd")
		}
	} else if cfg.Containerd != nil {
		verrs.Add(pathPrefix+".containerd", "can only be set if type is 'containerd'")
	}

	if cfg.Version != "" {
		if strings.TrimSpace(cfg.Version) == "" {
			verrs.Add(pathPrefix+".version", "cannot be only whitespace if specified")
		} else if !util.IsValidRuntimeVersion(cfg.Version) { // Use util.IsValidRuntimeVersion
			verrs.Add(pathPrefix+".version", fmt.Sprintf("'%s' is not a recognized version format", cfg.Version))
		}
	}
}

// Local helper functions like isNumericSegment, isValidPort, parseInt,
// isAlphanumericHyphenSegment, and isValidRuntimeVersion are removed
// as their functionalities are expected to be covered by functions in pkg/util or standard libraries.
// Specifically, isValidRuntimeVersion, isValidPort, isValidIP, isValidDomainName, ValidateHostPortString
// are available in pkg/util/utils.go.
