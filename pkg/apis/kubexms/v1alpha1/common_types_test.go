package v1alpha1

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/mensylisir/kubexm/pkg/util" // Import the util package
	"github.com/mensylisir/kubexm/pkg/util/validation"
)

// TestSetDefaults_ContainerRuntimeConfig tests the SetDefaults_ContainerRuntimeConfig function.
func TestSetDefaults_ContainerRuntimeConfig(t *testing.T) {
	emptyDockerCfg := &DockerConfig{}
	SetDefaults_DockerConfig(emptyDockerCfg)

	emptyContainerdCfg := &ContainerdConfig{}
	SetDefaults_ContainerdConfig(emptyContainerdCfg)

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
				Type:       ContainerRuntimeContainerd, // Changed expected default
				Containerd: emptyContainerdCfg,     // Expect ContainerdConfig to be initialized
			},
		},
		{
			name: "type specified as containerd", // This test remains valid
			input: &ContainerRuntimeConfig{Type: ContainerRuntimeContainerd},
			expected: &ContainerRuntimeConfig{
				Type:       ContainerRuntimeContainerd,
				Containerd: emptyContainerdCfg,
			},
		},
		{
			name: "type specified as docker with existing empty docker config", // This test remains valid
			input: &ContainerRuntimeConfig{Type: ContainerRuntimeDocker, Docker: &DockerConfig{}},
			expected: &ContainerRuntimeConfig{
				Type:   ContainerRuntimeDocker,
				Docker: emptyDockerCfg,
			},
		},
		{
			name: "type specified as containerd with existing empty containerd config", // This test remains valid
			input: &ContainerRuntimeConfig{Type: ContainerRuntimeContainerd, Containerd: &ContainerdConfig{}},
			expected: &ContainerRuntimeConfig{
				Type:       ContainerRuntimeContainerd,
				Containerd: emptyContainerdCfg,
			},
		},
		{
			name: "version specified, type defaults to containerd",
			input: &ContainerRuntimeConfig{Version: "1.2.3"},
			expected: &ContainerRuntimeConfig{
				Type:       ContainerRuntimeContainerd, // Changed expected default
				Version:    "1.2.3",
				Containerd: emptyContainerdCfg,     // Expect ContainerdConfig
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
					SetDefaults_DockerConfig(cfg)
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
		{"starts with zero", "080", true},
		{"negative port", "-80", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := util.IsValidPort(tt.portStr); got != tt.want {
				t.Errorf("util.IsValidPort(%q) = %v, want %v", tt.portStr, got, tt.want)
			}
		})
	}
}

func Test_isValidRegistryHostPort(t *testing.T) {
	tests := []struct {
		name     string
		hostPort string
		want     bool
	}{
		{"empty string", "", false},
		{"just whitespace", "   ", false},
		{"valid domain name", "docker.io", true},
		{"valid domain name with hyphen", "my-registry.example.com", true},
		{"valid domain name localhost", "localhost", true},
		{"valid IPv4 address", "192.168.1.1", true},
		{"valid IPv6 address ::1", "::1", true},
		{"valid full IPv6 address", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", true},
		{"valid domain name with port", "docker.io:5000", true},
		{"valid localhost with port", "localhost:5000", true},
		{"valid IPv4 address with port", "192.168.1.1:443", true},
		{"valid IPv6 address with port and brackets", "[::1]:8080", true},
		{"valid full IPv6 address with port and brackets", "[2001:db8::1]:5003", true},
		{"invalid domain name chars", "invalid_domain!.com", false},
		{"invalid IP address", "999.999.999.999", false},
		{"domain name with invalid port string", "docker.io:abc", false},
		{"domain name with port too high", "docker.io:70000", false},
		{"domain name with port zero", "docker.io:0", false},
		{"IPv4 with invalid port", "192.168.1.1:abc", false},
		{"IPv6 with brackets, invalid port", "[::1]:abc", false},
		{"IPv6 with port but no brackets", "::1:8080", true},
		{"Bracketed IPv6 without port", "[::1]", true},
		{"Incomplete bracketed IPv6 with port (missing opening)", "::1]:8080", false},
		{"Incomplete bracketed IPv6 with port (missing closing)", "[::1:8080", false},
		{"Domain with trailing colon", "domain.com:", false},
		{"IP with trailing colon", "1.2.3.4:", false},
		{"Bracketed IP with trailing colon", "[::1]:", false},
		{"Only port", ":8080", false},
		{"Hostname with only numeric TLD", "myhost.123", false},
		{"Valid Hostname like registry-1", "registry-1", true},
		{"Valid Hostname like registry-1 with port", "registry-1:5000", true},
		{"IP with leading zeros in segments", "192.168.001.010", false},
		{"IP with port and leading zeros", "192.168.001.010:5000", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := util.ValidateHostPortString(tt.hostPort); got != tt.want {
				t.Errorf("util.ValidateHostPortString(%q) = %v, want %v", tt.hostPort, got, tt.want)
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
		{"empty string", "", false},
		{"just whitespace", "   ", false},
		{"just v", "v", false},
		{"simple main version", "1.2.3", true},
		{"main version with v", "v1.2.3", true},
		{"docker style version", "20.10.7", true},
		{"containerd style version", "1.6.8", true},
		{"containerd with pre-release", "1.6.0-beta.2", true},
		{"complex k3s/rke2 like", "v1.21.5+k3s1-custom", true},
		{"another complex", "v1.18.20-eks-1-20-13", true},
		{"version like in tests", "1.20.3_beta", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := util.IsValidRuntimeVersion(tt.version); got != tt.want {
				t.Errorf("util.IsValidRuntimeVersion() for %s = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestValidate_ContainerRuntimeConfig(t *testing.T) {
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
			name:        "empty type (now defaults to containerd), docker config erroneously set",
			input:       &ContainerRuntimeConfig{Type: "", Docker: validDockerConfig}, // Type defaults to containerd, but Docker field is set
			expectErr:   true, // This should now cause an error
			errContains: []string{".docker: can only be set if type is 'docker'"},
		},
		{
			name:        "empty type (defaults to containerd, valid with nil docker struct and containerd gets defaulted)",
			input:       &ContainerRuntimeConfig{Type: ""},
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
			expectErr:   false,
		},
		{
			name:        "containerd type, containerd config nil (should be defaulted, so valid)",
			input:       &ContainerRuntimeConfig{Type: ContainerRuntimeContainerd, Containerd: nil},
			expectErr:   false,
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
			expectErr:   false,
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
			if tt.input != nil && tt.name != "config nil" {
				SetDefaults_ContainerRuntimeConfig(tt.input)
			}

			verrs := &validation.ValidationErrors{}
			Validate_ContainerRuntimeConfig(tt.input, verrs, "spec.containerRuntime")

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
