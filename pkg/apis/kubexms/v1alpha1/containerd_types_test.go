package v1alpha1

import (
	"testing"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/util"
	"github.com/mensylisir/kubexm/pkg/util/validation"
	"github.com/stretchr/testify/assert"
)

func TestSetDefaults_ContainerdConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    *ContainerdConfig
		expected *ContainerdConfig
	}{
		{
			name:  "nil input",
			input: nil,
		},
		{
			name: "empty config",
			input: &ContainerdConfig{},
			expected: &ContainerdConfig{
				RegistryMirrors:    map[string][]string{},
				InsecureRegistries: []string{},
				UseSystemdCgroup:   util.BoolPtr(true),
				ConfigPath:         util.StrPtr(common.ContainerdDefaultConfigFile),
				DisabledPlugins:    []string{},
				RequiredPlugins:    []string{common.ContainerdPluginCRI},
				Imports:            []string{},
			},
		},
		{
			name: "UseSystemdCgroup explicitly false",
			input: &ContainerdConfig{
				UseSystemdCgroup: util.BoolPtr(false),
			},
			expected: &ContainerdConfig{
				RegistryMirrors:    map[string][]string{},
				InsecureRegistries: []string{},
				UseSystemdCgroup:   util.BoolPtr(false),
				ConfigPath:         util.StrPtr(common.ContainerdDefaultConfigFile),
				DisabledPlugins:    []string{},
				RequiredPlugins:    []string{common.ContainerdPluginCRI},
				Imports:            []string{},
			},
		},
		{
			name: "ConfigPath explicitly set",
			input: &ContainerdConfig{
				ConfigPath: util.StrPtr("/custom/containerd.toml"),
			},
			expected: &ContainerdConfig{
				RegistryMirrors:    map[string][]string{},
				InsecureRegistries: []string{},
				UseSystemdCgroup:   util.BoolPtr(true),
				ConfigPath:         util.StrPtr("/custom/containerd.toml"),
				DisabledPlugins:    []string{},
				RequiredPlugins:    []string{common.ContainerdPluginCRI},
				Imports:            []string{},
			},
		},
		{
			name: "RequiredPlugins already set",
			input: &ContainerdConfig{
				RequiredPlugins: []string{"custom.plugin.cri"},
			},
			expected: &ContainerdConfig{
				RegistryMirrors:    map[string][]string{},
				InsecureRegistries: []string{},
				UseSystemdCgroup:   util.BoolPtr(true),
				ConfigPath:         util.StrPtr(common.ContainerdDefaultConfigFile),
				DisabledPlugins:    []string{},
				RequiredPlugins:    []string{"custom.plugin.cri"},
				Imports:            []string{},
			},
		},
		{
			name: "All fields already set",
			input: &ContainerdConfig{
				Version:            "1.5.5",
				RegistryMirrors:    map[string][]string{"docker.io": {"https://mirror.example.com"}},
				InsecureRegistries: []string{"insecure.registry:5000"},
				UseSystemdCgroup:   util.BoolPtr(false),
				ExtraTomlConfig:    "some_toml_config",
				ConfigPath:         util.StrPtr("/etc/alternative/containerd.toml"),
				DisabledPlugins:    []string{"some.plugin"},
				RequiredPlugins:    []string{"another.plugin"},
				Imports:            []string{"/etc/containerd/conf.d/custom.toml"},
			},
			expected: &ContainerdConfig{
				Version:            "1.5.5",
				RegistryMirrors:    map[string][]string{"docker.io": {"https://mirror.example.com"}},
				InsecureRegistries: []string{"insecure.registry:5000"},
				UseSystemdCgroup:   util.BoolPtr(false),
				ExtraTomlConfig:    "some_toml_config",
				ConfigPath:         util.StrPtr("/etc/alternative/containerd.toml"),
				DisabledPlugins:    []string{"some.plugin"},
				RequiredPlugins:    []string{"another.plugin"},
				Imports:            []string{"/etc/containerd/conf.d/custom.toml"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_ContainerdConfig(tt.input)
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}

func TestValidate_ContainerdConfig(t *testing.T) {
	tests := []struct {
		name        string
		input       *ContainerdConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid minimal (valid after defaults)",
			input:       &ContainerdConfig{}, // Defaults will be applied by SetDefaults before validation
			expectError: false,
		},
		{
			name: "Valid with mirrors and insecure registries",
			input: &ContainerdConfig{
				RegistryMirrors:    map[string][]string{"docker.io": {"http://localhost:5000"}},
				InsecureRegistries: []string{"localhost:5000", "127.0.0.1:5001"},
			},
			expectError: false,
		},
		{
			name: "Valid with plugins and imports",
			input: &ContainerdConfig{
				DisabledPlugins: []string{"test.plugin"},
				RequiredPlugins: []string{"cri"},
				Imports:         []string{"/path/to/import.toml"},
			},
			expectError: false,
		},
		{
			name: "Valid with version and extra toml",
			input: &ContainerdConfig{
				Version:         "1.6.0",
				ExtraTomlConfig: "[plugins]\n  [plugins.\"io.containerd.grpc.v1.cri\"]\n    sandbox_image = \"k8s.gcr.io/pause:3.2\"",
			},
			expectError: false,
		},
		{
			name: "Valid with v-prefix version",
			input: &ContainerdConfig{
				Version: "v1.6.0",
			},
			expectError: false,
		},
		{
			name: "Valid with extended version",
			input: &ContainerdConfig{
				Version: "v1.4.3-k3s1",
			},
			expectError: false,
		},
		{
			name: "Invalid empty mirror key",
			input: &ContainerdConfig{
				RegistryMirrors: map[string][]string{" ": {"http://m.com"}},
			},
			expectError: true,
			errorMsg:    ".registryMirrors: registry host key cannot be empty",
		},
		{
			name: "Invalid empty mirror list",
			input: &ContainerdConfig{
				RegistryMirrors: map[string][]string{"docker.io": {}},
			},
			expectError: true,
			errorMsg:    ".registryMirrors[\"docker.io\"]: must contain at least one mirror URL",
		},
		{
			name: "Invalid empty mirror url",
			input: &ContainerdConfig{
				RegistryMirrors: map[string][]string{"docker.io": {" "}},
			},
			expectError: true,
			errorMsg:    ".registryMirrors[\"docker.io\"][0]: mirror URL cannot be empty",
		},
		{
			name: "Invalid empty insecure reg",
			input: &ContainerdConfig{
				InsecureRegistries: []string{" "},
			},
			expectError: true,
			errorMsg:    ".insecureRegistries[0]: registry host cannot be empty",
		},
		{
			name: "Invalid empty config path",
			input: &ContainerdConfig{
				ConfigPath: util.StrPtr(" "),
			},
			expectError: true,
			errorMsg:    ".configPath: cannot be empty if specified",
		},
		{
			name: "Invalid disabledplugins empty item",
			input: &ContainerdConfig{
				DisabledPlugins: []string{" "},
			},
			expectError: true,
			errorMsg:    ".disabledPlugins[0]: plugin name cannot be empty",
		},
		{
			name: "Invalid requiredplugins empty item",
			input: &ContainerdConfig{
				RequiredPlugins: []string{" "},
			},
			expectError: true,
			errorMsg:    ".requiredPlugins[0]: plugin name cannot be empty",
		},
		{
			name: "Invalid imports empty item",
			input: &ContainerdConfig{
				Imports: []string{" "},
			},
			expectError: true,
			errorMsg:    ".imports[0]: import path cannot be empty",
		},
		{
			name: "Invalid version is whitespace",
			input: &ContainerdConfig{
				Version: "  ",
			},
			expectError: true,
			errorMsg:    ".version: cannot be only whitespace if specified",
		},
		{
			name: "Invalid version invalid format alphanum",
			input: &ContainerdConfig{
				Version: "1.beta",
			},
			expectError: true,
			errorMsg:    ".version: '1.beta' is not a recognized version format",
		},
		{
			name: "Invalid version invalid chars underscore",
			input: &ContainerdConfig{
				Version: "1.2_3",
			},
			expectError: true,
			errorMsg:    ".version: '1.2_3' is not a recognized version format",
		},
		{
			name: "Invalid invalid mirror url scheme",
			input: &ContainerdConfig{
				RegistryMirrors: map[string][]string{"docker.io": {"ftp://mirror.invalid"}},
			},
			expectError: true,
			errorMsg:    "invalid URL format for mirror 'ftp://mirror.invalid' (must be http or https)",
		},
		{
			name: "Invalid invalid mirror url format",
			input: &ContainerdConfig{
				RegistryMirrors: map[string][]string{"docker.io": {"http//invalid"}},
			},
			expectError: true,
			errorMsg:    "invalid URL format for mirror 'http//invalid' (must be http or https)",
		},
		{
			name: "Invalid invalid insecure registry format bad port",
			input: &ContainerdConfig{
				InsecureRegistries: []string{"myreg:port"},
			},
			expectError: true,
			errorMsg:    "invalid host:port format for insecure registry 'myreg:port'",
		},
		{
			name: "Invalid invalid insecure registry format bad host",
			input: &ContainerdConfig{
				InsecureRegistries: []string{"invalid_host!"},
			},
			expectError: true,
			errorMsg:    "invalid host:port format for insecure registry 'invalid_host!'",
		},
		{
			name: "Valid insecure registry ipv6 with port", // Corrected test name prefix
			input: &ContainerdConfig{
				InsecureRegistries: []string{"[::1]:5000"},
			},
			expectError: false,
		},
		{
			name: "Valid insecure registry ipv4 with port", // Corrected test name prefix
			input: &ContainerdConfig{
				InsecureRegistries: []string{"127.0.0.1:5000"},
			},
			expectError: false,
		},
		{
			name: "Valid insecure registry hostname with port", // Corrected test name prefix
			input: &ContainerdConfig{
				InsecureRegistries: []string{"my.registry.com:5000"},
			},
			expectError: false,
		},
		{
			name: "Valid insecure registry hostname no port", // Corrected test name prefix
			input: &ContainerdConfig{
				InsecureRegistries: []string{"my.registry.com"},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputToTest := tt.input
			// Apply defaults only if the test case implies a valid state after defaulting
			if tt.name == "Valid minimal (valid after defaults)" && inputToTest != nil {
				SetDefaults_ContainerdConfig(inputToTest)
			}

			verrs := &validation.ValidationErrors{}
			Validate_ContainerdConfig(inputToTest, verrs, "spec.containerd")
			if tt.expectError {
				assert.True(t, verrs.HasErrors(), "Expected error for test '%s' but got none. Input: %+v", tt.name, tt.input)
				if tt.errorMsg != "" {
					assert.Contains(t, verrs.Error(), tt.errorMsg, "Error message for test '%s' does not contain '%s'. Full error: %s", tt.name, tt.errorMsg, verrs.Error())
				}
			} else {
				assert.False(t, verrs.HasErrors(), "Unexpected error for test '%s': %s. Input: %+v", tt.name, verrs.Error(), tt.input)
			}
		})
	}
}
