package v1alpha1

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// stringPtr, boolPtr, intPtr are expected to be in zz_helpers.go or similar within the package.

func TestSetDefaults_DockerConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    *DockerConfig
		expected *DockerConfig
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:  "empty config",
			input: &DockerConfig{},
			expected: &DockerConfig{
				RegistryMirrors:        []string{},
				InsecureRegistries:     []string{},
				ExecOpts:               []string{},
				LogOpts:                make(map[string]string),
				DefaultAddressPools:    []DockerAddressPool{},
				StorageOpts:            []string{},
				Runtimes:               make(map[string]DockerRuntime),
				MaxConcurrentDownloads: intPtr(3),
				MaxConcurrentUploads:   intPtr(5),
				Bridge:                 stringPtr("docker0"),
				InstallCRIDockerd:      boolPtr(true),
				LogDriver:              stringPtr("json-file"),
				IPTables:               boolPtr(true),
				IPMasq:                 boolPtr(true),
				Experimental:           boolPtr(false),
			},
		},
		{
			name: "InstallCRIDockerd explicitly false",
			input: &DockerConfig{InstallCRIDockerd: boolPtr(false)},
			expected: &DockerConfig{
				RegistryMirrors:        []string{},
				InsecureRegistries:     []string{},
				ExecOpts:               []string{},
				LogOpts:                make(map[string]string),
				DefaultAddressPools:    []DockerAddressPool{},
				StorageOpts:            []string{},
				Runtimes:               make(map[string]DockerRuntime),
				MaxConcurrentDownloads: intPtr(3),
				MaxConcurrentUploads:   intPtr(5),
				Bridge:                 stringPtr("docker0"),
				InstallCRIDockerd:      boolPtr(false), // Not overridden
				LogDriver:              stringPtr("json-file"),
				IPTables:               boolPtr(true),
				IPMasq:                 boolPtr(true),
				Experimental:           boolPtr(false),
			},
		},
		{
			name: "LogDriver already set",
			input: &DockerConfig{LogDriver: stringPtr("journald")},
			expected: &DockerConfig{
				RegistryMirrors:        []string{},
				InsecureRegistries:     []string{},
				ExecOpts:               []string{},
				LogOpts:                make(map[string]string),
				DefaultAddressPools:    []DockerAddressPool{},
				StorageOpts:            []string{},
				Runtimes:               make(map[string]DockerRuntime),
				MaxConcurrentDownloads: intPtr(3),
				MaxConcurrentUploads:   intPtr(5),
				Bridge:                 stringPtr("docker0"),
				InstallCRIDockerd:      boolPtr(true),
				LogDriver:              stringPtr("journald"), // Not overridden
				IPTables:               boolPtr(true),
				IPMasq:                 boolPtr(true),
				Experimental:           boolPtr(false),
			},
		},
		{
			name: "All fields specified",
			input: &DockerConfig{
				RegistryMirrors:     []string{"mirror.example.com"},
				InsecureRegistries:  []string{"insecure.example.com"},
				DataRoot:            stringPtr("/mnt/docker"),
				ExecOpts:            []string{"native.cgroupdriver=systemd"},
				LogDriver:           stringPtr("syslog"),
				LogOpts:             map[string]string{"tag": "docker"},
				BIP:                 stringPtr("172.28.0.1/16"),
				FixedCIDR:           stringPtr("172.29.0.0/16"),
				DefaultAddressPools: []DockerAddressPool{{Base: "192.168.0.0/16", Size: 24}},
				Experimental:        boolPtr(true),
				IPTables:            boolPtr(false),
				IPMasq:              boolPtr(false),
				StorageDriver:       stringPtr("overlay2"),
				StorageOpts:         []string{"overlay2.override_kernel_check=true"},
				DefaultRuntime:      stringPtr("nvidia"),
				Runtimes:            map[string]DockerRuntime{"nvidia": {Path: "/usr/bin/nvidia-container-runtime"}},
				MaxConcurrentDownloads: intPtr(10),
				MaxConcurrentUploads:   intPtr(10),
				Bridge:                 stringPtr("br-custom"),
				InstallCRIDockerd:      boolPtr(false),
				CRIDockerdVersion:      stringPtr("v0.2.3"),
			},
			expected: &DockerConfig{
				RegistryMirrors:     []string{"mirror.example.com"},
				InsecureRegistries:  []string{"insecure.example.com"},
				DataRoot:            stringPtr("/mnt/docker"),
				ExecOpts:            []string{"native.cgroupdriver=systemd"},
				LogDriver:           stringPtr("syslog"),
				LogOpts:             map[string]string{"tag": "docker"},
				BIP:                 stringPtr("172.28.0.1/16"),
				FixedCIDR:           stringPtr("172.29.0.0/16"),
				DefaultAddressPools: []DockerAddressPool{{Base: "192.168.0.0/16", Size: 24}},
				Experimental:        boolPtr(true),
				IPTables:            boolPtr(false),
				IPMasq:              boolPtr(false),
				StorageDriver:       stringPtr("overlay2"),
				StorageOpts:         []string{"overlay2.override_kernel_check=true"},
				DefaultRuntime:      stringPtr("nvidia"),
				Runtimes:            map[string]DockerRuntime{"nvidia": {Path: "/usr/bin/nvidia-container-runtime"}},
				MaxConcurrentDownloads: intPtr(10),
				MaxConcurrentUploads:   intPtr(10),
				Bridge:                 stringPtr("br-custom"),
				InstallCRIDockerd:      boolPtr(false),
				CRIDockerdVersion:      stringPtr("v0.2.3"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_DockerConfig(tt.input)
			if !reflect.DeepEqual(tt.input, tt.expected) {
				assert.Equal(t, tt.expected, tt.input)
			}
		})
	}
}

