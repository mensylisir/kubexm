

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/logger"
	"github.com/mensylisir/kubexm/pkg/util"
)



func ParseFromFile(filePath string) (*v1alpha1.Cluster, error) {
	log := logger.Get()
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
