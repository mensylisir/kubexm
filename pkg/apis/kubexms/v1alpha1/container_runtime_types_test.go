package v1alpha1

import (
	"testing"

	"github.com/mensylisir/kubexm/pkg/common" // Added import
	"github.com/mensylisir/kubexm/pkg/util"
	"github.com/mensylisir/kubexm/pkg/util/validation"
	"github.com/stretchr/testify/assert"
)

func TestSetDefaults_ContainerRuntimeConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    *ContainerRuntimeConfig
		expected *ContainerRuntimeConfig
	}{
		{
			name:  "nil config",
			input: nil,
		},
		{
			name: "empty config",
			input: &ContainerRuntimeConfig{},
			expected: &ContainerRuntimeConfig{
				Type: ContainerRuntimeContainerd, // Default is now common.RuntimeContainerd via ContainerRuntimeContainerd const
				Containerd: &ContainerdConfig{ // Expect containerd to be initialized and defaulted
					Version:            "", // Default from SetDefaults_ContainerdConfig
					RegistryMirrors:    map[string][]string{},
					InsecureRegistries: []string{},
					UseSystemdCgroup:   util.BoolPtr(true),
					ConfigPath:         util.StrPtr("/etc/containerd/config.toml"),
					DisabledPlugins:    []string{},
					RequiredPlugins:    []string{common.ContainerdPluginCRI}, // Using common constant
					Imports:            []string{},
					ExtraTomlConfig:    "",
				},
				Docker: nil, // Docker should be nil
			},
		},
		{
			name: "type specified as containerd",
			input: &ContainerRuntimeConfig{
				Type: ContainerRuntimeContainerd,
			},
			expected: &ContainerRuntimeConfig{
				Type: ContainerRuntimeContainerd,
				Containerd: &ContainerdConfig{
					Version:            "",
					RegistryMirrors:    map[string][]string{},
					InsecureRegistries: []string{},
					UseSystemdCgroup:   util.BoolPtr(true),
					ConfigPath:         util.StrPtr("/etc/containerd/config.toml"),
					DisabledPlugins:    []string{},
					RequiredPlugins:    []string{common.ContainerdPluginCRI},
					Imports:            []string{},
					ExtraTomlConfig:    "",
				},
				Docker: nil,
			},
		},
		{
			name: "type specified as docker, with existing empty docker config",
			input: &ContainerRuntimeConfig{
				Type:   ContainerRuntimeDocker, // This is common.RuntimeDocker
				Docker: &DockerConfig{},       // Empty DockerConfig provided
			},
			expected: &ContainerRuntimeConfig{
				Type: ContainerRuntimeDocker,
				Docker: &DockerConfig{ // Expect DockerConfig to be defaulted
					RegistryMirrors:        []string{},
					InsecureRegistries:     []string{},
					ExecOpts:               []string{},
					LogDriver:              util.StrPtr(common.DockerLogDriverJSONFile),
					LogOpts:                map[string]string{"max-size": common.DockerLogOptMaxSizeDefault, "max-file": common.DockerLogOptMaxFileDefault},
					DefaultAddressPools:    []DockerAddressPool{},
					StorageDriver:          nil, // No default for storage driver itself
					StorageOpts:            []string{},
					DefaultRuntime:         nil, // Docker's default is runc
					Runtimes:               map[string]DockerRuntime{},
					MaxConcurrentDownloads: util.IntPtr(common.DockerMaxConcurrentDownloadsDefault),
					MaxConcurrentUploads:   util.IntPtr(common.DockerMaxConcurrentUploadsDefault),
					Bridge:                 util.StrPtr(common.DefaultDockerBridgeName),
					InstallCRIDockerd:      util.BoolPtr(true),
					CRIDockerdVersion:      nil, // No default version for cri-dockerd here
					DataRoot:               nil, // No default DataRoot in DockerConfig defaults
					Experimental:           util.BoolPtr(false),
					IPTables:               util.BoolPtr(true),
					IPMasq:                 util.BoolPtr(true),
					Auths:                  map[string]DockerRegistryAuth{},
					ExtraJSONConfig:        nil,
				},
				Containerd: nil,
			},
		},
		{
			name: "type specified as containerd, with existing empty containerd config",
			input: &ContainerRuntimeConfig{
				Type:       ContainerRuntimeContainerd, // This is common.RuntimeContainerd
				Containerd: &ContainerdConfig{},       // Empty ContainerdConfig provided
			},
			expected: &ContainerRuntimeConfig{
				Type: ContainerRuntimeContainerd,
				Containerd: &ContainerdConfig{
					Version:            "",
					RegistryMirrors:    map[string][]string{},
					InsecureRegistries: []string{},
					UseSystemdCgroup:   util.BoolPtr(true),
					ConfigPath:         util.StrPtr("/etc/containerd/config.toml"),
					DisabledPlugins:    []string{},
					RequiredPlugins:    []string{common.ContainerdPluginCRI},
					Imports:            []string{},
					ExtraTomlConfig:    "",
				},
				Docker: nil,
			},
		},
		{
			name: "version specified, type defaults to containerd",
			input: &ContainerRuntimeConfig{
				Version: "1.5.0",
			},
			expected: &ContainerRuntimeConfig{
				Type:    ContainerRuntimeContainerd,
				Version: "1.5.0",
				Containerd: &ContainerdConfig{
					Version:            "", // Version in ContainerdConfig is separate
					RegistryMirrors:    map[string][]string{},
					InsecureRegistries: []string{},
					UseSystemdCgroup:   util.BoolPtr(true),
					ConfigPath:         util.StrPtr("/etc/containerd/config.toml"),
					DisabledPlugins:    []string{},
					RequiredPlugins:    []string{common.ContainerdPluginCRI},
					Imports:            []string{},
					ExtraTomlConfig:    "",
				},
				Docker: nil,
			},
		},
		{
			name: "docker type with pre-filled docker config",
			input: &ContainerRuntimeConfig{
				Type: ContainerRuntimeDocker,
				Docker: &DockerConfig{
					InstallCRIDockerd: util.BoolPtr(false),
					LogDriver:         util.StrPtr(common.DockerLogDriverJournald),
				},
			},
			expected: &ContainerRuntimeConfig{
				Type: ContainerRuntimeDocker,
				Docker: &DockerConfig{
					RegistryMirrors:        []string{},
					InsecureRegistries:     []string{},
					ExecOpts:               []string{},
					LogDriver:              util.StrPtr(common.DockerLogDriverJournald), // User's value preserved
					LogOpts:                map[string]string{"max-size": common.DockerLogOptMaxSizeDefault, "max-file": common.DockerLogOptMaxFileDefault},
					DefaultAddressPools:    []DockerAddressPool{},
					StorageDriver:          nil,
					StorageOpts:            []string{},
					DefaultRuntime:         nil,
					Runtimes:               map[string]DockerRuntime{},
					MaxConcurrentDownloads: util.IntPtr(common.DockerMaxConcurrentDownloadsDefault),
					MaxConcurrentUploads:   util.IntPtr(common.DockerMaxConcurrentUploadsDefault),
					Bridge:                 util.StrPtr(common.DefaultDockerBridgeName),
					InstallCRIDockerd:      util.BoolPtr(false), // User's value preserved
					CRIDockerdVersion:      nil,
					DataRoot:               nil,
					Experimental:           util.BoolPtr(false),
					IPTables:               util.BoolPtr(true),
					IPMasq:                 util.BoolPtr(true),
					Auths:                  map[string]DockerRegistryAuth{},
					ExtraJSONConfig:        nil,
				},
				Containerd: nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_ContainerRuntimeConfig(tt.input)
			if tt.input == nil {
				assert.Nil(t, tt.expected)
			} else {
				assert.Equal(t, tt.expected, tt.input)
			}
		})
	}
}

