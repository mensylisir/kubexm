package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
)

type ExternalLoadBalancerConfig struct {
	Enabled                   *bool             `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Type                      string            `json:"type,omitempty" yaml:"type,omitempty"`
	Keepalived                *KeepalivedConfig `json:"keepalived,omitempty" yaml:"keepalived,omitempty"`
	HAProxy                   *HAProxyConfig    `json:"haproxy,omitempty" yaml:"haproxy,omitempty"`
	NginxLB                   *NginxLBConfig    `json:"nginxLB,omitempty" yaml:"nginxLB,omitempty"`
	LoadBalancerHostGroupName *string           `json:"loadBalancerHostGroupName,omitempty" yaml:"loadBalancerHostGroupName,omitempty"`
}

type InternalLoadBalancerConfig struct {
	Enabled           *bool          `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Type              string         `json:"type,omitempty" yaml:"type,omitempty"`
	KubeVIP           *KubeVIPConfig `json:"kubevip,omitempty" yaml:"kubevip,omitempty"`
	WorkerNodeHAProxy *HAProxyConfig `json:"workerNodeHAProxy,omitempty" yaml:"workerNodeHAProxy,omitempty"`
	WorkerNodeNginxLB *NginxLBConfig `json:"workerNodeNginxLB,omitempty" yaml:"workerNodeNginxLB,omitempty"` // Reuses HAProxyConfig
}

