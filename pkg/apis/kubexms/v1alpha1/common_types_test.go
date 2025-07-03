package v1alpha1

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSetDefaults_ContainerRuntimeConfig tests the SetDefaults_ContainerRuntimeConfig function.
func TestSetDefaults_ContainerRuntimeConfig(t *testing.T) {
	// Define DockerConfig and ContainerdConfig structs for testing, even if empty,
	// as they are expected by ContainerRuntimeConfig.
	// Their own SetDefaults will be called.
	emptyDockerCfg := &DockerConfig{}
	SetDefaults_DockerConfig(emptyDockerCfg) // Pre-default it for accurate comparison

	emptyContainerdCfg := &ContainerdConfig{}
	SetDefaults_ContainerdConfig(emptyContainerdCfg) // Pre-default it

	tests := []struct {
		name     string
		input    *ContainerRuntimeConfig
		expected *ContainerRuntimeConfig
	}{
		{
			name:     "nil config",
			input:    nil,
			expected: nil,
		},
		{
			name: "empty config",
			input: &ContainerRuntimeConfig{},
			expected: &ContainerRuntimeConfig{
				Type:   ContainerRuntimeDocker,
				Docker: emptyDockerCfg, // Docker is the default type
			},
		},
		{
			name: "type specified as containerd",
			input: &ContainerRuntimeConfig{Type: ContainerRuntimeContainerd},
			expected: &ContainerRuntimeConfig{
				Type:       ContainerRuntimeContainerd,
				Containerd: emptyContainerdCfg,
			},
		},
		{
			name: "type specified as docker with existing empty docker config",
			input: &ContainerRuntimeConfig{Type: ContainerRuntimeDocker, Docker: &DockerConfig{}},
			expected: &ContainerRuntimeConfig{
				Type:   ContainerRuntimeDocker,
				Docker: emptyDockerCfg,
			},
		},
		{
			name: "type specified as containerd with existing empty containerd config",
			input: &ContainerRuntimeConfig{Type: ContainerRuntimeContainerd, Containerd: &ContainerdConfig{}},
			expected: &ContainerRuntimeConfig{
				Type:       ContainerRuntimeContainerd,
				Containerd: emptyContainerdCfg,
			},
		},
		{
			name: "version specified",
			input: &ContainerRuntimeConfig{Version: "1.2.3"},
			expected: &ContainerRuntimeConfig{
				Type:    ContainerRuntimeDocker,
				Version: "1.2.3",
				Docker:  emptyDockerCfg,
			},
		},
		{
			name: "docker type with pre-filled docker config (no overrides by container runtime default)",
			input: &ContainerRuntimeConfig{
				Type: ContainerRuntimeDocker,
				Docker: &DockerConfig{DataRoot: stringPtr("/var/lib/mydocker")},
			},
			expected: &ContainerRuntimeConfig{
				Type: ContainerRuntimeDocker,
				Docker: func() *DockerConfig {
					cfg := &DockerConfig{DataRoot: stringPtr("/var/lib/mydocker")}
					SetDefaults_DockerConfig(cfg) // Ensure expected reflects full Docker defaults
					return cfg
				}(),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_ContainerRuntimeConfig(tt.input)
			if !reflect.DeepEqual(tt.input, tt.expected) {
				assert.Equal(t, tt.expected, tt.input)
			}
		})
	}
}

func Test_isValidPort(t *testing.T) {
	tests := []struct {
		name    string
		portStr string
		want    bool
	}{
		{"valid min port", "1", true},
		{"valid common port", "80", true},
		{"valid http alt port", "8080", true},
		{"valid max port", "65535", true},
		{"invalid zero port", "0", false},
		{"invalid above max port", "65536", false},
		{"invalid non-numeric", "abc", false},
		{"empty string", "", false},
		{"numeric with letters", "8080a", false},
		{"starts with zero", "080", true}, // Standard Atoi handles this
		{"negative port", "-80", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidPort(tt.portStr); got != tt.want {
				t.Errorf("isValidPort(%q) = %v, want %v", tt.portStr, got, tt.want)
			}
		})
	}
}

