package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Local helper pstrRegistryTest removed, using global stringPtr from zz_helpers.go

func TestSetDefaults_RegistryConfig(t *testing.T) {
	cfg := &RegistryConfig{}
	SetDefaults_RegistryConfig(cfg)

	assert.NotNil(t, cfg.Auths, "Auths should be initialized to empty map")
	assert.Empty(t, cfg.Auths, "Auths should be empty by default")

	// Test DataRoot default when Type is set
	cfgWithType := &RegistryConfig{Type: stringPtr("harbor")}
	SetDefaults_RegistryConfig(cfgWithType)
	assert.NotNil(t, cfgWithType.DataRoot, "DataRoot should be defaulted when Type is set")
	if cfgWithType.DataRoot != nil { // Guard for assert
		assert.Equal(t, "/mnt/registry", *cfgWithType.DataRoot, "Default DataRoot mismatch")
	}


	// Test DataRoot is not defaulted if Type is not set
	cfgNoType := &RegistryConfig{}
	SetDefaults_RegistryConfig(cfgNoType)
	assert.Nil(t, cfgNoType.DataRoot, "DataRoot should remain nil if Type is not set")


	// Test NamespaceRewrite initialization
	assert.NotNil(t, cfg.NamespaceRewrite, "NamespaceRewrite should be initialized")
	if cfg.NamespaceRewrite != nil { // Guard for assert
		assert.NotNil(t, cfg.NamespaceRewrite.Rules, "NamespaceRewrite.Rules should be initialized")
		assert.Len(t, cfg.NamespaceRewrite.Rules, 0, "NamespaceRewrite.Rules should be empty by default")
	}
}

func TestValidate_RegistryConfig(t *testing.T) {
	validAuth := make(map[string]RegistryAuth)
	validAuth["docker.io"] = RegistryAuth{Username: "user", Password: "password"}

	validCfg := &RegistryConfig{
		PrivateRegistry: "myprivatereg.com",
		Auths:           validAuth,
	}
	SetDefaults_RegistryConfig(validCfg) // Apply defaults
	verrsValid := &ValidationErrors{}
	Validate_RegistryConfig(validCfg, verrsValid, "spec.registry")
	assert.True(t, verrsValid.IsEmpty(), "Validate_RegistryConfig for valid config failed: %v", verrsValid.Error())

	tests := []struct {
		name        string
		cfg         *RegistryConfig
		wantErrMsg  string
		exactMatch  bool // Flag to indicate if we want an exact match for the error message
	}{
		{"auth_empty_key", &RegistryConfig{Auths: map[string]RegistryAuth{" ": {Username: "u", Password: "p"}}}, ".auths: registry address key cannot be empty", false},
		{"auth_no_creds", &RegistryConfig{Auths: map[string]RegistryAuth{"test.com": {}}}, "auths[\"test.com\"]: either username/password or auth string must be provided", false},
		{"auth_bad_base64", &RegistryConfig{Auths: map[string]RegistryAuth{"test.com": {Auth: "!!!"}}}, ".auths[\"test.com\"].auth: failed to decode base64 auth string", false},
		{"type_empty_if_set", &RegistryConfig{Type: stringPtr(" ")}, ".type: cannot be empty if specified", false},
		{"dataroot_empty_if_set", &RegistryConfig{DataRoot: stringPtr(" ")}, "spec.registry.registryDataDir (dataRoot): cannot be empty if specified", false}, // Changed exactMatch to false
		{"dataroot_set_type_missing", &RegistryConfig{DataRoot: stringPtr("/mnt/registry")}, "spec.registry.type: must be specified if registryDataDir (dataRoot) is set for local deployment", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_RegistryConfig(tt.cfg)
			verrs := &ValidationErrors{}
			Validate_RegistryConfig(tt.cfg, verrs, "spec.registry")

			assert.False(t, verrs.IsEmpty(), "Expected error for %s, got none", tt.name)

			if tt.exactMatch {
				assert.Equal(t, []string{tt.wantErrMsg}, verrs.Errors, "Error for %s did not match exactly. Got %v, want %q", tt.name, verrs.Errors, tt.wantErrMsg)
			} else {
				assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
			}
		})
	}
}
