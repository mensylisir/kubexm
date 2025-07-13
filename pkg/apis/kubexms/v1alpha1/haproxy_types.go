package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
)

type HAProxyBackendServer struct {
	Name        string              `json:"name" yaml:"name"`
	Address     string              `json:"address" yaml:"address"`
	Port        int                 `json:"port" yaml:"port"`
	Weight      *int                `json:"weight,omitempty" yaml:"weight,omitempty"`
	HealthCheck *HAProxyHealthCheck `json:"healthCheck,omitempty" yaml:"healthCheck,omitempty"`
}

type HAProxyHealthCheck struct {
	Enabled  *bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Interval *string `json:"interval,omitempty" yaml:"interval,omitempty"`
	Rise     *int    `json:"rise,omitempty" yaml:"rise,omitempty"`
	Fall     *int    `json:"fall,omitempty" yaml:"fall,omitempty"`
}

type HAProxyConfig struct {
	FrontendBindAddress *string                `json:"frontendBindAddress,omitempty" yaml:"frontendBindAddress,omitempty"`
	FrontendPort        *int                   `json:"frontendPort,omitempty" yaml:"frontendPort,omitempty"`
	Mode                *string                `json:"mode,omitempty" yaml:"mode,omitempty"`
	BalanceAlgorithm    *string                `json:"balanceAlgorithm,omitempty" yaml:"balanceAlgorithm,omitempty"`
	BackendServers      []HAProxyBackendServer `json:"backendServers,omitempty" yaml:"backendServers,omitempty"`
	ExtraGlobalConfig   []string               `json:"extraGlobalConfig,omitempty" yaml:"extraGlobalConfig,omitempty"`
	ExtraDefaultsConfig []string               `json:"extraDefaultsConfig,omitempty" yaml:"extraDefaultsConfig,omitempty"`
	ExtraFrontendConfig []string               `json:"extraFrontendConfig,omitempty" yaml:"extraFrontendConfig,omitempty"`
	ExtraBackendConfig  []string               `json:"extraBackendConfig,omitempty" yaml:"extraBackendConfig,omitempty"`
	SkipInstall         *bool                  `json:"skipInstall,omitempty" yaml:"skipInstall,omitempty"`
}

func SetDefaults_HAProxyConfig(cfg *HAProxyConfig) {
	if cfg == nil {
		return
	}
	if cfg.FrontendBindAddress == nil {
		cfg.FrontendBindAddress = helpers.StrPtr("0.0.0.0")
	}
	if cfg.FrontendPort == nil {
		cfg.FrontendPort = helpers.IntPtr(common.HAProxyDefaultFrontendPort)
	}
	if cfg.Mode == nil {
		cfg.Mode = helpers.StrPtr(common.DefaultHAProxyMode)
	}
	if cfg.BalanceAlgorithm == nil {
		cfg.BalanceAlgorithm = helpers.StrPtr(common.DefaultHAProxyAlgorithm)
	}
	if cfg.SkipInstall == nil {
		cfg.SkipInstall = helpers.BoolPtr(false)
	}

	for i := range cfg.BackendServers {
		SetDefaults_HAProxyBackendServer(&cfg.BackendServers[i])
	}
}

func SetDefaults_HAProxyBackendServer(server *HAProxyBackendServer) {
	if server.Weight == nil {
		server.Weight = helpers.IntPtr(common.DefaultHAProxyWeight)
	}
	if server.HealthCheck == nil {
		server.HealthCheck = &HAProxyHealthCheck{}
	}
	SetDefaults_HAProxyHealthCheck(server.HealthCheck)
}

func SetDefaults_HAProxyHealthCheck(check *HAProxyHealthCheck) {
	if check.Enabled == nil {
		check.Enabled = helpers.BoolPtr(true)
	}
	if check.Interval == nil {
		check.Interval = helpers.StrPtr(common.DefaultHaproxyHealthCheckInterval)
	}
	if check.Rise == nil {
		check.Rise = helpers.IntPtr(common.DefaultHaproxyRise)
	}
	if check.Fall == nil {
		check.Fall = helpers.IntPtr(common.DefaultHaproxyFall)
	}
}

