package v1alpha1

import (
	"testing"

	"github.com/mensylisir/kubexm/pkg/util/validation"
	"github.com/stretchr/testify/assert"
)

func TestSetDefaults_ControlPlaneEndpointSpec(t *testing.T) {
	tests := []struct {
		name     string
		input    *ControlPlaneEndpointSpec
		expected *ControlPlaneEndpointSpec
	}{
		{
			name:  "nil config",
			input: nil,
		},
		{
			name: "empty config",
			input: &ControlPlaneEndpointSpec{},
			expected: &ControlPlaneEndpointSpec{
				Port: 6443, // Default port
			},
		},
		{
			name: "port already set",
			input: &ControlPlaneEndpointSpec{
				Port: 8443,
			},
			expected: &ControlPlaneEndpointSpec{
				Port: 8443,
			},
		},
		{
			name: "all fields set",
			input: &ControlPlaneEndpointSpec{
				Domain:      "api.example.com",
				Address:     "192.168.1.100",
				Port:        6443,
				ExternalDNS: true,
			},
			expected: &ControlPlaneEndpointSpec{
				Domain:      "api.example.com",
				Address:     "192.168.1.100",
				Port:        6443,
				ExternalDNS: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_ControlPlaneEndpointSpec(tt.input)
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}

func TestValidate_ControlPlaneEndpointSpec(t *testing.T) {
	tests := []struct {
		name        string
		input       *ControlPlaneEndpointSpec
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid with domain",
			input: &ControlPlaneEndpointSpec{
				Domain: "api.example.com",
				// Port will be defaulted to 6443 if 0
			},
			expectError: false,
		},
		{
			name: "valid with address",
			input: &ControlPlaneEndpointSpec{
				Address: "1.2.3.4",
				// Port will be defaulted
			},
			expectError: false,
		},
		{
			name: "valid with domain and address",
			input: &ControlPlaneEndpointSpec{
				Domain:  "api.example.com",
				Address: "1.2.3.4",
				Port:    6443, // Explicitly set
			},
			expectError: false,
		},
		{
			name: "missing domain and address",
			input: &ControlPlaneEndpointSpec{
				// Port will be defaulted
			},
			expectError: true,
			errorMsg:    "either domain or address (lb_address in YAML) must be specified",
		},
		{
			name: "invalid domain format",
			input: &ControlPlaneEndpointSpec{
				Domain: "-invalid.com",
			},
			expectError: true,
			errorMsg:    "'-invalid.com' is not a valid domain name",
		},
		{
			name: "invalid address format",
			input: &ControlPlaneEndpointSpec{
				Address: "1.2.3.4.5",
			},
			expectError: true,
			errorMsg:    "invalid IP address format for '1.2.3.4.5'",
		},
		{
			name: "port too low (user set, not default 0)", // Testing user explicitly providing 0
			input: &ControlPlaneEndpointSpec{
				Domain: "api.example.com",
				Port:   0,
			},
			expectError: true,
			errorMsg:    "invalid port 0, must be between 1-65535",
		},
		{
			name: "port explicitly too low",
			input: &ControlPlaneEndpointSpec{
				Domain: "api.example.com",
				Port:   -1,
			},
			expectError: true,
			errorMsg:    "invalid port -1, must be between 1-65535",
		},
		{
			name: "port too high",
			input: &ControlPlaneEndpointSpec{
				Domain: "api.example.com",
				Port:   65536,
			},
			expectError: true,
			errorMsg:    "invalid port 65536, must be between 1-65535",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputToTest := tt.input
			if inputToTest != nil {
				// Create a copy to avoid modifying the input for subsequent tests if it's reused
				copiedInput := *inputToTest
				inputToTest = &copiedInput
				// Apply defaults only if the test is not specifically targeting the non-defaulted state of Port=0
				if !(tt.name == "port too low (user set, not default 0)" && inputToTest.Port == 0) {
					SetDefaults_ControlPlaneEndpointSpec(inputToTest)
				}
			}

			verrs := &validation.ValidationErrors{}
			Validate_ControlPlaneEndpointSpec(inputToTest, verrs, "spec.controlPlaneEndpoint")
			if tt.expectError {
				assert.True(t, verrs.HasErrors(), "Expected error for test '%s', but got none. Input: %+v, DefaultedOrOriginal: %+v", tt.name, tt.input, inputToTest)
				if tt.errorMsg != "" {
					assert.Contains(t, verrs.Error(), tt.errorMsg, "Error message for test '%s' does not contain '%s'. Full error: %s", tt.name, tt.errorMsg, verrs.Error())
				}
			} else {
				assert.False(t, verrs.HasErrors(), "Unexpected error for test '%s': %s. Input: %+v, DefaultedOrOriginal: %+v", tt.name, verrs.Error(), tt.input, inputToTest)
			}
		})
	}
}
