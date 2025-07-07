package v1alpha1

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"github.com/mensylisir/kubexm/pkg/util"
	"github.com/mensylisir/kubexm/pkg/util/validation"
	"github.com/mensylisir/kubexm/pkg/common"
)

// NginxLBUpstreamServer defines a backend server for Nginx load balancing.
// Address should be in "host:port" format.
type NginxLBUpstreamServer struct {
	// Address is the IP:port or resolvable hostname:port of the backend server.
	Address string `json:"address"`
	// Weight for weighted load balancing algorithms (optional).
	Weight *int `json:"weight,omitempty"`
}

// NginxLBConfig defines settings for using Nginx as a TCP/HTTP load balancer,
// typically for services like the Kubernetes API server.
type NginxLBConfig struct {
	ListenAddress *string `json:"listenAddress,omitempty"`
	ListenPort *int `json:"listenPort,omitempty"`
	Mode *string `json:"mode,omitempty"`
	BalanceAlgorithm *string `json:"balanceAlgorithm,omitempty"`
	UpstreamServers []NginxLBUpstreamServer `json:"upstreamServers,omitempty"`
	ExtraHTTPConfig []string `json:"extraHttpConfig,omitempty"`
	ExtraStreamConfig []string `json:"extraStreamConfig,omitempty"`
	ExtraServerConfig []string `json:"extraServerConfig,omitempty"`
	ConfigFilePath *string `json:"configFilePath,omitempty"`
	SkipInstall *bool `json:"skipInstall,omitempty"`
}

// SetDefaults_NginxLBConfig sets default values for NginxLBConfig.
func SetDefaults_NginxLBConfig(cfg *NginxLBConfig) {
	if cfg == nil {
		return
	}
	if cfg.ListenAddress == nil {
		cfg.ListenAddress = util.StrPtr("0.0.0.0")
	}
	if cfg.ListenPort == nil {
		cfg.ListenPort = util.IntPtr(common.DefaultNginxListenPort) // Assuming 6443 or a specific const
	}
	if cfg.Mode == nil {
		cfg.Mode = util.StrPtr(common.DefaultNginxMode)
	}
	if cfg.BalanceAlgorithm == nil {
		cfg.BalanceAlgorithm = util.StrPtr(common.DefaultNginxAlgorithm)
	}
	if cfg.UpstreamServers == nil {
		cfg.UpstreamServers = []NginxLBUpstreamServer{}
	}
	for i := range cfg.UpstreamServers {
		server := &cfg.UpstreamServers[i]
		if server.Weight == nil {
			server.Weight = util.IntPtr(1)
		}
	}
	if cfg.ExtraHTTPConfig == nil { cfg.ExtraHTTPConfig = []string{} }
	if cfg.ExtraStreamConfig == nil { cfg.ExtraStreamConfig = []string{} }
	if cfg.ExtraServerConfig == nil { cfg.ExtraServerConfig = []string{} }
	if cfg.ConfigFilePath == nil {
		cfg.ConfigFilePath = util.StrPtr(common.DefaultNginxConfigFilePath) // Assuming "/etc/nginx/nginx.conf"
	}
	if cfg.SkipInstall == nil {
		cfg.SkipInstall = util.BoolPtr(false)
	}
}

// Validate_NginxLBConfig validates NginxLBConfig.
func Validate_NginxLBConfig(cfg *NginxLBConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.SkipInstall != nil && *cfg.SkipInstall {
		return
	}

	if cfg.ListenAddress != nil {
		if strings.TrimSpace(*cfg.ListenAddress) == "" {
			verrs.Add(pathPrefix+".listenAddress", "cannot be empty if specified")
		} else if net.ParseIP(*cfg.ListenAddress) == nil && *cfg.ListenAddress != "0.0.0.0" && *cfg.ListenAddress != "::" {
			verrs.Add(pathPrefix+".listenAddress", fmt.Sprintf("invalid IP address format '%s'", *cfg.ListenAddress))
		}
	}
	// ListenAddress can be nil, allowing Nginx to use its own default binding.

	if cfg.ListenPort == nil { // Should be set by defaults
		verrs.Add(pathPrefix+".listenPort", "is required and should have a default value")
	} else if *cfg.ListenPort <= 0 || *cfg.ListenPort > 65535 {
		verrs.Add(pathPrefix+".listenPort", fmt.Sprintf("invalid port %d", *cfg.ListenPort))
	}

	if cfg.Mode == nil { // Should be set by defaults
		verrs.Add(pathPrefix+".mode", "is required and should have a default value 'tcp'")
	} else if *cfg.Mode != "" { // Validate only if user provided a non-empty value
	   validModes := []string{"tcp", "http"} // These could be constants
	   if !util.ContainsString(validModes, *cfg.Mode) {
		   verrs.Add(pathPrefix+".mode", fmt.Sprintf("invalid mode '%s', must be one of %v", *cfg.Mode, validModes))
	   }
	}

	if cfg.BalanceAlgorithm == nil { // Should be set by defaults
		verrs.Add(pathPrefix+".balanceAlgorithm", "is required and should have a default value 'round_robin'")
	} else if *cfg.BalanceAlgorithm != "" { // Validate only if user provided a non-empty value
		// Nginx's default is round_robin for stream, and various for http depending on context.
		// For simplicity, we list common ones. If empty, Nginx will use its internal default.
		validAlgos := []string{"round_robin", "least_conn", "ip_hash", "hash", "random", "least_time"} // These could be constants
		if !util.ContainsString(validAlgos, *cfg.BalanceAlgorithm) {
			verrs.Add(pathPrefix+".balanceAlgorithm", fmt.Sprintf("invalid algorithm '%s', must be one of %v or empty for Nginx default", *cfg.BalanceAlgorithm, validAlgos))
		}
	}

	if len(cfg.UpstreamServers) == 0 {
		verrs.Add(pathPrefix+".upstreamServers", "must specify at least one upstream server")
	}
	for i, server := range cfg.UpstreamServers {
		serverPath := fmt.Sprintf("%s.upstreamServers[%d]", pathPrefix, i)
		if strings.TrimSpace(server.Address) == "" {
			verrs.Add(serverPath+".address", "upstream server address cannot be empty")
		} else {
			host, portStr, err := net.SplitHostPort(server.Address)
			if err != nil {
				verrs.Add(serverPath+".address", fmt.Sprintf("upstream server address '%s' must be in 'host:port' format", server.Address))
			} else {
				if strings.TrimSpace(host) == "" {
					verrs.Add(serverPath+".address", fmt.Sprintf("host part of upstream server address '%s' cannot be empty", server.Address))
				} else if !util.IsValidIP(host) && !util.IsValidDomainName(host) {
					verrs.Add(serverPath+".address", fmt.Sprintf("host part '%s' of upstream server address '%s' is not a valid host or IP", host, server.Address))
				}
				if port, errConv := strconv.Atoi(portStr); errConv != nil || port <= 0 || port > 65535 {
					verrs.Add(serverPath+".address", fmt.Sprintf("port part '%s' of upstream server address '%s' is not a valid port number", portStr, server.Address))
				}
			}
		}
		if server.Weight != nil && *server.Weight < 0 {
			verrs.Add(serverPath+".weight", fmt.Sprintf("cannot be negative, got %d", *server.Weight))
		}
	}
	if cfg.ConfigFilePath != nil && strings.TrimSpace(*cfg.ConfigFilePath) == "" {
	   verrs.Add(pathPrefix+".configFilePath", "cannot be empty if specified")
	}
}
