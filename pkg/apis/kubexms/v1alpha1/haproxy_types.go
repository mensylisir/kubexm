package v1alpha1

import (
	"fmt"
	// "net" // For IP validation - will be replaced by util.IsValidIP
	"strings"
	"github.com/mensylisir/kubexm/pkg/util" // Import the util package
	"github.com/mensylisir/kubexm/pkg/common" // Import the common package
	"github.com/mensylisir/kubexm/pkg/util/validation"
)

const (
	// HAProxyModeTCP is a valid mode for HAProxy.
	HAProxyModeTCP = "tcp"
	// HAProxyModeHTTP is a valid mode for HAProxy.
	HAProxyModeHTTP = "http"
)

var (
	// validHAProxyModes lists the supported HAProxy modes.
	validHAProxyModes = []string{HAProxyModeTCP, HAProxyModeHTTP}
	// validHAProxyBalanceAlgorithms lists the supported HAProxy balance algorithms.
	validHAProxyBalanceAlgorithms = []string{"roundrobin", "static-rr", "leastconn", "first", "source", "uri", "url_param", "hdr", "rdp-cookie"}
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
		cfg.FrontendBindAddress = util.StrPtr("0.0.0.0")
	}
	if cfg.FrontendPort == nil {
		cfg.FrontendPort = util.IntPtr(common.HAProxyDefaultFrontendPort)
	}
	if cfg.Mode == nil {
		cfg.Mode = util.StrPtr(common.DefaultHAProxyMode)
	}
	if cfg.BalanceAlgorithm == nil {
		cfg.BalanceAlgorithm = util.StrPtr(common.DefaultHAProxyAlgorithm)
	}
	if cfg.BackendServers == nil {
		cfg.BackendServers = []HAProxyBackendServer{}
	}
	for i := range cfg.BackendServers {
		server := &cfg.BackendServers[i]
		if server.Weight == nil {
			server.Weight = util.IntPtr(1) // Default weight
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
		cfg.SkipInstall = util.BoolPtr(false) // Default to managing HAProxy installation
	}
}

// --- Validation Functions ---

// Validate_HAProxyConfig validates HAProxyConfig.
func Validate_HAProxyConfig(cfg *HAProxyConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.SkipInstall != nil && *cfg.SkipInstall {
		return
	}

	if cfg.FrontendBindAddress != nil {
		trimmedAddr := strings.TrimSpace(*cfg.FrontendBindAddress)
		if trimmedAddr == "" {
			verrs.Add(pathPrefix+".frontendBindAddress", "cannot be empty if specified")
		} else if !util.IsValidIP(trimmedAddr) && trimmedAddr != "0.0.0.0" && trimmedAddr != "::" {
			verrs.Add(pathPrefix+".frontendBindAddress", fmt.Sprintf("invalid IP address format '%s'", trimmedAddr))
		}
	}

	if cfg.FrontendPort == nil {
		verrs.Add(pathPrefix+".frontendPort", "is required and should have a default value")
	} else if *cfg.FrontendPort <= 0 || *cfg.FrontendPort > 65535 {
		verrs.Add(pathPrefix+".frontendPort", fmt.Sprintf("invalid port %d", *cfg.FrontendPort))
	}

	if cfg.Mode == nil {
		verrs.Add(pathPrefix+".mode", "is required and should have a default value 'tcp'")
	} else if *cfg.Mode != "" && !util.ContainsString(validHAProxyModes, *cfg.Mode) {
		verrs.Add(pathPrefix+".mode", fmt.Sprintf("invalid mode '%s', must be one of %v or empty for default", *cfg.Mode, validHAProxyModes))
	}

	if cfg.BalanceAlgorithm == nil {
		verrs.Add(pathPrefix+".balanceAlgorithm", "is required and should have a default value 'roundrobin'")
	} else if *cfg.BalanceAlgorithm != "" && !util.ContainsString(validHAProxyBalanceAlgorithms, *cfg.BalanceAlgorithm) {
		verrs.Add(pathPrefix+".balanceAlgorithm", fmt.Sprintf("invalid algorithm '%s', must be one of %v", *cfg.BalanceAlgorithm, validHAProxyBalanceAlgorithms))
	}

	if len(cfg.BackendServers) == 0 {
		verrs.Add(pathPrefix+".backendServers", "must specify at least one backend server")
	}
	for i, server := range cfg.BackendServers {
		serverPath := fmt.Sprintf("%s.backendServers[%d:%s]", pathPrefix, i, server.Name)
		if strings.TrimSpace(server.Name) == "" {
			verrs.Add(serverPath+".name", "backend server name cannot be empty")
		}
		if strings.TrimSpace(server.Address) == "" {
			verrs.Add(serverPath+".address", "backend server address cannot be empty")
		} else if !util.ValidateHostPortString(server.Address) && !util.IsValidIP(server.Address) && !util.IsValidDomainName(server.Address) {
			verrs.Add(serverPath+".address", fmt.Sprintf("invalid backend server address format '%s'", server.Address))
		}
		if server.Port <= 0 || server.Port > 65535 {
			verrs.Add(serverPath+".port", fmt.Sprintf("invalid backend server port %d", server.Port))
		}
		if server.Weight != nil && *server.Weight < 0 {
			verrs.Add(serverPath+".weight", fmt.Sprintf("cannot be negative, got %d", *server.Weight))
		}
	}
}
