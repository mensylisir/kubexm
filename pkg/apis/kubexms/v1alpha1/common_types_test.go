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