func Validate_HAProxyConfig(cfg *HAProxyConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if cfg.SkipInstall != nil && *cfg.SkipInstall {
		return
	}

	if cfg.FrontendBindAddress != nil {
		addr := *cfg.FrontendBindAddress
		if !helpers.IsValidIP(addr) {
			verrs.Add(pathPrefix+".frontendBindAddress", fmt.Sprintf("invalid IP address format '%s'", addr))
		}
	}

	if cfg.FrontendPort != nil {
		if !helpers.IsValidPort(*cfg.FrontendPort) {
			verrs.Add(pathPrefix+".frontendPort", fmt.Sprintf("invalid port %d, must be between 1 and 65535", *cfg.FrontendPort))
		}
	} else {
		verrs.Add(pathPrefix+".frontendPort", "is a required field")
	}

	if cfg.Mode != nil {
		if !helpers.ContainsString(common.ValidHAProxyModes, *cfg.Mode) {
			verrs.Add(pathPrefix+".mode", fmt.Sprintf("invalid mode '%s', must be one of %v", *cfg.Mode, common.ValidHAProxyModes))
		}
	}

	if cfg.BalanceAlgorithm != nil {
		if !helpers.ContainsString(common.ValidHAProxyBalanceAlgorithms, *cfg.BalanceAlgorithm) {
			verrs.Add(pathPrefix+".balanceAlgorithm", fmt.Sprintf("invalid algorithm '%s', must be one of %v", *cfg.BalanceAlgorithm, common.ValidHAProxyBalanceAlgorithms))
		}
	}

	if len(cfg.BackendServers) == 0 {
		verrs.Add(pathPrefix+".backendServers", "must specify at least one backend server")
	}
	for i, server := range cfg.BackendServers {
		serverPath := fmt.Sprintf("%s.backendServers[%d]", pathPrefix, i)
		Validate_HAProxyBackendServer(&server, verrs, serverPath)
	}
}

func Validate_HAProxyBackendServer(server *HAProxyBackendServer, verrs *validation.ValidationErrors, path string) {
	if !helpers.IsValidNonEmptyString(server.Name) {
		verrs.Add(path+".name", "cannot be empty")
	}
	if helpers.IsValidNonEmptyString(server.Name) {
		path = fmt.Sprintf("%s(name=%s)", path, server.Name)
	}

	if !helpers.IsValidIP(server.Address) && !helpers.IsValidDomainName(server.Address) {
		verrs.Add(path+".address", fmt.Sprintf("invalid address format '%s'; must be a valid IP or domain name", server.Address))
	}
	if !helpers.IsValidPort(server.Port) {
		verrs.Add(path+".port", fmt.Sprintf("invalid port %d; must be between 1 and 65535", server.Port))
	}
	if server.Weight != nil && !helpers.IsValidNonNegativeInteger(*server.Weight) {
		verrs.Add(path+".weight", fmt.Sprintf("cannot be negative, got %d", *server.Weight))
	}

	if server.HealthCheck != nil {
		Validate_HAProxyHealthCheck(server.HealthCheck, verrs, path+".healthCheck")
	}
}

func Validate_HAProxyHealthCheck(check *HAProxyHealthCheck, verrs *validation.ValidationErrors, path string) {
	if check.Interval != nil {
		if !helpers.IsValidDuration(*check.Interval) {
			verrs.Add(path+".interval", fmt.Sprintf("invalid duration format: '%s'", *check.Interval))
		}
	}
	if check.Rise != nil {
		if !helpers.IsValidPositiveInteger(*check.Rise) {
			verrs.Add(path+".rise", fmt.Sprintf("must be a positive integer, got %d", *check.Rise))
		}
	}
	if check.Fall != nil {
		if !helpers.IsValidPositiveInteger(*check.Fall) {
			verrs.Add(path+".fall", fmt.Sprintf("must be a positive integer, got %d", *check.Fall))
		}
	}
}
