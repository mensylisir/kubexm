package v1alpha1

import (
	"fmt"
	"strings"
	"github.com/mensylisir/kubexm/pkg/util" // Import the util package
	"github.com/mensylisir/kubexm/pkg/util/validation"
)

// StorageConfig defines the storage configurations for the cluster.
type StorageConfig struct {
	OpenEBS             *OpenEBSConfig `json:"openebs,omitempty" yaml:"openebs,omitempty"`
	DefaultStorageClass *string        `json:"defaultStorageClass,omitempty" yaml:"defaultStorageClass,omitempty"`
}

// OpenEBSConfig defines settings for OpenEBS storage provisioner.
type OpenEBSConfig struct {
	BasePath string                 `json:"basePath,omitempty" yaml:"basePath,omitempty"`
	Enabled  *bool                  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Version  *string                `json:"version,omitempty" yaml:"version,omitempty"`
	Engines  *OpenEBSEngineConfig `json:"engines,omitempty" yaml:"engines,omitempty"`
}

// OpenEBSEngineConfig allows specifying configurations for different OpenEBS storage engines.
type OpenEBSEngineConfig struct {
	Mayastor      *OpenEBSEngineMayastorConfig      `json:"mayastor,omitempty" yaml:"mayastor,omitempty"`
	Jiva          *OpenEBSEngineJivaConfig          `json:"jiva,omitempty" yaml:"jiva,omitempty"`
	CStor         *OpenEBSEnginecStorConfig         `json:"cstor,omitempty" yaml:"cstor,omitempty"`
	LocalHostPath *OpenEBSEngineLocalHostPathConfig `json:"localHostPath,omitempty" yaml:"localHostPath,omitempty"`
}

// OpenEBSEngineMayastorConfig holds Mayastor specific settings.
type OpenEBSEngineMayastorConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// OpenEBSEngineJivaConfig holds Jiva specific settings.
type OpenEBSEngineJivaConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// OpenEBSEnginecStorConfig holds cStor specific settings.
type OpenEBSEnginecStorConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// OpenEBSEngineLocalHostPathConfig holds LocalHostPath specific settings.
type OpenEBSEngineLocalHostPathConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

// SetDefaults_StorageConfig sets default values for StorageConfig.
func SetDefaults_StorageConfig(cfg *StorageConfig) {
	if cfg == nil {
		return
	}
	if cfg.OpenEBS != nil {
		SetDefaults_OpenEBSConfig(cfg.OpenEBS)
	}
}

// SetDefaults_OpenEBSConfig sets default values for OpenEBSConfig.
func SetDefaults_OpenEBSConfig(cfg *OpenEBSConfig) {
	if cfg == nil {
		return
	}
	if cfg.Enabled == nil {
		cfg.Enabled = boolPtr(true)
	}

	if cfg.Enabled != nil && *cfg.Enabled {
		if cfg.BasePath == "" {
			cfg.BasePath = "/var/openebs/local"
		}
		if cfg.Engines == nil {
			cfg.Engines = &OpenEBSEngineConfig{}
		}
		if cfg.Engines.LocalHostPath == nil {
			cfg.Engines.LocalHostPath = &OpenEBSEngineLocalHostPathConfig{}
		}
		if cfg.Engines.LocalHostPath.Enabled == nil {
			cfg.Engines.LocalHostPath.Enabled = boolPtr(true)
		}
		if cfg.Engines.Mayastor == nil {
			cfg.Engines.Mayastor = &OpenEBSEngineMayastorConfig{Enabled: boolPtr(false)}
		}
		if cfg.Engines.Jiva == nil {
			cfg.Engines.Jiva = &OpenEBSEngineJivaConfig{Enabled: boolPtr(false)}
		}
		if cfg.Engines.CStor == nil {
			cfg.Engines.CStor = &OpenEBSEnginecStorConfig{Enabled: boolPtr(false)}
		}
	} else {
		if cfg.Engines != nil {
			if cfg.Engines.LocalHostPath != nil {
				cfg.Engines.LocalHostPath.Enabled = boolPtr(false)
			}
			if cfg.Engines.Mayastor != nil {
				cfg.Engines.Mayastor.Enabled = boolPtr(false)
			}
			if cfg.Engines.Jiva != nil {
				cfg.Engines.Jiva.Enabled = boolPtr(false)
			}
			if cfg.Engines.CStor != nil {
				cfg.Engines.CStor.Enabled = boolPtr(false)
			}
		}
	}
}

// Validate_StorageConfig validates StorageConfig.
func Validate_StorageConfig(cfg *StorageConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.OpenEBS != nil {
		Validate_OpenEBSConfig(cfg.OpenEBS, verrs, pathPrefix+".openebs")
	}
	if cfg.DefaultStorageClass != nil && strings.TrimSpace(*cfg.DefaultStorageClass) == "" {
		verrs.Add(pathPrefix+".defaultStorageClass", "cannot be empty if specified")
	}
}

// Validate_OpenEBSConfig validates OpenEBSConfig.
func Validate_OpenEBSConfig(cfg *OpenEBSConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.Enabled != nil && *cfg.Enabled {
		if strings.TrimSpace(cfg.BasePath) == "" {
			verrs.Add(pathPrefix+".basePath", "cannot be empty if OpenEBS is enabled")
		}

		if cfg.Version != nil {
			if strings.TrimSpace(*cfg.Version) == "" {
				verrs.Add(pathPrefix+".version", "cannot be only whitespace if specified")
			} else if !util.IsValidRuntimeVersion(*cfg.Version) {
				verrs.Add(pathPrefix+".version", fmt.Sprintf("'%s' is not a recognized version format", *cfg.Version))
			}
		}
	}
}
