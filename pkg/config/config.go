package config

import (
	"encoding/json"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"github.com/mensylisir/kubexm/pkg/logger"
	"gopkg.in/yaml.v3"
	"os"
)

func ParseYAML(filePath string) (*v1alpha1.Cluster, error) {
	log := logger.Get()
	log.Infof("Reading YAML configuration file from: %s", filePath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file %s: %w", filePath, err)
	}

	var clusterConfig v1alpha1.Cluster
	log.Debugf("Unmarshalling YAML content into struct...")
	if err := yaml.Unmarshal(data, &clusterConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML from '%s': %w", filePath, err)
	}

	if err := processClusterConfig(&clusterConfig); err != nil {
		return nil, fmt.Errorf("failed to process configuration from YAML file %s: %w", filePath, err)
	}

	log.Infof("Successfully parsed and processed YAML configuration from: %s", filePath)
	return &clusterConfig, nil
}

func ParseJSON(filePath string) (*v1alpha1.Cluster, error) {
	log := logger.Get()
	log.Infof("Reading JSON configuration file from: %s", filePath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file %s: %w", filePath, err)
	}

	var clusterConfig v1alpha1.Cluster
	log.Debugf("Unmarshalling JSON content into struct...")
	if err := json.Unmarshal(data, &clusterConfig); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON from '%s': %w", filePath, err)
	}

	if err := processClusterConfig(&clusterConfig); err != nil {
		return nil, fmt.Errorf("failed to process configuration from JSON file %s: %w", filePath, err)
	}

	log.Infof("Successfully parsed and processed JSON configuration from: %s", filePath)
	return &clusterConfig, nil
}

func processClusterConfig(clusterConfig *v1alpha1.Cluster) error {
	log := logger.Get()

	log.Debugf("Setting default values for the cluster configuration...")
	v1alpha1.SetDefaults_Cluster(clusterConfig)
	log.Debugf("Successfully set default values.")

	log.Debugf("Validating the cluster configuration...")
	verrs := &validation.ValidationErrors{}
	v1alpha1.Validate_Cluster(clusterConfig, verrs)
	if verrs.HasErrors() {
		return fmt.Errorf("configuration validation failed: %w", verrs)
	}
	log.Debugf("Successfully validated cluster configuration.")

	if clusterConfig.Spec.RoleGroups != nil {
		rg := clusterConfig.Spec.RoleGroups
		var errExpansion error

		rg.Master, errExpansion = ExpandRoleGroupHosts(rg.Master)
		if errExpansion != nil {
			return fmt.Errorf("failed to expand hosts for master role group: %w", errExpansion)
		}

		rg.Worker, errExpansion = ExpandRoleGroupHosts(rg.Worker)
		if errExpansion != nil {
			return fmt.Errorf("failed to expand hosts for worker role group: %w", errExpansion)
		}

		rg.Etcd, errExpansion = ExpandRoleGroupHosts(rg.Etcd)
		if errExpansion != nil {
			return fmt.Errorf("failed to expand hosts for etcd role group: %w", errExpansion)
		}

		rg.LoadBalancer, errExpansion = ExpandRoleGroupHosts(rg.LoadBalancer)
		if errExpansion != nil {
			return fmt.Errorf("failed to expand hosts for loadbalancer role group: %w", errExpansion)
		}

		rg.Storage, errExpansion = ExpandRoleGroupHosts(rg.Storage)
		if errExpansion != nil {
			return fmt.Errorf("failed to expand hosts for storage role group: %w", errExpansion)
		}

		rg.Registry, errExpansion = ExpandRoleGroupHosts(rg.Registry)
		if errExpansion != nil {
			return fmt.Errorf("failed to expand hosts for registry role group: %w", errExpansion)
		}
		log.Debug("Successfully expanded host ranges.")
		if clusterConfig.Spec.RoleGroups != nil {
			if err := ValidateRoleGroupHosts(clusterConfig.Spec.RoleGroups, clusterConfig.Spec.Hosts); err != nil {
				return fmt.Errorf("roleGroup validation failed: %w", err)
			}
		}
		log.Debug("Successfully validated role group hosts.")
	}

	log.Debugf("Successfully validated role group hosts.")
	return nil
}
