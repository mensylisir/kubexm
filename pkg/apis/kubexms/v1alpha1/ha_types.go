package v1alpha1

import (
	"fmt"
	"strings"
	"github.com/mensylisir/kubexm/pkg/util/validation"
)

// ExternalLoadBalancerConfig defines settings for an external load balancing solution.
type ExternalLoadBalancerConfig struct {
	// Type specifies the kind of external load balancer.
	// Examples: "UserProvided", "ManagedKeepalivedHAProxy", "ManagedKeepalivedNginxLB".
	Type string `json:"type,omitempty"` // e.g., UserProvided, ManagedKeepalivedHAProxy

	Keepalived *KeepalivedConfig `json:"keepalived,omitempty"`
	HAProxy    *HAProxyConfig    `json:"haproxy,omitempty"`
	NginxLB    *NginxLBConfig    `json:"nginxLB,omitempty"`

	LoadBalancerHostGroupName *string `json:"loadBalancerHostGroupName,omitempty"`
}

// InternalLoadBalancerConfig defines settings for an internal load balancing solution.
type InternalLoadBalancerConfig struct {
	// Type specifies the kind of internal load balancer.
	// Examples: "KubeVIP", "WorkerNodeHAProxy".
	Type string `json:"type,omitempty"`

	KubeVIP           *KubeVIPConfig `json:"kubevip,omitempty"`
	WorkerNodeHAProxy *HAProxyConfig `json:"workerNodeHAProxy,omitempty"`
}

