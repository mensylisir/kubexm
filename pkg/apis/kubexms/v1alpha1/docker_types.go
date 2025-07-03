package v1alpha1

import (
	"fmt"
	"net/url" // Added for URL parsing
	"strings"
	// Assuming isValidCIDR is available from kubernetes_types.go or similar
	// Assuming isValidHostPort is available from containerd_types.go or similar (if not, it would need to be defined/imported)
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
	if cfg.MaxConcurrentDownloads == nil { cfg.MaxConcurrentDownloads = intPtr(3) } // Docker default
	if cfg.MaxConcurrentUploads == nil { cfg.MaxConcurrentUploads = intPtr(5) }   // Docker default
	if cfg.Bridge == nil { cfg.Bridge = stringPtr("docker0") }
	// DefaultRuntime: Docker's default is typically "runc". Let Docker handle if not specified.

	if cfg.InstallCRIDockerd == nil {
		cfg.InstallCRIDockerd = boolPtr(true) // Default to installing cri-dockerd with Docker for Kubernetes
	}
	// No default for CRIDockerdVersion, let install logic handle it or require user input if specific version needed.

	if cfg.LogDriver == nil { cfg.LogDriver = stringPtr("json-file") }
	// Default DataRoot depends on OS, often /var/lib/docker. Let Docker daemon handle its own default if not set.
	// if cfg.DataRoot == nil { cfg.DataRoot = stringPtr("/var/lib/docker") } // Example if we wanted to enforce it

	if cfg.IPTables == nil { cfg.IPTables = boolPtr(true) } // Docker default is true
	if cfg.IPMasq == nil { cfg.IPMasq = boolPtr(true) }     // Docker default is true
	if cfg.Experimental == nil { cfg.Experimental = boolPtr(false) }
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
		} else if !isValidHostPort(insecureReg) { // Assuming isValidHostPort is available in the package
			verrs.Add("%s.insecureRegistries[%d]: invalid host:port format for insecure registry '%s'", pathPrefix, i, insecureReg)
		}
	}
	if cfg.DataRoot != nil && strings.TrimSpace(*cfg.DataRoot) == "" {
		verrs.Add("%s.dataRoot: cannot be empty if specified", pathPrefix)
	}
	if cfg.LogDriver != nil {
	   validLogDrivers := []string{"json-file", "journald", "syslog", "fluentd", "none", ""} // Allow empty for Docker default
	   isValid := false
	   for _, v := range validLogDrivers { if *cfg.LogDriver == v { isValid = true; break } }
	   if !isValid {
			verrs.Add("%s.logDriver: invalid log driver '%s'", pathPrefix, *cfg.LogDriver)
	   }
	}
	if cfg.BIP != nil && !isValidCIDR(*cfg.BIP) {
		verrs.Add("%s.bip: invalid CIDR format '%s'", pathPrefix, *cfg.BIP)
	}
	if cfg.FixedCIDR != nil && !isValidCIDR(*cfg.FixedCIDR) {
		verrs.Add("%s.fixedCIDR: invalid CIDR format '%s'", pathPrefix, *cfg.FixedCIDR)
	}
	for i, pool := range cfg.DefaultAddressPools {
	   poolPath := fmt.Sprintf("%s.defaultAddressPools[%d]", pathPrefix, i)
	   if !isValidCIDR(pool.Base) { verrs.Add("%s.base: invalid CIDR format '%s'", poolPath, pool.Base) }
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
		} else if !isValidRuntimeVersion(*cfg.CRIDockerdVersion) { // Use the common validator
			verrs.Add("%s.criDockerdVersion: '%s' is not a recognized version format", pathPrefix, *cfg.CRIDockerdVersion)
		}
	}
	// No specific validation for InstallCRIDockerd (boolean pointer) beyond type checking.
}
