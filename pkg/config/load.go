package config

import (
	"os"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"gopkg.in/yaml.v3"
)

// ParseFromFile reads a YAML configuration file and unmarshals it into a Cluster object.
// It also sets default values and validates the configuration.
func ParseFromFile(filepath string) (*v1alpha1.Cluster, error) {
	content, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	var cfg v1alpha1.Cluster
	if err := yaml.Unmarshal(content, &cfg); err != nil {
		return nil, err
	}

	// Set defaults before validation
	v1alpha1.SetDefaults_Cluster(&cfg)

	// Validate the configuration
	if err := v1alpha1.Validate_Cluster(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
