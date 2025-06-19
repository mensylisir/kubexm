package v1alpha1

import "strings"

// StorageConfig defines the storage configurations for the cluster.
// For now, it primarily includes OpenEBS settings based on KubeKey examples.
// This can be expanded to include other storage provisioners or settings.
type StorageConfig struct {
	OpenEBS *OpenEBSConfig `json:"openebs,omitempty"`
	// DefaultStorageClass an optional field to set the default storage class for the cluster.
	// If not set, the system might not have a default, or one might be set by a storage provisioner.
	DefaultStorageClass *string `json:"defaultStorageClass,omitempty"`
	// TODO: Add configurations for other storage provisioners like Ceph, NFS, LocalVolumeProvisioner
	// e.g. LocalVolumeProvisioner *LocalVolumeProvisionerConfig `json:"localVolumeProvisioner,omitempty"`
}

// OpenEBSConfig defines settings for OpenEBS storage provisioner.
type OpenEBSConfig struct {
	BasePath string `json:"basePath,omitempty"`
	Enabled *bool `json:"enabled,omitempty"`
	// Version of OpenEBS to install.
	Version *string `json:"version,omitempty"`
	// Engines allows specifying which OpenEBS engines to enable or configure.
	// Example: Engines: { Mayastor: {Enabled: true}, cStor: {Enabled: false}}
	Engines *OpenEBSEngineConfig `json:"engines,omitempty"`
}

type OpenEBSEngineConfig struct {
	Mayastor *OpenEBSEngineMayastorConfig `json:"mayastor,omitempty"`
	Jiva     *OpenEBSEngineJivaConfig     `json:"jiva,omitempty"`
	cStor    *OpenEBSEnginecStorConfig    `json:"cstor,omitempty"`
	// LocalHostPath is typically for hostPath based provisioning.
	LocalHostPath *OpenEBSEngineLocalHostPathConfig `json:"localHostPath,omitempty"`
}

type OpenEBSEngineMayastorConfig struct { Enabled *bool `json:"enabled,omitempty"` /* TODO: Mayastor specific fields */ }
type OpenEBSEngineJivaConfig struct { Enabled *bool `json:"enabled,omitempty"` /* TODO: Jiva specific fields */ }
type OpenEBSEnginecStorConfig struct { Enabled *bool `json:"enabled,omitempty"` /* TODO: cStor specific fields */ }
type OpenEBSEngineLocalHostPathConfig struct { Enabled *bool `json:"enabled,omitempty"` /* BasePath above is related */ }

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
	if cfg == nil { // Should not happen if called from SetDefaults_StorageConfig correctly
		return
	}
	if cfg.Enabled == nil {
	   b := false // Default OpenEBS to disabled unless explicitly enabled
	   cfg.Enabled = &b
	}
	if cfg.Enabled != nil && *cfg.Enabled && cfg.BasePath == "" {
		// Only default BasePath if OpenEBS is being enabled and no path is set.
		cfg.BasePath = "/var/openebs/local"
	}
	if cfg.Enabled != nil && *cfg.Enabled { // Only default engines if OpenEBS is enabled
		if cfg.Engines == nil { cfg.Engines = &OpenEBSEngineConfig{} }
		if cfg.Engines.LocalHostPath == nil { cfg.Engines.LocalHostPath = &OpenEBSEngineLocalHostPathConfig{} }
		if cfg.Engines.LocalHostPath.Enabled == nil { def := true; cfg.Engines.LocalHostPath.Enabled = &def } // Default LocalHostPath engine if OpenEBS enabled
		// Mayastor, Jiva, cStor default to disabled unless specified
		if cfg.Engines.Mayastor == nil { cfg.Engines.Mayastor = &OpenEBSEngineMayastorConfig{Enabled: pboolStorage(false)} }
		if cfg.Engines.Jiva == nil { cfg.Engines.Jiva = &OpenEBSEngineJivaConfig{Enabled: pboolStorage(false)} }
		if cfg.Engines.cStor == nil { cfg.Engines.cStor = &OpenEBSEnginecStorConfig{Enabled: pboolStorage(false)} }
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