func TestValidate_ContainerRuntimeConfig(t *testing.T) {
	// Create valid defaulted sub-configs to use in tests
	// Create a fully defaulted DockerConfig for expected values
	fullyDefaultedDockerCfg := &DockerConfig{}
	SetDefaults_DockerConfig(fullyDefaultedDockerCfg)

	// Create a fully defaulted ContainerdConfig for expected values
	fullyDefaultedContainerdCfg := &ContainerdConfig{}
	SetDefaults_ContainerdConfig(fullyDefaultedContainerdCfg)


	tests := []struct {
		name        string
		input       *ContainerRuntimeConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid docker type",
			input: &ContainerRuntimeConfig{
				Type:   ContainerRuntimeDocker, // This is common.RuntimeDocker
				Docker: &DockerConfig{},       // Will be defaulted
			},
			expectError: false,
		},
		{
			name: "valid containerd type",
			input: &ContainerRuntimeConfig{
				Type:       ContainerRuntimeContainerd, // This is common.RuntimeContainerd
				Containerd: &ContainerdConfig{},       // Will be defaulted
			},
			expectError: false,
		},
		{
			name: "empty type (defaults to containerd), docker config erroneously set",
			input: &ContainerRuntimeConfig{
				// Type is empty, will default to containerd
				Docker: &DockerConfig{}, // Error: Docker config set when type will be containerd
			},
			expectError: true,
			errorMsg:    ".docker: can only be set if type is 'docker'",
		},
		{
			name: "empty type (defaults to containerd, valid with nil docker struct)",
			input: &ContainerRuntimeConfig{
				// Type is empty, will default to containerd
				Docker: nil, // Correctly nil
			},
			expectError: false,
		},
		{
			name: "invalid type",
			input: &ContainerRuntimeConfig{
				Type: "crio", // Invalid type string
			},
			expectError: true,
			errorMsg:    ".type: invalid container runtime type 'crio', must be 'docker' or 'containerd'",
		},
		{
			name: "docker type with containerd config set",
			input: &ContainerRuntimeConfig{
				Type:       ContainerRuntimeDocker,
				Docker:     &DockerConfig{},
				Containerd: &ContainerdConfig{}, // Error
			},
			expectError: true,
			errorMsg:    ".containerd: can only be set if type is 'containerd'",
		},
		{
			name: "containerd type with docker config set",
			input: &ContainerRuntimeConfig{
				Type:       ContainerRuntimeContainerd,
				Containerd: &ContainerdConfig{},
				Docker:     &DockerConfig{}, // Error
			},
			expectError: true,
			errorMsg:    ".docker: can only be set if type is 'docker'",
		},
		{
			name: "docker type, docker config nil (defaulted)",
			input: &ContainerRuntimeConfig{
				Type:   ContainerRuntimeDocker,
				Docker: nil, // Will be defaulted
			},
			expectError: false,
		},
		{
			name: "containerd type, containerd config nil (defaulted)",
			input: &ContainerRuntimeConfig{
				Type:       ContainerRuntimeContainerd,
				Containerd: nil, // Will be defaulted
			},
			expectError: false,
		},
		{
			name:        "config nil",
			input:       nil,
			expectError: true,
			errorMsg:    "section cannot be nil",
		},
		{
			name: "version is only whitespace",
			input: &ContainerRuntimeConfig{
				Type:    ContainerRuntimeContainerd, // Default type
				Version: "   ",
			},
			expectError: true,
			errorMsg:    ".version: cannot be only whitespace if specified",
		},
		{
			name: "type set, version is empty (currently allowed by code comment)",
			input: &ContainerRuntimeConfig{
				Type:    ContainerRuntimeContainerd, // Default type
				Version: "",
			},
			expectError: false,
		},
		{
			name: "type set, version is valid",
			input: &ContainerRuntimeConfig{
				Type:    ContainerRuntimeContainerd, // Default type
				Version: "1.5.2",
			},
			expectError: false,
		},
		{
			name: "type set, version is invalid format",
			input: &ContainerRuntimeConfig{
				Type:    ContainerRuntimeContainerd, // Default type
				Version: "1.beta",
			},
			expectError: true,
			errorMsg:    ".version: '1.beta' is not a recognized version format",
		},
		{
			name: "type set, version is valid with v prefix",
			input: &ContainerRuntimeConfig{
				Type:    ContainerRuntimeContainerd, // Default type
				Version: "v1.6.0",
			},
			expectError: false,
		},
		{
			name: "type set, version is valid with extended format",
			input: &ContainerRuntimeConfig{
				Type:    ContainerRuntimeContainerd, // Default type
				Version: "v1.4.3-k3s1", // Assuming IsValidRuntimeVersion handles this
			},
			expectError: false, // This depends on IsValidRuntimeVersion's behavior
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var inputToTest *ContainerRuntimeConfig
			if tt.input != nil {
				// Create a deep copy for SetDefaults to modify, preserving original tt.input for inspection
				copiedInput := *tt.input
				if copiedInput.Docker != nil {
					dockerCopy := *copiedInput.Docker
					copiedInput.Docker = &dockerCopy
				}
				if copiedInput.Containerd != nil {
					containerdCopy := *copiedInput.Containerd
					copiedInput.Containerd = &containerdCopy
				}
				inputToTest = &copiedInput
				SetDefaults_ContainerRuntimeConfig(inputToTest) // Apply defaults
			} else {
				inputToTest = nil
			}

			verrs := &validation.ValidationErrors{}
			Validate_ContainerRuntimeConfig(inputToTest, verrs, "spec.containerRuntime")

			if tt.expectError {
				assert.True(t, verrs.HasErrors(), "Expected error for test '%s', but got none. Input: %+v, Defaulted: %+v", tt.name, tt.input, inputToTest)
				if tt.errorMsg != "" {
					assert.Contains(t, verrs.Error(), tt.errorMsg, "Error message for test '%s' does not contain expected substring '%s'. Full error: %s", tt.name, tt.errorMsg, verrs.Error())
				}
			} else {
				assert.False(t, verrs.HasErrors(), "Unexpected error for test '%s': %s. Input: %+v, Defaulted: %+v", tt.name, verrs.Error(), tt.input, inputToTest)
			}
		})
	}
}
