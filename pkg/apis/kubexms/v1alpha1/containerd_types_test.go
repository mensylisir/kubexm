package v1alpha1

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// stringPtr and boolPtr are expected to be in zz_helpers.go or similar within the package.

func TestSetDefaults_ContainerdConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    *ContainerdConfig
		expected *ContainerdConfig
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:  "empty config",
			input: &ContainerdConfig{},
			expected: &ContainerdConfig{
				RegistryMirrors:    make(map[string][]string),
				InsecureRegistries: []string{},
				UseSystemdCgroup:   boolPtr(true),
				ConfigPath:         stringPtr("/etc/containerd/config.toml"),
				DisabledPlugins:    []string{},
				RequiredPlugins:    []string{"io.containerd.grpc.v1.cri"},
				Imports:            []string{},
			},
		},
		{
			name: "UseSystemdCgroup explicitly false",
			input: &ContainerdConfig{UseSystemdCgroup: boolPtr(false)},
			expected: &ContainerdConfig{
				RegistryMirrors:    make(map[string][]string),
				InsecureRegistries: []string{},
				UseSystemdCgroup:   boolPtr(false), // Not overridden
				ConfigPath:         stringPtr("/etc/containerd/config.toml"),
				DisabledPlugins:    []string{},
				RequiredPlugins:    []string{"io.containerd.grpc.v1.cri"},
				Imports:            []string{},
			},
		},
		{
			name: "ConfigPath explicitly set",
			input: &ContainerdConfig{ConfigPath: stringPtr("/custom/path/config.toml")},
			expected: &ContainerdConfig{
				RegistryMirrors:    make(map[string][]string),
				InsecureRegistries: []string{},
				UseSystemdCgroup:   boolPtr(true),
				ConfigPath:         stringPtr("/custom/path/config.toml"), // Not overridden
				DisabledPlugins:    []string{},
				RequiredPlugins:    []string{"io.containerd.grpc.v1.cri"},
				Imports:            []string{},
			},
		},
		{
			name: "RequiredPlugins already set",
			input: &ContainerdConfig{RequiredPlugins: []string{"custom.plugin"}},
			expected: &ContainerdConfig{
				RegistryMirrors:    make(map[string][]string),
				InsecureRegistries: []string{},
				UseSystemdCgroup:   boolPtr(true),
				ConfigPath:         stringPtr("/etc/containerd/config.toml"),
				DisabledPlugins:    []string{},
				RequiredPlugins:    []string{"custom.plugin"}, // Not overridden
				Imports:            []string{},
			},
		},
		{
			name: "All fields already set",
			input: &ContainerdConfig{
				Version:            "1.5.5",
				RegistryMirrors:    map[string][]string{"docker.io": {"https://mirror.internal"}},
				InsecureRegistries: []string{"insecure.repo:5000"},
				UseSystemdCgroup:   boolPtr(false),
				ExtraTomlConfig:    "some_toml_config",
				ConfigPath:         stringPtr("/opt/containerd/config.toml"),
				DisabledPlugins:    []string{"some.plugin.to.disable"},
				RequiredPlugins:    []string{"io.containerd.grpc.v1.cri", "another.required.plugin"},
				Imports:            []string{"/etc/containerd/conf.d/extra.toml"},
			},
			expected: &ContainerdConfig{
				Version:            "1.5.5",
				RegistryMirrors:    map[string][]string{"docker.io": {"https://mirror.internal"}},
				InsecureRegistries: []string{"insecure.repo:5000"},
				UseSystemdCgroup:   boolPtr(false),
				ExtraTomlConfig:    "some_toml_config",
				ConfigPath:         stringPtr("/opt/containerd/config.toml"),
				DisabledPlugins:    []string{"some.plugin.to.disable"},
				RequiredPlugins:    []string{"io.containerd.grpc.v1.cri", "another.required.plugin"},
				Imports:            []string{"/etc/containerd/conf.d/extra.toml"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_ContainerdConfig(tt.input)
			if !reflect.DeepEqual(tt.input, tt.expected) {
				assert.Equal(t, tt.expected, tt.input)
			}
		})
	}
}

