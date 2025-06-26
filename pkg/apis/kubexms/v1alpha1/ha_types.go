package v1alpha1

import (
	// "net" // Removed as unused locally, assuming isValidIP is from elsewhere or not used here
	"strings"
	// "fmt"     // Removed as unused locally
)

// isValidIP helper (if not already present or imported from a shared location)
// func isValidIP(ip string) bool { return net.ParseIP(ip) != nil }


// ExternalLoadBalancerConfig defines settings for an external load balancing solution.
type ExternalLoadBalancerConfig struct {
	// Enabled indicates if an external load balancer solution is to be used or configured.
	Enabled *bool `json:"enabled,omitempty"`

	// Type specifies the kind of external load balancer.
	// Examples: "UserProvided", "ManagedKeepalivedHAProxy", "ManagedKeepalivedNginxLB".
	Type string `json:"type,omitempty"` // e.g., UserProvided, ManagedKeepalivedHAProxy

	// Keepalived configuration, used if Type involves "ManagedKeepalived*".
	Keepalived *KeepalivedConfig `json:"keepalived,omitempty"`
	// HAProxy configuration, used if Type is "ManagedKeepalivedHAProxy".
	HAProxy *HAProxyConfig `json:"haproxy,omitempty"`
	// NginxLB configuration, used if Type is "ManagedKeepalivedNginxLB".
	NginxLB *NginxLBConfig `json:"nginxLB,omitempty"`

	// LoadBalancerHostGroupName specifies the group of hosts (from ClusterSpec.Hosts)
	// that will run the managed load balancer components (Keepalived, HAProxy, NginxLB).
	// If empty, it might default to control-plane nodes or require explicit node roles.
	LoadBalancerHostGroupName *string `json:"loadBalancerHostGroupName,omitempty"`
}

// InternalLoadBalancerConfig defines settings for an internal load balancing solution.
type InternalLoadBalancerConfig struct {
	// Enabled indicates if an internal load balancer solution is to be used.
	Enabled *bool `json:"enabled,omitempty"`

	// Type specifies the kind of internal load balancer.
	// Examples: "KubeVIP", "WorkerNodeHAProxy" (HAProxy pods on workers).
	Type string `json:"type,omitempty"`

	// KubeVIP configuration, used if Type is "KubeVIP".
	KubeVIP *KubeVIPConfig `json:"kubevip,omitempty"` // KubeVIPConfig to be defined in kubevip_types.go

	// WorkerNodeHAProxy configuration, used if Type is "WorkerNodeHAProxy".
	// This might reuse HAProxyConfig or a simplified version. For now, assume HAProxyConfig.
	WorkerNodeHAProxy *HAProxyConfig `json:"workerNodeHAProxy,omitempty"`
}

// HighAvailabilityConfig defines settings for cluster high availability.
type HighAvailabilityConfig struct {
	// Enabled flag allows to completely turn on or off the HA configuration processing.
	// If false, all other HA settings might be ignored. Defaults to false.
	Enabled *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"`

	// External load balancing configuration.
	External *ExternalLoadBalancerConfig `json:"external,omitempty" yaml:"external,omitempty"`

	// Internal load balancing configuration.
	Internal *InternalLoadBalancerConfig `json:"internal,omitempty" yaml:"internal,omitempty"`

	// ControlPlaneEndpoint field is removed as it's directly in ClusterSpec.
	// VIP field is removed as it's deprecated and covered by ClusterSpec.ControlPlaneEndpoint.Address.
}


// --- Defaulting Functions ---
func SetDefaults_HighAvailabilityConfig(cfg *HighAvailabilityConfig) {
	if cfg == nil {
		return
	}
	if cfg.Enabled == nil {
		cfg.Enabled = boolPtr(false) // HA features off by default unless specified
	}

	// ControlPlaneEndpoint is no longer part of this struct.
	// Its defaults are handled when SetDefaults_Cluster calls SetDefaults_ControlPlaneEndpointSpec.

	if !*cfg.Enabled { // If HA is not enabled, don't default specific HA sub-configs like External/Internal LBs
		return
	}

	// Default External LB config only if HA is enabled
	if cfg.External == nil {
		cfg.External = &ExternalLoadBalancerConfig{}
	}
	SetDefaults_ExternalLoadBalancerConfig(cfg.External) // Removed parentHA argument

	// Default Internal LB config
	if cfg.Internal == nil {
		cfg.Internal = &InternalLoadBalancerConfig{}
	}
	SetDefaults_InternalLoadBalancerConfig(cfg.Internal)
}

