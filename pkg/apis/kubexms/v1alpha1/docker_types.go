package v1alpha1

import (
	"encoding/base64" // Added for DockerRegistryAuth validation
	"fmt"
	"net/url" // Added for URL parsing
	"strings"

	"k8s.io/apimachinery/pkg/runtime" // Added for RawExtension
	// Assuming isValidCIDR is available from kubernetes_types.go or similar
	"github.com/mensylisir/kubexm/pkg/common" // Import the common package
	"github.com/mensylisir/kubexm/pkg/util"   // Import the util package
)

// DockerAddressPool defines a range of IP addresses for Docker networks.
type DockerAddressPool struct {
   Base string `json:"base" yaml:"base"` // Base IP address for the pool (e.g., "172.30.0.0/16")
   Size int    `json:"size" yaml:"size"` // Size of the subnets to allocate from the base pool (e.g., 24 for /24 subnets)
}

// DockerConfig defines specific settings for the Docker runtime.
// These settings are only applicable if ContainerRuntimeConfig.Type is "docker".
// Corresponds to `kubernetes.containerRuntime.docker` in YAML.
type DockerConfig struct {
	// RegistryMirrors for Docker. Corresponds to `registryMirrors` in YAML.
	RegistryMirrors     []string            `json:"registryMirrors,omitempty" yaml:"registryMirrors,omitempty"`
	// InsecureRegistries for Docker. Corresponds to `insecureRegistries` in YAML.
	InsecureRegistries  []string            `json:"insecureRegistries,omitempty" yaml:"insecureRegistries,omitempty"`
	// DataRoot is Docker's root directory. Corresponds to `dataRoot` in YAML.
	DataRoot            *string             `json:"dataRoot,omitempty" yaml:"dataRoot,omitempty"`
	// ExecOpts for Docker daemon. Corresponds to `execOpts` in YAML.
	ExecOpts            []string            `json:"execOpts,omitempty" yaml:"execOpts,omitempty"`
	LogDriver           *string             `json:"logDriver,omitempty" yaml:"logDriver,omitempty"`
	LogOpts             map[string]string   `json:"logOpts,omitempty" yaml:"logOpts,omitempty"`
	BIP                 *string             `json:"bip,omitempty" yaml:"bip,omitempty"`
	FixedCIDR           *string             `json:"fixedCIDR,omitempty" yaml:"fixedCIDR,omitempty"`
	DefaultAddressPools []DockerAddressPool `json:"defaultAddressPools,omitempty" yaml:"defaultAddressPools,omitempty"`
	Experimental        *bool               `json:"experimental,omitempty" yaml:"experimental,omitempty"`
	IPTables            *bool               `json:"ipTables,omitempty" yaml:"ipTables,omitempty"` // YAML might use 'iptables'
	IPMasq              *bool               `json:"ipMasq,omitempty" yaml:"ipMasq,omitempty"`    // YAML might use 'ip-masq'
	StorageDriver       *string             `json:"storageDriver,omitempty" yaml:"storageDriver,omitempty"`
	StorageOpts         []string            `json:"storageOpts,omitempty" yaml:"storageOpts,omitempty"`
	DefaultRuntime      *string             `json:"defaultRuntime,omitempty" yaml:"defaultRuntime,omitempty"`
	Runtimes            map[string]DockerRuntime `json:"runtimes,omitempty" yaml:"runtimes,omitempty"`
	MaxConcurrentDownloads *int `json:"maxConcurrentDownloads,omitempty" yaml:"maxConcurrentDownloads,omitempty"`
	MaxConcurrentUploads   *int `json:"maxConcurrentUploads,omitempty" yaml:"maxConcurrentUploads,omitempty"`
	Bridge                 *string `json:"bridge,omitempty" yaml:"bridge,omitempty"`

	// InstallCRIDockerd indicates whether to install cri-dockerd shim.
	// Corresponds to `installCRIDockerd` in YAML.
	InstallCRIDockerd *bool `json:"installCRIDockerd,omitempty" yaml:"installCRIDockerd,omitempty"`

	// CRIDockerdVersion specifies the version of cri-dockerd to install.
	// No direct YAML field, usually determined by installer based on K8s version or a default.
	CRIDockerdVersion *string `json:"criDockerdVersion,omitempty" yaml:"criDockerdVersion,omitempty"`
	// ExtraJSONConfig provides a passthrough for any daemon.json settings not
	// explicitly defined in this struct. It will be merged with the generated
	// configuration. In case of conflicts, settings from ExtraJSONConfig take precedence.
	ExtraJSONConfig *runtime.RawExtension `json:"extraJsonConfig,omitempty" yaml:"extraJsonConfig,omitempty"`
	// Auths provides authentication details for registries, keyed by registry FQDN.
	// This allows Docker to pull images from private registries.
	Auths map[string]DockerRegistryAuth `json:"auths,omitempty" yaml:"auths,omitempty"`
}

