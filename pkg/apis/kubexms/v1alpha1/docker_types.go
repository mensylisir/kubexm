package v1alpha1

import (
	"encoding/base64" // Added for DockerRegistryAuth validation
	"fmt"
	"net/url" // Added for URL parsing
	"strings"
	"k8s.io/apimachinery/pkg/runtime" // Added for RawExtension
	// Assuming isValidCIDR is available from kubernetes_types.go or similar
	"github.com/mensylisir/kubexm/pkg/util" // Import the util package
)

// DockerAddressPool defines a range of IP addresses for Docker networks.
type DockerAddressPool struct {
   Base string `json:"base"` // Base IP address for the pool (e.g., "172.30.0.0/16")
   Size int    `json:"size"` // Size of the subnets to allocate from the base pool (e.g., 24 for /24 subnets)
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
	Path string `json:"path"`
	RuntimeArgs []string `json:"runtimeArgs,omitempty"`
}

// SetDefaults_DockerConfig sets default values for DockerConfig.
func SetDefaults_DockerConfig(cfg *DockerConfig) {
	if cfg == nil {
		return
	}
	if cfg.RegistryMirrors == nil { cfg.RegistryMirrors = []string{} }
	if cfg.InsecureRegistries == nil { cfg.InsecureRegistries = []string{} }
	if cfg.ExecOpts == nil { cfg.ExecOpts = []string{} }
	if cfg.LogOpts == nil { cfg.LogOpts = make(map[string]string) }
	if cfg.DefaultAddressPools == nil { cfg.DefaultAddressPools = []DockerAddressPool{} }
	if cfg.StorageOpts == nil { cfg.StorageOpts = []string{} }
	if cfg.Runtimes == nil { cfg.Runtimes = make(map[string]DockerRuntime) }
	if cfg.Auths == nil { cfg.Auths = make(map[string]DockerRegistryAuth) }
	if cfg.MaxConcurrentDownloads == nil { cfg.MaxConcurrentDownloads = util.IntPtr(3) } // Docker default
	if cfg.MaxConcurrentUploads == nil { cfg.MaxConcurrentUploads = util.IntPtr(5) }   // Docker default
	if cfg.Bridge == nil { cfg.Bridge = util.StrPtr("docker0") }
	// DefaultRuntime: Docker's default is typically "runc". Let Docker handle if not specified.

	if cfg.InstallCRIDockerd == nil {
		cfg.InstallCRIDockerd = util.BoolPtr(true) // Default to installing cri-dockerd with Docker for Kubernetes
	}
	// No default for CRIDockerdVersion, let install logic handle it or require user input if specific version needed.

	if cfg.LogDriver == nil { cfg.LogDriver = util.StrPtr("json-file") }
	// Default DataRoot depends on OS, often /var/lib/docker. Let Docker daemon handle its own default if not set.
	// if cfg.DataRoot == nil { cfg.DataRoot = util.StrPtr("/var/lib/docker") } // Example if we wanted to enforce it

	if cfg.IPTables == nil { cfg.IPTables = util.BoolPtr(true) } // Docker default is true
	if cfg.IPMasq == nil { cfg.IPMasq = util.BoolPtr(true) }     // Docker default is true
	if cfg.Experimental == nil { cfg.Experimental = util.BoolPtr(false) }
}