// SetDefaults_ExternalLoadBalancerConfig now does not need parentHA for VIP logic.
// That logic should be handled by whatever sets ClusterSpec.ControlPlaneEndpoint.Address,
// possibly using information from cfg.Keepalived.VIP if managed Keepalived is chosen.
func SetDefaults_ExternalLoadBalancerConfig(cfg *ExternalLoadBalancerConfig) {
	if cfg == nil { return }
	if cfg.Enabled == nil {
		enabledVal := false
		if cfg.Type != "" && (strings.Contains(cfg.Type, "Managed") || cfg.Type == "UserProvided") {
			enabledVal = true
		}
		cfg.Enabled = boolPtr(enabledVal)
	}

	if cfg.Enabled != nil && *cfg.Enabled {
		if strings.Contains(cfg.Type, "Keepalived") {
			if cfg.Keepalived == nil { cfg.Keepalived = &KeepalivedConfig{} }
			SetDefaults_KeepalivedConfig(cfg.Keepalived)
		}
		if strings.Contains(cfg.Type, "HAProxy") {
			if cfg.HAProxy == nil { cfg.HAProxy = &HAProxyConfig{} }
			SetDefaults_HAProxyConfig(cfg.HAProxy)
		}
		if strings.Contains(cfg.Type, "NginxLB") {
			if cfg.NginxLB == nil { cfg.NginxLB = &NginxLBConfig{} }
			SetDefaults_NginxLBConfig(cfg.NginxLB)
		}
	}
}

func SetDefaults_InternalLoadBalancerConfig(cfg *InternalLoadBalancerConfig) {
	if cfg == nil { return }
	if cfg.Enabled == nil { cfg.Enabled = boolPtr(false) } // Internal LB not enabled by default

	if cfg.Enabled != nil && *cfg.Enabled {
		if cfg.Type == "KubeVIP" { // Example type
			if cfg.KubeVIP == nil { cfg.KubeVIP = &KubeVIPConfig{} }
			SetDefaults_KubeVIPConfig(cfg.KubeVIP)
		}
		if cfg.Type == "WorkerNodeHAProxy" { // Example type
			if cfg.WorkerNodeHAProxy == nil { cfg.WorkerNodeHAProxy = &HAProxyConfig{} }
			SetDefaults_HAProxyConfig(cfg.WorkerNodeHAProxy)
		}
	}
}


// --- Validation Functions ---
func Validate_HighAvailabilityConfig(cfg *HighAvailabilityConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { return }

	if cfg.Enabled != nil && *cfg.Enabled {
		// If HA is enabled, there should be some configuration for either external or internal LB,
		// If HA is enabled, there should be some configuration for either external or internal LB.
		// The actual ControlPlaneEndpoint (domain/address/port) is validated as part of ClusterSpec.
		isExternalLBConfigured := cfg.External != nil && cfg.External.Enabled != nil && *cfg.External.Enabled
		isInternalLBConfigured := cfg.Internal != nil && cfg.Internal.Enabled != nil && *cfg.Internal.Enabled

		// This validation might be too strict if HA.Enabled=true but user provides ControlPlaneEndpoint manually
		// without explicitly defining external/internal LB types within HAConfig.
		// The primary driver for LB deployment should be ControlPlaneEndpointSpec.ExternalLoadBalancerType etc.
		// For now, let's assume if HA.Enabled=true, one of the LB configs within HAConfig should also be enabled.
		if !isExternalLBConfigured && !isInternalLBConfigured {
			// verrs.Add("%s: if HA is enabled, either external or internal load balancing sub-configuration should also be enabled", pathPrefix)
			// This might be too strong. HA.Enabled could just mean "HA is desired", and CPE defines how.
			// Let's remove this specific check for now. The validation of CPE itself is more important.
		}

		// Validate External and Internal LB configs if they are present
		if cfg.External != nil {
			Validate_ExternalLoadBalancerConfig(cfg.External, verrs, pathPrefix+".external")
		}
		if cfg.Internal != nil {
			Validate_InternalLoadBalancerConfig(cfg.Internal, verrs, pathPrefix+".internal")
		}
		// VIP field removed. ControlPlaneEndpoint is validated at ClusterSpec level.

	} else { // HA not enabled
	   // If HA is disabled, external and internal LBs (if defined) should also be effectively disabled or validated as such.
	   // Current logic correctly flags external/internal.enabled=true as an error if HA.enabled=false.
	   if cfg.External != nil && cfg.External.Enabled != nil && *cfg.External.Enabled {
		   verrs.Add("%s.external.enabled: cannot be true if global HA is not enabled", pathPrefix)
	   }
	   if cfg.Internal != nil && cfg.Internal.Enabled != nil && *cfg.Internal.Enabled {
		   verrs.Add("%s.internal.enabled: cannot be true if global HA is not enabled", pathPrefix)
	   }
	}
}

