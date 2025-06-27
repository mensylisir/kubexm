package v1alpha1

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSetDefaults_ControlPlaneEndpointSpec tests the SetDefaults_ControlPlaneEndpointSpec function.
func TestSetDefaults_ControlPlaneEndpointSpec(t *testing.T) {
	tests := []struct {
		name     string
		input    *ControlPlaneEndpointSpec
		expected *ControlPlaneEndpointSpec
	}{
		{
			name:     "nil config",
			input:    nil,
			expected: nil,
		},
		{
			name:  "empty config",
			input: &ControlPlaneEndpointSpec{},
			expected: &ControlPlaneEndpointSpec{
				Port: 6443, // Default port
				// ExternalDNS defaults to false (zero value for bool)
			},
		},
		{
			name:  "port already set",
			input: &ControlPlaneEndpointSpec{Port: 8443},
			expected: &ControlPlaneEndpointSpec{
				Port: 8443, // Not overridden
			},
		},
		{
			name:  "all fields set",
			input: &ControlPlaneEndpointSpec{Domain: "k8s.example.com", Address: "192.168.1.100", Port: 6443, ExternalDNS: true, ExternalLoadBalancerType: "external", InternalLoadBalancerType: "haproxy"},
			expected: &ControlPlaneEndpointSpec{Domain: "k8s.example.com", Address: "192.168.1.100", Port: 6443, ExternalDNS: true, ExternalLoadBalancerType: "external", InternalLoadBalancerType: "haproxy"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_ControlPlaneEndpointSpec(tt.input)
			if !reflect.DeepEqual(tt.input, tt.expected) {
				assert.Equal(t, tt.expected, tt.input)
			}
		})
	}
}

// TestValidate_ControlPlaneEndpointSpec tests the Validate_ControlPlaneEndpointSpec function.
func TestValidate_ControlPlaneEndpointSpec(t *testing.T) {
	validDomain := "api.example.com"
	validAddress := "192.168.0.1"

	tests := []struct {
		name        string
		input       *ControlPlaneEndpointSpec
		expectErr   bool
		errContains []string
	}{
		{
			name:        "valid with domain",
			input:       &ControlPlaneEndpointSpec{Domain: validDomain, Port: 6443},
			expectErr:   false,
		},
		{
			name:        "valid with address",
			input:       &ControlPlaneEndpointSpec{Address: validAddress, Port: 6443},
			expectErr:   false,
		},
		{
			name:        "valid with domain and address",
			input:       &ControlPlaneEndpointSpec{Domain: validDomain, Address: validAddress, Port: 6443},
			expectErr:   false,
		},
		{
			name:        "missing domain and address",
			input:       &ControlPlaneEndpointSpec{Port: 6443},
			expectErr:   true,
			errContains: []string{": either domain or address (lb_address in YAML) must be specified"},
		},
		{
			name:        "invalid domain format",
			input:       &ControlPlaneEndpointSpec{Domain: "invalid_domain!", Port: 6443},
			expectErr:   true,
			errContains: []string{".domain: 'invalid_domain!' is not a valid domain name"},
		},
		{
			name:        "invalid address format",
			input:       &ControlPlaneEndpointSpec{Address: "not-an-ip", Port: 6443},
			expectErr:   true,
			errContains: []string{".address: invalid IP address format for 'not-an-ip'"},
		},
		{
			name:        "port too low (user set, not default 0)",
			input:       &ControlPlaneEndpointSpec{Domain: validDomain, Port: 0}, // Set to 0, but SetDefaults will change it to 6443 before validation
			expectErr:   false, // After defaulting, port will be 6443, so it's valid
		},
		{
			name:        "port explicitly too low",
			input:       &ControlPlaneEndpointSpec{Domain: validDomain, Port: -1},
			expectErr:   true,
			errContains: []string{".port: invalid port -1"},
		},
		{
			name:        "port too high",
			input:       &ControlPlaneEndpointSpec{Domain: validDomain, Port: 70000},
			expectErr:   true,
			errContains: []string{".port: invalid port 70000"},
		},
		{
			name:        "valid external LB type",
			input:       &ControlPlaneEndpointSpec{Domain: validDomain, Port: 6443, ExternalLoadBalancerType: "kubexm"},
			expectErr:   false,
		},
		{
			name:        "invalid external LB type",
			input:       &ControlPlaneEndpointSpec{Domain: validDomain, Port: 6443, ExternalLoadBalancerType: "invalid"},
			expectErr:   true,
			errContains: []string{".externalLoadBalancerType: invalid type 'invalid'"},
		},
		{
			name:        "valid internal LB type",
			input:       &ControlPlaneEndpointSpec{Domain: validDomain, Port: 6443, InternalLoadBalancerType: "kube-vip"},
			expectErr:   false,
		},
		{
			name:        "invalid internal LB type",
			input:       &ControlPlaneEndpointSpec{Domain: validDomain, Port: 6443, InternalLoadBalancerType: "invalid"},
			expectErr:   true,
			errContains: []string{".internalLoadbalancer: invalid type 'invalid'"},
		},
		{
			name:        "empty external LB type (valid)",
			input:       &ControlPlaneEndpointSpec{Domain: validDomain, Port: 6443, ExternalLoadBalancerType: ""},
			expectErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply defaults before validation.
			// Note: For "port too low (user set, not default 0)", the input port:0 will become 6443 after defaults.
			// The validation `cfg.Port != 0` is to check user-supplied non-defaulted invalid ports.
			if tt.input != nil { // Avoid panic on nil input
				SetDefaults_ControlPlaneEndpointSpec(tt.input)
			}

			verrs := &ValidationErrors{} // Assuming ValidationErrors is available
			Validate_ControlPlaneEndpointSpec(tt.input, verrs, "spec.controlPlaneEndpoint")

			if tt.expectErr {
				assert.False(t, verrs.IsEmpty(), "Expected validation errors for test: %s, but got none", tt.name)
				if len(tt.errContains) > 0 {
					combinedErrors := verrs.Error()
					for _, errStr := range tt.errContains {
						assert.Contains(t, combinedErrors, errStr, "Error message for test '%s' does not contain '%s'. Full error: %s", tt.name, errStr, combinedErrors)
					}
				}
			} else {
				assert.True(t, verrs.IsEmpty(), "Expected no validation errors for test: %s, but got: %s", tt.name, verrs.Error())
			}
		})
	}
}
