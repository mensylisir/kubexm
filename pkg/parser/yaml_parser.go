package parser

import (
	"fmt"

	"gopkg.in/yaml.v3"
	// Assuming the module path will be correctly handled by the build system.
	// For a typical Go project, this might be "github.com/user/repo/pkg/config"
	// Using a placeholder for now, which might need to be adjusted.
	"{{MODULE_NAME}}/pkg/config"
)

// ParseClusterYAML takes a byte slice of YAML data and attempts to parse it
// into a config.Cluster struct.
// It returns a pointer to the populated config.Cluster struct or an error
// if parsing fails.
func ParseClusterYAML(yamlData []byte) (*config.Cluster, error) {
	var clusterConfig config.Cluster

	// Unmarshal the YAML data into the struct
	err := yaml.Unmarshal(yamlData, &clusterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster YAML: %w", err)
	}

	// Basic validation or defaulting after parsing can be added here if needed,
	// though extensive validation is often handled by the config types themselves
	// or a dedicated validation step.

	// For example, ensuring APIVersion and Kind are set to expected values if not present,
	// or if they are, validating them.
	if clusterConfig.APIVersion == "" {
		// This could default or error, depending on requirements.
		// For now, let's assume it's an error if not provided,
		// or it could be defaulted by a higher-level component.
		// Alternatively, the config.Cluster struct itself might have default tags,
		// but YAML parsing usually requires explicit values for struct fields unless pointers.
	}

	if clusterConfig.Kind == "" {
		// Similar to APIVersion.
	}

	return &clusterConfig, nil
}
