package parser

import (
	"fmt"

	"gopkg.in/yaml.v3"
	// Import the v1alpha1 package directly
	"{{MODULE_NAME}}/pkg/apis/kubexms/v1alpha1"
)

// ParseClusterYAML takes a byte slice of YAML data and attempts to parse it
// directly into a v1alpha1.Cluster struct.
// It returns a pointer to the populated v1alpha1.Cluster struct or an error
// if parsing fails.
func ParseClusterYAML(yamlData []byte) (*v1alpha1.Cluster, error) {
	var clusterV1alpha1 v1alpha1.Cluster

	// Unmarshal the YAML data into the struct
	// The v1alpha1.Cluster struct (and its sub-structs) should have `yaml:""` tags
	// for this to work correctly with YAML field names.
	err := yaml.Unmarshal(yamlData, &clusterV1alpha1)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal cluster YAML into v1alpha1.Cluster: %w", err)
	}

	// Basic validation or defaulting after parsing can be added here if needed.
	// For example, checking TypeMeta fields.
	if clusterV1alpha1.APIVersion == "" {
		// Depending on requirements, this could be an error or defaulted.
		// For now, the parser's job is just to parse.
	}

	if clusterV1alpha1.Kind == "" {
		// Similar to APIVersion.
	}

	return &clusterV1alpha1, nil
}