// Validate_ExternalLoadBalancerConfig validates ExternalLoadBalancerConfig.
// The parentHA argument has been removed as ControlPlaneEndpoint is now part of ClusterSpec.
// Cross-validation with ControlPlaneEndpoint should occur at a higher level if needed.
func Validate_ExternalLoadBalancerConfig(cfg *ExternalLoadBalancerConfig, verrs *ValidationErrors, pathPrefix string) { // Ensure this has 3 parameters
	if cfg == nil || cfg.Enabled == nil || !*cfg.Enabled { return } // Only validate if explicitly enabled

	// Type validation now uses ControlPlaneEndpointConfig.ExternalLoadBalancerType
	// The 'Type' field in ExternalLoadBalancerConfig might become redundant or used for more specific managed types.
	// For now, assuming cfg.Type is still relevant for "ManagedKeepalivedHAProxy" etc.
	// This part needs careful review based on how ControlPlaneEndpointConfig.ExternalLoadBalancerType and ExternalLoadBalancerConfig.Type interact.

	// Example based on current structure:
	validManagedTypes := []string{"ManagedKeepalivedHAProxy", "ManagedKeepalivedNginxLB"} // Assuming these are distinct from CPE types
	isManagedType := false
	for _, mt := range validManagedTypes {
		if cfg.Type == mt {
			isManagedType = true
			break
		}
	}
	if cfg.Type == "UserProvided" {
		// Validation for ControlPlaneEndpoint being set for UserProvided LBs should be done
		// at a higher level where ClusterSpec.ControlPlaneEndpoint is accessible.
		if cfg.Keepalived != nil { verrs.Add("%s.keepalived: should not be set for UserProvided external LB type", pathPrefix) }
		if cfg.HAProxy != nil { verrs.Add("%s.haproxy: should not be set for UserProvided external LB type", pathPrefix) }
		if cfg.NginxLB != nil { verrs.Add("%s.nginxLB: should not be set for UserProvided external LB type", pathPrefix) }

	} else if isManagedType {
		if strings.Contains(cfg.Type, "Keepalived") {
			if cfg.Keepalived == nil { verrs.Add("%s.keepalived: section must be present if type includes 'Keepalived'", pathPrefix)
			} else { Validate_KeepalivedConfig(cfg.Keepalived, verrs, pathPrefix+".keepalived") } // Validate_KeepalivedConfig is now in endpoint_types.go
		}
		if strings.Contains(cfg.Type, "HAProxy") {
			if cfg.HAProxy == nil { verrs.Add("%s.haproxy: section must be present if type includes 'HAProxy'", pathPrefix)
			} else { Validate_HAProxyConfig(cfg.HAProxy, verrs, pathPrefix+".haproxy") } // Validate_HAProxyConfig is now in endpoint_types.go
		}
		if strings.Contains(cfg.Type, "NginxLB") {
			if cfg.NginxLB == nil { verrs.Add("%s.nginxLB: section must be present if type includes 'NginxLB'", pathPrefix)
			} else { Validate_NginxLBConfig(cfg.NginxLB, verrs, pathPrefix+".nginxLB") } // Validate_NginxLBConfig is now in endpoint_types.go
		}
		if cfg.LoadBalancerHostGroupName != nil && strings.TrimSpace(*cfg.LoadBalancerHostGroupName) == "" {
			verrs.Add("%s.loadBalancerHostGroupName: cannot be empty if specified for managed external LB", pathPrefix)
		}
	} else if cfg.Type != "" { // Type is set but not "UserProvided" or a known managed type
		verrs.Add("%s.type: unknown external LB type '%s'", pathPrefix, cfg.Type)
	}
}


func Validate_InternalLoadBalancerConfig(cfg *InternalLoadBalancerConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil || cfg.Enabled == nil || !*cfg.Enabled { return }

	// Similar to External, InternalLoadBalancerConfig.Type vs ControlPlaneEndpointConfig.InternalLoadBalancerType needs clarity.
	// Assuming cfg.Type is for specific implementations like "KubeVIP", "WorkerNodeHAProxy".

	if cfg.Type == "KubeVIP" {
		if cfg.KubeVIP == nil { verrs.Add("%s.kubevip: section must be present if type is 'KubeVIP'", pathPrefix)
		} else { Validate_KubeVIPConfig(cfg.KubeVIP, verrs, pathPrefix+".kubevip") } // Validate_KubeVIPConfig is now in endpoint_types.go
	} else if cfg.Type == "WorkerNodeHAProxy" {
		if cfg.WorkerNodeHAProxy == nil { verrs.Add("%s.workerNodeHAProxy: section must be present if type is 'WorkerNodeHAProxy'", pathPrefix)
		} else { Validate_HAProxyConfig(cfg.WorkerNodeHAProxy, verrs, pathPrefix+".workerNodeHAProxy") } // Validate_HAProxyConfig is now in endpoint_types.go
	} else if cfg.Type != "" {
		verrs.Add("%s.type: unknown internal LB type '%s'", pathPrefix, cfg.Type)
	}
}

// func isValidIP(ip string) bool { return net.ParseIP(ip) != nil } // This is now in endpoint_types.go
