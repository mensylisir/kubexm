// Package config provides functions for loading, validating,
// and setting default values for Kubexm cluster configurations.
// The primary entry point is ParseFromFile, which handles the
// lifecycle of configuration processing from a YAML file.
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	// Ensure logger is imported if you plan to use it for logging within this package.
	// "github.com/mensylisir/kubexm/pkg/logger"
)

// ParseFromFile reads a YAML configuration file from the given path,
// unmarshals it into a v1alpha1.Cluster object, sets default values,
// and validates the configuration.
func ParseFromFile(filePath string) (*v1alpha1.Cluster, error) {
	// log := logger.Get() // Example: Get a logger instance

	// log.Infof("Reading configuration file from: %s", filePath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file '%s': %w", filePath, err)
	}

	// log.Debugf("Successfully read configuration file content.")

	var clusterConfig v1alpha1.Cluster
	// log.Debugf("Unmarshalling YAML content into v1alpha1.Cluster struct...")
	if err := yaml.Unmarshal(data, &clusterConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML from '%s': %w", filePath, err)
	}
	// log.Debugf("Successfully unmarshalled YAML.")

	// Set default values for the cluster configuration.
	// log.Debugf("Setting default values for the cluster configuration...")
	v1alpha1.SetDefaults_Cluster(&clusterConfig)
	// log.Debugf("Successfully set default values.")

	// Validate the cluster configuration.
	// log.Debugf("Validating the cluster configuration...")
	if err := v1alpha1.Validate_Cluster(&clusterConfig); err != nil {
		return nil, fmt.Errorf("cluster configuration validation failed for '%s': %w", filePath, err)
	}
	// log.Debugf("Successfully validated cluster configuration.")

	// log.Infof("Successfully parsed and validated configuration from: %s", filePath)
	return &clusterConfig, nil
}
