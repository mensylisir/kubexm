package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
)

// Load reads a cluster configuration YAML file from the given path,
// unmarshals it into a Cluster struct, sets default values, and validates it.
func Load(configPath string) (*v1alpha1.Cluster, error) { // Changed return type
	if configPath == "" {
		return nil, fmt.Errorf("configuration file path cannot be empty")
	}

	yamlFile, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", configPath, err)
	}

	return LoadFromBytes(yamlFile)
}

// LoadFromBytes unmarshals YAML content from a byte slice into a Cluster struct,
// then (in future steps) sets default values and validates it.
// This is the core loading logic, also used by Load().
func LoadFromBytes(yamlBytes []byte) (*v1alpha1.Cluster, error) { // Changed return type
	var cfg v1alpha1.Cluster // Changed type to v1alpha1.Cluster

	if err := yaml.Unmarshal(yamlBytes, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml config: %w", err)
	}

	// Apply default values using the v1alpha1 package's function
	v1alpha1.SetDefaults_Cluster(&cfg)

	// Validate the configuration using the v1alpha1 package's function
	if err := v1alpha1.Validate_Cluster(&cfg); err != nil {
	    return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	return &cfg, nil
}


// Note: The Load and LoadFromBytes functions now handle reading, unmarshalling,
// applying defaults from v1alpha1.SetDefaults_Cluster, and validation from v1alpha1.Validate_Cluster.
