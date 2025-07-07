package v1alpha1

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/mensylisir/kubexm/pkg/util/validation"
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
				Port: 6443,
			},
		},
		{
			name:  "port already set",
			input: &ControlPlaneEndpointSpec{Port: 8443},
			expected: &ControlPlaneEndpointSpec{
				Port: 8443,
			},
		},
		{
			name:  "all fields set",
			input: &ControlPlaneEndpointSpec{Domain: "k8s.example.com", Address: "192.168.1.100", Port: 6443, ExternalDNS: true},
			expected: &ControlPlaneEndpointSpec{Domain: "k8s.example.com", Address: "192.168.1.100", Port: 6443, ExternalDNS: true},
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
			input:       &ControlPlaneEndpointSpec{Domain: validDomain, Port: 0},
			expectErr:   false,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.input != nil {
				SetDefaults_ControlPlaneEndpointSpec(tt.input)
			}

			verrs := &validation.ValidationErrors{}
			Validate_ControlPlaneEndpointSpec(tt.input, verrs, "spec.controlPlaneEndpoint")

			if tt.expectErr {
				assert.True(t, verrs.HasErrors(), "Expected validation errors for test: %s, but got none", tt.name)
				if len(tt.errContains) > 0 {
					combinedErrors := verrs.Error()
					for _, errStr := range tt.errContains {
						assert.Contains(t, combinedErrors, errStr, "Error message for test '%s' does not contain '%s'. Full error: %s", tt.name, errStr, combinedErrors)
					}
				}
			} else {
				assert.False(t, verrs.HasErrors(), "Expected no validation errors for test: %s, but got: %s", tt.name, verrs.Error())
			}
		})
	}
}
