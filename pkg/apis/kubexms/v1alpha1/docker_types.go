package v1alpha1

import (
	"fmt"
	"strings"
	// net/url is not directly needed here, but isValidCIDR uses net.ParseCIDR
)

// DockerAddressPool defines a range of IP addresses for Docker networks.
type DockerAddressPool struct {
   Base string `json:"base"` // Base IP address for the pool (e.g., "172.30.0.0/16")
   Size int    `json:"size"` // Size of the subnets to allocate from the base pool (e.g., 24 for /24 subnets)
}

// DockerConfig defines specific settings for the Docker runtime.
// These settings are only applicable if ContainerRuntimeConfig.Type is "docker".
type DockerConfig struct {
	RegistryMirrors     []string            `json:"registryMirrors,omitempty"`
	InsecureRegistries  []string            `json:"insecureRegistries,omitempty"`
	DataRoot            *string             `json:"dataRoot,omitempty"`            // Docker's root directory
	ExecOpts            []string            `json:"execOpts,omitempty"`            // e.g., ["native.cgroupdriver=systemd"]
	LogDriver           *string             `json:"logDriver,omitempty"`           // e.g., "json-file", "journald"
	LogOpts             map[string]string   `json:"logOpts,omitempty"`             // e.g., {"max-size": "100m"}
	BIP                 *string             `json:"bip,omitempty"`                 // For the docker0 bridge IP and netmask
	FixedCIDR           *string             `json:"fixedCIDR,omitempty"`           // Restrict the IP range for the docker0 bridge
	DefaultAddressPools []DockerAddressPool `json:"defaultAddressPools,omitempty"` // Default address pools for networks
	Experimental        *bool               `json:"experimental,omitempty"`
	IPTables            *bool               `json:"ipTables,omitempty"`            // Note: case difference from KubeKey's yaml
	IPMasq              *bool               `json:"ipMasq,omitempty"`              // Note: case difference from KubeKey's yaml
	StorageDriver       *string             `json:"storageDriver,omitempty"`
	StorageOpts         []string            `json:"storageOpts,omitempty"`
	DefaultRuntime      *string             `json:"defaultRuntime,omitempty"` // e.g., "runc"
	Runtimes            map[string]DockerRuntime `json:"runtimes,omitempty"` // For configuring other runtimes like kata, nvidia
	MaxConcurrentDownloads *int `json:"maxConcurrentDownloads,omitempty"`
	MaxConcurrentUploads   *int `json:"maxConcurrentUploads,omitempty"`
	Bridge                 *string `json:"bridge,omitempty"`

	// InstallCRIDockerd indicates whether to install cri-dockerd shim.
	// Required for using Docker with Kubernetes versions that have removed dockershim.
	// Defaults to true if Docker is the selected runtime.
	InstallCRIDockerd *bool `json:"installCRIDockerd,omitempty"`

	// CRIDockerdVersion specifies the version of cri-dockerd to install.
	// If empty, the installation logic may choose a default compatible version.
	CRIDockerdVersion *string `json:"criDockerdVersion,omitempty"`
}

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
	if cfg.MaxConcurrentDownloads == nil { mcd := 3; cfg.MaxConcurrentDownloads = &mcd } // Docker default
	if cfg.MaxConcurrentUploads == nil { mcu := 5; cfg.MaxConcurrentUploads = &mcu }   // Docker default
	if cfg.Bridge == nil { bridgeName := "docker0"; cfg.Bridge = &bridgeName }
	// DefaultRuntime: Docker's default is typically "runc". Let Docker handle if not specified.

	if cfg.InstallCRIDockerd == nil {
		b := true // Default to installing cri-dockerd with Docker for Kubernetes
		cfg.InstallCRIDockerd = &b
	}
	// No default for CRIDockerdVersion, let install logic handle it or require user input if specific version needed.

	if cfg.LogDriver == nil { defaultLogDriver := "json-file"; cfg.LogDriver = &defaultLogDriver }
	// Default DataRoot depends on OS, often /var/lib/docker. Let Docker daemon handle its own default if not set.
	// if cfg.DataRoot == nil { defaultDataRoot := "/var/lib/docker"; cfg.DataRoot = &defaultDataRoot }

	if cfg.IPTables == nil { b := true; cfg.IPTables = &b } // Docker default is true
	if cfg.IPMasq == nil { b := true; cfg.IPMasq = &b }     // Docker default is true
	if cfg.Experimental == nil { b := false; cfg.Experimental = &b }
}

// Validate_DockerConfig validates DockerConfig.
// Note: ValidationErrors type is expected to be defined in cluster_types.go or a common errors file.
// isValidCIDR is expected to be defined in kubernetes_types.go (in the same package).
func Validate_DockerConfig(cfg *DockerConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	for i, mirror := range cfg.RegistryMirrors {
		if strings.TrimSpace(mirror) == "" { verrs.Add("%s.registryMirrors[%d]: mirror URL cannot be empty", pathPrefix, i) }
		// Basic URL validation could be added using net/url.ParseRequestURI
	}
	for i, insecureReg := range cfg.InsecureRegistries {
		if strings.TrimSpace(insecureReg) == "" { verrs.Add("%s.insecureRegistries[%d]: registry host cannot be empty", pathPrefix, i) }
		// Could add host:port validation
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

	if cfg.CRIDockerdVersion != nil && strings.TrimSpace(*cfg.CRIDockerdVersion) == "" {
		verrs.Add("%s.criDockerdVersion: cannot be empty if specified", pathPrefix)
		// Could add version format validation here if needed, e.g., starts with 'v'
	}
	// No specific validation for InstallCRIDockerd (boolean pointer) beyond type checking.
}
