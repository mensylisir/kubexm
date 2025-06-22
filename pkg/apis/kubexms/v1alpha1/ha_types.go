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

	// ControlPlaneEndpoint specifies the main endpoint for accessing the Kubernetes API server.
	// This might be user-provided if using an unmanaged external LB, or derived from Keepalived VIP.
	ControlPlaneEndpoint *ControlPlaneEndpointSpec `json:"controlPlaneEndpoint,omitempty" yaml:"controlPlaneEndpoint,omitempty"` // Type is already ControlPlaneEndpointSpec, ensure YAML tag is present

	// External load balancing configuration.
	External *ExternalLoadBalancerConfig `json:"external,omitempty" yaml:"external,omitempty"`

	// Internal load balancing configuration.
	Internal *InternalLoadBalancerConfig `json:"internal,omitempty"`

	// VIP is the virtual IP address. This field is DEPRECATED in favor of
	// ControlPlaneEndpoint.Address when managed by Keepalived, or directly set in ControlPlaneEndpoint.
	// It might still be used by KeepalivedConfig internally.
	// For backward compatibility or direct Keepalived setup, it can remain.
	// Consider if this top-level VIP is still needed or if all VIP logic moves into KeepalivedConfig
	// and ControlPlaneEndpointConfig.Address becomes the source of truth.
	// For now, let's keep it but note its potential deprecation for endpoint configuration.
	VIP string `json:"vip,omitempty"`
	// Fields like ControlPlaneEndpointDomain, ControlPlaneEndpointAddress, ControlPlaneEndpointPort
	// are now moved into the ControlPlaneEndpointConfig struct.
}


// --- Defaulting Functions ---
func SetDefaults_HighAvailabilityConfig(cfg *HighAvailabilityConfig) {
	if cfg == nil {
		return
	}
	if cfg.Enabled == nil {
		b := false // HA features off by default unless specified
		cfg.Enabled = &b
	}

	// Always ensure ControlPlaneEndpoint is initialized if HighAvailabilityConfig itself exists.
	if cfg.ControlPlaneEndpoint == nil {
		cfg.ControlPlaneEndpoint = &ControlPlaneEndpointSpec{}
	}
	// Call its defaults regardless of HA.Enabled, as endpoint might be set manually.
	SetDefaults_ControlPlaneEndpointSpec(cfg.ControlPlaneEndpoint) // This function is in endpoint_types.go


	if !*cfg.Enabled { // If HA is not enabled, don't default specific HA sub-configs like External/Internal LBs
		return
	}

	// Default External LB config only if HA is enabled
	if cfg.External == nil {
		cfg.External = &ExternalLoadBalancerConfig{}
	}
	SetDefaults_ExternalLoadBalancerConfig(cfg.External, cfg) // Pass parent HA cfg for context

	// Default Internal LB config
	if cfg.Internal == nil {
		cfg.Internal = &InternalLoadBalancerConfig{}
	}
	SetDefaults_InternalLoadBalancerConfig(cfg.Internal)
}

