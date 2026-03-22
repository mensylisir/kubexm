package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/internal/apis/kubexms/v1alpha1"
	"github.com/mensylisir/kubexm/internal/errors/validation"
	"github.com/mensylisir/kubexm/internal/logger"
	"gopkg.in/yaml.v3"
)

type ParseOptions struct {
	SkipHostValidation bool
}

func ParseFromFile(filePath string) (*v1alpha1.Cluster, error) {
	return ParseFromFileWithOptions(filePath, ParseOptions{})
}

func ParseFromFileWithOptions(filePath string, opts ParseOptions) (*v1alpha1.Cluster, error) {
	if strings.TrimSpace(filePath) == "" {
		return nil, fmt.Errorf("config file path cannot be empty")
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".yaml", ".yml":
		return ParseYAMLWithOptions(filePath, opts)
	case ".json":
		return ParseJSONWithOptions(filePath, opts)
	default:
		// Default to YAML for unknown extensions
		return ParseYAMLWithOptions(filePath, opts)
	}
}

func ParseYAML(filePath string) (*v1alpha1.Cluster, error) {
	return ParseYAMLWithOptions(filePath, ParseOptions{})
}

func ParseYAMLWithOptions(filePath string, opts ParseOptions) (*v1alpha1.Cluster, error) {
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

	if err := processClusterConfig(&clusterConfig, opts); err != nil {
		return nil, fmt.Errorf("failed to process configuration from YAML file %s: %w", filePath, err)
	}

	log.Infof("Successfully parsed and processed YAML configuration from: %s", filePath)
	return &clusterConfig, nil
}

func ParseJSON(filePath string) (*v1alpha1.Cluster, error) {
	return ParseJSONWithOptions(filePath, ParseOptions{})
}

func ParseJSONWithOptions(filePath string, opts ParseOptions) (*v1alpha1.Cluster, error) {
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

	if err := processClusterConfig(&clusterConfig, opts); err != nil {
		return nil, fmt.Errorf("failed to process configuration from JSON file %s: %w", filePath, err)
	}

	log.Infof("Successfully parsed and processed JSON configuration from: %s", filePath)
	return &clusterConfig, nil
}

func processClusterConfig(clusterConfig *v1alpha1.Cluster, opts ParseOptions) error {
	log := logger.Get()

	log.Debugf("Setting default values for the cluster configuration...")
	v1alpha1.SetDefaults_Cluster(clusterConfig)
	log.Debugf("Successfully set default values.")

	normalizeDeploymentTypes(clusterConfig)
	if err := ensureLocalHostIfEmpty(clusterConfig); err != nil {
		return fmt.Errorf("failed to ensure local host defaults: %w", err)
	}

	log.Debugf("Validating the cluster configuration...")
	verrs := &validation.ValidationErrors{}
	v1alpha1.Validate_Cluster(clusterConfig, verrs)
	if opts.SkipHostValidation {
		verrs = filterHostValidationErrors(verrs)
	}
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
		if !opts.SkipHostValidation {
			if err := ValidateRoleGroupHosts(clusterConfig.Spec.RoleGroups, clusterConfig.Spec.Hosts); err != nil {
				return fmt.Errorf("roleGroup validation failed: %w", err)
			}
			log.Debug("Successfully validated role group hosts.")
		}
	}

	if err := ApplyRoleGroupsToHosts(clusterConfig.Spec.RoleGroups, clusterConfig.Spec.Hosts); err != nil {
		return fmt.Errorf("failed to apply roleGroups to hosts: %w", err)
	}

	log.Debugf("Successfully validated role group hosts.")
	return nil
}
