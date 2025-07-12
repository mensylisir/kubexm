package v1alpha1

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/mensylisir/kubexm/pkg/common" // Import the common package
	"github.com/mensylisir/kubexm/pkg/util"   // Import the util package
	"k8s.io/apimachinery/pkg/runtime"        // Added for RawExtension in tests
)

// util.StrPtr, util.BoolPtr, util.IntPtr are expected to be in zz_helpers.go or similar within the package.
// We will now use util.StrPtr, util.BoolPtr, util.IntPtr

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
				LogOpts:                map[string]string{"max-size": common.DockerLogOptMaxSizeDefault, "max-file": common.DockerLogOptMaxFileDefault},
				DefaultAddressPools:    []DockerAddressPool{},
				StorageOpts:            []string{},
				Runtimes:               make(map[string]DockerRuntime),
				MaxConcurrentDownloads: util.IntPtr(common.DockerMaxConcurrentDownloadsDefault),
				MaxConcurrentUploads:   util.IntPtr(common.DockerMaxConcurrentUploadsDefault),
				Bridge:                 util.StrPtr(common.DefaultDockerBridgeName),
				InstallCRIDockerd:      util.BoolPtr(true),
				LogDriver:              util.StrPtr(common.DockerLogDriverJSONFile),
				IPTables:               util.BoolPtr(true),
				IPMasq:                 util.BoolPtr(true),
				Experimental:           util.BoolPtr(false),
				Auths:                  map[string]DockerRegistryAuth{},
			},
		},
		{
			name: "LogOpts already set - should not be overwritten by defaults for map itself, but individual keys could be if logic was different",
			input: &DockerConfig{LogOpts: map[string]string{"custom": "value"}},
			expected: &DockerConfig{
				RegistryMirrors:        []string{},
				InsecureRegistries:     []string{},
				ExecOpts:               []string{},
				LogOpts:                map[string]string{"custom": "value"}, // User's value preserved, default map is not merged for LogOpts
				DefaultAddressPools:    []DockerAddressPool{},
				StorageOpts:            []string{},
				Runtimes:               make(map[string]DockerRuntime),
				MaxConcurrentDownloads: util.IntPtr(common.DockerMaxConcurrentDownloadsDefault),
				MaxConcurrentUploads:   util.IntPtr(common.DockerMaxConcurrentUploadsDefault),
				Bridge:                 util.StrPtr(common.DefaultDockerBridgeName),
				InstallCRIDockerd:      util.BoolPtr(true),
				LogDriver:              util.StrPtr(common.DockerLogDriverJSONFile),
				IPTables:               util.BoolPtr(true),
				IPMasq:                 util.BoolPtr(true),
				Experimental:           util.BoolPtr(false),
				Auths:                  map[string]DockerRegistryAuth{},
			},
		},
		{
			name: "InstallCRIDockerd explicitly false",
			input: &DockerConfig{InstallCRIDockerd: util.BoolPtr(false)},
			expected: &DockerConfig{
				RegistryMirrors:        []string{},
				InsecureRegistries:     []string{},
				ExecOpts:               []string{},
				LogOpts:                map[string]string{"max-size": common.DockerLogOptMaxSizeDefault, "max-file": common.DockerLogOptMaxFileDefault}, // Default LogOpts applied
				DefaultAddressPools:    []DockerAddressPool{},
				StorageOpts:            []string{},
				Runtimes:               make(map[string]DockerRuntime),
				MaxConcurrentDownloads: util.IntPtr(common.DockerMaxConcurrentDownloadsDefault),
				MaxConcurrentUploads:   util.IntPtr(common.DockerMaxConcurrentUploadsDefault),
				Bridge:                 util.StrPtr(common.DefaultDockerBridgeName),
				InstallCRIDockerd:      util.BoolPtr(false),
				LogDriver:              util.StrPtr(common.DockerLogDriverJSONFile),
				IPTables:               util.BoolPtr(true),
				IPMasq:                 util.BoolPtr(true),
				Experimental:           util.BoolPtr(false),
				Auths:                  map[string]DockerRegistryAuth{},
			},
		},
		{
			name: "LogDriver already set",
			input: &DockerConfig{LogDriver: util.StrPtr(common.DockerLogDriverJournald)}, // Use constant for input if it's a valid default alternative
			expected: &DockerConfig{
				RegistryMirrors:        []string{},
				InsecureRegistries:     []string{},
				ExecOpts:               []string{},
				LogOpts:                map[string]string{"max-size": common.DockerLogOptMaxSizeDefault, "max-file": common.DockerLogOptMaxFileDefault}, // Expect default LogOpts
				DefaultAddressPools:    []DockerAddressPool{},
				StorageOpts:            []string{},
				Runtimes:               make(map[string]DockerRuntime),
				MaxConcurrentDownloads: util.IntPtr(common.DockerMaxConcurrentDownloadsDefault),
				MaxConcurrentUploads:   util.IntPtr(common.DockerMaxConcurrentUploadsDefault),
				Bridge:                 util.StrPtr(common.DefaultDockerBridgeName),
				InstallCRIDockerd:      util.BoolPtr(true),
				LogDriver:              util.StrPtr(common.DockerLogDriverJournald),
				IPTables:               util.BoolPtr(true),
				IPMasq:                 util.BoolPtr(true),
				Experimental:           util.BoolPtr(false),
				Auths:                  map[string]DockerRegistryAuth{},
			},
		},
		{
			name: "All fields specified",
			input: &DockerConfig{
				RegistryMirrors:     []string{"mirror.example.com"},
				InsecureRegistries:  []string{"insecure.example.com"},
				DataRoot:            util.StrPtr("/mnt/docker"),
				ExecOpts:            []string{"native.cgroupdriver=systemd"},
				LogDriver:           util.StrPtr("syslog"),
				LogOpts:             map[string]string{"tag": "docker"},
				BIP:                 util.StrPtr("172.28.0.1/16"),
				FixedCIDR:           util.StrPtr("172.29.0.0/16"),
				DefaultAddressPools: []DockerAddressPool{{Base: "192.168.0.0/16", Size: 24}},
				Experimental:        util.BoolPtr(true),
				IPTables:            util.BoolPtr(false),
				IPMasq:              util.BoolPtr(false),
				StorageDriver:       util.StrPtr("overlay2"),
				StorageOpts:         []string{"overlay2.override_kernel_check=true"},
				DefaultRuntime:      util.StrPtr("nvidia"),
				Runtimes:            map[string]DockerRuntime{"nvidia": {Path: "/usr/bin/nvidia-container-runtime"}},
				MaxConcurrentDownloads: util.IntPtr(10),
				MaxConcurrentUploads:   util.IntPtr(10),
				Bridge:                 util.StrPtr("br-custom"),
				InstallCRIDockerd:      util.BoolPtr(false),
				CRIDockerdVersion:      util.StrPtr("v0.2.3"),
				Auths:                  map[string]DockerRegistryAuth{"test.com": {Auth: "dXNlcjpwYXNz"}},
			},
			expected: &DockerConfig{
				RegistryMirrors:     []string{"mirror.example.com"},
				InsecureRegistries:  []string{"insecure.example.com"},
				DataRoot:            util.StrPtr("/mnt/docker"),
				ExecOpts:            []string{"native.cgroupdriver=systemd"},
				LogDriver:           util.StrPtr("syslog"),
				LogOpts:             map[string]string{"tag": "docker"},
				BIP:                 util.StrPtr("172.28.0.1/16"),
				FixedCIDR:           util.StrPtr("172.29.0.0/16"),
				DefaultAddressPools: []DockerAddressPool{{Base: "192.168.0.0/16", Size: 24}},
				Experimental:        util.BoolPtr(true),
				IPTables:            util.BoolPtr(false),
				IPMasq:              util.BoolPtr(false),
				StorageDriver:       util.StrPtr("overlay2"),
				StorageOpts:         []string{"overlay2.override_kernel_check=true"},
				DefaultRuntime:      util.StrPtr("nvidia"),
				Runtimes:            map[string]DockerRuntime{"nvidia": {Path: "/usr/bin/nvidia-container-runtime"}},
				MaxConcurrentDownloads: util.IntPtr(10),
				MaxConcurrentUploads:   util.IntPtr(10),
				Bridge:                 util.StrPtr("br-custom"),
				InstallCRIDockerd:      util.BoolPtr(false),
				CRIDockerdVersion:      util.StrPtr("v0.2.3"),
				Auths:                  map[string]DockerRegistryAuth{"test.com": {Auth: "dXNlcjpwYXNz"}},
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
			input: &DockerConfig{},
		},
		{
			name: "valid with specific log driver and data root",
			input: &DockerConfig{
				LogDriver: util.StrPtr("journald"),
				DataRoot:  util.StrPtr("/var/lib/docker-custom"),
				BIP:       util.StrPtr("172.20.0.1/16"),
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
		{
			name: "valid cri-dockerd version",
			input: &DockerConfig{
				CRIDockerdVersion: util.StrPtr("0.3.1"),
			},
		},
		{
			name: "valid cri-dockerd version with v prefix",
			input: &DockerConfig{
				CRIDockerdVersion: util.StrPtr("v0.3.1"),
			},
		},
		{
			name: "valid ExtraJSONConfig",
			input: &DockerConfig{
				ExtraJSONConfig: &runtime.RawExtension{Raw: []byte(`{"debug": true}`)},
			},
		},
		{
			name: "valid Auths with user-pass",
			input: &DockerConfig{
				Auths: map[string]DockerRegistryAuth{"docker.io": {Username: "user", Password: "password"}},
			},
		},
		{
			name: "valid Auths with base64 auth",
			input: &DockerConfig{
				Auths: map[string]DockerRegistryAuth{"my.registry.com:5000": {Auth: "dXNlcjpwYXNzd29yZA=="}},
			},
		},
		{
			name: "valid DataRoot",
			input: &DockerConfig{DataRoot: util.StrPtr("/mnt/docker_data")},
		},
	}

	for _, tt := range validCases {
		t.Run("Valid_"+tt.name, func(t *testing.T) {
			SetDefaults_DockerConfig(tt.input)
			verrs := &ValidationErrors{}
			Validate_DockerConfig(tt.input, verrs, "spec.containerRuntime.docker")
			assert.False(t, verrs.HasErrors(), "Expected no validation errors for '%s', but got: %s", tt.name, verrs.Error())
		})
	}

	invalidCases := []struct {
		name        string
		cfg         *DockerConfig
		errContains []string
	}{
		{"empty_mirror", &DockerConfig{RegistryMirrors: []string{" "}}, []string{".registryMirrors[0]: mirror URL cannot be empty"}},
		{"empty_insecure", &DockerConfig{InsecureRegistries: []string{" "}}, []string{".insecureRegistries[0]: registry host cannot be empty"}},
		{"empty_dataroot", &DockerConfig{DataRoot: util.StrPtr(" ")}, []string{".dataRoot: cannot be empty if specified"}},
		{"invalid_logdriver", &DockerConfig{LogDriver: util.StrPtr("badlog")}, []string{".logDriver: invalid log driver 'badlog'"}},
		{"invalid_mirror_url_scheme", &DockerConfig{RegistryMirrors: []string{"ftp://badmirror.com"}}, []string{"invalid URL format for mirror", "must be http or https"}},
		{"invalid_mirror_url_format", &DockerConfig{RegistryMirrors: []string{"http://invalid domain/"}}, []string{"invalid URL format for mirror"}},
		{"invalid_insecure_registry_format_bad_port", &DockerConfig{InsecureRegistries: []string{"myreg:port"}}, []string{"invalid host:port format for insecure registry"}},
		{"invalid_insecure_registry_format_bad_host", &DockerConfig{InsecureRegistries: []string{"invalid_host!"}}, []string{"invalid host:port format for insecure registry"}},
		{"invalid_bip", &DockerConfig{BIP: util.StrPtr("invalid")}, []string{".bip: invalid CIDR format 'invalid'"}},
		{"invalid_fixedcidr", &DockerConfig{FixedCIDR: util.StrPtr("invalid")}, []string{".fixedCIDR: invalid CIDR format 'invalid'"}},
		{"addrpool_bad_base", &DockerConfig{DefaultAddressPools: []DockerAddressPool{{Base: "invalid", Size: 24}}}, []string{".defaultAddressPools[0].base: invalid CIDR format 'invalid'"}},
		{"addrpool_bad_size_low", &DockerConfig{DefaultAddressPools: []DockerAddressPool{{Base: "172.30.0.0/16", Size: 0}}}, []string{".defaultAddressPools[0].size: invalid subnet size 0"}},
		{"addrpool_bad_size_high", &DockerConfig{DefaultAddressPools: []DockerAddressPool{{Base: "172.30.0.0/16", Size: 33}}}, []string{".defaultAddressPools[0].size: invalid subnet size 33"}},
		{"empty_storagedriver", &DockerConfig{StorageDriver: util.StrPtr(" ")}, []string{".storageDriver: cannot be empty if specified"}},
		{"runtime_empty_name", &DockerConfig{Runtimes: map[string]DockerRuntime{" ": {Path: "/p"}}}, []string{".runtimes: runtime name key cannot be empty"}},
		{"runtime_empty_path", &DockerConfig{Runtimes: map[string]DockerRuntime{"rt1": {Path: " "}}}, []string{".runtimes['rt1'].path: path cannot be empty"}},
		{"mcd_zero", &DockerConfig{MaxConcurrentDownloads: util.IntPtr(0)}, []string{".maxConcurrentDownloads: must be positive if specified"}},
		{"mcu_zero", &DockerConfig{MaxConcurrentUploads: util.IntPtr(0)}, []string{".maxConcurrentUploads: must be positive if specified"}},
		{"empty_bridge", &DockerConfig{Bridge: util.StrPtr(" ")}, []string{".bridge: name cannot be empty"}},
		{"empty_cridockerd_version", &DockerConfig{CRIDockerdVersion: util.StrPtr(" ")}, []string{".criDockerdVersion: cannot be only whitespace if specified"}},
		{
			"invalid_cridockerd_version_format",
			&DockerConfig{CRIDockerdVersion: util.StrPtr("0.bad.1")},
			[]string{".criDockerdVersion: '0.bad.1' is not a recognized version format"},
		},
		{
			"invalid_cridockerd_version_char",
			&DockerConfig{CRIDockerdVersion: util.StrPtr("v0.2.3_alpha")},
			[]string{".criDockerdVersion: 'v0.2.3_alpha' is not a recognized version format"},
		},
		{"extraJsonConfig_empty_raw", &DockerConfig{ExtraJSONConfig: &runtime.RawExtension{Raw: []byte("")}}, []string{".extraJsonConfig: raw data cannot be empty"}},
		{"auths_empty_key", &DockerConfig{Auths: map[string]DockerRegistryAuth{" ": {Username: "u", Password: "p"}}}, []string{"registry address key cannot be empty"}},
		{"auths_invalid_key_format", &DockerConfig{Auths: map[string]DockerRegistryAuth{"http//invalid.com": {Username: "u", Password: "p"}}}, []string{"registry key 'http//invalid.com' is not a valid hostname or host:port"}},
		{"auths_no_auth_method", &DockerConfig{Auths: map[string]DockerRegistryAuth{"docker.io": {}}}, []string{".auths[\"docker.io\"]: either username/password or auth string must be provided"}},
		{"auths_invalid_base64", &DockerConfig{Auths: map[string]DockerRegistryAuth{"docker.io": {Auth: "invalid-base64!"}}}, []string{".auths[\"docker.io\"].auth: failed to decode base64"}},
		{"auths_auth_bad_format", &DockerConfig{Auths: map[string]DockerRegistryAuth{"docker.io": {Auth: "dXNlcg=="}}}, []string{".auths[\"docker.io\"].auth: decoded auth string must be in 'username:password' format"}},
		{"dataroot_tmp", &DockerConfig{DataRoot: util.StrPtr("/tmp")}, []string{".dataRoot: path '/tmp' is not recommended"}},
		{"dataroot_var_tmp", &DockerConfig{DataRoot: util.StrPtr("/var/tmp")}, []string{".dataRoot: path '/var/tmp' is not recommended"}},
	}

	for _, tt := range invalidCases {
		t.Run("Invalid_"+tt.name, func(t *testing.T) {
			SetDefaults_DockerConfig(tt.cfg)
			verrs := &ValidationErrors{}
			Validate_DockerConfig(tt.cfg, verrs, "spec.containerRuntime.docker")
			assert.True(t, verrs.HasErrors(), "Expected validation errors for '%s', but got none", tt.name)
			if len(tt.errContains) > 0 {
				fullError := verrs.Error()
				for _, errStr := range tt.errContains {
					assert.Contains(t, fullError, errStr, "Error message for '%s' does not contain expected substring '%s'. Full error: %s", tt.name, errStr, fullError)
				}
			}
		})
	}
}