func TestValidate_DockerConfig(t *testing.T) {
	validCases := []struct {
		name  string
		input *DockerConfig
	}{
		{
			name:  "minimal valid after defaults",
			input: &DockerConfig{}, // Defaults will make it valid for basic fields
		},
		{
			name: "valid with specific log driver and data root",
			input: &DockerConfig{
				LogDriver: stringPtr("journald"),
				DataRoot:  stringPtr("/var/lib/docker-custom"),
				BIP:       stringPtr("172.20.0.1/16"), // isValidCIDR is from kubernetes_types
			},
		},
		{
			name: "valid with address pools",
			input: &DockerConfig{
				DefaultAddressPools: []DockerAddressPool{
					{Base: "10.10.0.0/16", Size: 24},
					{Base: "10.11.0.0/16", Size: 20},
				},
			},
		},
		{
			name: "valid runtimes",
			input: &DockerConfig{
				Runtimes: map[string]DockerRuntime{
					"runc":         {Path: "/usr/bin/runc"},
					"custom-kata": {Path: "/opt/kata/bin/kata-runtime", RuntimeArgs: []string{"--log-level=debug"}},
				},
			},
		},
	}

	for _, tt := range validCases {
		t.Run("Valid_"+tt.name, func(t *testing.T) {
			SetDefaults_DockerConfig(tt.input) // Apply defaults
			verrs := &ValidationErrors{}
			Validate_DockerConfig(tt.input, verrs, "spec.containerRuntime.docker")
			assert.True(t, verrs.IsEmpty(), "Expected no validation errors for '%s', but got: %s", tt.name, verrs.Error())
		})
	}

	invalidCases := []struct {
		name        string
		cfg         *DockerConfig
		errContains []string
	}{
		{"empty_mirror", &DockerConfig{RegistryMirrors: []string{" "}}, []string{".registryMirrors[0]: mirror URL cannot be empty"}},
		{"empty_insecure", &DockerConfig{InsecureRegistries: []string{" "}}, []string{".insecureRegistries[0]: registry host cannot be empty"}},
		{"empty_dataroot", &DockerConfig{DataRoot: stringPtr(" ")}, []string{".dataRoot: cannot be empty if specified"}},
		{"invalid_logdriver", &DockerConfig{LogDriver: stringPtr("badlog")}, []string{".logDriver: invalid log driver 'badlog'"}},
		{"invalid_bip", &DockerConfig{BIP: stringPtr("invalid")}, []string{".bip: invalid CIDR format 'invalid'"}}, // Assuming isValidCIDR is from kubernetes_types
		{"invalid_fixedcidr", &DockerConfig{FixedCIDR: stringPtr("invalid")}, []string{".fixedCIDR: invalid CIDR format 'invalid'"}},
		{"addrpool_bad_base", &DockerConfig{DefaultAddressPools: []DockerAddressPool{{Base: "invalid", Size: 24}}}, []string{".defaultAddressPools[0].base: invalid CIDR format 'invalid'"}},
		{"addrpool_bad_size_low", &DockerConfig{DefaultAddressPools: []DockerAddressPool{{Base: "172.30.0.0/16", Size: 0}}}, []string{".defaultAddressPools[0].size: invalid subnet size 0"}},
		{"addrpool_bad_size_high", &DockerConfig{DefaultAddressPools: []DockerAddressPool{{Base: "172.30.0.0/16", Size: 33}}}, []string{".defaultAddressPools[0].size: invalid subnet size 33"}},
		{"empty_storagedriver", &DockerConfig{StorageDriver: stringPtr(" ")}, []string{".storageDriver: cannot be empty if specified"}},
		{"runtime_empty_name", &DockerConfig{Runtimes: map[string]DockerRuntime{" ": {Path: "/p"}}}, []string{".runtimes: runtime name key cannot be empty"}},
		{"runtime_empty_path", &DockerConfig{Runtimes: map[string]DockerRuntime{"rt1": {Path: " "}}}, []string{".runtimes['rt1'].path: path cannot be empty"}},
		{"mcd_zero", &DockerConfig{MaxConcurrentDownloads: intPtr(0)}, []string{".maxConcurrentDownloads: must be positive if specified"}},
		{"mcu_zero", &DockerConfig{MaxConcurrentUploads: intPtr(0)}, []string{".maxConcurrentUploads: must be positive if specified"}},
		{"empty_bridge", &DockerConfig{Bridge: stringPtr(" ")}, []string{".bridge: name cannot be empty"}},
		{"empty_cridockerd_version", &DockerConfig{CRIDockerdVersion: stringPtr(" ")}, []string{".criDockerdVersion: cannot be empty if specified"}},
	}

	for _, tt := range invalidCases {
		t.Run("Invalid_"+tt.name, func(t *testing.T) {
			// It's important to apply defaults before validation as validation logic might depend on it.
			// For example, if a field is nil and validation doesn't check for nil but expects a value after defaulting.
			SetDefaults_DockerConfig(tt.cfg)
			verrs := &ValidationErrors{}
			Validate_DockerConfig(tt.cfg, verrs, "spec.containerRuntime.docker")
			assert.False(t, verrs.IsEmpty(), "Expected validation errors for '%s', but got none", tt.name)
			if len(tt.errContains) > 0 {
				fullError := verrs.Error()
				for _, errStr := range tt.errContains {
					assert.Contains(t, fullError, errStr, "Error message for '%s' does not contain expected substring '%s'. Full error: %s", tt.name, errStr, fullError)
				}
			}
		})
	}
}

// Note: isValidCIDR is expected to be defined in kubernetes_types.go or a shared file in the package.
// If not, the tests relying on it (BIP, FixedCIDR, DefaultAddressPools.Base) might not compile or might behave unexpectedly.
// For the purpose of this test suite, we assume its availability and correct functioning.
// The actual `isValidCIDR` is in `kubernetes_types.go` and uses `net.ParseCIDR`.
// The error messages for CIDR validation in the tests above reflect that.
