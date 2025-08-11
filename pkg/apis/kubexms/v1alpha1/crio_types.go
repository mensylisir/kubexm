package v1alpha1

import (
	"encoding/base64"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"strings"
)

type Crio struct {
	Endpoint        string                         `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	Version         string                         `json:"version,omitempty" yaml:"version,omitempty"`
	Conmon          *string                        `json:"conmon,omitempty" yaml:"conmon,omitempty"`
	CgroupDriver    *string                        `json:"cgroupManager,omitempty" yaml:"cgroupManager,omitempty"`
	StorageDriver   *string                        `json:"storageDriver,omitempty" yaml:"storageDriver,omitempty"`
	StorageOption   []string                       `json:"storageOption,omitempty" yaml:"storageOption,omitempty"`
	LogLevel        *string                        `json:"logLevel,omitempty" yaml:"logLevel,omitempty"`
	LogFilter       string                         `json:"logFilter,omitempty" yaml:"logFilter,omitempty"`
	ManageNetwork   *bool                          `json:"manageNetwork,omitempty" yaml:"manageNetwork,omitempty"`
	NetworkDir      *string                        `json:"networkDir,omitempty" yaml:"networkDir,omitempty"`
	PluginDirs      []string                       `json:"pluginDirs,omitempty" yaml:"pluginDirs,omitempty"`
	Runtimes        map[string]CrioRuntime         `json:"runtimes,omitempty" yaml:"runtimes,omitempty"`
	Registry        *CrioRegistry                  `json:"registry,omitempty" yaml:"registry,omitempty"`
	ExtraTomlConfig string                         `json:"extraTomlConfig,omitempty" yaml:"extraTomlConfig,omitempty"`
	Root            *string                        `json:"root,omitempty" yaml:"root,omitempty"`
	Runroot         *string                        `json:"runroot,omitempty" yaml:"runroot,omitempty"`
	Auths           map[ServerAddress]RegistryAuth `json:"auths,omitempty" yaml:"auths,omitempty"`
	Pause           string                         `json:"pause,omitempty" yaml:"pause,omitempty"`
}

type CrioRuntime struct {
	RuntimePath string `json:"runtime_path,omitempty" yaml:"runtime_path,omitempty"`
	RuntimeType string `json:"runtime_type,omitempty" yaml:"runtime_type,omitempty"`
	RuntimeRoot string `json:"runtime_root,omitempty" yaml:"runtime_root,omitempty"`
}

type CrioRegistry struct {
	UnqualifiedSearchRegistries []string         `json:"unqualifiedSearchRegistries,omitempty" yaml:"unqualifiedSearchRegistries,omitempty"`
	Registries                  []RegistryMirror `json:"registries,omitempty" yaml:"registries,omitempty"`
}

type RegistryMirror struct {
	Prefix   string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	Location string `json:"location,omitempty" yaml:"location,omitempty"`
	Insecure *bool  `json:"insecure,omitempty" yaml:"insecure,omitempty"`
	Blocked  *bool  `json:"blocked,omitempty" yaml:"blocked,omitempty"`
}

func SetDefaults_CrioConfig(cfg *Crio) {
	if cfg == nil {
		return
	}
	if cfg.CgroupDriver == nil {
		cfg.CgroupDriver = helpers.StrPtr(common.CgroupDriverSystemd)
	}
	if cfg.StorageOption == nil {
		cfg.StorageOption = []string{}
	}
	if cfg.LogLevel == nil {
		cfg.LogLevel = helpers.StrPtr("warn")
	}
	if cfg.ManageNetwork == nil {
		cfg.ManageNetwork = helpers.BoolPtr(false)
	}
	if cfg.NetworkDir == nil {
		cfg.NetworkDir = helpers.StrPtr(common.DefaultCNIBinDirTarget)
	}
	if cfg.PluginDirs == nil {
		cfg.PluginDirs = []string{common.DefaultCNIBinDirTarget}
	}
	if cfg.Runtimes == nil {
		cfg.Runtimes = make(map[string]CrioRuntime)
		cfg.Runtimes["runc"] = CrioRuntime{
			RuntimePath: common.DefaultRuncPath,
			RuntimeType: common.DefaultRuntimeType,
			RuntimeRoot: common.DefaultRuntimeRoot,
		}
	}
	if cfg.Registry == nil {
		cfg.Registry = &CrioRegistry{}
	}
	if cfg.Registry.UnqualifiedSearchRegistries == nil {
		cfg.Registry.UnqualifiedSearchRegistries = []string{"docker.io", "quay.io"}
	}
	if cfg.Registry.Registries == nil {
		cfg.Registry.Registries = []RegistryMirror{}
	}
	if cfg.Version == "" {
		cfg.Version = common.CRIODefaultVersion
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = common.CRIODefaultEndpoint
	}
	if cfg.Root == nil {
		cfg.Root = helpers.StrPtr(common.CRIODefaultGraphRoot)
	}
	if cfg.Runroot == nil {
		cfg.Runroot = helpers.StrPtr(common.CRIODefaultRunRoot)
	}
	if cfg.Auths == nil {
		cfg.Auths = make(map[ServerAddress]RegistryAuth)
	}
	if cfg.Pause == "" {
		cfg.Pause = common.DefaultPauseImage
	}
}

func Validate_CrioConfig(cfg *Crio, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.Version != "" {
		if !helpers.IsValidRuntimeVersion(cfg.Version) {
			verrs.Add(pathPrefix+".version", "invalid version format")
		}
	}
	if cfg.CgroupDriver != nil {
		if *cfg.CgroupDriver != common.CgroupDriverSystemd && *cfg.CgroupDriver != common.CgroupDriverCgroupfs {
			verrs.Add(pathPrefix+".cgroupManager", "must be 'systemd' or 'cgroupfs'")
		}
	}
	if cfg.LogLevel != nil {
		validLevels := []string{"fatal", "panic", "error", "warn", "info", "debug"}
		if !helpers.IsInStringSlice(validLevels, *cfg.LogLevel) {
			verrs.Add(pathPrefix+".logLevel", "invalid log level")
		}
	}
	for name, rt := range cfg.Runtimes {
		if strings.TrimSpace(name) == "" {
			verrs.Add(pathPrefix+".runtimes", "runtime name cannot be empty")
		}
		if rt.RuntimePath == "" {
			verrs.Add(fmt.Sprintf("%s.runtimes['%s'].runtime_path", pathPrefix, name), "runtime_path cannot be empty")
		}
	}

	if cfg.Registry != nil {
		registryPath := pathPrefix + ".registry"
		for i, reg := range cfg.Registry.UnqualifiedSearchRegistries {
			if strings.TrimSpace(reg) == "" {
				verrs.Add(fmt.Sprintf("%s.unqualifiedSearchRegistries[%d]", registryPath, i), "registry name cannot be empty")
			}
		}
		for i, mirror := range cfg.Registry.Registries {
			mirrorPath := fmt.Sprintf("%s.registries[%d]", registryPath, i)
			if mirror.Prefix == "" && mirror.Location == "" {
				verrs.Add(mirrorPath, "either prefix or location must be specified")
			}
		}
	}

	if cfg.Root != nil && strings.TrimSpace(*cfg.Root) == "" {
		verrs.Add(pathPrefix+".root", "cannot be empty if specified")
	}
	if cfg.Runroot != nil && strings.TrimSpace(*cfg.Runroot) == "" {
		verrs.Add(pathPrefix+".runroot", "cannot be empty if specified")
	}

	if cfg.Pause != "" {
		pausePath := pathPrefix + ".pause"
		if strings.TrimSpace(cfg.Pause) == "" {
			verrs.Add(pausePath, "cannot be only whitespace if specified")
		} else if !helpers.IsValidImageReference(cfg.Pause) {
			verrs.Add(pausePath, "invalid image reference format: '"+cfg.Pause+"'")
		}
	}

	for regAddr, auth := range cfg.Auths {
		authMapPath := pathPrefix + ".auths"
		authEntryPath := fmt.Sprintf("%s[\"%s\"]", authMapPath, regAddr)
		if strings.TrimSpace(string(regAddr)) == "" {
			verrs.Add(authMapPath, "registry address key cannot be empty")
		} else if !helpers.ValidateHostPortString(string(regAddr)) && !helpers.IsValidDomainName(string(regAddr)) {
			verrs.Add(authEntryPath + ": registry key '" + string(regAddr) + "' is not a valid hostname or host:port")
		}

		hasUserPass := auth.Username != "" && auth.Password != ""
		hasAuthStr := auth.Auth != ""

		if !hasUserPass && !hasAuthStr {
			verrs.Add(authEntryPath, "either username/password or auth string must be provided")
		}
		if hasAuthStr {
			decoded, err := base64.StdEncoding.DecodeString(auth.Auth)
			if err != nil {
				verrs.Add(authEntryPath + ".auth: failed to decode base64 auth string: " + fmt.Sprintf("%v", err))
			} else if !strings.Contains(string(decoded), ":") {
				verrs.Add(authEntryPath+".auth", "decoded auth string must be in 'username:password' format")
			}
		}
	}
}
