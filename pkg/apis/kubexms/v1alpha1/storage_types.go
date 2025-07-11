package v1alpha1

import "strings"

// StorageConfig defines the storage configurations for the cluster.
type StorageConfig struct {
	OpenEBS             *OpenEBSConfig `json:"openebs,omitempty" yaml:"openebs,omitempty"`
	DefaultStorageClass *string        `json:"defaultStorageClass,omitempty" yaml:"defaultStorageClass,omitempty"`
}

// OpenEBSConfig defines settings for OpenEBS storage provisioner.
type OpenEBSConfig struct {
	BasePath string               `json:"basePath,omitempty" yaml:"basePath,omitempty"`
	Enabled  *bool                `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Version  *string              `json:"version,omitempty" yaml:"version,omitempty"`
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
	// ReplicaCount *int `json:"replicaCount,omitempty" yaml:"replicaCount,omitempty"` // Example suggested improvement
}

// OpenEBSEnginecStorConfig holds cStor specific settings.
type OpenEBSEnginecStorConfig struct { // Name kept as OpenEBSEnginecStorConfig due to existing references in doc
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	// BlockDevices []string `json:"blockDevices,omitempty" yaml:"blockDevices,omitempty"` // Example suggested improvement
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
	// Only initialize OpenEBS if the user provides an 'openebs: {}' block.
	// If cfg.OpenEBS is not nil here, it means user specified it.
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
		b := true // Default OpenEBS to enabled if the 'openebs:' block exists
		cfg.Enabled = &b
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
			defEngine := true
			cfg.Engines.LocalHostPath.Enabled = &defEngine
		}
		if cfg.Engines.Mayastor == nil {
			cfg.Engines.Mayastor = &OpenEBSEngineMayastorConfig{Enabled: pboolStorage(false)}
		}
		if cfg.Engines.Jiva == nil {
			cfg.Engines.Jiva = &OpenEBSEngineJivaConfig{Enabled: pboolStorage(false)}
		}
		if cfg.Engines.CStor == nil {
			cfg.Engines.CStor = &OpenEBSEnginecStorConfig{Enabled: pboolStorage(false)}
		}
	} else { // OpenEBS is explicitly disabled or not enabled by default
		if cfg.Engines != nil {
			if cfg.Engines.LocalHostPath != nil { cfg.Engines.LocalHostPath.Enabled = pboolStorage(false) }
			if cfg.Engines.Mayastor != nil { cfg.Engines.Mayastor.Enabled = pboolStorage(false) }
			if cfg.Engines.Jiva != nil { cfg.Engines.Jiva.Enabled = pboolStorage(false) }
			if cfg.Engines.CStor != nil { cfg.Engines.CStor.Enabled = pboolStorage(false) }
		}
	}
}

// Validate_StorageConfig validates StorageConfig.
func Validate_StorageConfig(cfg *StorageConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.OpenEBS != nil {
		Validate_OpenEBSConfig(cfg.OpenEBS, verrs, pathPrefix+".openebs")
	}
	if cfg.DefaultStorageClass != nil && strings.TrimSpace(*cfg.DefaultStorageClass) == "" {
		verrs.Add("%s.defaultStorageClass: cannot be empty if specified", pathPrefix)
	}
}

// Validate_OpenEBSConfig validates OpenEBSConfig.
func Validate_OpenEBSConfig(cfg *OpenEBSConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.Enabled != nil && *cfg.Enabled {
		if strings.TrimSpace(cfg.BasePath) == "" {
			verrs.Add("%s.basePath: cannot be empty if OpenEBS is enabled", pathPrefix)
		}
		// Further validation for Engines can be added here if specific rules apply
		// e.g. if only one engine can be enabled at a time.
		// if cfg.Engines != nil {
		// 	enabledCount := 0
		// 	if cfg.Engines.LocalHostPath != nil && cfg.Engines.LocalHostPath.Enabled != nil && *cfg.Engines.LocalHostPath.Enabled { enabledCount++ }
		// 	if cfg.Engines.Mayastor != nil && cfg.Engines.Mayastor.Enabled != nil && *cfg.Engines.Mayastor.Enabled { enabledCount++ }
		// 	if cfg.Engines.Jiva != nil && cfg.Engines.Jiva.Enabled != nil && *cfg.Engines.Jiva.Enabled { enabledCount++ }
		// 	if cfg.Engines.CStor != nil && cfg.Engines.CStor.Enabled != nil && *cfg.Engines.CStor.Enabled { enabledCount++ }
		// 	if enabledCount > 1 {
		// 		verrs.Add(pathPrefix+".engines", "only one OpenEBS engine can be enabled at a time")
		// 	}
		// }
	}
}

func pboolStorage(b bool) *bool { return &b } // Helper for pointer to bool

// Assuming ValidationErrors is defined in cluster_types.go or a shared util.
// NOTE: DeepCopy methods should be generated by controller-gen.
// Updated SetDefaults_OpenEBSConfig logic based on document's refinement.
// Renamed OpenEBSEnginecStorConfig to CStor for Go convention in struct field name, but kept original in type name for doc consistency.
// Kept pboolStorage helper local as it's small and specific.
// Added import "strings".
