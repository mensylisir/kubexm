package v1alpha1

import (
	"fmt"
	"net" // For IP validation
	"strings"
)

// HAProxyBackendServer defines a backend server for HAProxy load balancing.
type HAProxyBackendServer struct {
	// Name is an identifier for the backend server.
	Name string `json:"name" yaml:"name"`
	// Address is the IP address or resolvable hostname of the backend server.
	Address string `json:"address" yaml:"address"`
	// Port is the port on which the backend server is listening.
	Port int `json:"port" yaml:"port"`
	// Weight for weighted load balancing algorithms (optional).
	Weight *int `json:"weight,omitempty" yaml:"weight,omitempty"`
	// TODO: Add other server options like 'check', 'inter', 'rise', 'fall' if needed.
}

// HAProxyConfig defines settings for HAProxy service used for HA load balancing.
type HAProxyConfig struct {
	// FrontendBindAddress is the address HAProxy should bind its frontend to.
	// Defaults to "0.0.0.0" (all interfaces).
	FrontendBindAddress *string `json:"frontendBindAddress,omitempty" yaml:"frontendBindAddress,omitempty"`

	// FrontendPort is the port HAProxy listens on for the load-balanced service (e.g., Kubernetes API).
	// Defaults to 6443 or a common load balancer port like 8443.
	FrontendPort *int `json:"frontendPort,omitempty" yaml:"frontendPort,omitempty"`

	// Mode for HAProxy (e.g., "tcp", "http"). Defaults to "tcp" for API server.
	Mode *string `json:"mode,omitempty" yaml:"mode,omitempty"`

	// BalanceAlgorithm for backend server selection.
	// e.g., "roundrobin", "leastconn", "source". Defaults to "roundrobin".
	BalanceAlgorithm *string `json:"balanceAlgorithm,omitempty" yaml:"balanceAlgorithm,omitempty"`

	// BackendServers is a list of backend servers to load balance.
	// Typically, these are the control-plane nodes for kube-apiserver.
	BackendServers []HAProxyBackendServer `json:"backendServers,omitempty" yaml:"backendServers,omitempty"`

	// ExtraGlobalConfig allows adding raw lines to the 'global' section of haproxy.cfg.
	ExtraGlobalConfig []string `json:"extraGlobalConfig,omitempty" yaml:"extraGlobalConfig,omitempty"`
	// ExtraDefaultsConfig allows adding raw lines to the 'defaults' section of haproxy.cfg.
	ExtraDefaultsConfig []string `json:"extraDefaultsConfig,omitempty" yaml:"extraDefaultsConfig,omitempty"`
	// ExtraFrontendConfig allows adding raw lines to the specific frontend section of haproxy.cfg.
	ExtraFrontendConfig []string `json:"extraFrontendConfig,omitempty" yaml:"extraFrontendConfig,omitempty"`
	// ExtraBackendConfig allows adding raw lines to the specific backend section of haproxy.cfg.
	ExtraBackendConfig []string `json:"extraBackendConfig,omitempty" yaml:"extraBackendConfig,omitempty"`

	// SkipInstall, if true, assumes HAProxy is already installed and configured externally.
	SkipInstall *bool `json:"skipInstall,omitempty" yaml:"skipInstall,omitempty"`
}

// --- Defaulting Functions ---

// SetDefaults_HAProxyConfig sets default values for HAProxyConfig.
func SetDefaults_HAProxyConfig(cfg *HAProxyConfig) {
	if cfg == nil {
		return
	}
	if cfg.FrontendBindAddress == nil {
		cfg.FrontendBindAddress = stringPtr("0.0.0.0")
	}
	if cfg.FrontendPort == nil {
		cfg.FrontendPort = intPtr(6443) // Default Kube API server port
	}
	if cfg.Mode == nil {
		cfg.Mode = stringPtr("tcp")
	}
	if cfg.BalanceAlgorithm == nil {
		cfg.BalanceAlgorithm = stringPtr("roundrobin")
	}
	if cfg.BackendServers == nil {
		cfg.BackendServers = []HAProxyBackendServer{}
	}
	for i := range cfg.BackendServers {
		server := &cfg.BackendServers[i]
		if server.Weight == nil {
			server.Weight = intPtr(1) // Default weight
		}
	}

	if cfg.ExtraGlobalConfig == nil {
		cfg.ExtraGlobalConfig = []string{}
	}
	if cfg.ExtraDefaultsConfig == nil {
		cfg.ExtraDefaultsConfig = []string{}
	}
	if cfg.ExtraFrontendConfig == nil {
		cfg.ExtraFrontendConfig = []string{}
	}
	if cfg.ExtraBackendConfig == nil {
		cfg.ExtraBackendConfig = []string{}
	}

	if cfg.SkipInstall == nil {
		cfg.SkipInstall = boolPtr(false) // Default to managing HAProxy installation
	}
}