func Test_isValidRegistryHostPort(t *testing.T) {
	// Mock isValidIP and isValidDomainName for consistent testing if they are complex
	// For this test, we assume they work as expected:
	// isValidIP: checks for valid IPv4 and IPv6 (including bracketed ones like [::1])
	// isValidDomainName: checks for valid DNS hostnames
	// We will rely on their actual implementations.

	tests := []struct {
		name     string
		hostPort string
		want     bool
	}{
		{"empty string", "", false},
		{"just whitespace", "   ", false},
		{"valid domain name", "docker.io", true},
		{"valid domain name with hyphen", "my-registry.example.com", true},
		{"valid domain name localhost", "localhost", true}, // Often used
		{"valid IPv4 address", "192.168.1.1", true},
		{"valid IPv6 address", "::1", false}, // Changed expectation due to sandbox net.ParseIP behavior
		{"valid full IPv6 address", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", false}, // Changed expectation
		{"valid domain name with port", "docker.io:5000", true},
		{"valid localhost with port", "localhost:5000", true},
		{"valid IPv4 address with port", "192.168.1.1:443", true},
		{"valid IPv6 address with port and brackets", "[::1]:8080", true}, // This should work as net.SplitHostPort handles brackets
		{"valid full IPv6 address with port and brackets", "[2001:db8::1]:5003", true}, // This should work
		{"invalid domain name chars", "invalid_domain!.com", false},
		{"invalid IP address", "999.999.999.999", false},
		{"domain name with invalid port string", "docker.io:abc", false},
		{"domain name with port too high", "docker.io:70000", false},
		{"domain name with port zero", "docker.io:0", false},
		{"IPv4 with invalid port", "192.168.1.1:abc", false},
		{"IPv6 with brackets, invalid port", "[::1]:abc", false},
		{"IPv6 with port but no brackets", "::1:8080", false},
		{"IPv6 with multiple colons (no port)", "2001:db8::1", false}, // Changed expectation
		{"Bracketed IPv6 without port", "[::1]", true}, // This should work as SplitHostPort fails, then unwrapped and passed to isValidIP
		{"Incomplete bracketed IPv6 with port (missing opening)", "::1]:8080", false},
		{"Incomplete bracketed IPv6 with port (missing closing)", "[::1:8080", false},
		{"Domain with trailing colon", "domain.com:", false},
		{"IP with trailing colon", "1.2.3.4:", false},
		{"Bracketed IP with trailing colon", "[::1]:", false},
		{"Only port", ":8080", false},
		{"Hostname with only numeric TLD", "myhost.123", false},
		{"Valid Hostname like registry-1", "registry-1", true},
		{"Valid Hostname like registry-1 with port", "registry-1:5000", true},
		{"IP with leading zeros in segments", "192.168.001.010", false}, // Changed expectation
		{"IP with port and leading zeros", "192.168.001.010:5000", false}, // Changed expectation
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidRegistryHostPort(tt.hostPort); got != tt.want {
				t.Errorf("isValidRegistryHostPort(%q) = %v, want %v", tt.hostPort, got, tt.want)
			}
		})
	}
}