// DockerRegistryAuth defines authentication credentials for a specific Docker registry.
type DockerRegistryAuth struct {
	// Username for the registry.
	Username string `json:"username,omitempty" yaml:"username,omitempty"`
	// Password for the registry.
	Password string `json:"password,omitempty" yaml:"password,omitempty"`
	// Auth is a base64 encoded string of "username:password".
	// This is the format typically stored in .docker/config.json.
	Auth string `json:"auth,omitempty" yaml:"auth,omitempty"`
	// ServerAddress is the FQDN of the registry. While map key in DockerConfig.Auths serves this,
	// having it here can be useful if this struct is used in a list elsewhere. Optional.
	ServerAddress string `json:"serverAddress,omitempty" yaml:"serverAddress,omitempty"`
}

// DockerRuntime defines a custom runtime for Docker.
type DockerRuntime struct {
	Path string `json:"path" yaml:"path"`
	RuntimeArgs []string `json:"runtimeArgs,omitempty" yaml:"runtimeArgs,omitempty"`
}

// SetDefaults_DockerConfig sets default values for DockerConfig.
func SetDefaults_DockerConfig(cfg *DockerConfig) {
	if cfg == nil {
		return
	}
	if cfg.RegistryMirrors == nil { cfg.RegistryMirrors = []string{} }
	if cfg.InsecureRegistries == nil { cfg.InsecureRegistries = []string{} }
	if cfg.ExecOpts == nil { cfg.ExecOpts = []string{} }
	if cfg.LogOpts == nil {
		cfg.LogOpts = map[string]string{
			"max-size": common.DockerLogOptMaxSizeDefault,
			"max-file": common.DockerLogOptMaxFileDefault,
		}
	}
	if cfg.DefaultAddressPools == nil { cfg.DefaultAddressPools = []DockerAddressPool{} }
	if cfg.StorageOpts == nil { cfg.StorageOpts = []string{} }
	if cfg.Runtimes == nil { cfg.Runtimes = make(map[string]DockerRuntime) }
	if cfg.Auths == nil { cfg.Auths = make(map[string]DockerRegistryAuth) }
	if cfg.MaxConcurrentDownloads == nil { cfg.MaxConcurrentDownloads = util.IntPtr(common.DockerMaxConcurrentDownloadsDefault) }
	if cfg.MaxConcurrentUploads == nil { cfg.MaxConcurrentUploads = util.IntPtr(common.DockerMaxConcurrentUploadsDefault) }
	if cfg.Bridge == nil { cfg.Bridge = util.StrPtr(common.DefaultDockerBridgeName) }
	// DefaultRuntime: Docker's default is typically "runc". Let Docker handle if not specified.

	if cfg.InstallCRIDockerd == nil {
		cfg.InstallCRIDockerd = util.BoolPtr(true) // Default to installing cri-dockerd with Docker for Kubernetes
	}
	// No default for CRIDockerdVersion, let install logic handle it or require user input if specific version needed.

	if cfg.LogDriver == nil { cfg.LogDriver = util.StrPtr(common.DockerLogDriverJSONFile) }
	// Default DataRoot depends on OS, often /var/lib/docker. Let Docker daemon handle its own default if not set.
	// if cfg.DataRoot == nil { cfg.DataRoot = util.StrPtr("/var/lib/docker") } // Example if we wanted to enforce it

	if cfg.IPTables == nil { cfg.IPTables = util.BoolPtr(true) } // Docker default is true
	if cfg.IPMasq == nil { cfg.IPMasq = util.BoolPtr(true) }     // Docker default is true
	if cfg.Experimental == nil { cfg.Experimental = util.BoolPtr(false) }
}

