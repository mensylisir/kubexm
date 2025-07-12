package v1alpha1

import (
	"strings"
	// Assuming ValidationErrors is in cluster_types.go or a shared util in this package
	// Assuming KeepalivedConfig, HAProxyConfig, NginxLBConfig, KubeVIPConfig are defined elsewhere in this package
)

// ExternalLoadBalancerConfig defines settings for an external load balancing solution.
type ExternalLoadBalancerConfig struct {
	Enabled                   *bool   `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Type                      string  `json:"type,omitempty" yaml:"type,omitempty"`
	Keepalived                *KeepalivedConfig `json:"keepalived,omitempty" yaml:"keepalived,omitempty"`
	HAProxy                   *HAProxyConfig    `json:"haproxy,omitempty" yaml:"haproxy,omitempty"`
	NginxLB                   *NginxLBConfig    `json:"nginxLB,omitempty" yaml:"nginxLB,omitempty"`
	LoadBalancerHostGroupName *string `json:"loadBalancerHostGroupName,omitempty" yaml:"loadBalancerHostGroupName,omitempty"`
}

// InternalLoadBalancerConfig defines settings for an internal load balancing solution.
type InternalLoadBalancerConfig struct {
	Enabled           *bool          `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	Type              string         `json:"type,omitempty" yaml:"type,omitempty"`
	KubeVIP           *KubeVIPConfig `json:"kubevip,omitempty" yaml:"kubevip,omitempty"`
	WorkerNodeHAProxy *HAProxyConfig `json:"workerNodeHAProxy,omitempty" yaml:"workerNodeHAProxy,omitempty"` // Reuses HAProxyConfig
}

// HighAvailabilityConfig defines settings for cluster high availability.
type HighAvailabilityConfig struct {
	Enabled  *bool                       `json:"enabled,omitempty" yaml:"enabled,omitempty"`
	External *ExternalLoadBalancerConfig `json:"external,omitempty" yaml:"external,omitempty"`
	Internal *InternalLoadBalancerConfig `json:"internal,omitempty" yaml:"internal,omitempty"`
}

// SetDefaults_HighAvailabilityConfig sets default values for HighAvailabilityConfig.
func SetDefaults_HighAvailabilityConfig(cfg *HighAvailabilityConfig) {
	if cfg == nil {
		return
	}
	if cfg.Enabled == nil {
		b := false
		cfg.Enabled = &b
	}
	if !*cfg.Enabled {
		return
	}
	if cfg.External == nil {
		cfg.External = &ExternalLoadBalancerConfig{}
	}
	SetDefaults_ExternalLoadBalancerConfig(cfg.External)
	if cfg.Internal == nil {
		cfg.Internal = &InternalLoadBalancerConfig{}
	}
	SetDefaults_InternalLoadBalancerConfig(cfg.Internal)
}

func SetDefaults_ExternalLoadBalancerConfig(cfg *ExternalLoadBalancerConfig) {
	if cfg == nil { return }
	if cfg.Enabled == nil {
		b := false
		if cfg.Type != "" && (strings.Contains(cfg.Type, "Managed") || cfg.Type == "UserProvided") {
			b = true // Enable if a type is specified that implies it should be active
		}
		cfg.Enabled = &b
	}
	if cfg.Enabled != nil && *cfg.Enabled {
		if strings.Contains(cfg.Type, "Keepalived") {
			if cfg.Keepalived == nil { cfg.Keepalived = &KeepalivedConfig{} }
			SetDefaults_KeepalivedConfig(cfg.Keepalived) // Assumes this func exists
		}
		if strings.Contains(cfg.Type, "HAProxy") { // Covers ManagedKeepalivedHAProxy
			if cfg.HAProxy == nil { cfg.HAProxy = &HAProxyConfig{} }
			SetDefaults_HAProxyConfig(cfg.HAProxy) // Assumes this func exists
		}
		if strings.Contains(cfg.Type, "NginxLB") { // Covers ManagedKeepalivedNginxLB
			if cfg.NginxLB == nil { cfg.NginxLB = &NginxLBConfig{} }
			SetDefaults_NginxLBConfig(cfg.NginxLB) // Assumes this func exists
		}
	}
}

func SetDefaults_InternalLoadBalancerConfig(cfg *InternalLoadBalancerConfig) {
	if cfg == nil { return }
	if cfg.Enabled == nil { b := false; cfg.Enabled = &b }
	if cfg.Enabled != nil && *cfg.Enabled {
		if cfg.Type == "KubeVIP" {
			if cfg.KubeVIP == nil { cfg.KubeVIP = &KubeVIPConfig{} }
			SetDefaults_KubeVIPConfig(cfg.KubeVIP) // Assumes this func exists
		}
		if cfg.Type == "WorkerNodeHAProxy" {
			if cfg.WorkerNodeHAProxy == nil { cfg.WorkerNodeHAProxy = &HAProxyConfig{} }
			SetDefaults_HAProxyConfig(cfg.WorkerNodeHAProxy) // Reuses HAProxy defaults
		}
	}
}

