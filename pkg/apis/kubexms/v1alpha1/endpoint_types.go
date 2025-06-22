package v1alpha1

import (
	"net" // For isValidIP
	"regexp" // Added for regexp.MatchString
	"strings" // For validation
)

// ControlPlaneEndpointSpec defines the configuration for the cluster's control plane endpoint.
// This endpoint is used by nodes and external clients to communicate with the Kubernetes API server.
type ControlPlaneEndpointSpec struct {
	// Domain is the DNS name for the control plane endpoint.
	// Example: "k8s-api.example.com"
	Domain string `json:"domain,omitempty" yaml:"domain,omitempty"`

	// Address is the IP address for the control plane endpoint.
	// This could be a VIP managed by Keepalived, an external load balancer IP, etc.
	// Corresponds to `lb_address` in some YAML configurations if `domain` is not used.
	Address string `json:"address,omitempty" yaml:"address,omitempty"`

	// Port is the port number for the control plane endpoint.
	// Defaults to 6443.
	Port int `json:"port,omitempty" yaml:"port,omitempty"` // Changed to int

	// ExternalDNS indicates if an external DNS record should be assumed or managed for the domain.
	// This field might influence how the endpoint is resolved or advertised.
	ExternalDNS bool `json:"externalDNS,omitempty" yaml:"externalDNS,omitempty"` // Changed to bool

	// ExternalLoadBalancerType specifies the type of external load balancer used or to be deployed by KubeXMS.
	// Examples from YAML: "kubexm" (managed by KubeXMS), "external" (user-provided).
	// This field helps determine behavior for HA setup.
	ExternalLoadBalancerType string `json:"externalLoadBalancerType,omitempty" yaml:"externalLoadBalancer,omitempty"`

	// InternalLoadBalancerType specifies the type of internal load balancer for intra-cluster communication to the API server.
	// Examples from YAML: "haproxy", "nginx", "kube-vip".
	InternalLoadBalancerType string `json:"internalLoadBalancerType,omitempty" yaml:"internalLoadbalancer,omitempty"`
}

// SetDefaults_ControlPlaneEndpointSpec sets default values for ControlPlaneEndpointSpec.
func SetDefaults_ControlPlaneEndpointSpec(cfg *ControlPlaneEndpointSpec) {
	if cfg == nil {
		return
	}
	if cfg.Port == 0 { // Changed from cfg.Port == nil
		cfg.Port = 6443 // Default Kubernetes API server port
	}
	// For ExternalDNS (bool), its zero value is false, which is the default.
	// If we wanted default true, we'd do:
	// if !cfg.ExternalDNS { // This logic is flawed if we want to distinguish "not set" from "set to false"
	//    cfg.ExternalDNS = defaultValueForExternalDNS // e.g. true
	// }
	// Given it's bool, if not specified in YAML, it will be false. If specified as false, it's false.
	// The previous pointer logic `if cfg.ExternalDNS == nil { b := false; cfg.ExternalDNS = &b }`
	// effectively made the default false if not present. So, for bool type, no explicit default needed if false is the desired default.
}


// Validate_ControlPlaneEndpointSpec validates ControlPlaneEndpointSpec.
func Validate_ControlPlaneEndpointSpec(cfg *ControlPlaneEndpointSpec, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if strings.TrimSpace(cfg.Domain) == "" && strings.TrimSpace(cfg.Address) == "" {
		verrs.Add("%s: either domain or address (lb_address in YAML) must be specified", pathPrefix)
	}
	if cfg.Domain != "" {
		if matched, _ := regexp.MatchString(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`, cfg.Domain); !matched {
			verrs.Add("%s.domain: '%s' is not a valid domain name", pathPrefix, cfg.Domain)
		}
	}
	if cfg.Address != "" && !isValidIP(cfg.Address) {
		verrs.Add("%s.address: invalid IP address format for '%s'", pathPrefix, cfg.Address)
	}
	// cfg.Port is now int. If 0, it's defaulted to 6443. Validation is for user-provided values.
	if cfg.Port != 0 && (cfg.Port <= 0 || cfg.Port > 65535) {
		verrs.Add("%s.port: invalid port %d, must be between 1-65535", pathPrefix, cfg.Port)
	}

	validExternalTypes := []string{"kubexm", "external", ""}
	if cfg.ExternalLoadBalancerType != "" && !containsString(validExternalTypes, cfg.ExternalLoadBalancerType) {
		verrs.Add("%s.externalLoadBalancerType: invalid type '%s', must be one of %v", pathPrefix, cfg.ExternalLoadBalancerType, validExternalTypes)
	}
	// Removed duplicate declaration of validExternalTypes and its corresponding if block.
	validInternalTypes := []string{"haproxy", "nginx", "kube-vip", ""}
	if cfg.InternalLoadBalancerType != "" && !containsString(validInternalTypes, cfg.InternalLoadBalancerType) {
		verrs.Add("%s.internalLoadbalancer: invalid type '%s', must be one of %v", pathPrefix, cfg.InternalLoadBalancerType, validInternalTypes)
	}
}

// containsString is a helper function.
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// isValidIP helper function
func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}
