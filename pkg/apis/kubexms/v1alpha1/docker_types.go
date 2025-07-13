package v1alpha1

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"k8s.io/apimachinery/pkg/runtime"
)

type DockerAddressPool struct {
	Base string `json:"base" yaml:"base"` // Base IP address for the pool (e.g., "172.30.0.0/16")
	Size int    `json:"size" yaml:"size"` // Size of the subnets to allocate from the base pool (e.g., 24 for /24 subnets)
}

type Docker struct {
	Endpoint string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`

	Version                string                   `json:"version,omitempty" yaml:"version,omitempty"`
	RegistryMirrors        []string                 `json:"registryMirrors,omitempty" yaml:"registryMirrors,omitempty"`
	InsecureRegistries     []string                 `json:"insecureRegistries,omitempty" yaml:"insecureRegistries,omitempty"`
	DataRoot               *string                  `json:"dataRoot,omitempty" yaml:"dataRoot,omitempty"`
	ExecOpts               []string                 `json:"execOpts,omitempty" yaml:"execOpts,omitempty"`
	LogDriver              *string                  `json:"logDriver,omitempty" yaml:"logDriver,omitempty"`
	LogOpts                map[string]string        `json:"logOpts,omitempty" yaml:"logOpts,omitempty"`
	BIP                    *string                  `json:"bip,omitempty" yaml:"bip,omitempty"`
	FixedCIDR              *string                  `json:"fixedCIDR,omitempty" yaml:"fixedCIDR,omitempty"`
	DefaultAddressPools    []DockerAddressPool      `json:"defaultAddressPools,omitempty" yaml:"defaultAddressPools,omitempty"`
	Experimental           *bool                    `json:"experimental,omitempty" yaml:"experimental,omitempty"`
	IPTables               *bool                    `json:"ipTables,omitempty" yaml:"ipTables,omitempty"`
	IPMasq                 *bool                    `json:"ipMasq,omitempty" yaml:"ipMasq,omitempty"`
	StorageDriver          *string                  `json:"storageDriver,omitempty" yaml:"storageDriver,omitempty"`
	StorageOpts            []string                 `json:"storageOpts,omitempty" yaml:"storageOpts,omitempty"`
	DefaultRuntime         *string                  `json:"defaultRuntime,omitempty" yaml:"defaultRuntime,omitempty"`
	Runtimes               map[string]DockerRuntime `json:"runtimes,omitempty" yaml:"runtimes,omitempty"`
	MaxConcurrentDownloads *int                     `json:"maxConcurrentDownloads,omitempty" yaml:"maxConcurrentDownloads,omitempty"`
	MaxConcurrentUploads   *int                     `json:"maxConcurrentUploads,omitempty" yaml:"maxConcurrentUploads,omitempty"`
	Bridge                 *string                  `json:"bridge,omitempty" yaml:"bridge,omitempty"`
	CgroupDriver           *string                  `json:"cgroupDriver,omitempty" yaml:"cgroupDriver,omitempty"`
	LiveRestore            *bool                    `json:"liveRestore,omitempty" yaml:"liveRestore,omitempty"`

	InstallCRIDockerd *bool `json:"installCRIDockerd,omitempty" yaml:"installCRIDockerd,omitempty"`

	CRIDockerdVersion *string                              `json:"criDockerdVersion,omitempty" yaml:"criDockerdVersion,omitempty"`
	ExtraJSONConfig   *runtime.RawExtension                `json:"extraJsonConfig,omitempty" yaml:"extraJsonConfig,omitempty"`
	Auths             map[ServerAddress]DockerRegistryAuth `json:"auths,omitempty" yaml:"auths,omitempty"`

	Pause string `json:"pause,omitempty" yaml:"pause,omitempty"`
}

type ServerAddress string

type DockerRegistryAuth struct {
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
	Auth     string `json:"auth,omitempty" yaml:"auth,omitempty"`
}

type DockerRuntime struct {
	Path        string   `json:"path" yaml:"path"`
	RuntimeArgs []string `json:"runtimeArgs,omitempty" yaml:"runtimeArgs,omitempty"`
}

func SetDefaults_DockerConfig(cfg *Docker) {
	if cfg == nil {
		return
	}
	if cfg.RegistryMirrors == nil {
		cfg.RegistryMirrors = []string{}
	}
	if cfg.InsecureRegistries == nil {
		cfg.InsecureRegistries = []string{}
	}
	if cfg.ExecOpts == nil {
		cfg.ExecOpts = []string{}
	}
	if cfg.LogOpts == nil {
		cfg.LogOpts = map[string]string{
			"max-size": common.DockerLogOptMaxSizeDefault,
			"max-file": common.DockerLogOptMaxFileDefault,
		}
	}
	if cfg.DefaultAddressPools == nil {
		cfg.DefaultAddressPools = []DockerAddressPool{}
	}
	if cfg.StorageOpts == nil {
		cfg.StorageOpts = []string{}
	}
	if cfg.Runtimes == nil {
		cfg.Runtimes = make(map[string]DockerRuntime)
	}
	if cfg.Auths == nil {
		cfg.Auths = make(map[ServerAddress]DockerRegistryAuth)
	}
	if cfg.MaxConcurrentDownloads == nil {
		cfg.MaxConcurrentDownloads = helpers.IntPtr(common.DockerMaxConcurrentDownloadsDefault)
	}
	if cfg.MaxConcurrentUploads == nil {
		cfg.MaxConcurrentUploads = helpers.IntPtr(common.DockerMaxConcurrentUploadsDefault)
	}
	if cfg.Bridge == nil {
		cfg.Bridge = helpers.StrPtr(common.DefaultDockerBridgeName)
	}
	if cfg.InstallCRIDockerd == nil {
		cfg.InstallCRIDockerd = helpers.BoolPtr(true)
		cfg.CRIDockerdVersion = helpers.StrPtr(common.DefaultCriDockerdVersion)
	}
	if cfg.LogDriver == nil {
		cfg.LogDriver = helpers.StrPtr(common.DockerLogDriverJSONFile)
	}
	if cfg.DataRoot == nil {
		cfg.DataRoot = helpers.StrPtr(common.DockerDefaultDataRoot)
	}

	if cfg.IPTables == nil {
		cfg.IPTables = helpers.BoolPtr(true)
	}
	if cfg.IPMasq == nil {
		cfg.IPMasq = helpers.BoolPtr(true)
	}
	if cfg.Experimental == nil {
		cfg.Experimental = helpers.BoolPtr(false)
	}
	if cfg.Version == "" {
		cfg.Version = common.DefaultDockerVersion
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = common.DefaultDockerEndpoint
	}
	if cfg.CgroupDriver == nil {
		cfg.CgroupDriver = helpers.StrPtr(common.CgroupDriverSystemd)
	}
	if cfg.LiveRestore == nil {
		cfg.LiveRestore = helpers.BoolPtr(true)
	}
	if cfg.Pause == "" {
		cfg.Pause = common.DefaultPauseImage
	}
}

func Validate_DockerConfig(cfg *Docker, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	for i, mirror := range cfg.RegistryMirrors {
		mirrorPath := fmt.Sprintf("%s.registryMirrors[%d]", pathPrefix, i)
		if strings.TrimSpace(mirror) == "" {
			verrs.Add(mirrorPath, "mirror URL cannot be empty")
		} else {
			u, err := url.ParseRequestURI(mirror)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				verrs.Add(mirrorPath + ": invalid URL format for mirror '" + mirror + "' (must be http or https and valid URI)")
			}
		}
	}
	for i, insecureReg := range cfg.InsecureRegistries {
		regPath := fmt.Sprintf("%s.insecureRegistries[%d]", pathPrefix, i)
		if strings.TrimSpace(insecureReg) == "" {
			verrs.Add(regPath, "registry host cannot be empty")
		} else if !helpers.ValidateHostPortString(insecureReg) && !helpers.IsValidDomainName(insecureReg) {
			verrs.Add(regPath + ": invalid host:port or domain name format for insecure registry '" + insecureReg + "'")
		}
	}
	if cfg.DataRoot != nil {
		trimmedDataRoot := strings.TrimSpace(*cfg.DataRoot)
		if trimmedDataRoot == "" {
			verrs.Add(pathPrefix+".dataRoot", "cannot be empty if specified")
		} else if trimmedDataRoot == "/tmp" || trimmedDataRoot == "/var/tmp" {
			verrs.Add(pathPrefix + ".dataRoot: path '" + trimmedDataRoot + "' is not recommended for Docker data root")
		}
	}
	if cfg.LogDriver != nil {
		validLogDrivers := []string{common.DockerLogDriverJSONFile, common.DockerLogDriverJournald, common.DockerLogDriverSyslog, common.DockerLogDriverFluentd, common.DockerLogDriverNone, ""}
		isValid := false
		for _, v := range validLogDrivers {
			if *cfg.LogDriver == v {
				isValid = true
				break
			}
		}
		if !isValid {
			verrs.Add(pathPrefix + ".logDriver: invalid log driver '" + *cfg.LogDriver + "', must be one of " + fmt.Sprintf("%v", validLogDrivers) + " or empty for default")
		}
	}
	if cfg.BIP != nil && !helpers.IsValidCIDR(*cfg.BIP) {
		verrs.Add(pathPrefix + ".bip: invalid CIDR format '" + *cfg.BIP + "'")
	}
	if cfg.FixedCIDR != nil && !helpers.IsValidCIDR(*cfg.FixedCIDR) {
		verrs.Add(pathPrefix + ".fixedCIDR: invalid CIDR format '" + *cfg.FixedCIDR + "'")
	}
	for i, pool := range cfg.DefaultAddressPools {
		poolPath := fmt.Sprintf("%s.defaultAddressPools[%d]", pathPrefix, i)
		if !helpers.IsValidCIDR(pool.Base) {
			verrs.Add(poolPath + ".base: invalid CIDR format '" + pool.Base + "'")
		}
		if pool.Size <= 0 || pool.Size > 32 {
			verrs.Add(poolPath + ".size: invalid subnet size " + fmt.Sprintf("%d", pool.Size) + ", must be > 0 and <= 32")
		}
	}
	if cfg.StorageDriver != nil && strings.TrimSpace(*cfg.StorageDriver) == "" {
		verrs.Add(pathPrefix+".storageDriver", "cannot be empty if specified")
	}
	if cfg.MaxConcurrentDownloads != nil && *cfg.MaxConcurrentDownloads <= 0 {
		verrs.Add(pathPrefix+".maxConcurrentDownloads", "must be positive if specified")
	}
	if cfg.MaxConcurrentUploads != nil && *cfg.MaxConcurrentUploads <= 0 {
		verrs.Add(pathPrefix+".maxConcurrentUploads", "must be positive if specified")
	}
	for name, rt := range cfg.Runtimes {
		runtimePath := pathPrefix + ".runtimes['" + name + "']"
		if strings.TrimSpace(name) == "" {
			verrs.Add(pathPrefix+".runtimes", "runtime name key cannot be empty")
		}
		if strings.TrimSpace(rt.Path) == "" {
			verrs.Add(runtimePath+".path", "path cannot be empty")
		}
	}
	if cfg.Bridge != nil && strings.TrimSpace(*cfg.Bridge) == "" {
		verrs.Add(pathPrefix+".bridge", "name cannot be empty if specified")
	}

	if cfg.CRIDockerdVersion != nil {
		versionPath := pathPrefix + ".criDockerdVersion"
		if strings.TrimSpace(*cfg.CRIDockerdVersion) == "" {
			verrs.Add(versionPath, "cannot be only whitespace if specified")
		} else if !helpers.IsValidRuntimeVersion(*cfg.CRIDockerdVersion) {
			verrs.Add(versionPath + ": '" + *cfg.CRIDockerdVersion + "' is not a recognized version format")
		}
	}

	if cfg.ExtraJSONConfig != nil && len(cfg.ExtraJSONConfig.Raw) == 0 {
		verrs.Add(pathPrefix+".extraJsonConfig", "raw data cannot be empty if section is present")
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
	if cfg.CgroupDriver != nil {
		driver := *cfg.CgroupDriver
		if driver != common.CgroupDriverSystemd && driver != common.CgroupDriverCgroupfs {
			verrs.Add(pathPrefix+".cgroupDriver", "must be 'systemd' or 'cgroupfs'")
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
}