// Validate_HighAvailabilityConfig validates HighAvailabilityConfig.
func Validate_HighAvailabilityConfig(cfg *HighAvailabilityConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { return }
	if cfg.Enabled != nil && *cfg.Enabled {
		// If HA.Enabled is true, but ControlPlaneEndpoint does not specify any LB type,
		// then one of the internal/external sections here must be enabled and configured.
		// This cross-validation is complex and might be better handled at the ClusterSpec.Validate level.
		// For now, just validate sub-sections if present.

		if cfg.External != nil {
			Validate_ExternalLoadBalancerConfig(cfg.External, verrs, pathPrefix+".external")
		}
		if cfg.Internal != nil {
			Validate_InternalLoadBalancerConfig(cfg.Internal, verrs, pathPrefix+".internal")
		}
	} else { // HA not enabled (either nil or *Enabled is false)
	   if cfg.External != nil && cfg.External.Enabled != nil && *cfg.External.Enabled {
		   verrs.Add(pathPrefix+".external.enabled", "cannot be true if global HA (highAvailability.enabled) is not true")
	   }
	   if cfg.Internal != nil && cfg.Internal.Enabled != nil && *cfg.Internal.Enabled {
		   verrs.Add(pathPrefix+".internal.enabled", "cannot be true if global HA (highAvailability.enabled) is not true")
	   }
	}
}

func Validate_ExternalLoadBalancerConfig(cfg *ExternalLoadBalancerConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Enabled == nil || !*cfg.Enabled { return }

	// These types are for the specific managed solutions within ExternalLoadBalancerConfig
	validManagedTypes := []string{"ManagedKeepalivedHAProxy", "ManagedKeepalivedNginxLB", "UserProvided", ""}
	if !containsString(validManagedTypes, cfg.Type) { // Assuming containsString is available
		verrs.Add(pathPrefix + ".type: unknown or unsupported external LB type '" + cfg.Type + "' in HA config")
	}

	if cfg.Type == "UserProvided" {
		if cfg.Keepalived != nil { verrs.Add(pathPrefix+".keepalived", "should not be set for UserProvided external LB type") }
		if cfg.HAProxy != nil { verrs.Add(pathPrefix+".haproxy", "should not be set for UserProvided external LB type") }
		if cfg.NginxLB != nil { verrs.Add(pathPrefix+".nginxLB", "should not be set for UserProvided external LB type") }
	} else if strings.Contains(cfg.Type, "Managed") {
		if strings.Contains(cfg.Type, "Keepalived") {
			if cfg.Keepalived == nil { verrs.Add(pathPrefix+".keepalived", "section must be present if type includes 'Keepalived'")
			} else { Validate_KeepalivedConfig(cfg.Keepalived, verrs, pathPrefix+".keepalived") }
		}
		if strings.Contains(cfg.Type, "HAProxy") {
			if cfg.HAProxy == nil { verrs.Add(pathPrefix+".haproxy", "section must be present if type includes 'HAProxy'")
			} else { Validate_HAProxyConfig(cfg.HAProxy, verrs, pathPrefix+".haproxy") }
		}
		if strings.Contains(cfg.Type, "NginxLB") {
			if cfg.NginxLB == nil { verrs.Add(pathPrefix+".nginxLB", "section must be present if type includes 'NginxLB'")
			} else { Validate_NginxLBConfig(cfg.NginxLB, verrs, pathPrefix+".nginxLB") }
		}
		if cfg.LoadBalancerHostGroupName != nil && strings.TrimSpace(*cfg.LoadBalancerHostGroupName) == "" {
			verrs.Add(pathPrefix+".loadBalancerHostGroupName", "cannot be empty if specified for managed external LB")
		}
	}
}

func Validate_InternalLoadBalancerConfig(cfg *InternalLoadBalancerConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Enabled == nil || !*cfg.Enabled { return }
	validInternalTypes := []string{"KubeVIP", "WorkerNodeHAProxy", ""}
	if !containsString(validInternalTypes, cfg.Type) { // Assuming containsString is available
		verrs.Add(pathPrefix + ".type: unknown or unsupported internal LB type '" + cfg.Type + "' in HA config")
	}

	if cfg.Type == "KubeVIP" {
		if cfg.KubeVIP == nil { verrs.Add(pathPrefix+".kubevip", "section must be present if type is 'KubeVIP'")
		} else { Validate_KubeVIPConfig(cfg.KubeVIP, verrs, pathPrefix+".kubevip") }
	} else if cfg.Type == "WorkerNodeHAProxy" {
		if cfg.WorkerNodeHAProxy == nil { verrs.Add(pathPrefix+".workerNodeHAProxy", "section must be present if type is 'WorkerNodeHAProxy'")
		} else { Validate_HAProxyConfig(cfg.WorkerNodeHAProxy, verrs, pathPrefix+".workerNodeHAProxy") }
	}
}

// NOTE: DeepCopy methods should be generated by controller-gen.
// Assumes KeepalivedConfig, HAProxyConfig, NginxLBConfig, KubeVIPConfig and their SetDefaults/Validate functions
// are defined in their respective files (e.g., keepalived_types.go, haproxy_types.go, etc.)
// Assumes ValidationErrors and containsString helper are available from cluster_types.go or a shared util.
// Corrected logic in SetDefaults_ExternalLoadBalancerConfig for enabling.
// Refined validation logic for HA.Enabled vs sub-LB enabled states.
// Simplified validation for ExternalLoadBalancerConfig and InternalLoadBalancerConfig types.
