package v1alpha1

import (
	"net" // Add to imports of ha_types.go
	"strings"
	"fmt"     // For pathPrefix in validation calls
)

// HighAvailabilityConfig defines settings for cluster high availability.
type HighAvailabilityConfig struct {
	// Type of HA. e.g., "keepalived+haproxy", "keepalived+nginx_lb", "external_lb", "none".
	// "none" or empty implies no specific HA mechanism managed by this config.
	Type string `json:"type,omitempty"`

	// VIP is the virtual IP address for HA configurations like keepalived.
	// Required if Type involves keepalived.
	VIP string `json:"vip,omitempty"`

	ControlPlaneEndpointDomain string `json:"controlPlaneEndpointDomain,omitempty"`
	ControlPlaneEndpointAddress string `json:"controlPlaneEndpointAddress,omitempty"`
	ControlPlaneEndpointPort *int `json:"controlPlaneEndpointPort,omitempty"`

	// Keepalived configuration, applicable if Type includes "keepalived".
	Keepalived *KeepalivedConfig `json:"keepalived,omitempty"`

	// HAProxy configuration, applicable if Type includes "haproxy".
	HAProxy *HAProxyConfig `json:"haproxy,omitempty"`

	// NginxLB configuration, applicable if Type includes "nginx_lb".
	NginxLB *NginxLBConfig `json:"nginxLB,omitempty"`
}

// SetDefaults_HighAvailabilityConfig sets default values for HighAvailabilityConfig.
func SetDefaults_HighAvailabilityConfig(cfg *HighAvailabilityConfig) {
	if cfg == nil {
		return
	}
	// if cfg.Type == "" { // No default HA type, must be explicit or handled by higher logic
	//    cfg.Type = "none"
	// }
	if cfg.ControlPlaneEndpointPort == nil {
		defaultPort := 6443
		cfg.ControlPlaneEndpointPort = &defaultPort
	}

	// Initialize specific HA component configs based on Type
	// and call their SetDefaults.
	// This ensures that if a user specifies a type like "keepalived+haproxy"
	// but omits the 'keepalived: {}' or 'haproxy: {}' blocks,
	// those blocks get initialized and their own defaults applied.
	if strings.Contains(cfg.Type, "keepalived") {
		if cfg.Keepalived == nil {
			cfg.Keepalived = &KeepalivedConfig{}
		}
		SetDefaults_KeepalivedConfig(cfg.Keepalived)
	}
	if strings.Contains(cfg.Type, "haproxy") {
		if cfg.HAProxy == nil {
			cfg.HAProxy = &HAProxyConfig{}
		}
		SetDefaults_HAProxyConfig(cfg.HAProxy)
	}
	if strings.Contains(cfg.Type, "nginx_lb") { // Assuming "nginx_lb" is the type string
		if cfg.NginxLB == nil {
			cfg.NginxLB = &NginxLBConfig{}
		}
		SetDefaults_NginxLBConfig(cfg.NginxLB)
	}
}

// Validate_HighAvailabilityConfig validates HighAvailabilityConfig.
func Validate_HighAvailabilityConfig(cfg *HighAvailabilityConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	// More specific types are now possible, e.g., "keepalived+haproxy"
	// Basic validation could check for known keywords or patterns if Type is not empty.
	// For now, we'll validate sub-configs based on presence and Type hints.
	// Example: if cfg.Type == "" && (cfg.Keepalived != nil || cfg.HAProxy != nil || cfg.NginxLB != nil) {
	//    verrs.Add("%s.type: must be specified if keepalived, haproxy, or nginxLB sections are configured", pathPrefix)
	// }


	// VIP is required if any keepalived mechanism is implied by Type
	if strings.Contains(cfg.Type, "keepalived") && strings.TrimSpace(cfg.VIP) == "" {
		verrs.Add("%s.vip: must be set if HA type involves 'keepalived'", pathPrefix)
	}

	if cfg.VIP != "" && !isValidIP(cfg.VIP) {
		verrs.Add("%s.vip: invalid IP address format '%s'", pathPrefix, cfg.VIP)
	}
	if cfg.ControlPlaneEndpointAddress != "" && !isValidIP(cfg.ControlPlaneEndpointAddress) {
		verrs.Add("%s.controlPlaneEndpointAddress: invalid IP address format '%s'", pathPrefix, cfg.ControlPlaneEndpointAddress)
	}
	if cfg.ControlPlaneEndpointPort != nil && (*cfg.ControlPlaneEndpointPort <= 0 || *cfg.ControlPlaneEndpointPort > 65535) {
		verrs.Add("%s.controlPlaneEndpointPort: invalid port %d", pathPrefix, *cfg.ControlPlaneEndpointPort)
	}
	// ControlPlaneEndpointDomain could be validated for hostname format.

	// Validate Keepalived config if present or implied by type
	if strings.Contains(cfg.Type, "keepalived") {
		if cfg.Keepalived == nil {
			verrs.Add("%s.keepalived: configuration section must be present if type includes 'keepalived'", pathPrefix)
		} else {
			Validate_KeepalivedConfig(cfg.Keepalived, verrs, pathPrefix+".keepalived")
		}
	} else if cfg.Keepalived != nil { // Keepalived config present but type doesn't mention it
		verrs.Add("%s.type: must include 'keepalived' if keepalived configuration section is present", pathPrefix)
	}

	// Validate HAProxy config if present or implied by type
	if strings.Contains(cfg.Type, "haproxy") {
		if cfg.HAProxy == nil {
			verrs.Add("%s.haproxy: configuration section must be present if type includes 'haproxy'", pathPrefix)
		} else {
			Validate_HAProxyConfig(cfg.HAProxy, verrs, pathPrefix+".haproxy")
		}
	} else if cfg.HAProxy != nil {
		verrs.Add("%s.type: must include 'haproxy' if haproxy configuration section is present", pathPrefix)
	}

	// Validate NginxLB config if present or implied by type
	if strings.Contains(cfg.Type, "nginx_lb") { // Assuming "nginx_lb" is the type string
		if cfg.NginxLB == nil {
			verrs.Add("%s.nginxLB: configuration section must be present if type includes 'nginx_lb'", pathPrefix)
		} else {
			Validate_NginxLBConfig(cfg.NginxLB, verrs, pathPrefix+".nginxLB")
		}
	} else if cfg.NginxLB != nil {
		verrs.Add("%s.type: must include 'nginx_lb' if nginxLB configuration section is present", pathPrefix)
	}

	// Check for external_lb specifics
	if cfg.Type == "external_lb" {
		 if strings.TrimSpace(cfg.ControlPlaneEndpointAddress) == "" && strings.TrimSpace(cfg.ControlPlaneEndpointDomain) == "" {
			 verrs.Add("%s: either controlPlaneEndpointAddress or controlPlaneEndpointDomain must be set for external_lb type", pathPrefix)
		 }
		 // Ensure component-specific LB configs are not set for external_lb type
		 if cfg.Keepalived != nil { verrs.Add("%s.keepalived: should not be set for external_lb type", pathPrefix) }
		 if cfg.HAProxy != nil { verrs.Add("%s.haproxy: should not be set for external_lb type", pathPrefix) }
		 if cfg.NginxLB != nil { verrs.Add("%s.nginxLB: should not be set for external_lb type", pathPrefix) }
	}
}

func isValidIP(ip string) bool { return net.ParseIP(ip) != nil }
