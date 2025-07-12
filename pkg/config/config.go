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
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/util" // Added import for host range expansion
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

// validateRoleGroupHosts validates that all hosts referenced in RoleGroups exist in the hosts spec
func validateRoleGroupHosts(roleGroups *v1alpha1.RoleGroupsSpec, hosts []v1alpha1.HostSpec) error {
	// Create a map of valid host names
	validHosts := make(map[string]bool)
	for _, host := range hosts {
		validHosts[host.Name] = true
	}

	// Check each role group
	allHosts := []string{}
	allHosts = append(allHosts, roleGroups.Master...)
	allHosts = append(allHosts, roleGroups.Worker...)
	allHosts = append(allHosts, roleGroups.Etcd...)
	allHosts = append(allHosts, roleGroups.LoadBalancer...)
	allHosts = append(allHosts, roleGroups.Storage...)
	allHosts = append(allHosts, roleGroups.Registry...)

	for _, hostName := range allHosts {
		if !validHosts[hostName] {
			return fmt.Errorf("host '%s' referenced in roleGroups but not defined in hosts spec", hostName)
		}
	}
	return nil
}

// ParseFromFile reads a YAML configuration file from the given path,
// unmarshals it into a v1alpha1.Cluster object, sets default values,
// and validates the configuration.
func ParseFromFile(filePath string) (*v1alpha1.Cluster, error) {
	log := logger.Get() // Get a logger instance

	log.Infof("Reading configuration file from: %s", filePath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration file %s: %w", filePath, err)
	}

	log.Debugf("Successfully read configuration file content.")

	var clusterConfig v1alpha1.Cluster
	log.Debugf("Unmarshalling YAML content into v1alpha1.Cluster struct...")
	if err := yaml.Unmarshal(data, &clusterConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML from '%s': %w", filePath, err)
	}
	log.Debugf("Successfully unmarshalled YAML.")

	// Set default values for the cluster configuration.
	log.Debugf("Setting default values for the cluster configuration...")
	v1alpha1.SetDefaults_Cluster(&clusterConfig)
	log.Debugf("Successfully set default values.")

	// Validate the cluster configuration.
	log.Debugf("Validating the cluster configuration...")
	if err := v1alpha1.Validate_Cluster(&clusterConfig); err != nil {
		return nil, fmt.Errorf("configuration validation failed for %s: %w", filePath, err)
	}
	log.Debugf("Successfully validated cluster configuration.")

	// Validate RoleGroups host references before expansion
	if clusterConfig.Spec.RoleGroups != nil {
		if err := validateRoleGroupHosts(clusterConfig.Spec.RoleGroups, clusterConfig.Spec.Hosts); err != nil {
			return nil, fmt.Errorf("roleGroup validation failed for %s: %w", filePath, err)
		}
	}

	// Expand host ranges in RoleGroups after successful validation
	if clusterConfig.Spec.RoleGroups != nil {
		rg := clusterConfig.Spec.RoleGroups
		var errExpansion error

		rg.Master, errExpansion = expandRoleGroupHosts(rg.Master)
		if errExpansion != nil {
			return nil, fmt.Errorf("failed to expand hosts for master role group: %w", errExpansion)
		}

		rg.Worker, errExpansion = expandRoleGroupHosts(rg.Worker)
		if errExpansion != nil {
			return nil, fmt.Errorf("failed to expand hosts for worker role group: %w", errExpansion)
		}

		rg.Etcd, errExpansion = expandRoleGroupHosts(rg.Etcd)
		if errExpansion != nil {
			return nil, fmt.Errorf("failed to expand hosts for etcd role group: %w", errExpansion)
		}

		rg.LoadBalancer, errExpansion = expandRoleGroupHosts(rg.LoadBalancer)
		if errExpansion != nil {
			return nil, fmt.Errorf("failed to expand hosts for loadbalancer role group: %w", errExpansion)
		}

		rg.Storage, errExpansion = expandRoleGroupHosts(rg.Storage)
		if errExpansion != nil {
			return nil, fmt.Errorf("failed to expand hosts for storage role group: %w", errExpansion)
		}

		rg.Registry, errExpansion = expandRoleGroupHosts(rg.Registry)
		if errExpansion != nil {
			return nil, fmt.Errorf("failed to expand hosts for registry role group: %w", errExpansion)
		}

		// CustomRoles field removed from RoleGroupsSpec
	}

	log.Infof("Successfully parsed, validated, and processed configuration from: %s", filePath)
	return &clusterConfig, nil
}

// Ensure util is imported
// The import "github.com/mensylisir/kubexm/pkg/util" is already present at the top of the file.