// Validate_DockerConfig validates DockerConfig.
func Validate_DockerConfig(cfg *DockerConfig, verrs *ValidationErrors, pathPrefix string) {
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
		} else if !util.ValidateHostPortString(insecureReg) { // Use util.ValidateHostPortString
			verrs.Add(regPath + ": invalid host:port format for insecure registry '" + insecureReg + "'")
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
		for _, v := range validLogDrivers { if *cfg.LogDriver == v { isValid = true; break } }
		if !isValid {
			verrs.Add(pathPrefix + ".logDriver: invalid log driver '" + *cfg.LogDriver + "', must be one of " + fmt.Sprintf("%v", validLogDrivers) + " or empty for default")
		}
	}
	if cfg.BIP != nil && !util.IsValidCIDR(*cfg.BIP) {
		verrs.Add(pathPrefix + ".bip: invalid CIDR format '" + *cfg.BIP + "'")
	}
	if cfg.FixedCIDR != nil && !util.IsValidCIDR(*cfg.FixedCIDR) {
		verrs.Add(pathPrefix + ".fixedCIDR: invalid CIDR format '" + *cfg.FixedCIDR + "'")
	}
	for i, pool := range cfg.DefaultAddressPools {
	   poolPath := fmt.Sprintf("%s.defaultAddressPools[%d]", pathPrefix, i)
	   if !util.IsValidCIDR(pool.Base) { verrs.Add(poolPath + ".base: invalid CIDR format '" + pool.Base + "'") }
	   if pool.Size <= 0 || pool.Size > 32 { verrs.Add(poolPath + ".size: invalid subnet size " + fmt.Sprintf("%d", pool.Size) + ", must be > 0 and <= 32") }
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
		if strings.TrimSpace(name) == "" { verrs.Add(pathPrefix+".runtimes", "runtime name key cannot be empty")}
		if strings.TrimSpace(rt.Path) == "" { verrs.Add(runtimePath+".path", "path cannot be empty")}
	}
	if cfg.Bridge != nil && strings.TrimSpace(*cfg.Bridge) == "" {
		verrs.Add(pathPrefix+".bridge", "name cannot be empty if specified")
	}

	if cfg.CRIDockerdVersion != nil {
		versionPath := pathPrefix + ".criDockerdVersion"
		if strings.TrimSpace(*cfg.CRIDockerdVersion) == "" {
			verrs.Add(versionPath, "cannot be only whitespace if specified")
		} else if !util.IsValidRuntimeVersion(*cfg.CRIDockerdVersion) {
			verrs.Add(versionPath + ": '" + *cfg.CRIDockerdVersion + "' is not a recognized version format")
		}
	}

	if cfg.ExtraJSONConfig != nil && len(cfg.ExtraJSONConfig.Raw) == 0 {
		verrs.Add(pathPrefix+".extraJsonConfig", "raw data cannot be empty if section is present")
	}

	for regAddr, auth := range cfg.Auths {
		authMapPath := pathPrefix + ".auths"
		authEntryPath := fmt.Sprintf("%s[\"%s\"]", authMapPath, regAddr)
		if strings.TrimSpace(regAddr) == "" {
			verrs.Add(authMapPath, "registry address key cannot be empty")
		} else if !util.ValidateHostPortString(regAddr) && !util.IsValidDomainName(regAddr) {
			verrs.Add(authEntryPath + ": registry key '" + regAddr + "' is not a valid hostname or host:port")
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
