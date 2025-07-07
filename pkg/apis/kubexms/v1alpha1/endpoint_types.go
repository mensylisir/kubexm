package v1alpha1

import (
	// "net" // No longer needed directly as util.IsValidIP is used and local isValidIP was removed
	"fmt"
	"regexp"
	"strings"
	"github.com/mensylisir/kubexm/pkg/util"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/util/validation"
)

var (
	domainValidationRegex = regexp.MustCompile(common.DomainValidationRegexString)
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
	Address string `json:"address,omitempty" yaml:"lb_address,omitempty"`

	// Port is the port number for the control plane endpoint.
	// Defaults to 6443.
	Port int `json:"port,omitempty" yaml:"port,omitempty"`

	// ExternalDNS indicates if an external DNS record should be assumed or managed for the domain.
	// This field might influence how the endpoint is resolved or advertised.
	ExternalDNS bool `json:"externalDNS,omitempty" yaml:"externalDNS,omitempty"`

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
func Validate_ControlPlaneEndpointSpec(cfg *ControlPlaneEndpointSpec, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	if strings.TrimSpace(cfg.Domain) == "" && strings.TrimSpace(cfg.Address) == "" {
		verrs.Add(pathPrefix, "either domain or address (lb_address in YAML) must be specified")
	}
	if cfg.Domain != "" {
		if !domainValidationRegex.MatchString(cfg.Domain) {
			verrs.Add(pathPrefix+".domain", fmt.Sprintf("'%s' is not a valid domain name", cfg.Domain))
		}
	}
	if cfg.Address != "" && !util.IsValidIP(cfg.Address) { // Use util.IsValidIP
		verrs.Add(pathPrefix+".address", fmt.Sprintf("invalid IP address format for '%s'", cfg.Address))
	}
	// cfg.Port is now int. If 0, it's defaulted to 6443 by SetDefaults_ControlPlaneEndpointSpec.
	// Validation should catch any value outside the valid port range 1-65535.
	if cfg.Port <= 0 || cfg.Port > 65535 {
		verrs.Add(pathPrefix+".port", fmt.Sprintf("invalid port %d, must be between 1-65535", cfg.Port))
	}

	// Validation for ExternalLoadBalancerType and InternalLoadBalancerType removed as fields are removed.
	// Specific LB type validation is now handled within HighAvailabilityConfig validation.
}

// isValidIP helper function was removed as util.IsValidIP is used.