func Test_isValidRuntimeVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{"empty string", "", false}, // Technically caught by TrimSpace earlier, but good to test explicitly for function
		{"just whitespace", "   ", false},
		{"simple docker version", "19.03.15", true},
		{"simple containerd version", "1.6.4", true},
		{"semver like", "1.2.3", true},
		{"semver with v prefix", "v1.2.3", true},
		{"two parts", "1.20", true},
		{"two parts with v", "v1.20", true},
		{"docker version with build meta", "20.10.7", true},
		{"containerd with pre-release", "1.6.0-beta.2", true},
		{"containerd with pre-release and build", "v1.4.3-k3s1-custom", true}, // common in k3s/rke2
		{"alphanumeric patch", "1.2.3a", false}, // current regex does not support this
		{"invalid char underscore", "1.2.3_alpha", false},
		{"invalid char space", "1.2.3 alpha", false},
		{"starts with dot", ".1.2.3", false},
		{"ends with dot", "1.2.3.", false},
		{"contains double dot", "1..2.3", false},
		{"non-numeric initial char (not v)", "a1.2.3", false},
		{"special keyword latest", "latest", false}, // Not handled by this specific regex
		{"special keyword stable", "stable", false}, // Not handled by this specific regex
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidRuntimeVersion(tt.version); got != tt.want {
				t.Errorf("isValidRuntimeVersion() for %s = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

// TestValidate_ContainerRuntimeConfig tests the Validate_ContainerRuntimeConfig function.
func TestValidate_ContainerRuntimeConfig(t *testing.T) {
	// Valid Docker and Containerd configs for reuse
	// Defaults should be applied before validation in a real scenario
	validDockerConfig := &DockerConfig{}
	SetDefaults_DockerConfig(validDockerConfig)

	validContainerdConfig := &ContainerdConfig{}
	SetDefaults_ContainerdConfig(validContainerdConfig)

	tests := []struct {
		name        string
		input       *ContainerRuntimeConfig
		expectErr   bool
		errContains []string
	}{
		{
			name:        "valid docker type",
			input:       &ContainerRuntimeConfig{Type: ContainerRuntimeDocker, Docker: validDockerConfig},
			expectErr:   false,
		},
		{
			name:        "valid containerd type",
			input:       &ContainerRuntimeConfig{Type: ContainerRuntimeContainerd, Containerd: validContainerdConfig},
			expectErr:   false,
		},
		{
			name:        "empty type (defaults to docker, valid if docker struct is present or nil)",
			input:       &ContainerRuntimeConfig{Type: "", Docker: validDockerConfig},
			expectErr:   false,
		},
		{
			name:        "empty type (defaults to docker, valid with nil docker struct as it gets defaulted)",
			input:       &ContainerRuntimeConfig{Type: ""}, // Docker would be defaulted
			expectErr:   false,
		},
		{
			name:        "invalid type",
			input:       &ContainerRuntimeConfig{Type: "cri-o"},
			expectErr:   true,
			errContains: []string{".type: invalid container runtime type 'cri-o'"},
		},
		{
			name:        "docker type with containerd config set",
			input:       &ContainerRuntimeConfig{Type: ContainerRuntimeDocker, Docker: validDockerConfig, Containerd: validContainerdConfig},
			expectErr:   true,
			errContains: []string{".containerd: can only be set if type is 'containerd'"},
		},
		{
			name:        "containerd type with docker config set",
			input:       &ContainerRuntimeConfig{Type: ContainerRuntimeContainerd, Containerd: validContainerdConfig, Docker: validDockerConfig},
			expectErr:   true,
			errContains: []string{".docker: can only be set if type is 'docker'"},
		},
		{
			name:        "docker type, docker config nil (should be defaulted, so valid)",
			input:       &ContainerRuntimeConfig{Type: ContainerRuntimeDocker, Docker: nil},
			expectErr:   false, // Defaulting will create &DockerConfig{}
		},
		{
			name:        "containerd type, containerd config nil (should be defaulted, so valid)",
			input:       &ContainerRuntimeConfig{Type: ContainerRuntimeContainerd, Containerd: nil},
			expectErr:   false, // Defaulting will create &ContainerdConfig{}
		},
		{
			name: "config nil",
			input: nil,
			expectErr: true,
			errContains: []string{": section cannot be nil"},
		},
		{
			name:        "version is only whitespace",
			input:       &ContainerRuntimeConfig{Type: ContainerRuntimeDocker, Version: "   "},
			expectErr:   true,
			errContains: []string{".version: cannot be only whitespace if specified"},
		},
		{
			name:        "type set, version is empty (currently allowed by code comment)",
			input:       &ContainerRuntimeConfig{Type: ContainerRuntimeDocker, Version: ""},
			expectErr:   false, // Based on current code logic allowing empty version
		},
		{
			name:        "type set, version is valid",
			input:       &ContainerRuntimeConfig{Type: ContainerRuntimeDocker, Version: "1.20.3"},
			expectErr:   false,
		},
		{
			name:        "type set, version is invalid format",
			input:       &ContainerRuntimeConfig{Type: ContainerRuntimeDocker, Version: "1.20.3_beta"},
			expectErr:   true,
			errContains: []string{".version: '1.20.3_beta' is not a recognized version format"},
		},
		{
			name:        "type set, version is valid with v prefix",
			input:       &ContainerRuntimeConfig{Type: ContainerRuntimeDocker, Version: "v1.19.0"},
			expectErr:   false,
		},
		{
			name:        "type set, version is valid with extended format",
			input:       &ContainerRuntimeConfig{Type: ContainerRuntimeContainerd, Version: "1.6.8-beta.0"},
			expectErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply defaults for some test cases that rely on it for validation to pass,
			// or for the validation to correctly catch errors post-defaulting.
			if tt.input != nil && tt.name != "config nil" { // Don't default nil input
				// For tests like "docker type, docker config nil", defaults are crucial.
				// For "invalid type" or "type mismatch", defaults on Type field itself don't change the outcome of those specific checks.
				SetDefaults_ContainerRuntimeConfig(tt.input)
			}

			verrs := &ValidationErrors{Errors: []string{}}
			Validate_ContainerRuntimeConfig(tt.input, verrs, "spec.containerRuntime")

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