// Validate_DockerConfig validates DockerConfig.
// Note: ValidationErrors type is expected to be defined in cluster_types.go or a common errors file.
// isValidCIDR is expected to be defined in kubernetes_types.go (in the same package).
func Validate_DockerConfig(cfg *DockerConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	for i, mirror := range cfg.RegistryMirrors {
		if strings.TrimSpace(mirror) == "" {
			verrs.Add("%s.registryMirrors[%d]: mirror URL cannot be empty", pathPrefix, i)
		} else {
			// Validate mirror URL format
			u, err := url.ParseRequestURI(mirror)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
				verrs.Add("%s.registryMirrors[%d]: invalid URL format for mirror '%s' (must be http or https and valid URI)", pathPrefix, i, mirror)
			}
		}
	}
	for i, insecureReg := range cfg.InsecureRegistries {
		if strings.TrimSpace(insecureReg) == "" {
			verrs.Add("%s.insecureRegistries[%d]: registry host cannot be empty", pathPrefix, i)
		} else if !util.ValidateHostPortString(insecureReg) { // Use util.ValidateHostPortString
			verrs.Add("%s.insecureRegistries[%d]: invalid host:port format for insecure registry '%s'", pathPrefix, i, insecureReg)
		}
	}
	if cfg.DataRoot != nil {
		trimmedDataRoot := strings.TrimSpace(*cfg.DataRoot)
		if trimmedDataRoot == "" {
			verrs.Add("%s.dataRoot: cannot be empty if specified", pathPrefix)
		} else if trimmedDataRoot == "/tmp" || trimmedDataRoot == "/var/tmp" {
			verrs.Add("%s.dataRoot: path '%s' is not recommended for Docker data root", pathPrefix, trimmedDataRoot)
		}
		// Further path validation (e.g., absolute path) could be added if needed.
	}
	if cfg.LogDriver != nil {
	   validLogDrivers := []string{"json-file", "journald", "syslog", "fluentd", "none", ""} // Allow empty for Docker default
	   isValid := false
	   for _, v := range validLogDrivers { if *cfg.LogDriver == v { isValid = true; break } }
	   if !isValid {
			verrs.Add("%s.logDriver: invalid log driver '%s'", pathPrefix, *cfg.LogDriver)
	   }
	}
	if cfg.BIP != nil && !util.IsValidCIDR(*cfg.BIP) { // Use util.IsValidCIDR
		verrs.Add("%s.bip: invalid CIDR format '%s'", pathPrefix, *cfg.BIP)
	}
	if cfg.FixedCIDR != nil && !util.IsValidCIDR(*cfg.FixedCIDR) { // Use util.IsValidCIDR
		verrs.Add("%s.fixedCIDR: invalid CIDR format '%s'", pathPrefix, *cfg.FixedCIDR)
	}
	for i, pool := range cfg.DefaultAddressPools {
	   poolPath := fmt.Sprintf("%s.defaultAddressPools[%d]", pathPrefix, i)
	   if !util.IsValidCIDR(pool.Base) { verrs.Add("%s.base: invalid CIDR format '%s'", poolPath, pool.Base) } // Use util.IsValidCIDR
	   if pool.Size <= 0 || pool.Size > 32 { verrs.Add("%s.size: invalid subnet size %d, must be > 0 and <= 32", poolPath, pool.Size) }
	}
	if cfg.StorageDriver != nil && strings.TrimSpace(*cfg.StorageDriver) == "" {
		verrs.Add("%s.storageDriver: cannot be empty if specified", pathPrefix)
	}
	if cfg.MaxConcurrentDownloads != nil && *cfg.MaxConcurrentDownloads <= 0 {
		verrs.Add("%s.maxConcurrentDownloads: must be positive if specified", pathPrefix)
	}
	if cfg.MaxConcurrentUploads != nil && *cfg.MaxConcurrentUploads <= 0 {
		verrs.Add("%s.maxConcurrentUploads: must be positive if specified", pathPrefix)
	}
	for name, rt := range cfg.Runtimes {
		if strings.TrimSpace(name) == "" { verrs.Add("%s.runtimes: runtime name key cannot be empty", pathPrefix)}
		if strings.TrimSpace(rt.Path) == "" { verrs.Add("%s.runtimes['%s'].path: path cannot be empty", pathPrefix, name)}
	}
	if cfg.Bridge != nil && strings.TrimSpace(*cfg.Bridge) == "" {
		verrs.Add("%s.bridge: name cannot be empty if specified", pathPrefix)
	}

	if cfg.CRIDockerdVersion != nil {
		if strings.TrimSpace(*cfg.CRIDockerdVersion) == "" {
			verrs.Add("%s.criDockerdVersion: cannot be only whitespace if specified", pathPrefix)
		} else if !util.IsValidRuntimeVersion(*cfg.CRIDockerdVersion) { // Use util.IsValidRuntimeVersion
			verrs.Add("%s.criDockerdVersion: '%s' is not a recognized version format", pathPrefix, *cfg.CRIDockerdVersion)
		}
	}
	// No specific validation for InstallCRIDockerd (boolean pointer) beyond type checking.

	if cfg.ExtraJSONConfig != nil && len(cfg.ExtraJSONConfig.Raw) == 0 {
		verrs.Add("%s.extraJsonConfig: raw data cannot be empty if section is present", pathPrefix)
	}

	for regAddr, auth := range cfg.Auths {
		authPathPrefix := fmt.Sprintf("%s.auths[\"%s\"]", pathPrefix, regAddr)
		if strings.TrimSpace(regAddr) == "" {
			verrs.Add("%s.auths: registry address key cannot be empty", pathPrefix) // Use pathPrefix for the map key error itself
		} else if !util.ValidateHostPortString(regAddr) && !util.IsValidDomainName(regAddr) { // Key should be a valid registry address
			verrs.Add("%s: registry key '%s' is not a valid hostname or host:port", authPathPrefix, regAddr)
		}

		hasUserPass := auth.Username != "" && auth.Password != ""
		hasAuthStr := auth.Auth != ""

		if !hasUserPass && !hasAuthStr {
			verrs.Add("%s: either username/password or auth string must be provided", authPathPrefix)
		}
		if hasAuthStr {
			decoded, err := base64.StdEncoding.DecodeString(auth.Auth)
			if err != nil {
				verrs.Add("%s.auth: failed to decode base64 auth string: %v", authPathPrefix, err)
			} else if !strings.Contains(string(decoded), ":") {
				verrs.Add("%s.auth: decoded auth string must be in 'username:password' format", authPathPrefix)
			}
		}
		// ServerAddress in DockerRegistryAuth is optional and for informational purposes if used in a list,
		// so no specific validation here unless we want to enforce it matches the map key.
	}
}