func TestValidate_ContainerdConfig(t *testing.T) {
	validCases := []struct {
		name  string
		input *ContainerdConfig
	}{
		{
			name:  "minimal valid after defaults",
			input: &ContainerdConfig{}, // Defaults will make it valid
		},
		{
			name: "valid with mirrors and insecure registries",
			input: &ContainerdConfig{
				RegistryMirrors:    map[string][]string{"docker.io": {"https://mirror.example.com"}},
				InsecureRegistries: []string{"my.registry:5000"},
				ConfigPath:         stringPtr("/custom/config.toml"),
			},
		},
		{
			name: "valid with plugins and imports",
			input: &ContainerdConfig{
				DisabledPlugins: []string{"unwanted.plugin"},
				RequiredPlugins: []string{"io.containerd.grpc.v1.cri", "my.plugin"},
				Imports:         []string{"/etc/containerd/custom.toml"},
			},
		},
		{
			name: "valid with version and extra toml",
			input: &ContainerdConfig{
				Version:         "1.6.0", // Valid version
				ExtraTomlConfig: "[plugins.\"io.containerd.grpc.v1.cri\".registry]\n  config_path = \"/etc/containerd/certs.d\"",
			},
		},
		{
			name: "valid with v-prefix version",
			input: &ContainerdConfig{
				Version: "v1.6.0",
			},
		},
		{
			name: "valid with extended version",
			input: &ContainerdConfig{
				Version: "1.6.8-beta.0",
			},
		},
	}

	for _, tt := range validCases {
		t.Run("Valid_"+tt.name, func(t *testing.T) {
			SetDefaults_ContainerdConfig(tt.input) // Apply defaults
			verrs := &ValidationErrors{}
			Validate_ContainerdConfig(tt.input, verrs, "spec.containerd")
			assert.True(t, verrs.IsEmpty(), "Expected no validation errors for '%s', but got: %s", tt.name, verrs.Error())
		})
	}

	invalidCases := []struct {
		name        string
		input       *ContainerdConfig
		errContains []string
	}{
		{"empty_mirror_key", &ContainerdConfig{RegistryMirrors: map[string][]string{" ": {"m1"}}}, []string{"registry host key cannot be empty"}},
		{"empty_mirror_list", &ContainerdConfig{RegistryMirrors: map[string][]string{"docker.io": {}}}, []string{"must contain at least one mirror URL"}},
		{"empty_mirror_url", &ContainerdConfig{RegistryMirrors: map[string][]string{"docker.io": {" "}}}, []string{"mirror URL cannot be empty"}},
		{"empty_insecure_reg", &ContainerdConfig{InsecureRegistries: []string{" "}}, []string{"registry host cannot be empty"}},
		{"empty_config_path", &ContainerdConfig{ConfigPath: stringPtr(" ")}, []string{"configPath: cannot be empty if specified"}},
		{"disabledplugins_empty_item", &ContainerdConfig{DisabledPlugins: []string{" "}}, []string{".disabledPlugins[0]: plugin name cannot be empty"}},
		{"requiredplugins_empty_item", &ContainerdConfig{RequiredPlugins: []string{" "}}, []string{".requiredPlugins[0]: plugin name cannot be empty"}},
		{"imports_empty_item", &ContainerdConfig{Imports: []string{" "}}, []string{".imports[0]: import path cannot be empty"}},
		{"version_is_whitespace", &ContainerdConfig{Version: "   "}, []string{".version: cannot be only whitespace if specified"}},
		{
			"version_invalid_format_alphanum",
			&ContainerdConfig{Version: "1.2.3a"}, // Should fail with isValidRuntimeVersion
			[]string{".version: '1.2.3a' is not a recognized version format"},
		},
		{
			"version_invalid_chars_underscore",
			&ContainerdConfig{Version: "1.2.3_beta"}, // Should fail
			[]string{".version: '1.2.3_beta' is not a recognized version format"},
		},
		{
			"invalid_mirror_url_scheme",
			&ContainerdConfig{RegistryMirrors: map[string][]string{"docker.io": {"ftp://badmirror.com"}}},
			[]string{"invalid URL format for mirror", "must be http or https"},
		},
		{
			"invalid_mirror_url_format",
			&ContainerdConfig{RegistryMirrors: map[string][]string{"docker.io": {"http://invalid domain/"}}},
			[]string{"invalid URL format for mirror"},
		},
		{
			"invalid_insecure_registry_format_bad_port",
			&ContainerdConfig{InsecureRegistries: []string{"myreg:port"}},
			[]string{"invalid host:port format for insecure registry"},
		},
		{
			"invalid_insecure_registry_format_bad_host",
			&ContainerdConfig{InsecureRegistries: []string{"invalid_host!"}},
			[]string{"invalid host:port format for insecure registry"},
		},
		// Cases that should remain valid for InsecureRegistries
		{
			"valid_insecure_registry_ipv6_with_port",
			&ContainerdConfig{InsecureRegistries: []string{"[::1]:5000"}},
			nil,
		},
		{
			"valid_insecure_registry_ipv4_with_port",
			&ContainerdConfig{InsecureRegistries: []string{"127.0.0.1:5000"}},
			nil,
		},
		{
			"valid_insecure_registry_hostname_with_port",
			&ContainerdConfig{InsecureRegistries: []string{"my.registry.com:5000"}},
			nil,
		},
		{
			"valid_insecure_registry_hostname_no_port",
			&ContainerdConfig{InsecureRegistries: []string{"my.registry.com"}},
			nil,
		},
	}

	for _, tt := range invalidCases {
		t.Run("Invalid_"+tt.name, func(t *testing.T) {
			// Defaults are generally not applied here to test raw validation logic,
			// but for version validation, it's independent of defaults on other fields.
			// SetDefaults_ContainerdConfig(tt.input)
			verrs := &ValidationErrors{}
			Validate_ContainerdConfig(tt.input, verrs, "spec.containerd")

			if tt.errContains == nil {
				assert.True(t, verrs.IsEmpty(), "Expected no validation errors for '%s', but got: %s", tt.name, verrs.Error())
			} else {
				assert.False(t, verrs.IsEmpty(), "Expected validation errors for '%s', but got none", tt.name)
				fullError := verrs.Error()
				for _, errStr := range tt.errContains {
					assert.Contains(t, fullError, errStr, "Error message for '%s' does not contain expected substring '%s'. Full error: %s", tt.name, errStr, fullError)
				}
			}
		})
	}
}