// HighAvailabilityConfig defines settings for cluster high availability.
type HighAvailabilityConfig struct {
	Enabled  *bool                       `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	External *ExternalLoadBalancerConfig `json:"external,omitempty" yaml:"external,omitempty"`
	Internal *InternalLoadBalancerConfig `json:"internal,omitempty" yaml:"internal,omitempty"`
}

func SetDefaults_HighAvailabilityConfig(cfg *HighAvailabilityConfig) {
	if cfg == nil {
		return
	}
	if cfg.Enabled == nil {
		cfg.Enabled = boolPtr(false)
	}

	if !*cfg.Enabled {
		// If HA is not enabled, ensure External and Internal are nil to prevent defaulting/validation of sub-configs.
		// This makes the intent clearer: if HA is off, specific LB configs are irrelevant.
		cfg.External = nil
		cfg.Internal = nil
		return
	}

	// If HA is enabled, and External/Internal sections are provided (even if empty), default them.
	if cfg.External != nil {
		SetDefaults_ExternalLoadBalancerConfig(cfg.External)
	} else {
        // If HA is enabled but External is nil, we might want to initialize it
        // if there's a sensible default for External an HA setup.
        // For now, we assume if External is nil, user does not want external LB.
	}

	if cfg.Internal != nil {
		SetDefaults_InternalLoadBalancerConfig(cfg.Internal)
	} else {
        // Similarly for Internal.
	}
}

func SetDefaults_ExternalLoadBalancerConfig(cfg *ExternalLoadBalancerConfig) {
	if cfg == nil {
		return
	}
	// Type being set implies the user wants this LB.
	// Sub-configs are defaulted if their corresponding type is matched.
	if cfg.Type != "" {
		if strings.Contains(cfg.Type, "Keepalived") {
			if cfg.Keepalived == nil {
				cfg.Keepalived = &KeepalivedConfig{}
			}
			SetDefaults_KeepalivedConfig(cfg.Keepalived)
		}
		if strings.Contains(cfg.Type, "HAProxy") { // This applies to ManagedKeepalivedHAProxy
			if cfg.HAProxy == nil {
				cfg.HAProxy = &HAProxyConfig{}
			}
			SetDefaults_HAProxyConfig(cfg.HAProxy)
		}
		if strings.Contains(cfg.Type, "NginxLB") {
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
	// Type being set implies the user wants this LB.
	if cfg.Type == "KubeVIP" {
		if cfg.KubeVIP == nil {
			cfg.KubeVIP = &KubeVIPConfig{}
		}
		SetDefaults_KubeVIPConfig(cfg.KubeVIP)
	}
	if cfg.Type == "WorkerNodeHAProxy" {
		if cfg.WorkerNodeHAProxy == nil {
			cfg.WorkerNodeHAProxy = &HAProxyConfig{}
		}
		SetDefaults_HAProxyConfig(cfg.WorkerNodeHAProxy)
	}
}

func Validate_HighAvailabilityConfig(cfg *HighAvailabilityConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	if cfg.Enabled != nil && *cfg.Enabled {
		if cfg.External != nil {
			Validate_ExternalLoadBalancerConfig(cfg.External, verrs, pathPrefix+".external")
		}
		if cfg.Internal != nil {
			Validate_InternalLoadBalancerConfig(cfg.Internal, verrs, pathPrefix+".internal")
		}
	} else {
		if cfg.External != nil && cfg.External.Type != "" {
			verrs.Add(pathPrefix+".external", "cannot be configured if global HA is disabled")
		}
		if cfg.Internal != nil && cfg.Internal.Type != "" {
			verrs.Add(pathPrefix+".internal", "cannot be configured if global HA is disabled")
		}
	}
}

func Validate_ExternalLoadBalancerConfig(cfg *ExternalLoadBalancerConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Type == "" {
		return
	}

	validManagedTypes := []string{"ManagedKeepalivedHAProxy", "ManagedKeepalivedNginxLB"}
	isManagedType := false
	for _, mt := range validManagedTypes {
		if cfg.Type == mt {
			isManagedType = true
			break
		}
	}

	if cfg.Type == "UserProvided" {
		if cfg.Keepalived != nil {
			verrs.Add(pathPrefix+".keepalived", "should not be set for UserProvided external LB type")
		}
		if cfg.HAProxy != nil {
			verrs.Add(pathPrefix+".haproxy", "should not be set for UserProvided external LB type")
		}
		if cfg.NginxLB != nil {
			verrs.Add(pathPrefix+".nginxLB", "should not be set for UserProvided external LB type")
		}
	} else if isManagedType {
		if strings.Contains(cfg.Type, "Keepalived") {
			if cfg.Keepalived == nil {
				verrs.Add(pathPrefix+".keepalived", "section must be present if type includes 'Keepalived'")
			} else {
				Validate_KeepalivedConfig(cfg.Keepalived, verrs, pathPrefix+".keepalived")
			}
		}
		if strings.Contains(cfg.Type, "HAProxy") {
			if cfg.HAProxy == nil {
				verrs.Add(pathPrefix+".haproxy", "section must be present if type includes 'HAProxy'")
			} else {
				Validate_HAProxyConfig(cfg.HAProxy, verrs, pathPrefix+".haproxy")
			}
		}
		if strings.Contains(cfg.Type, "NginxLB") {
			if cfg.NginxLB == nil {
				verrs.Add(pathPrefix+".nginxLB", "section must be present if type includes 'NginxLB'")
			} else {
				Validate_NginxLBConfig(cfg.NginxLB, verrs, pathPrefix+".nginxLB")
			}
		}
		if cfg.LoadBalancerHostGroupName != nil && strings.TrimSpace(*cfg.LoadBalancerHostGroupName) == "" {
			verrs.Add(pathPrefix+".loadBalancerHostGroupName", "cannot be empty if specified for managed external LB")
		}
	} else {
		verrs.Add(pathPrefix+".type", fmt.Sprintf("unknown external LB type '%s'", cfg.Type))
	}
}

func Validate_InternalLoadBalancerConfig(cfg *InternalLoadBalancerConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Type == "" {
		return
	}

	if cfg.Type == "KubeVIP" {
		if cfg.KubeVIP == nil {
			verrs.Add(pathPrefix+".kubevip", "section must be present if type is 'KubeVIP'")
		} else {
			Validate_KubeVIPConfig(cfg.KubeVIP, verrs, pathPrefix+".kubevip")
		}
	} else if cfg.Type == "WorkerNodeHAProxy" {
		if cfg.WorkerNodeHAProxy == nil {
			verrs.Add(pathPrefix+".workerNodeHAProxy", "section must be present if type is 'WorkerNodeHAProxy'")
		} else {
			Validate_HAProxyConfig(cfg.WorkerNodeHAProxy, verrs, pathPrefix+".workerNodeHAProxy")
		}
	} else {
		verrs.Add(pathPrefix+".type", fmt.Sprintf("unknown internal LB type '%s'", cfg.Type))
	}
}
