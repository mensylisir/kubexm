package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/mensylisir/kubexm/pkg/util/validation"
	"github.com/mensylisir/kubexm/pkg/util" // Added import
)

// Local helper pstrRegistryTest removed, using global stringPtr from util package

func TestSetDefaults_RegistryConfig(t *testing.T) {
	cfg := &RegistryConfig{}
	SetDefaults_RegistryConfig(cfg)

	assert.NotNil(t, cfg.Auths, "Auths should be initialized to empty map")
	assert.Empty(t, cfg.Auths, "Auths should be empty by default")

	// Test DataRoot IS defaulted if Type is set
	cfgWithType := &RegistryConfig{Type: util.StrPtr("harbor")}
	SetDefaults_RegistryConfig(cfgWithType)
	assert.NotNil(t, cfgWithType.DataRoot, "DataRoot should be defaulted by SetDefaults_RegistryConfig if Type is set")
	if cfgWithType.DataRoot != nil {
		assert.Equal(t, "/var/lib/registry", *cfgWithType.DataRoot, "Default DataRoot mismatch")
	}

	// Test DataRoot is not defaulted if Type is not set
	cfgNoType := &RegistryConfig{}
	SetDefaults_RegistryConfig(cfgNoType)
	assert.Nil(t, cfgNoType.DataRoot, "DataRoot should remain nil if Type is not set")


	assert.NotNil(t, cfg.NamespaceRewrite, "NamespaceRewrite should be initialized")
	if cfg.NamespaceRewrite != nil {
		assert.NotNil(t, cfg.NamespaceRewrite.Rules, "NamespaceRewrite.Rules should be initialized")
		assert.Len(t, cfg.NamespaceRewrite.Rules, 0, "NamespaceRewrite.Rules should be empty by default")
	}
}

