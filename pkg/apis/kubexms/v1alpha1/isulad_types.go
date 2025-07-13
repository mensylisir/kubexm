package v1alpha1

import (
	"encoding/base64"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"k8s.io/apimachinery/pkg/runtime"
	"strings"
)

type Isulad struct {
	Version            string                         `json:"version,omitempty" yaml:"version,omitempty"`
	Endpoint           string                         `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
	DataRoot           *string                        `json:"data-root,omitempty" yaml:"data-root,omitempty"`
	StorageDriver      *string                        `json:"storage-driver,omitempty" yaml:"storage-driver,omitempty"`
	StorageOpts        []string                       `json:"storage-opts,omitempty" yaml:"storage-opts,omitempty"`
	RegistryMirrors    []string                       `json:"registry-mirrors,omitempty" yaml:"registry-mirrors,omitempty"`
	InsecureRegistries []string                       `json:"insecure-registries,omitempty" yaml:"insecure-registries,omitempty"`
	Auths              map[ServerAddress]RegistryAuth `json:"auths,omitempty" yaml:"auths,omitempty"`
	CgroupParent       *string                        `json:"cgroup-parent,omitempty" yaml:"cgroup-parent,omitempty"`
	CgroupManager      *string                        `json:"cgroupManager,omitempty" yaml:"cgroupManager,omitempty"`
	LogLevel           *string                        `json:"log-level,omitempty" yaml:"log-level,omitempty"`
	LogDriver          *string                        `json:"log-driver,omitempty" yaml:"log-driver,omitempty"`
	LogOpts            map[string]string              `json:"log-opts,omitempty" yaml:"log-opts,omitempty"`
	NetworkPlugin      *string                        `json:"network-plugin,omitempty" yaml:"network-plugin,omitempty"`
	CniConfDir         *string                        `json:"cni-conf-dir,omitempty" yaml:"cni-conf-dir,omitempty"`
	CniBinDir          *string                        `json:"cni-bin-dir,omitempty" yaml:"cni-bin-dir,omitempty"`
	PidFile            *string                        `json:"pid-file,omitempty" yaml:"pid-file,omitempty"`
	HooksDir           *string                        `json:"hooks-dir,omitempty" yaml:"hooks-dir,omitempty"`
	ExtraJSONConfig    *runtime.RawExtension          `json:"extraJsonConfig,omitempty" yaml:"extraJsonConfig,omitempty"`
	Pause              string                         `json:"pause,omitempty" yaml:"pause,omitempty"`
}

func SetDefaults_IsuladConfig(cfg *Isulad) {
	if cfg == nil {
		return
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = common.IsuladDefaultEndpoint
	}
	if cfg.StorageOpts == nil {
		cfg.StorageOpts = []string{}
	}
	if cfg.RegistryMirrors == nil {
		cfg.RegistryMirrors = []string{}
	}
	if cfg.InsecureRegistries == nil {
		cfg.InsecureRegistries = []string{}
	}
	if cfg.Auths == nil {
		cfg.Auths = make(map[ServerAddress]RegistryAuth)
	}
	if cfg.CgroupManager == nil {
		cfg.CgroupManager = helpers.StrPtr(common.CgroupDriverSystemd)
	}
	if cfg.LogLevel == nil {
		cfg.LogLevel = helpers.StrPtr("info")
	}
	if cfg.LogDriver == nil {
		cfg.LogDriver = helpers.StrPtr("json-file")
	}
	if cfg.CniConfDir == nil {
		cfg.CniConfDir = helpers.StrPtr(common.DefaultCNIConfDirTarget)
	}
	if cfg.CniBinDir == nil {
		cfg.CniBinDir = helpers.StrPtr(common.DefaultCNIBinDirTarget)
	}
	if cfg.PidFile == nil {
		cfg.PidFile = helpers.StrPtr(common.IsuladDefaultPidFile)
	}
	if cfg.Version == "" {
		cfg.Version = common.IsuladDefaultVersion
	}
	if cfg.DataRoot == nil {
		cfg.DataRoot = helpers.StrPtr(common.IsuladDefaultDataRoot)
	}
	if cfg.LogOpts == nil {
		cfg.LogOpts = map[string]string{
			"max-size": common.IsuladLogOptMaxSizeDefault,
			"max-file": common.IsuladLogOptMaxFileDefault,
		}
	}
	if cfg.Pause == "" {
		cfg.Pause = common.DefaultPauseImage
	}
}

func Validate_IsuladConfig(cfg *Isulad, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.Version != "" && !helpers.IsValidRuntimeVersion(cfg.Version) {
		verrs.Add(pathPrefix+".version", "invalid version format")
	}
	if cfg.CgroupManager != nil {
		if *cfg.CgroupManager != common.CgroupDriverSystemd && *cfg.CgroupManager != common.CgroupDriverCgroupfs {
			verrs.Add(pathPrefix+".cgroupManager", "must be 'systemd' or 'cgroupfs'")
		}
	}
	if cfg.LogLevel != nil {
		validLevels := []string{"debug", "info", "warn", "error", "fatal"}
		if !helpers.IsInStringSlice(validLevels, *cfg.LogLevel) {
			verrs.Add(pathPrefix+".logLevel", fmt.Sprintf("invalid log level, must be one of %v", validLevels))
		}
	}
	for i, mirror := range cfg.RegistryMirrors {
		if !helpers.IsValidURL(mirror) {
			verrs.Add(fmt.Sprintf("%s.registryMirrors[%d]", pathPrefix, i), "invalid URL format")
		}
	}
	for i, insecureReg := range cfg.InsecureRegistries {
		regPath := fmt.Sprintf("%s.insecureRegistries[%d]", pathPrefix, i)
		if !helpers.ValidateHostPortString(insecureReg) && !helpers.IsValidDomainName(insecureReg) {
			verrs.Add(regPath, ": invalid host:port or domain name format for insecure registry '"+insecureReg+"'")
		}
	}
	if cfg.Pause != "" {
		pausePath := pathPrefix + ".pause"
		if strings.TrimSpace(cfg.Pause) == "" {
			verrs.Add(pausePath, "cannot be only whitespace if specified")
		} else if !helpers.IsValidImageReference(cfg.Pause) {
			verrs.Add(pausePath, "invalid image reference format: '"+cfg.Pause+"'")
		}
	}
	if cfg.DataRoot != nil && strings.TrimSpace(*cfg.DataRoot) == "" {
		verrs.Add(pathPrefix+".data-root", "cannot be empty if specified")
	}
	Validate_RegistryAuths(cfg.Auths, verrs, pathPrefix+".auths")
}

func Validate_RegistryAuths(auths map[ServerAddress]RegistryAuth, verrs *ValidationErrors, pathPrefix string) {
	for regAddr, auth := range auths {
		authEntryPath := fmt.Sprintf("%s[\"%s\"]", pathPrefix, regAddr)
		if strings.TrimSpace(string(regAddr)) == "" {
			verrs.Add(pathPrefix, "registry address key cannot be empty")
			continue
		}
		if !helpers.ValidateHostPortString(string(regAddr)) && !helpers.IsValidDomainName(string(regAddr)) {
			verrs.Add(authEntryPath, ": registry key '"+string(regAddr)+"' is not a valid hostname or host:port")
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
