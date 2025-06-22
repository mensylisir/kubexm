package v1alpha1

import "strings"

// StorageConfig defines the storage configurations for the cluster.
// For now, it primarily includes OpenEBS settings based on KubeKey examples.
// This can be expanded to include other storage provisioners or settings.
type StorageConfig struct {
	OpenEBS             *OpenEBSConfig `json:"openebs,omitempty" yaml:"openebs,omitempty"`
	DefaultStorageClass *string        `json:"defaultStorageClass,omitempty" yaml:"defaultStorageClass,omitempty"`
}

// OpenEBSConfig defines settings for OpenEBS storage provisioner.
type OpenEBSConfig struct {
	BasePath string                   `json:"basePath,omitempty" yaml:"basePath,omitempty"`
	Enabled  *bool                    `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Version  *string                  `json:"version,omitempty" yaml:"version,omitempty"`
	Engines  *OpenEBSEngineConfig     `json:"engines,omitempty" yaml:"engines,omitempty"`
}

type OpenEBSEngineConfig struct {
	Mayastor      *OpenEBSEngineMayastorConfig      `json:"mayastor,omitempty" yaml:"mayastor,omitempty"`
	Jiva          *OpenEBSEngineJivaConfig          `json:"jiva,omitempty" yaml:"jiva,omitempty"`
	cStor         *OpenEBSEnginecStorConfig         `json:"cstor,omitempty" yaml:"cstor,omitempty"`
	LocalHostPath *OpenEBSEngineLocalHostPathConfig `json:"localHostPath,omitempty" yaml:"localHostPath,omitempty"`
}

type OpenEBSEngineMayastorConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"` /* TODO: Mayastor specific fields */
}
type OpenEBSEngineJivaConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"` /* TODO: Jiva specific fields */
}
type OpenEBSEnginecStorConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"` /* TODO: cStor specific fields */
}
type OpenEBSEngineLocalHostPathConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"` /* BasePath above is related */
}

// --- Defaulting Functions ---

// SetDefaults_StorageConfig sets default values for StorageConfig.
func SetDefaults_StorageConfig(cfg *StorageConfig) {
	if cfg == nil {
		return
	}
	if cfg.OpenEBS == nil {
		// Only initialize OpenEBS if it's intended to be the default or if explicitly added.
		// For now, let's assume it's only defaulted if the user includes an 'openebs: {}' section.
		// A more proactive default might be: cfg.OpenEBS = &OpenEBSConfig{}
		// Based on revised plan, if StorageConfig itself exists, OpenEBSConfig can be initialized here.
		// However, the plan also revised SetDefaults_Cluster to always init StorageConfig.
		// So, if OpenEBS is the only option for now, it's reasonable to init it here too.
		// Let's not initialize it by default here, but let SetDefaults_OpenEBSConfig handle its own if called on nil.
		// This means if user wants OpenEBS, they MUST provide `storage: { openebs: {} }` at minimum.
		// OR, SetDefaults_Cluster should init Storage.OpenEBS if Storage itself is present.
		// The current plan initializes Storage in SetDefaults_Cluster, then calls this.
		// So, if cfg.OpenEBS is nil here, it means user didn't specify "openebs: {}"
		// If they did, it would be an empty struct, not nil.
	}
	// If OpenEBS section exists (even if empty), apply its defaults.
	if cfg.OpenEBS != nil {
	   SetDefaults_OpenEBSConfig(cfg.OpenEBS)
	}
	// No default for DefaultStorageClass, it's purely optional.
}

// SetDefaults_OpenEBSConfig sets default values for OpenEBSConfig.
func SetDefaults_OpenEBSConfig(cfg *OpenEBSConfig) {
	if cfg == nil {
		return
	}
	// If the openebs block is present in YAML, cfg won't be nil.
	// In this case, we default Enabled to true if it's not specified by the user.
	if cfg.Enabled == nil {
	   b := true // Default OpenEBS to enabled if the 'openebs:' block exists
	   cfg.Enabled = &b
	}

	if cfg.Enabled != nil && *cfg.Enabled { // If OpenEBS is effectively enabled
		if cfg.BasePath == "" {
			// Only default BasePath if OpenEBS is being enabled and no path is set.
			cfg.BasePath = "/var/openebs/local"
		}
		if cfg.Engines == nil { cfg.Engines = &OpenEBSEngineConfig{} }
		if cfg.Engines.LocalHostPath == nil { cfg.Engines.LocalHostPath = &OpenEBSEngineLocalHostPathConfig{} }
		if cfg.Engines.LocalHostPath.Enabled == nil {
			defEngine := true; // Default LocalHostPath engine to true if OpenEBS is enabled
			cfg.Engines.LocalHostPath.Enabled = &defEngine
		}
		// Mayastor, Jiva, cStor default to disabled unless specified by user
		if cfg.Engines.Mayastor == nil { cfg.Engines.Mayastor = &OpenEBSEngineMayastorConfig{Enabled: pboolStorage(false)} }
		if cfg.Engines.Jiva == nil { cfg.Engines.Jiva = &OpenEBSEngineJivaConfig{Enabled: pboolStorage(false)} }
		if cfg.Engines.cStor == nil { cfg.Engines.cStor = &OpenEBSEnginecStorConfig{Enabled: pboolStorage(false)} }
	} else { // OpenEBS is explicitly disabled (cfg.Enabled is not nil and is false)
		// If OpenEBS is disabled, ensure sub-engines are also marked as disabled if they exist.
		// This handles the case where a user might have `enabled: false` at top level
		// but still has engine blocks defined.
		if cfg.Engines != nil {
			if cfg.Engines.LocalHostPath != nil { cfg.Engines.LocalHostPath.Enabled = pboolStorage(false) }
			if cfg.Engines.Mayastor != nil { cfg.Engines.Mayastor.Enabled = pboolStorage(false) }
			if cfg.Engines.Jiva != nil { cfg.Engines.Jiva.Enabled = pboolStorage(false) }
			if cfg.Engines.cStor != nil { cfg.Engines.cStor.Enabled = pboolStorage(false) }
		}
	}
}

// --- Validation Functions ---

// Validate_StorageConfig validates StorageConfig.
// Note: ValidationErrors type is expected to be defined in cluster_types.go or a common errors file.
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
	// Validate other storage types if added.
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
	   // Could add validation for path format if necessary.
	   // No specific validation for Engines sub-fields yet, beyond them being optional.
	}
}

func pboolStorage(b bool) *bool { return &b }
