package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"path"
	"strings"
)

type Storage struct {
	DefaultStorageClass *string         `json:"defaultStorageClass,omitempty" yaml:"defaultStorageClass,omitempty"`
	OpenEBS             *OpenEBSConfig  `json:"openebs,omitempty" yaml:"openebs,omitempty"`
	NFS                 *NFSConfig      `json:"nfs,omitempty" yaml:"nfs,omitempty"`
	RookCeph            *RookCephConfig `json:"rookCeph,omitempty" yaml:"rookCeph,omitempty"`
	Longhorn            *LonghornConfig `json:"longhorn,omitempty" yaml:"longhorn,omitempty"`
}

type OpenEBSConfig struct {
	Source  AddonSource          `json:"source,omitempty" yaml:"sources,omitempty"`
	Enabled *bool                `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Version string               `json:"version,omitempty" yaml:"version,omitempty"`
	Engines *OpenEBSEngineConfig `json:"engines,omitempty" yaml:"engines,omitempty"`
}

type OpenEBSEngineConfig struct {
	Mayastor      *MayastorEngineConfig      `json:"mayastor,omitempty" yaml:"mayastor,omitempty"`
	Jiva          *JivaEngineConfig          `json:"jiva,omitempty" yaml:"jiva,omitempty"`
	CStor         *CStorEngineConfig         `json:"cstor,omitempty" yaml:"cstor,omitempty"`
	LocalHostpath *LocalHostpathEngineConfig `json:"localHostpath,omitempty" yaml:"localHostpath,omitempty"`
}

type MayastorEngineConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

type JivaEngineConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

type CStorEngineConfig struct {
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

type LocalHostpathEngineConfig struct {
	Enabled  *bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	BasePath *string `json:"basePath,omitempty" yaml:"basePath,omitempty"`
}

type NFSConfig struct {
	Source           AddonSource `json:"source,omitempty" yaml:"sources,omitempty"`
	Enabled          *bool       `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Server           string      `json:"server,omitempty" yaml:"server,omitempty"`
	Path             string      `json:"path,omitempty" yaml:"path,omitempty"`
	StorageClassName *string     `json:"storageClassName,omitempty" yaml:"storageClassName,omitempty"`
}

type RookCephConfig struct {
	Source           AddonSource `json:"source,omitempty" yaml:"sources,omitempty"`
	Enabled          *bool       `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Version          string      `json:"version,omitempty" yaml:"version,omitempty"`
	Devices          []string    `json:"devices,omitempty" yaml:"devices,omitempty"`
	UseAllDevices    *bool       `json:"useAllDevices,omitempty" yaml:"useAllDevices,omitempty"`
	CreateBlockPool  *bool       `json:"createBlockPool,omitempty" yaml:"createBlockPool,omitempty"`
	CreateFilesystem *bool       `json:"createFilesystem,omitempty" yaml:"createFilesystem,omitempty"`
}

type LonghornConfig struct {
	Source               AddonSource `json:"source,omitempty" yaml:"sources,omitempty"`
	Enabled              *bool       `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Version              string      `json:"version,omitempty" yaml:"version,omitempty"`
	DefaultDataPath      *string     `json:"defaultDataPath,omitempty" yaml:"defaultDataPath,omitempty"`
	PurgeDataOnUninstall *bool       `json:"purgeDataOnUninstall,omitempty" yaml:"purgeDataOnUninstall,omitempty"`
}

func SetDefaults_Storage(cfg *Storage) {
	if cfg == nil {
		return
	}
	if cfg.DefaultStorageClass == nil {
		cfg.DefaultStorageClass = helpers.StrPtr(common.DefaultStorageClass)
	}

	if cfg.OpenEBS == nil {
		cfg.OpenEBS = &OpenEBSConfig{}
	}
	SetDefaults_OpenEBS(cfg.OpenEBS)

	if cfg.NFS == nil {
		cfg.NFS = &NFSConfig{}
	}
	SetDefaults_NFS(cfg.NFS)

	if cfg.RookCeph == nil {
		cfg.RookCeph = &RookCephConfig{}
	}
	SetDefaults_RookCeph(cfg.RookCeph)

	if cfg.Longhorn == nil {
		cfg.Longhorn = &LonghornConfig{}
	}
	SetDefaults_Longhorn(cfg.Longhorn)
}

func SetDefaults_OpenEBS(cfg *OpenEBSConfig) {
	if cfg.Enabled == nil {
		cfg.Enabled = helpers.BoolPtr(common.DefaultOpenEBSConfigEnabled)
	}

	if *cfg.Enabled {
		if cfg.Engines == nil {
			cfg.Engines = &OpenEBSEngineConfig{}
		}
		if cfg.Engines.LocalHostpath == nil {
			cfg.Engines.LocalHostpath = &LocalHostpathEngineConfig{}
		}
		if cfg.Engines.LocalHostpath.Enabled == nil {
			cfg.Engines.LocalHostpath.Enabled = helpers.BoolPtr(true)
			cfg.Engines.LocalHostpath.BasePath = helpers.StrPtr(common.DefaultLocalHostpathEngineConfigBasePath)
		}
		if cfg.Version == "" {
			cfg.Version = common.DefaultOpenEBSConfigVersion
		}
	}
}

func SetDefaults_NFS(cfg *NFSConfig) {
	if cfg.Enabled == nil {
		cfg.Enabled = helpers.BoolPtr(common.DefaultNFSConfigEnabled)
	}
	if *cfg.Enabled && cfg.StorageClassName == nil {
		cfg.StorageClassName = helpers.StrPtr(common.DefaultNFSConfigStorageClassName)
	}
}

