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
	"github.com/mensylisir/kubexm/pkg/util" // Added import for host range expansion
	// Ensure logger is imported if you plan to use it for logging within this package.
	// "github.com/mensylisir/kubexm/pkg/logger"
)

// expandRoleGroupHosts processes a slice of host strings, expanding any ranges.
func expandRoleGroupHosts(hosts []string) ([]string, error) {
	if hosts == nil {
		return nil, nil
	}
	expanded := make([]string, 0, len(hosts))
	for _, h := range hosts {
		currentHosts, err := util.ExpandHostRange(h)
		if err != nil {
			return nil, fmt.Errorf("error expanding host range '%s': %w", h, err)
		}
		expanded = append(expanded, currentHosts...)
	}
	return expanded, nil
}

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

	// Expand host ranges in RoleGroups after successful validation
	if clusterConfig.Spec.RoleGroups != nil {
		rg := clusterConfig.Spec.RoleGroups
		var errExpansion error

		rg.Master.Hosts, errExpansion = expandRoleGroupHosts(rg.Master.Hosts)
		if errExpansion != nil {
			return nil, fmt.Errorf("failed to expand hosts for master role group: %w", errExpansion)
		}

		rg.Worker.Hosts, errExpansion = expandRoleGroupHosts(rg.Worker.Hosts)
		if errExpansion != nil {
			return nil, fmt.Errorf("failed to expand hosts for worker role group: %w", errExpansion)
		}

		rg.Etcd.Hosts, errExpansion = expandRoleGroupHosts(rg.Etcd.Hosts)
		if errExpansion != nil {
			return nil, fmt.Errorf("failed to expand hosts for etcd role group: %w", errExpansion)
		}

		rg.LoadBalancer.Hosts, errExpansion = expandRoleGroupHosts(rg.LoadBalancer.Hosts)
		if errExpansion != nil {
			return nil, fmt.Errorf("failed to expand hosts for loadbalancer role group: %w", errExpansion)
		}

		rg.Storage.Hosts, errExpansion = expandRoleGroupHosts(rg.Storage.Hosts)
		if errExpansion != nil {
			return nil, fmt.Errorf("failed to expand hosts for storage role group: %w", errExpansion)
		}

		rg.Registry.Hosts, errExpansion = expandRoleGroupHosts(rg.Registry.Hosts)
		if errExpansion != nil {
			return nil, fmt.Errorf("failed to expand hosts for registry role group: %w", errExpansion)
		}

		for i := range rg.CustomRoles {
			// Need to operate on a pointer to the element to modify it in the slice
			customRole := &rg.CustomRoles[i]
			customRole.Hosts, errExpansion = expandRoleGroupHosts(customRole.Hosts)
			if errExpansion != nil {
				return nil, fmt.Errorf("failed to expand hosts for custom role group '%s': %w", customRole.Name, errExpansion)
			}
		}
	}

	// log.Infof("Successfully parsed, validated, and processed configuration from: %s", filePath)
	return &clusterConfig, nil
}

// Ensure util is imported
// The import "github.com/mensylisir/kubexm/pkg/util" is already present at the top of the file.
