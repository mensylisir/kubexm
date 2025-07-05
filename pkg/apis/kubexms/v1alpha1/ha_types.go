package v1alpha1

import (
	"strings"
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

func Validate_HighAvailabilityConfig(cfg *HighAvailabilityConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	if cfg.Enabled != nil && *cfg.Enabled {
		// If HA is enabled, at least one of External or Internal should be configured if HA is truly desired.
		// However, an empty HA block (External=nil, Internal=nil) with Enabled=true is not strictly invalid by itself,
		// it just means no specific HA LBs are being set up by this config.
		// The actual requirement for an LB might come from ControlPlaneEndpoint settings.

		if cfg.External != nil {
			Validate_ExternalLoadBalancerConfig(cfg.External, verrs, pathPrefix+".external")
		}
		if cfg.Internal != nil {
			Validate_InternalLoadBalancerConfig(cfg.Internal, verrs, pathPrefix+".internal")
		}
	} else { // HA is explicitly disabled (Enabled is *false) or not set (defaults to false)
		// If HA is disabled, then External and Internal LB configurations should not be present or should be empty.
		if cfg.External != nil && cfg.External.Type != "" {
			verrs.Add("%s.external: cannot be configured if global HA is disabled", pathPrefix)
		}
		if cfg.Internal != nil && cfg.Internal.Type != "" {
			verrs.Add("%s.internal: cannot be configured if global HA is disabled", pathPrefix)
		}
	}
}

func Validate_ExternalLoadBalancerConfig(cfg *ExternalLoadBalancerConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Type == "" { // If no type is specified, it's considered not configured.
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
			verrs.Add("%s.keepalived: should not be set for UserProvided external LB type", pathPrefix)
		}
		if cfg.HAProxy != nil {
			verrs.Add("%s.haproxy: should not be set for UserProvided external LB type", pathPrefix)
		}
		if cfg.NginxLB != nil {
			verrs.Add("%s.nginxLB: should not be set for UserProvided external LB type", pathPrefix)
		}
	} else if isManagedType {
		if strings.Contains(cfg.Type, "Keepalived") {
			if cfg.Keepalived == nil {
				verrs.Add("%s.keepalived: section must be present if type includes 'Keepalived'", pathPrefix)
			} else {
				Validate_KeepalivedConfig(cfg.Keepalived, verrs, pathPrefix+".keepalived")
			}
		}
		if strings.Contains(cfg.Type, "HAProxy") { // Specifically for ManagedKeepalivedHAProxy
			if cfg.HAProxy == nil {
				verrs.Add("%s.haproxy: section must be present if type includes 'HAProxy'", pathPrefix)
			} else {
				Validate_HAProxyConfig(cfg.HAProxy, verrs, pathPrefix+".haproxy")
			}
		}
		if strings.Contains(cfg.Type, "NginxLB") { // Specifically for ManagedKeepalivedNginxLB
			if cfg.NginxLB == nil {
				verrs.Add("%s.nginxLB: section must be present if type includes 'NginxLB'", pathPrefix)
			} else {
				Validate_NginxLBConfig(cfg.NginxLB, verrs, pathPrefix+".nginxLB")
			}
		}
		if cfg.LoadBalancerHostGroupName != nil && strings.TrimSpace(*cfg.LoadBalancerHostGroupName) == "" {
			verrs.Add("%s.loadBalancerHostGroupName: cannot be empty if specified for managed external LB", pathPrefix)
		}
	} else { // Type is set but not "UserProvided" or a known managed type
		verrs.Add("%s.type: unknown external LB type '%s'", pathPrefix, cfg.Type)
	}
}

func Validate_InternalLoadBalancerConfig(cfg *InternalLoadBalancerConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Type == "" { // If no type is specified, it's considered not configured.
		return
	}

	if cfg.Type == "KubeVIP" {
		if cfg.KubeVIP == nil {
			verrs.Add("%s.kubevip: section must be present if type is 'KubeVIP'", pathPrefix)
		} else {
			Validate_KubeVIPConfig(cfg.KubeVIP, verrs, pathPrefix+".kubevip")
		}
	} else if cfg.Type == "WorkerNodeHAProxy" {
		if cfg.WorkerNodeHAProxy == nil {
			verrs.Add("%s.workerNodeHAProxy: section must be present if type is 'WorkerNodeHAProxy'", pathPrefix)
		} else {
			Validate_HAProxyConfig(cfg.WorkerNodeHAProxy, verrs, pathPrefix+".workerNodeHAProxy")
		}
	} else {
		verrs.Add("%s.type: unknown internal LB type '%s'", pathPrefix, cfg.Type)
	}
}