func SetDefaults_RookCeph(cfg *RookCephConfig) {
	if cfg.Enabled == nil {
		cfg.Enabled = helpers.BoolPtr(common.DefaultRookCephConfigEnabled)
	}
	if *cfg.Enabled {
		if cfg.UseAllDevices == nil {
			cfg.UseAllDevices = helpers.BoolPtr(common.DefaultRookCephConfigUseAllDevices)
		}
		if cfg.CreateBlockPool == nil {
			cfg.CreateBlockPool = helpers.BoolPtr(common.DefaultRookCephConfigCreateBlockPool)
		}
		if cfg.CreateFilesystem == nil {
			cfg.CreateFilesystem = helpers.BoolPtr(common.DefaultRookCephConfigCreateFilesystem)
		}
		if cfg.Version == "" {
			cfg.Version = common.DefaultRookCephConfigVersion
		}
	}
}

func SetDefaults_Longhorn(cfg *LonghornConfig) {
	if cfg.Enabled == nil {
		cfg.Enabled = helpers.BoolPtr(common.DefaultLonghornConfigEnabled)
	}
	if *cfg.Enabled && cfg.DefaultDataPath == nil {
		cfg.DefaultDataPath = helpers.StrPtr(common.DefaultLonghornConfigDefaultDataPath)
	}
	if cfg.Version == "" {
		cfg.Version = common.DefaultLonghornConfigVersion
	}
}

func Validate_Storage(cfg *Storage, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	p := path.Join(pathPrefix)

	enabledProviders := []string{}
	if cfg.OpenEBS != nil && cfg.OpenEBS.Enabled != nil && *cfg.OpenEBS.Enabled {
		enabledProviders = append(enabledProviders, common.StorageComponentOpenEBS)
		Validate_OpenEBS(cfg.OpenEBS, verrs, path.Join(p, common.StorageComponentOpenEBS))
	}
	if cfg.NFS != nil && cfg.NFS.Enabled != nil && *cfg.NFS.Enabled {
		enabledProviders = append(enabledProviders, common.StorageComponentNFS)
		Validate_NFS(cfg.NFS, verrs, path.Join(p, common.StorageComponentNFS))
	}
	if cfg.RookCeph != nil && cfg.RookCeph.Enabled != nil && *cfg.RookCeph.Enabled {
		enabledProviders = append(enabledProviders, common.StorageComponentRookCeph)
		Validate_RookCeph(cfg.RookCeph, verrs, path.Join(p, common.StorageComponentRookCeph))
	}
	if cfg.Longhorn != nil && cfg.Longhorn.Enabled != nil && *cfg.Longhorn.Enabled {
		enabledProviders = append(enabledProviders, common.StorageComponentLonghorn)
		Validate_Longhorn(cfg.Longhorn, verrs, path.Join(p, common.StorageComponentLonghorn))
	}

	if cfg.DefaultStorageClass != nil && *cfg.DefaultStorageClass != "" {
		if len(enabledProviders) == 0 {
			verrs.Add(fmt.Sprintf("%s.defaultStorageClass: cannot be set when no storage provider is enabled", p))
		}
	}
}

func Validate_OpenEBS(cfg *OpenEBSConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg.Engines == nil {
		verrs.Add(pathPrefix + ".engines: must be defined when openebs is enabled")
		return
	}

	enginesEnabled := 0
	if cfg.Engines.Mayastor != nil && cfg.Engines.Mayastor.Enabled != nil && *cfg.Engines.Mayastor.Enabled {
		enginesEnabled++
	}
	if cfg.Engines.Jiva != nil && cfg.Engines.Jiva.Enabled != nil && *cfg.Engines.Jiva.Enabled {
		enginesEnabled++
	}
	if cfg.Engines.CStor != nil && cfg.Engines.CStor.Enabled != nil && *cfg.Engines.CStor.Enabled {
		enginesEnabled++
	}
	if cfg.Engines.LocalHostpath != nil && cfg.Engines.LocalHostpath.Enabled != nil && *cfg.Engines.LocalHostpath.Enabled {
		enginesEnabled++
		if cfg.Engines.LocalHostpath.BasePath != nil && strings.TrimSpace(*cfg.Engines.LocalHostpath.BasePath) == "" {
			verrs.Add(path.Join(pathPrefix, "engines", "localHostpath") + ".basePath: cannot be empty if specified")
		}
	}

	if enginesEnabled == 0 {
		verrs.Add(pathPrefix + ".engines: at least one storage engine must be enabled")
	}
}

func Validate_NFS(cfg *NFSConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if strings.TrimSpace(cfg.Server) == "" {
		verrs.Add(pathPrefix + ".server: is required when nfs is enabled")
	}
	if strings.TrimSpace(cfg.Path) == "" {
		verrs.Add(pathPrefix + ".path: is required when nfs is enabled")
	}
	if cfg.StorageClassName != nil && strings.TrimSpace(*cfg.StorageClassName) == "" {
		verrs.Add(pathPrefix + ".storageClassName: cannot be empty if specified")
	}
}

func Validate_RookCeph(cfg *RookCephConfig, verrs *validation.ValidationErrors, pathPrefix string) {

	useAll := cfg.UseAllDevices != nil && *cfg.UseAllDevices
	useList := len(cfg.Devices) > 0

	if !useAll && !useList {
		verrs.Add(pathPrefix + ": either 'useAllDevices' must be true or at least one device must be specified in 'devices'")
	}

	if useAll && useList {
		verrs.Add(pathPrefix + ": 'devices' list cannot be set when 'useAllDevices' is true")
	}
}

func Validate_Longhorn(cfg *LonghornConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg.DefaultDataPath != nil && strings.TrimSpace(*cfg.DefaultDataPath) == "" {
		verrs.Add(pathPrefix + ".defaultDataPath: cannot be empty if specified")
	}
}