type HighAvailability struct {
	Enabled  *bool                       `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	External *ExternalLoadBalancerConfig `json:"external,omitempty" yaml:"external,omitempty"`
	Internal *InternalLoadBalancerConfig `json:"internal,omitempty" yaml:"internal,omitempty"`
}

func SetDefaults_HighAvailabilityConfig(cfg *HighAvailability) {
	if cfg == nil {
		return
	}
	if cfg.Enabled == nil {
		cfg.Enabled = helpers.BoolPtr(false)
	}

	if *cfg.Enabled {
		if cfg.External != nil {
			SetDefaults_ExternalLoadBalancerConfig(cfg.External)
		}
		if cfg.Internal != nil {
			SetDefaults_InternalLoadBalancerConfig(cfg.Internal)
		}
	}
	if *cfg.External.Enabled && *cfg.Internal.Enabled {
		cfg.External.Enabled = helpers.BoolPtr(false)
		cfg.Internal.Enabled = helpers.BoolPtr(true)
	}
}

func SetDefaults_ExternalLoadBalancerConfig(cfg *ExternalLoadBalancerConfig) {
	if cfg == nil {
		return
	}
	if cfg.Enabled == nil {
		cfg.Enabled = helpers.BoolPtr(cfg.Type != "")
	}

	if *cfg.Enabled {
		switch cfg.Type {
		case string(common.ExternalLBTypeKubexmKH):
			if cfg.Keepalived == nil {
				cfg.Keepalived = &KeepalivedConfig{}
			}
			SetDefaults_KeepalivedConfig(cfg.Keepalived)
			if cfg.HAProxy == nil {
				cfg.HAProxy = &HAProxyConfig{}
			}
			SetDefaults_HAProxyConfig(cfg.HAProxy)
		case string(common.ExternalLBTypeKubexmKN):
			if cfg.Keepalived == nil {
				cfg.Keepalived = &KeepalivedConfig{}
			}
			SetDefaults_KeepalivedConfig(cfg.Keepalived)
			if cfg.NginxLB == nil {
				cfg.NginxLB = &NginxLBConfig{}
			}
			SetDefaults_NginxLBConfig(cfg.NginxLB)
		}
	}
}

func SetDefaults_InternalLoadBalancerConfig(cfg *InternalLoadBalancerConfig) {
	if cfg == nil {
		return
	}
	if cfg.Enabled == nil {
		cfg.Enabled = helpers.BoolPtr(cfg.Type != "")
	}

	if *cfg.Enabled {
		switch cfg.Type {
		case string(common.InternalLBTypeKubeVIP):
			if cfg.KubeVIP == nil {
				cfg.KubeVIP = &KubeVIPConfig{}
			}
			SetDefaults_KubeVIPConfig(cfg.KubeVIP)
		case string(common.InternalLBTypeHAProxy):
			if cfg.WorkerNodeHAProxy == nil {
				cfg.WorkerNodeHAProxy = &HAProxyConfig{}
			}
			SetDefaults_HAProxyConfig(cfg.WorkerNodeHAProxy)
		case string(common.InternalLBTypeNginx):
			if cfg.WorkerNodeNginxLB == nil {
				cfg.WorkerNodeNginxLB = &NginxLBConfig{}
			}
			SetDefaults_NginxLBConfig(cfg.WorkerNodeNginxLB)
		}
	}
}

func Validate_HighAvailabilityConfig(cfg *HighAvailability, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	if cfg.Enabled == nil || !*cfg.Enabled {
		if cfg.External != nil && cfg.External.Enabled != nil && *cfg.External.Enabled {
			verrs.Add(pathPrefix+".external.enabled", "cannot be true if global HA (highAvailability.enabled) is not true")
		}
		if cfg.Internal != nil && cfg.Internal.Enabled != nil && *cfg.Internal.Enabled {
			verrs.Add(pathPrefix+".internal.enabled", "cannot be true if global HA (highAvailability.enabled) is not true")
		}
		return
	}

	if cfg.External != nil {
		Validate_ExternalLoadBalancerConfig(cfg.External, verrs, pathPrefix+".external")
	}
	if cfg.Internal != nil {
		Validate_InternalLoadBalancerConfig(cfg.Internal, verrs, pathPrefix+".internal")
	}

	if (cfg.External == nil || cfg.External.Enabled == nil || !*cfg.External.Enabled) &&
		(cfg.Internal == nil || cfg.Internal.Enabled == nil || !*cfg.Internal.Enabled) {
		verrs.Add(pathPrefix+".enabled", "is true, but no internal or external load balancer is enabled")
	}
	if *cfg.External.Enabled && *cfg.Internal.Enabled {
		verrs.Add(pathPrefix+".enabled", "is true, but no internal or external load balancer is enabled")
	}

}

func Validate_ExternalLoadBalancerConfig(cfg *ExternalLoadBalancerConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Enabled == nil || !*cfg.Enabled {
		return
	}

	validTypes := []string{string(common.ExternalLBTypeKubexmKH), string(common.ExternalLBTypeKubexmKN), string(common.ExternalLBTypeExternal)}
	if !helpers.ContainsString(validTypes, cfg.Type) {
		verrs.Add(pathPrefix+".type", fmt.Sprintf("invalid external LB type '%s', must be one of %v", cfg.Type, validTypes))
		return
	}

	switch cfg.Type {
	case string(common.ExternalLBTypeKubexmKH):
		if cfg.Keepalived == nil {
			verrs.Add(pathPrefix+".keepalived", "must be configured for the selected type")
		} else {
			Validate_KeepalivedConfig(cfg.Keepalived, verrs, pathPrefix+".keepalived")
		}
		if cfg.HAProxy == nil {
			verrs.Add(pathPrefix+".haproxy", "must be configured for the selected type")
		} else {
			Validate_HAProxyConfig(cfg.HAProxy, verrs, pathPrefix+".haproxy")
		}
		if cfg.NginxLB != nil {
			verrs.Add(pathPrefix+".nginxLB", "must not be configured for the selected type")
		}
	case string(common.ExternalLBTypeKubexmKN):
		if cfg.Keepalived == nil {
			verrs.Add(pathPrefix+".keepalived", "must be configured for the selected type")
		} else {
			Validate_KeepalivedConfig(cfg.Keepalived, verrs, pathPrefix+".keepalived")
		}
		if cfg.NginxLB == nil {
			verrs.Add(pathPrefix+".nginxLB", "must be configured for the selected type")
		} else {
			Validate_NginxLBConfig(cfg.NginxLB, verrs, pathPrefix+".nginxLB")
		}
		if cfg.HAProxy != nil {
			verrs.Add(pathPrefix+".haproxy", "must not be configured for the selected type")
		}
	case string(common.ExternalLBTypeExternal):
		if cfg.Keepalived != nil || cfg.HAProxy != nil || cfg.NginxLB != nil {
			verrs.Add(pathPrefix, "no specific LB configurations (keepalived, haproxy, nginxLB) should be provided when type is 'UserProvided'")
		}
	}

	if cfg.Type != string(common.ExternalLBTypeExternal) {
		if !helpers.IsValidNonEmptyString(*cfg.LoadBalancerHostGroupName) {
			verrs.Add(pathPrefix+".loadBalancerHostGroupName", "must be specified for managed external load balancers")
		}
	}
}

func Validate_InternalLoadBalancerConfig(cfg *InternalLoadBalancerConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Enabled == nil || !*cfg.Enabled {
		return
	}

	if !helpers.ContainsString(common.ValidInternalLoadbalancerTypes, cfg.Type) {
		verrs.Add(pathPrefix+".type", fmt.Sprintf("invalid internal LB type '%s', must be one of %v", cfg.Type, common.ValidInternalLoadbalancerTypes))
		return
	}

	switch cfg.Type {
	case string(common.InternalLBTypeKubeVIP):
		if cfg.KubeVIP == nil {
			verrs.Add(pathPrefix+".kubevip", "must be configured for the selected type")
		} else {
			Validate_KubeVIPConfig(cfg.KubeVIP, verrs, pathPrefix+".kubevip")
		}
		if cfg.WorkerNodeHAProxy != nil || cfg.WorkerNodeNginxLB != nil {
			verrs.Add(pathPrefix, "only the 'kubevip' configuration should be provided for the selected type")
		}
	case string(common.InternalLBTypeHAProxy):
		if cfg.WorkerNodeHAProxy == nil {
			verrs.Add(pathPrefix+".workerNodeHAProxy", "must be configured for the selected type")
		} else {
			Validate_HAProxyConfig(cfg.WorkerNodeHAProxy, verrs, pathPrefix+".workerNodeHAProxy")
		}
		if cfg.KubeVIP != nil || cfg.WorkerNodeNginxLB != nil {
			verrs.Add(pathPrefix, "only the 'workerNodeHAProxy' configuration should be provided for the selected type")
		}
	case string(common.InternalLBTypeNginx):
		if cfg.WorkerNodeNginxLB == nil {
			verrs.Add(pathPrefix+".workerNodeNginxLB", "must be configured for the selected type")
		} else {
			Validate_NginxLBConfig(cfg.WorkerNodeNginxLB, verrs, pathPrefix+".workerNodeNginxLB")
		}
		if cfg.KubeVIP != nil || cfg.WorkerNodeHAProxy != nil {
			verrs.Add(pathPrefix, "only the 'workerNodeNginxLB' configuration should be provided for the selected type")
		}
	}
}
