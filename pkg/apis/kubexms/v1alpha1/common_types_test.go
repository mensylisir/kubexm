package v1alpha1

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/mensylisir/kubexm/pkg/util" // Import the util package
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
			if got := util.IsValidPort(tt.portStr); got != tt.want { // Use util.IsValidPort
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
			if got := util.ValidateHostPortString(tt.hostPort); got != tt.want { // Use util.ValidateHostPortString
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
		{"empty string", "", false}, // Duplicated test name, but content is same
		{"just v", "v", false},
		{"simple main version", "1.2.3", true},
		{"main version with v", "v1.2.3", true},
		{"two part main version", "1.2", true},
		{"one part main version", "1", true},
		{"main version with pre-release", "1.2.3-alpha.1", true},
		{"main version with v and pre-release", "v1.2.3-rc.2", true},
		{"main version with build metadata", "1.2.3+build.100", true},
		{"main version with v, pre-release, and build metadata", "v1.2.3-beta+exp.sha.5114f85", true},
		{"pre-release with hyphenated identifiers", "1.0.0-alpha-beta", true},
		{"pre-release with numeric identifiers that are not 0-padded", "1.0.0-alpha.0", true},
		{"pre-release with leading zeros in numeric identifiers", "1.0.0-alpha.01", false},
		{"long pre-release", "1.0.0-alpha.beta.gamma.delta.epsilon.zeta.eta.theta.iota.kappa.lambda.mu", true},
		{"long build metadata", "1.0.0+build.this.is.a.very.long.build.metadata.string.which.is.allowed", true},
		{"version with only pre-release (invalid)", "v-alpha", false},
		{"version with only build (invalid)", "v+build", false},
		{"main version segments not numeric", "1.a.3", false},
		{"too many main segments", "1.2.3.4", false},
		{"empty segment in main", "1..2", false},
		{"empty segment in pre-release", "1.0.0-alpha..1", false},
		{"empty segment in build", "1.0.0+build..1", false},
		{"pre-release contains invalid char", "1.0.0-alpha!", false},
		{"build metadata contains invalid char", "1.0.0+build!", false},
		{"no main version before pre-release", "-alpha", false},
		{"no main version before build", "+build", false},
		{"just a dot", ".", false},
		{"just a hyphen", "-", false},
		{"just a plus", "+", false},
		{"leading dot", ".1.2.3", false},
		{"trailing dot on main", "1.2.3.", false},
		{"trailing dot on pre-release", "1.0.0-alpha.", false},
		{"trailing dot on build", "1.0.0+build.", false},
		{"non-numeric segment in main", "1.x.2", false},
		{"docker style version", "20.10.7", true},
		{"containerd style version", "1.6.8", true},
		{"containerd with pre-release", "1.6.0-beta.2", true},
		{"complex k3s/rke2 like", "v1.21.5+k3s1-custom", true},
		{"another complex", "v1.18.20-eks-1-20-13", true},
		{"version like in tests", "1.20.3_beta", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := util.IsValidRuntimeVersion(tt.version); got != tt.want { // Use util.IsValidRuntimeVersion
				t.Errorf("util.IsValidRuntimeVersion() for %s = %v, want %v", tt.version, got, tt.want)
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