// --- Validation Functions ---

// Validate_HAProxyConfig validates HAProxyConfig.
// Note: ValidationErrors type is expected to be defined in cluster_types.go or a common errors file.
func Validate_HAProxyConfig(cfg *HAProxyConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.SkipInstall != nil && *cfg.SkipInstall {
		return // If skipping install, most other fields are not KubeXMS's concern for setup validation.
	}

	if cfg.FrontendBindAddress != nil {
		if strings.TrimSpace(*cfg.FrontendBindAddress) == "" {
			verrs.Add("%s.frontendBindAddress: cannot be empty if specified", pathPrefix)
		} else if net.ParseIP(*cfg.FrontendBindAddress) == nil && *cfg.FrontendBindAddress != "0.0.0.0" && *cfg.FrontendBindAddress != "::" {
			// Allow "0.0.0.0" and "::" as special bind addresses, otherwise expect a valid IP.
			// Hostnames are generally not used for bind addresses in HAProxy for listening.
			verrs.Add("%s.frontendBindAddress: invalid IP address format '%s'", pathPrefix, *cfg.FrontendBindAddress)
		}
	}

	// FrontendPort is always defaulted, so check its value.
	if *cfg.FrontendPort <= 0 || *cfg.FrontendPort > 65535 {
		verrs.Add("%s.frontendPort: invalid port %d", pathPrefix, *cfg.FrontendPort)
	}

	if cfg.Mode != nil && *cfg.Mode != "" {
		validModes := []string{"tcp", "http"}
		if !containsString(validModes, *cfg.Mode) {
			verrs.Add("%s.mode: invalid mode '%s', must be one of %v or empty for default", pathPrefix, *cfg.Mode, validModes)
		}
	}

	if cfg.BalanceAlgorithm != nil && *cfg.BalanceAlgorithm != "" {
		validAlgos := []string{"roundrobin", "static-rr", "leastconn", "first", "source", "uri", "url_param", "hdr", "rdp-cookie"} // Common algos
		if !containsString(validAlgos, *cfg.BalanceAlgorithm) {
			verrs.Add("%s.balanceAlgorithm: invalid algorithm '%s'", pathPrefix, *cfg.BalanceAlgorithm)
		}
	}

	if len(cfg.BackendServers) == 0 {
		verrs.Add("%s.backendServers: must specify at least one backend server", pathPrefix)
	}
	for i, server := range cfg.BackendServers {
		serverPath := fmt.Sprintf("%s.backendServers[%d:%s]", pathPrefix, i, server.Name)
		if strings.TrimSpace(server.Name) == "" {
			verrs.Add("%s.name: backend server name cannot be empty", serverPath)
		}
		if strings.TrimSpace(server.Address) == "" {
			verrs.Add("%s.address: backend server address cannot be empty", serverPath)
		} else if !isValidHostOrIP(server.Address) { // Assuming isValidHostOrIP is available
			verrs.Add("%s.address: invalid backend server address format '%s'", serverPath, server.Address)
		}
		if server.Port <= 0 || server.Port > 65535 {
			verrs.Add("%s.port: invalid backend server port %d", serverPath, server.Port)
		}
		if server.Weight != nil && *server.Weight < 0 { // Weight usually 0-256
			verrs.Add("%s.weight: cannot be negative, got %d", serverPath, *server.Weight)
		}
	}
	// Validate ExtraConfig sections for non-empty lines if needed
}
