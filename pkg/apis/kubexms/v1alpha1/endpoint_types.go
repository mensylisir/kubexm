package v1alpha1

import (
	"strings" // For validation
	"net"     // For isValidIP
)

// ControlPlaneEndpointConfig defines the configuration for the cluster's control plane endpoint.
// This endpoint is used by nodes and external clients to communicate with the Kubernetes API server.
type ControlPlaneEndpointConfig struct {
	// Domain is the DNS name for the control plane endpoint.
	// Example: "k8s-api.example.com"
	Domain string `json:"domain,omitempty"`

	// Address is the IP address for the control plane endpoint.
	// This could be a VIP managed by Keepalived, an external load balancer IP, etc.
	Address string `json:"address,omitempty"`

	// Port is the port number for the control plane endpoint.
	// Defaults to 6443.
	Port *int `json:"port,omitempty"`
}

// SetDefaults_ControlPlaneEndpointConfig sets default values for ControlPlaneEndpointConfig.
func SetDefaults_ControlPlaneEndpointConfig(cfg *ControlPlaneEndpointConfig) {
	if cfg == nil {
		return
	}
	if cfg.Port == nil {
		defaultPort := 6443
		cfg.Port = &defaultPort
	}
}

// Validate_ControlPlaneEndpointConfig validates ControlPlaneEndpointConfig.
func Validate_ControlPlaneEndpointConfig(cfg *ControlPlaneEndpointConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil { // This might be an error if an endpoint is strictly required by a parent config
		verrs.Add("%s: controlPlaneEndpoint configuration cannot be nil if specified", pathPrefix)
		return
	}
	// At least one of Domain or Address should typically be set for a functional endpoint.
	if strings.TrimSpace(cfg.Domain) == "" && strings.TrimSpace(cfg.Address) == "" {
		verrs.Add("%s: either domain or address must be specified for the control plane endpoint", pathPrefix)
	}
	if cfg.Domain != "" && strings.TrimSpace(cfg.Domain) == "" { // If key exists but value is whitespace
		verrs.Add("%s.domain: cannot be empty if specified", pathPrefix)
		// TODO: Add hostname validation for Domain if needed
	}
	if cfg.Address != "" && !isValidIP(cfg.Address) { // isValidIP assumed to be available (defined in this package or imported)
		verrs.Add("%s.address: invalid IP address format '%s'", pathPrefix, cfg.Address)
	}
	if cfg.Port != nil && (*cfg.Port <= 0 || *cfg.Port > 65535) {
		verrs.Add("%s.port: invalid port %d, must be between 1-65535", pathPrefix, *cfg.Port)
	}
}

// isValidIP helper function
func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}
