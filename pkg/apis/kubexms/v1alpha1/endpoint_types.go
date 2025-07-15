package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"strings"
)

type ControlPlaneEndpointSpec struct {
	Domain                   string                          `json:"domain,omitempty" yaml:"domain,omitempty"`
	Address                  string                          `json:"address,omitempty" yaml:"lb_address,omitempty"`
	Port                     int                             `json:"port,omitempty" yaml:"port,omitempty"`
	ExternalDNS              *bool                           `json:"externalDNS,omitempty" yaml:"externalDNS,omitempty"`
	ExternalLoadBalancerType common.ExternalLoadBalancerType `json:"externalLoadBalancerType,omitempty" yaml:"externalLoadBalancer,omitempty"`
	InternalLoadBalancerType common.InternalLoadBalancerType `json:"internalLoadBalancerType,omitempty" yaml:"internalLoadbalancer,omitempty"`
	HighAvailability         *HighAvailability               `json:"highAvailability,omitempty" yaml:"highAvailability,omitempty"`
}

func SetDefaults_ControlPlaneEndpointSpec(cfg *ControlPlaneEndpointSpec) {
	if cfg == nil {
		return
	}
	if cfg.Port == 0 {
		cfg.Port = 6443
	}
	if cfg.ExternalLoadBalancerType == "" && cfg.InternalLoadBalancerType == "" {
		cfg.InternalLoadBalancerType = common.InternalLBTypeHAProxy
	}
	if cfg.Domain == "" {
		cfg.Domain = common.KubernetesDefaultDomain
	}
	if &cfg.ExternalDNS == nil {
		cfg.ExternalDNS = helpers.BoolPtr(false)
	}
	if cfg.Address != "" && helpers.IsValidIP(cfg.Address) && cfg.ExternalLoadBalancerType != "" {
		cfg.HighAvailability.Enabled = helpers.BoolPtr(true)
		cfg.HighAvailability.External.Enabled = helpers.BoolPtr(true)
		cfg.HighAvailability.External.Type = string(cfg.ExternalLoadBalancerType)
		cfg.HighAvailability.External.Keepalived.VRRPInstances[0].VirtualIPs[0] = cfg.Address
	}
	if cfg.InternalLoadBalancerType != "" {
		cfg.HighAvailability.Enabled = helpers.BoolPtr(true)
		cfg.HighAvailability.Internal.Enabled = helpers.BoolPtr(true)
		cfg.HighAvailability.Internal.Type = string(cfg.InternalLoadBalancerType)
	}
	if cfg.HighAvailability != nil {
		SetDefaults_HighAvailabilityConfig(cfg.HighAvailability)
	}
}

func Validate_ControlPlaneEndpointSpec(cfg *ControlPlaneEndpointSpec, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		// If this section is optional and not provided, it's valid.
		// If it's mandatory, the caller (e.g. Validate_ClusterSpec) should check for nil.
		return
	}
	if strings.TrimSpace(cfg.Domain) == "" {
		verrs.Add(pathPrefix + ": domain must be specified")
	}
	if cfg.ExternalLoadBalancerType != "" && strings.TrimSpace(cfg.Address) == "" {
		verrs.Add(pathPrefix + ": address must be specified when external-loadbalancer is enabled")
	}
	if cfg.ExternalLoadBalancerType == "" && strings.TrimSpace(cfg.Address) != "" {
		verrs.Add(pathPrefix + ": address must be empty when external-loadbalancer is not enabled")
	}
	if cfg.ExternalLoadBalancerType != "" && cfg.InternalLoadBalancerType != "" {
		verrs.Add(pathPrefix + ": internal-loadbalancer must be empty when external-loadbalancer is enabled")
	}
	if cfg.Domain != "" {
		if helpers.IsValidDomainName(cfg.Domain) {
			verrs.Add(fmt.Sprintf("%s.domain: '%s' is not a valid domain name", pathPrefix, cfg.Domain))
		}
	}
	if cfg.Address != "" {
		if helpers.IsValidIP(cfg.Address) {
			verrs.Add(fmt.Sprintf("%s.address (lb_address): invalid IP address format for '%s'", pathPrefix, cfg.Address))
		}
	}
	if cfg.Port != 0 && (cfg.Port <= 0 || cfg.Port > 65535) {
		verrs.Add(fmt.Sprintf("%s.port: invalid port %d, must be between 1-65535", pathPrefix, cfg.Port))
	}
	if cfg.ExternalLoadBalancerType != "" && !helpers.ContainsString(common.SupportedExternalLoadBalancerTypes, string(cfg.ExternalLoadBalancerType)) {
		verrs.Add(fmt.Sprintf("%s.externalLoadBalancerType: invalid type '%s', must be one of %v or empty",
			pathPrefix, cfg.ExternalLoadBalancerType, common.SupportedExternalLoadBalancerTypes))
	}

	if cfg.InternalLoadBalancerType != "" && !helpers.ContainsString(common.SupportedInternalLoadBalancerTypes, string(cfg.InternalLoadBalancerType)) {
		verrs.Add(fmt.Sprintf("%s.internalLoadBalancerType (internalLoadbalancer): invalid type '%s', must be one of %v or empty",
			pathPrefix, cfg.InternalLoadBalancerType, common.SupportedInternalLoadBalancerTypes))
	}

	if *cfg.HighAvailability.Enabled && *cfg.HighAvailability.External.Enabled && cfg.Address == "" {
		verrs.Add(fmt.Sprintf("%s.highAvailability.external.enabled and address cannot be empty", pathPrefix))
	}
	if cfg.ExternalDNS == helpers.BoolPtr(true) && cfg.Address != "" {
		verrs.Add(fmt.Sprintf("%s.externalDNS: cannot be true when address is provided", pathPrefix))
	}
}
