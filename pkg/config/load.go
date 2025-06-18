package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Load reads a cluster configuration YAML file from the given path,
// unmarshals it into a Cluster struct, sets default values, and validates it.
func Load(configPath string) (*Cluster, error) {
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
func LoadFromBytes(yamlBytes []byte) (*Cluster, error) {
	var cfg Cluster

	if err := yaml.Unmarshal(yamlBytes, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml config: %w", err)
	}

	// Ensure Kind and APIVersion are as expected, or set defaults if appropriate.
	// Current thinking: these fields should ideally be present in the YAML.
	// Validation (to be added later) will enforce this.
	// SetDefaults (to be added later) could populate them if they are empty and
	// if we decide that's the desired behavior (e.g. cfg.APIVersion = DefaultAPIVersion).
	// For now, LoadFromBytes focuses on unmarshalling.

	// --- Placeholder calls for SetDefaults and Validate ---
	// These functions will be implemented in defaults.go and validate.go respectively
	// in subsequent steps as per the plan.

	// Example of intended flow:
	// err := SetDefaults(&cfg) // Assuming SetDefaults modifies in place and might return an error
	// if err != nil {
	//     return nil, fmt.Errorf("failed to apply default configuration values: %w", err)
	// }
	//
	// err = Validate(&cfg) // Validate will return a comprehensive error if issues are found
	// if err != nil {
	//     return nil, fmt.Errorf("configuration validation failed: %w", err)
	// }

	// Apply default values
	SetDefaults(&cfg) // Called once

	// Validate the configuration
	if err := Validate(&cfg); err != nil {
	    return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	return &cfg, nil
}


// Note: The Load and LoadFromBytes functions now handle reading, unmarshalling, applying defaults, and validation.
// The calls to SetDefaults and Validate are placeholders for the logic that will be
// implemented in subsequent steps as per the plan. The actual implementation of these
// functions will reside in separate files (defaults.go, validate.go).
