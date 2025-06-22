package parser

import (
	"fmt"
	"os"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// ParseFromFile reads a YAML file from the given path, parses it into a v1alpha1.Cluster object,
// sets default values, and validates the configuration.
func ParseFromFile(filePath string) (*v1alpha1.Cluster, error) {
	yamlFile, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file '%s': %w", filePath, err)
	}

	clusterConfig := &v1alpha1.Cluster{}
	// Using k8s.io/apimachinery/pkg/util/yaml is generally robust for Kubernetes-style YAML.
	// It often uses json.Unmarshal internally after converting YAML to JSON.
	err = yaml.Unmarshal(yamlFile, clusterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML from '%s': %w", filePath, err)
	}

	// Set default values for the cluster configuration.
	// This is crucial for ensuring that optional fields have sensible defaults.
	v1alpha1.SetDefaults_Cluster(clusterConfig)

	// Validate the cluster configuration after defaults have been applied.
	// This checks for required fields, valid formats, ranges, etc.
	if err := v1alpha1.Validate_Cluster(clusterConfig); err != nil {
		return nil, fmt.Errorf("cluster configuration validation failed for '%s': %w", filePath, err)
	}

	return clusterConfig, nil
}