func SetDefaults_ExternalLoadBalancerConfig(cfg *ExternalLoadBalancerConfig, parentHA *HighAvailabilityConfig) {
	if cfg == nil { return }
	if cfg.Enabled == nil {
		b := false // External LB not enabled by default
		// If parentHA.Type implies external (e.g. "external_lb" or "Managed*"), this could be true.
		// For now, explicit enable.
		// Default Enabled to true if a specific Type is set for the external LB itself.
		// This avoids relying on a non-existent parentHA.Type.
		if cfg.Type != "" && (strings.Contains(cfg.Type, "Managed") || cfg.Type == "UserProvided") {
			b = true
		}
		cfg.Enabled = &b
	}

	if cfg.Enabled != nil && *cfg.Enabled {
		if strings.Contains(cfg.Type, "Keepalived") {
			if cfg.Keepalived == nil { cfg.Keepalived = &KeepalivedConfig{} }
			SetDefaults_KeepalivedConfig(cfg.Keepalived)
			// If Keepalived is used, its VIP might inform ControlPlaneEndpoint.Address.
			// This logic relies on parentHA.VIP and parentHA.ControlPlaneEndpoint.
			// It's kept for now but might need review if parentHA.VIP is fully deprecated.
			if parentHA != nil && parentHA.ControlPlaneEndpoint != nil &&
			   parentHA.ControlPlaneEndpoint.Address == "" && // Only if not already set
			   cfg.Keepalived != nil && parentHA.VIP != "" { // And VIP is available
				parentHA.ControlPlaneEndpoint.Address = parentHA.VIP
			}
		}
		if strings.Contains(cfg.Type, "HAProxy") {
			if cfg.HAProxy == nil { cfg.HAProxy = &HAProxyConfig{} }
			SetDefaults_HAProxyConfig(cfg.HAProxy)
		}
		if strings.Contains(cfg.Type, "NginxLB") { // Adjusted from parentHA.Type
			if cfg.NginxLB == nil { cfg.NginxLB = &NginxLBConfig{} }
			SetDefaults_NginxLBConfig(cfg.NginxLB)
		}
	}
}

func SetDefaults_InternalLoadBalancerConfig(cfg *InternalLoadBalancerConfig) {
	if cfg == nil { return }
	if cfg.Enabled == nil { b := false; cfg.Enabled = &b } // Internal LB not enabled by default

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
		// or a directly configured ControlPlaneEndpoint.
		isExternalLBConfigured := cfg.External != nil && cfg.External.Enabled != nil && *cfg.External.Enabled
		isInternalLBConfigured := cfg.Internal != nil && cfg.Internal.Enabled != nil && *cfg.Internal.Enabled
		isEndpointSetManually := cfg.ControlPlaneEndpoint != nil &&
								(cfg.ControlPlaneEndpoint.Address != "" || cfg.ControlPlaneEndpoint.Domain != "")

		if !isExternalLBConfigured && !isInternalLBConfigured && !isEndpointSetManually {
			verrs.Add("%s: if enabled, either external, internal load balancing, or a direct controlPlaneEndpoint must be configured", pathPrefix)
		}

		// Validate ControlPlaneEndpoint if HA is enabled
		if cfg.ControlPlaneEndpoint == nil && (isExternalLBConfigured || isInternalLBConfigured) { // Endpoint might be derived
			// If derived, it should have been populated by defaults or a planning step.
			// For static validation, if it's nil but expected to be derived, it's hard to check here.
			// Let's assume if HA components are set, endpoint should eventually be non-nil.
		} else if cfg.ControlPlaneEndpoint != nil {
			Validate_ControlPlaneEndpointSpec(cfg.ControlPlaneEndpoint, verrs, pathPrefix+".controlPlaneEndpoint") // This function is in endpoint_types.go
		}


		if cfg.External != nil { // cfg.External can exist even if not enabled
			Validate_ExternalLoadBalancerConfig(cfg.External, verrs, pathPrefix+".external", cfg)
		}
		if cfg.Internal != nil {
			Validate_InternalLoadBalancerConfig(cfg.Internal, verrs, pathPrefix+".internal")
		}
		// Top-level VIP validation (if still used)
		if cfg.VIP != "" && !isValidIP(cfg.VIP) { // isValidIP will be from endpoint_types.go
			verrs.Add("%s.vip: invalid IP address format '%s'", pathPrefix, cfg.VIP)
		}

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

func Validate_ExternalLoadBalancerConfig(cfg *ExternalLoadBalancerConfig, verrs *ValidationErrors, pathPrefix string, parentHA *HighAvailabilityConfig) {
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
		if parentHA == nil || parentHA.ControlPlaneEndpoint == nil || (parentHA.ControlPlaneEndpoint.Address == "" && parentHA.ControlPlaneEndpoint.Domain == "") {
			verrs.Add("%s: if type is UserProvided, parent controlPlaneEndpoint address or domain must be set", pathPrefix)
		}
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