func TestValidate_RegistryConfig(t *testing.T) {
	validAuth := make(map[string]RegistryAuth)
	validAuth["docker.io"] = RegistryAuth{Username: "user", Password: "password"}
	validAuth["myregistry.local:5000"] = RegistryAuth{Auth: "dXNlcjpwYXNzd29yZA=="}

	tests := []struct {
		name        string
		cfg         *RegistryConfig
		expectErr   bool
		errContains []string
	}{
		{
			name: "valid full config",
			cfg: &RegistryConfig{
				PrivateRegistry:   "myprivatereg.com",
				NamespaceOverride: "myorg",
				Auths:             validAuth,
				Type:              util.StrPtr("harbor"),
				DataRoot:          util.StrPtr("/data/harbor_reg"), // User explicitly sets DataRoot
				NamespaceRewrite: &NamespaceRewriteConfig{
					Enabled: true,
					Rules: []NamespaceRewriteRule{
						{Registry: "docker.io", OldNamespace: "library", NewNamespace: "dockerhub"},
						{OldNamespace: "k8s.gcr.io", NewNamespace: "google"},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "valid config with type set, DataRoot will be defaulted",
			cfg: &RegistryConfig{
				Type: util.StrPtr("registry"),
				// DataRoot is nil, will be defaulted
			},
			expectErr: false, // Should be valid after defaulting
		},
		{
			name: "valid minimal config (only auth)",
			cfg: &RegistryConfig{
				Auths: map[string]RegistryAuth{"another.com": {Username: "u", Password: "p"}},
			},
			expectErr: false,
		},
		{
			name: "privateRegistry whitespace",
			cfg:  &RegistryConfig{PrivateRegistry: "   "},
			expectErr:   true,
			errContains: []string{"spec.registry.privateRegistry: cannot be only whitespace if specified"},
		},
		{
			name: "privateRegistry invalid hostname",
			cfg:  &RegistryConfig{PrivateRegistry: "invalid_host!"},
			expectErr:   true,
			errContains: []string{"spec.registry.privateRegistry: invalid hostname/IP or host:port format 'invalid_host!'"},
		},
		{
			name: "namespaceOverride whitespace",
			cfg:  &RegistryConfig{NamespaceOverride: "   "},
			expectErr:   true,
			errContains: []string{"spec.registry.namespaceOverride: cannot be only whitespace if specified"},
		},
		{
			name: "auths empty key",
			cfg:  &RegistryConfig{Auths: map[string]RegistryAuth{" ": {Username: "u", Password: "p"}}},
			expectErr:   true,
			errContains: []string{"spec.registry.auths: registry address key cannot be empty"},
		},
		{
			name: "auths invalid key hostname",
			cfg:  &RegistryConfig{Auths: map[string]RegistryAuth{"bad_host!": {Username: "u", Password: "p"}}},
			expectErr:   true,
			errContains: []string{"spec.registry.auths[\"bad_host!\"]: registry address key 'bad_host!' is not a valid hostname or host:port"},
		},
		{
			name: "type whitespace",
			cfg:  &RegistryConfig{Type: util.StrPtr("   ")},
			expectErr:   true,
			errContains: []string{"spec.registry.type: cannot be empty if specified"},
		},
		{
			name: "dataRoot whitespace (when type is also set)", // If type is set, data root (even if whitespace) will be defaulted, so this might pass
			cfg:  &RegistryConfig{Type: util.StrPtr("registry"), DataRoot: util.StrPtr("   ")}, // DataRoot will be defaulted to /var/lib/registry
			expectErr:   false, // This should now pass as DataRoot gets a non-whitespace default
		},
		{
			name: "type set, dataRoot explicitly empty string (will be defaulted)",
			cfg:  &RegistryConfig{Type: util.StrPtr("registry"), DataRoot: util.StrPtr("")},
			expectErr:   false, // DataRoot will be defaulted
		},
		{
			name: "dataRoot set, type missing",
			cfg:  &RegistryConfig{DataRoot: util.StrPtr("/data/myreg")},
			expectErr:   true,
			errContains: []string{"spec.registry.type: must be specified if registryDataDir (dataRoot) is set for local deployment"},
		},
		{
			name: "namespaceRewrite enabled, no rules",
			cfg: &RegistryConfig{NamespaceRewrite: &NamespaceRewriteConfig{Enabled: true, Rules: []NamespaceRewriteRule{}}},
			expectErr:   true,
			errContains: []string{"spec.registry.namespaceRewrite.rules: must contain at least one rule if rewrite is enabled"},
		},
		{
			name: "namespaceRewrite rule empty oldNamespace",
			cfg: &RegistryConfig{NamespaceRewrite: &NamespaceRewriteConfig{Enabled: true, Rules: []NamespaceRewriteRule{{OldNamespace: " ", NewNamespace: "new"}}}},
			expectErr:   true,
			errContains: []string{"spec.registry.namespaceRewrite.rules[0].oldNamespace: cannot be empty"},
		},
		{
			name: "namespaceRewrite rule empty newNamespace",
			cfg: &RegistryConfig{NamespaceRewrite: &NamespaceRewriteConfig{Enabled: true, Rules: []NamespaceRewriteRule{{OldNamespace: "old", NewNamespace: " "}}}},
			expectErr:   true,
			errContains: []string{"spec.registry.namespaceRewrite.rules[0].newNamespace: cannot be empty"},
		},
		{
			name: "namespaceRewrite rule invalid registry hostname",
			cfg: &RegistryConfig{NamespaceRewrite: &NamespaceRewriteConfig{Enabled: true, Rules: []NamespaceRewriteRule{{Registry: "bad_host!", OldNamespace: "old", NewNamespace: "new"}}}},
			expectErr:   true,
			errContains: []string{"spec.registry.namespaceRewrite.rules[0].registry: invalid hostname or host:port format 'bad_host!'"},
		},
		{
			name: "namespaceRewrite rule registry only whitespace",
			cfg: &RegistryConfig{NamespaceRewrite: &NamespaceRewriteConfig{Enabled: true, Rules: []NamespaceRewriteRule{{Registry: "   ", OldNamespace: "old", NewNamespace: "new"}}}},
			expectErr:   true,
			errContains: []string{"spec.registry.namespaceRewrite.rules[0].registry: cannot be only whitespace if specified"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_RegistryConfig(tt.cfg)
			verrs := &validation.ValidationErrors{}
			Validate_RegistryConfig(tt.cfg, verrs, "spec.registry")

			if tt.expectErr {
				assert.True(t, verrs.HasErrors(), "Expected validation errors for test: %s, but got none. Errors: %v", tt.name, verrs.Error())
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

func TestSetDefaults_RegistryAuth(t *testing.T) {
	tests := []struct {
		name     string
		input    *RegistryAuth
		expected *RegistryAuth
	}{
		{"nil input", nil, nil},
		{"empty struct", &RegistryAuth{}, &RegistryAuth{SkipTLSVerify: util.BoolPtr(false), PlainHTTP: util.BoolPtr(false)}},
		{"SkipTLSVerify true", &RegistryAuth{SkipTLSVerify: util.BoolPtr(true)}, &RegistryAuth{SkipTLSVerify: util.BoolPtr(true), PlainHTTP: util.BoolPtr(false)}},
		{"PlainHTTP true", &RegistryAuth{PlainHTTP: util.BoolPtr(true)}, &RegistryAuth{SkipTLSVerify: util.BoolPtr(false), PlainHTTP: util.BoolPtr(true)}},
		{"both true", &RegistryAuth{SkipTLSVerify: util.BoolPtr(true), PlainHTTP: util.BoolPtr(true)}, &RegistryAuth{SkipTLSVerify: util.BoolPtr(true), PlainHTTP: util.BoolPtr(true)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_RegistryAuth(tt.input)
			assert.Equal(t, tt.expected, tt.input)
		})
	}
}

func TestValidate_RegistryAuth(t *testing.T) {
	tests := []struct {
		name        string
		auth        *RegistryAuth
		expectErr   bool
		errContains []string
	}{
		{"nil input", nil, false, nil},
		{"valid username/password", &RegistryAuth{Username: "user", Password: "password"}, false, nil},
		{"valid auth string", &RegistryAuth{Auth: "dXNlcjpwYXNzd29yZA=="}, false, nil},
		{"both username/password and auth string (allowed)", &RegistryAuth{Username: "user", Password: "password", Auth: "dXNlcjpwYXNzd29yZA=="}, false, nil},
		{"missing username/password and auth", &RegistryAuth{}, true, []string{": either username/password or auth string must be provided"}},
		{"invalid base64 auth", &RegistryAuth{Auth: "!!!"}, true, []string{".auth: failed to decode base64 auth string"}},
		{"invalid auth format (no colon)", &RegistryAuth{Auth: "dXNlcnBhc3N3b3Jk"}, true, []string{".auth: decoded auth string must be in 'username:password' format"}},
		{"certsPath whitespace", &RegistryAuth{Username: "u", Password: "p", CertsPath: "   "}, true, []string{".certsPath: cannot be only whitespace if specified"}},
		{"valid certsPath", &RegistryAuth{Username: "u", Password: "p", CertsPath: "/opt/certs"}, false, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verrs := &validation.ValidationErrors{}
			Validate_RegistryAuth(tt.auth, verrs, "spec.registry.auths[test.com]")

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
