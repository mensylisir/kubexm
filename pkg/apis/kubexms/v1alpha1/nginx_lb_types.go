package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"net"
	"strconv"
	"strings"
)

type NginxHealthCheck struct {
	MaxFails    *int    `json:"maxFails,omitempty" yaml:"maxFails,omitempty"`
	FailTimeout *string `json:"failTimeout,omitempty" yaml:"failTimeout,omitempty"`
}

type NginxLBUpstreamServer struct {
	Address     string            `json:"address" yaml:"address"`
	Weight      *int              `json:"weight,omitempty" yaml:"weight,omitempty"`
	HealthCheck *NginxHealthCheck `json:"healthCheck,omitempty" yaml:"healthCheck,omitempty"`
}

type NginxLBConfig struct {
	ListenAddress     *string                 `json:"listenAddress,omitempty" yaml:"listenAddress,omitempty"`
	ListenPort        *int                    `json:"listenPort,omitempty" yaml:"listenPort,omitempty"`
	Mode              *string                 `json:"mode,omitempty" yaml:"mode,omitempty"`
	BalanceAlgorithm  *string                 `json:"balanceAlgorithm,omitempty" yaml:"balanceAlgorithm,omitempty"`
	UpstreamServers   []NginxLBUpstreamServer `json:"upstreamServers,omitempty" yaml:"upstreamServers,omitempty"`
	ExtraHTTPConfig   []string                `json:"extraHttpConfig,omitempty" yaml:"extraHttpConfig,omitempty"`
	ExtraStreamConfig []string                `json:"extraStreamConfig,omitempty" yaml:"extraStreamConfig,omitempty"`
	ExtraServerConfig []string                `json:"extraServerConfig,omitempty" yaml:"extraServerConfig,omitempty"`
	ConfigFilePath    *string                 `json:"configFilePath,omitempty" yaml:"configFilePath,omitempty"`
	SkipInstall       *bool                   `json:"skipInstall,omitempty" yaml:"skipInstall,omitempty"`
}

func SetDefaults_NginxLBConfig(cfg *NginxLBConfig) {
	if cfg == nil {
		return
	}
	if cfg.ListenAddress == nil {
		cfg.ListenAddress = helpers.StrPtr(common.DefaultNginxListenAddress)
	}
	if cfg.ListenPort == nil {
		cfg.ListenPort = helpers.IntPtr(common.DefaultNginxListenPort)
	}
	if cfg.Mode == nil {
		cfg.Mode = helpers.StrPtr(common.DefaultNginxMode)
	}
	if cfg.BalanceAlgorithm == nil {
		cfg.BalanceAlgorithm = helpers.StrPtr(common.DefaultNginxAlgorithm)
	}
	if cfg.ConfigFilePath == nil {
		cfg.ConfigFilePath = helpers.StrPtr(common.DefaultNginxConfigFilePath)
	}
	if cfg.SkipInstall == nil {
		cfg.SkipInstall = helpers.BoolPtr(false)
	}

	for i := range cfg.UpstreamServers {
		SetDefaults_NginxLBUpstreamServer(&cfg.UpstreamServers[i])
	}
}

func SetDefaults_NginxLBUpstreamServer(server *NginxLBUpstreamServer) {
	if server.Weight == nil {
		server.Weight = helpers.IntPtr(common.DefaultNginxLBUpstreamServerWeight)
	}
	if server.HealthCheck == nil {
		server.HealthCheck = &NginxHealthCheck{}
	}
	SetDefaults_NginxHealthCheck(server.HealthCheck)
}

func SetDefaults_NginxHealthCheck(check *NginxHealthCheck) {
	if check.MaxFails == nil {
		check.MaxFails = helpers.IntPtr(common.DefaultNginxHealthCheckMaxFails)
	}
	if check.FailTimeout == nil {
		check.FailTimeout = helpers.StrPtr(common.DefaultNginxHealthCheckFailTimeout)
	}
}

func Validate_NginxLBConfig(cfg *NginxLBConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.SkipInstall != nil && *cfg.SkipInstall {
		return
	}

	if cfg.ListenAddress != nil {
		if strings.TrimSpace(*cfg.ListenAddress) == "" {
			verrs.Add(pathPrefix + ".listenAddress: cannot be empty if specified")
		} else if net.ParseIP(*cfg.ListenAddress) == nil && *cfg.ListenAddress != "0.0.0.0" && *cfg.ListenAddress != "::" {
			verrs.Add(pathPrefix + ".listenAddress: invalid IP address format '" + *cfg.ListenAddress + "'")
		}
	}

	if cfg.ListenPort == nil {
		verrs.Add(pathPrefix + ".listenPort: is required and should have a default value")
	} else if *cfg.ListenPort <= 0 || *cfg.ListenPort > 65535 {
		verrs.Add(pathPrefix + ".listenPort: invalid port " + fmt.Sprintf("%d", *cfg.ListenPort))
	}

	if cfg.Mode == nil {
		verrs.Add(pathPrefix + ".mode: is required and should have a default value 'tcp'")
	} else if *cfg.Mode != "" {
		if !helpers.ContainsString(common.ValidNginxLBModes, *cfg.Mode) {
			verrs.Add(pathPrefix + ".mode: invalid mode '" + *cfg.Mode + "', must be one of " + fmt.Sprintf("%v", common.ValidNginxLBModes))
		}
	}

	if cfg.BalanceAlgorithm == nil {
		verrs.Add(pathPrefix + ".balanceAlgorithm: is required and should have a default value 'round_robin'")
	} else if *cfg.BalanceAlgorithm != "" {
		if !helpers.ContainsString(common.ValidNginxLBAlgorithms, *cfg.BalanceAlgorithm) {
			verrs.Add(pathPrefix + ".balanceAlgorithm: invalid algorithm '" + *cfg.BalanceAlgorithm + "', must be one of " + fmt.Sprintf("%v", common.ValidNginxLBAlgorithms) + " or empty for Nginx default")
		}
	}

	if len(cfg.UpstreamServers) == 0 {
		verrs.Add(pathPrefix + ".upstreamServers: must specify at least one upstream server")
	}
	for i, server := range cfg.UpstreamServers {
		serverPath := fmt.Sprintf("%s.upstreamServers[%d]", pathPrefix, i)
		if strings.TrimSpace(server.Address) == "" {
			verrs.Add(serverPath + ".address: upstream server address cannot be empty")
		} else {
			host, portStr, err := net.SplitHostPort(server.Address)
			if err != nil {
				verrs.Add(serverPath + ".address: upstream server address '" + server.Address + "' must be in 'host:port' format")
			} else {
				if strings.TrimSpace(host) == "" {
					verrs.Add(serverPath + ".address: host part of upstream server address '" + server.Address + "' cannot be empty")
				} else if !helpers.IsValidIP(host) && !helpers.IsValidDomainName(host) {
					verrs.Add(serverPath + ".address: host part '" + host + "' of upstream server address '" + server.Address + "' is not a valid host or IP")
				}
				if port, errConv := strconv.Atoi(portStr); errConv != nil || port <= 0 || port > 65535 {
					verrs.Add(serverPath + ".address: port part '" + portStr + "' of upstream server address '" + server.Address + "' is not a valid port number")
				}
			}
		}
		if server.Weight != nil && *server.Weight < 0 {
			verrs.Add(serverPath + ".weight: cannot be negative")
		}
	}
	if cfg.ConfigFilePath != nil && strings.TrimSpace(*cfg.ConfigFilePath) == "" {
		verrs.Add(pathPrefix + ".configFilePath: cannot be empty if specified")
	}
}
