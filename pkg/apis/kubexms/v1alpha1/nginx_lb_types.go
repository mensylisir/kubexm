package v1alpha1

import (
	"fmt"
	"strings"
	// "net" // For IP/port validation if needed for BackendServers
)

// NginxLBUpstreamServer defines a backend server for Nginx load balancing.
// Address should be in "host:port" format.
type NginxLBUpstreamServer struct {
	// Address is the IP:port or resolvable hostname:port of the backend server.
	Address string `json:"address"`
	// Weight for weighted load balancing algorithms (optional).
	Weight *int `json:"weight,omitempty"`
	// TODO: Add other server options like 'max_fails', 'fail_timeout' if needed.
}

// NginxLBConfig defines settings for using Nginx as a TCP/HTTP load balancer,
// typically for services like the Kubernetes API server.
type NginxLBConfig struct {
	// ListenAddress is the address Nginx should bind its server block to.
	// Defaults to "0.0.0.0" (all interfaces).
	ListenAddress *string `json:"listenAddress,omitempty"`

	// ListenPort is the port Nginx listens on for the load-balanced service.
	// Defaults to 6443 or a common load balancer port like 443 for HTTPS.
	ListenPort *int `json:"listenPort,omitempty"`

	// Mode indicates the type of load balancing.
	// "tcp" (for stream block, e.g. Kube API) or "http" (for http block).
	// Defaults to "tcp".
	Mode *string `json:"mode,omitempty"`

	// BalanceAlgorithm for upstream server selection (used in 'upstream' block).
	// e.g., "round_robin" (implicit default if none specified), "least_conn", "ip_hash".
	// If empty, Nginx defaults to round-robin.
	BalanceAlgorithm *string `json:"balanceAlgorithm,omitempty"`

	// UpstreamServers is a list of backend servers to load balance.
	UpstreamServers []NginxLBUpstreamServer `json:"upstreamServers,omitempty"`

	// ExtraHTTPConfig allows adding raw lines to the 'http' block of nginx.conf.
	ExtraHTTPConfig []string `json:"extraHttpConfig,omitempty"`
	// ExtraStreamConfig allows adding raw lines to the 'stream' block of nginx.conf (for TCP LB).
	ExtraStreamConfig []string `json:"extraStreamConfig,omitempty"`
	// ExtraServerConfig allows adding raw lines to the specific 'server' block being configured.
	ExtraServerConfig []string `json:"extraServerConfig,omitempty"`

	// ConfigFilePath is the path to the main nginx.conf file.
	// Defaults to a system path like "/etc/nginx/nginx.conf".
	ConfigFilePath *string `json:"configFilePath,omitempty"`

	// SkipInstall, if true, assumes Nginx is already installed and configured externally.
	SkipInstall *bool `json:"skipInstall,omitempty"`
}

// --- Defaulting Functions ---

// SetDefaults_NginxLBConfig sets default values for NginxLBConfig.
func SetDefaults_NginxLBConfig(cfg *NginxLBConfig) {
	if cfg == nil {
		return
	}
	if cfg.ListenAddress == nil {
		cfg.ListenAddress = stringPtr("0.0.0.0")
	}
	if cfg.ListenPort == nil {
		cfg.ListenPort = intPtr(6443) // Default for Kube API, or 443 if mode is HTTP for general LB
	}
	if cfg.Mode == nil {
		cfg.Mode = stringPtr("tcp") // Default to TCP load balancing for API server like use-cases
	}
	// No default for BalanceAlgorithm, Nginx uses round-robin by default.

	if cfg.UpstreamServers == nil {
		cfg.UpstreamServers = []NginxLBUpstreamServer{}
	}
	for i := range cfg.UpstreamServers {
		server := &cfg.UpstreamServers[i]
		if server.Weight == nil {
			server.Weight = intPtr(1) // Default weight
		}
	}

	if cfg.ExtraHTTPConfig == nil { cfg.ExtraHTTPConfig = []string{} }
	if cfg.ExtraStreamConfig == nil { cfg.ExtraStreamConfig = []string{} }
	if cfg.ExtraServerConfig == nil { cfg.ExtraServerConfig = []string{} }

	if cfg.ConfigFilePath == nil {
		cfg.ConfigFilePath = stringPtr("/etc/nginx/nginx.conf") // Common default path
	}

	if cfg.SkipInstall == nil {
		cfg.SkipInstall = boolPtr(false) // Default to managing Nginx installation
	}
}

// --- Validation Functions ---

// Validate_NginxLBConfig validates NginxLBConfig.
// Note: ValidationErrors type is expected to be defined in cluster_types.go or a common errors file.
func Validate_NginxLBConfig(cfg *NginxLBConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.SkipInstall != nil && *cfg.SkipInstall {
		return
	}

	if cfg.ListenAddress != nil && strings.TrimSpace(*cfg.ListenAddress) == "" {
	   verrs.Add("%s.listenAddress: cannot be empty if specified", pathPrefix)
	   // Could add IP address validation here
	}

	// ListenPort is always defaulted, so check its value.
	if *cfg.ListenPort <= 0 || *cfg.ListenPort > 65535 {
		verrs.Add("%s.listenPort: invalid port %d", pathPrefix, *cfg.ListenPort)
	}

	if cfg.Mode != nil && *cfg.Mode != "" {
	   validModes := []string{"tcp", "http"}
	   if !containsString(validModes, *cfg.Mode) {
		   verrs.Add("%s.mode: invalid mode '%s', must be 'tcp' or 'http'", pathPrefix, *cfg.Mode)
	   }
	}

	if cfg.BalanceAlgorithm != nil && *cfg.BalanceAlgorithm != "" {
	   // Nginx built-in for stream: round_robin (default), least_conn, hash, random
	   // Nginx built-in for http: round_robin (default), least_conn, ip_hash, generic_hash, random, least_time
	   validAlgos := []string{"round_robin", "least_conn", "ip_hash", "hash", "random", "least_time"}
	   if !containsString(validAlgos, *cfg.BalanceAlgorithm) {
		   verrs.Add("%s.balanceAlgorithm: invalid algorithm '%s'", pathPrefix, *cfg.BalanceAlgorithm)
	   }
	}

	if len(cfg.UpstreamServers) == 0 {
		verrs.Add("%s.upstreamServers: must specify at least one upstream server", pathPrefix)
	}
	for i, server := range cfg.UpstreamServers {
		serverPath := fmt.Sprintf("%s.upstreamServers[%d]", pathPrefix, i)
		if strings.TrimSpace(server.Address) == "" {
			verrs.Add("%s.address: upstream server address cannot be empty", serverPath)
		} else {
			// Validate "host:port" format
			parts := strings.Split(server.Address, ":")
			if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
				verrs.Add("%s.address: upstream server address '%s' must be in 'host:port' format", serverPath, server.Address)
			}
			// Further validation for host and port parts can be added.
		}
		if server.Weight != nil && *server.Weight < 0 {
		   verrs.Add("%s.weight: cannot be negative, got %d", serverPath, *server.Weight)
		}
	}
	if cfg.ConfigFilePath != nil && strings.TrimSpace(*cfg.ConfigFilePath) == "" {
	   verrs.Add("%s.configFilePath: cannot be empty if specified", pathPrefix)
	}
}
