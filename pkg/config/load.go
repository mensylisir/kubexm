package config

import (
	"fmt"
	"os"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"gopkg.in/yaml.v3"
)

// ParseFromFile reads a YAML configuration file and unmarshals it into a Cluster object.
// It also sets default values and validates the configuration.
func ParseFromFile(filepath string) (*v1alpha1.Cluster, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file %s: %w", filepath, err)
	}

	var cfg v1alpha1.Cluster
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML from %s: %w", filepath, err)
	}

	// Set defaults before validation
	v1alpha1.SetDefaults_Cluster(&cfg)

	// Validate the configuration
	// Assuming v1alpha1.Validate_Cluster returns a single error, matching the original code structure.
	// If it actually returns []error, this part would need to be:
	// if errs := v1alpha1.Validate_Cluster(&cfg); len(errs) > 0 {
	//    // Potentially join errors or return a new error wrapping them.
	//    // For simplicity, returning the first error if it's a slice.
	//    return nil, fmt.Errorf("configuration validation failed for %s: %w", filepath, errs[0])
	// }
	// Sticking to the original pattern:
	if err := v1alpha1.Validate_Cluster(&cfg); err != nil {
		return nil, fmt.Errorf("configuration validation failed for %s: %w", filepath, err)
	}

	return &cfg, nil
}
