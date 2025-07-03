package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetDefaults_StorageConfig(t *testing.T) {
	cfg := &StorageConfig{}
	SetDefaults_StorageConfig(cfg)

	assert.Nil(t, cfg.DefaultStorageClass, "DefaultStorageClass should be nil by default")
	assert.Nil(t, cfg.OpenEBS, "OpenEBS should be nil if not specified in YAML")

	// Test with OpenEBS explicitly present but empty
	cfgWithEmptyOpenEBS := &StorageConfig{OpenEBS: &OpenEBSConfig{}}
	SetDefaults_StorageConfig(cfgWithEmptyOpenEBS)
	assert.NotNil(t, cfgWithEmptyOpenEBS.OpenEBS, "OpenEBS should remain initialized if passed as empty struct")
	// Further OpenEBS defaults are tested in TestSetDefaults_OpenEBSConfig
}

func TestSetDefaults_OpenEBSConfig(t *testing.T) {
	cfg := &OpenEBSConfig{}
	SetDefaults_OpenEBSConfig(cfg)

	assert.NotNil(t, cfg.Enabled, "OpenEBS Enabled should be defaulted")
	assert.True(t, *cfg.Enabled, "Default OpenEBS Enabled should be true")
	assert.Equal(t, "/var/openebs/local", cfg.BasePath, "Default OpenEBS BasePath mismatch when enabled by default")

	// Test when Enabled is true (explicitly)
	cfgEnabled := &OpenEBSConfig{Enabled: boolPtr(true)}
	SetDefaults_OpenEBSConfig(cfgEnabled)
	assert.Equal(t, "/var/openebs/local", cfgEnabled.BasePath, "Default OpenEBS BasePath for explicitly enabled")

	assert.NotNil(t, cfgEnabled.Engines, "OpenEBS Engines should be initialized when enabled")
	if cfgEnabled.Engines != nil {
		assert.NotNil(t, cfgEnabled.Engines.LocalHostPath, "LocalHostPath engine should be initialized")
		assert.NotNil(t, cfgEnabled.Engines.LocalHostPath.Enabled, "LocalHostPath.Enabled should be defaulted")
		assert.True(t, *cfgEnabled.Engines.LocalHostPath.Enabled, "LocalHostPath engine should be enabled by default")

		assert.NotNil(t, cfgEnabled.Engines.Mayastor, "Mayastor engine should be initialized")
		assert.NotNil(t, cfgEnabled.Engines.Mayastor.Enabled, "Mayastor.Enabled should be defaulted")
		assert.False(t, *cfgEnabled.Engines.Mayastor.Enabled, "Mayastor engine should be disabled by default")
		// Similar checks for Jiva and CStor
		assert.NotNil(t, cfgEnabled.Engines.Jiva, "Jiva engine should be initialized")
		assert.NotNil(t, cfgEnabled.Engines.Jiva.Enabled, "Jiva.Enabled should be defaulted")
		assert.False(t, *cfgEnabled.Engines.Jiva.Enabled, "Jiva engine should be disabled by default")
		assert.NotNil(t, cfgEnabled.Engines.CStor, "CStor engine should be initialized")
		assert.NotNil(t, cfgEnabled.Engines.CStor.Enabled, "CStor.Enabled should be defaulted")
		assert.False(t, *cfgEnabled.Engines.CStor.Enabled, "CStor engine should be disabled by default")
	}
	assert.Nil(t, cfgEnabled.Version, "OpenEBS Version should be nil by default")

	// Test when Enabled is explicitly false
	cfgDisabled := &OpenEBSConfig{Enabled: boolPtr(false), BasePath: "custom/path", Engines: &OpenEBSEngineConfig{LocalHostPath: &OpenEBSEngineLocalHostPathConfig{Enabled: boolPtr(true)}}}
	SetDefaults_OpenEBSConfig(cfgDisabled)
	assert.Equal(t, "custom/path", cfgDisabled.BasePath, "BasePath should not be overridden when OpenEBS is disabled")
	assert.NotNil(t, cfgDisabled.Engines.LocalHostPath.Enabled, "LocalHostPath.Enabled should still be present")
	assert.False(t, *cfgDisabled.Engines.LocalHostPath.Enabled, "LocalHostPath engine should be forced to disabled")

	// Test Version is not defaulted
	cfgWithVersion := &OpenEBSConfig{Version: stringPtr("1.2.3")}
	SetDefaults_OpenEBSConfig(cfgWithVersion) // Should enable OpenEBS by default
	assert.NotNil(t, cfgWithVersion.Enabled)
	assert.True(t, *cfgWithVersion.Enabled)
	assert.NotNil(t, cfgWithVersion.Version)
	assert.Equal(t, "1.2.3", *cfgWithVersion.Version, "Version should not be changed by defaults")
}

func TestValidate_StorageConfig(t *testing.T) {
	tests := []struct {
		name        string
		cfg         *StorageConfig
		expectErr   bool
		errContains []string
	}{
		{
			name: "valid full config",
			cfg: &StorageConfig{
				OpenEBS:             &OpenEBSConfig{Enabled: boolPtr(true), BasePath: "/data/openebs", Version: stringPtr("1.0.0")},
				DefaultStorageClass: stringPtr("my-default-sc"),
			},
			expectErr: false,
		},
		{
			name: "openebs enabled, empty basepath",
			cfg:  &StorageConfig{OpenEBS: &OpenEBSConfig{Enabled: boolPtr(true), BasePath: " "}},
			expectErr:   true,
			errContains: []string{".openebs.basePath: cannot be empty if OpenEBS is enabled"},
		},
		{
			name: "openebs enabled, version whitespace",
			cfg:  &StorageConfig{OpenEBS: &OpenEBSConfig{Enabled: boolPtr(true), BasePath: "/var/openebs", Version: stringPtr("   ")}},
			expectErr:   true,
			errContains: []string{".openebs.version: cannot be only whitespace if specified"},
		},
		{
			name: "openebs enabled, version invalid format",
			cfg:  &StorageConfig{OpenEBS: &OpenEBSConfig{Enabled: boolPtr(true), BasePath: "/var/openebs", Version: stringPtr("1.2.3_invalid")}},
			expectErr:   true,
			errContains: []string{".openebs.version: '1.2.3_invalid' is not a recognized version format"},
		},
		{
			name: "empty defaultStorageClass",
			cfg:  &StorageConfig{DefaultStorageClass: stringPtr(" ")},
			expectErr:   true,
			errContains: []string{".defaultStorageClass: cannot be empty if specified"},
		},
		{
			name: "openebs disabled, version still validated if present and invalid (though maybe not typical)",
			cfg:  &StorageConfig{OpenEBS: &OpenEBSConfig{Enabled: boolPtr(false), Version: stringPtr("1.2.3_invalid")}},
			// Current logic: version is validated only if Enabled=true. So this should NOT error for version.
			// If it were to error, the errContains would be: []string{".openebs.version: '1.2.3_invalid' is not a recognized version format"}
			expectErr: false,
		},
		{
			name: "openebs disabled, version whitespace (should not error for version)",
			cfg:  &StorageConfig{OpenEBS: &OpenEBSConfig{Enabled: boolPtr(false), Version: stringPtr("   ")}},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Apply defaults because some valid states depend on defaults (like OpenEBS enabled by default)
			SetDefaults_StorageConfig(tt.cfg)
			verrs := &ValidationErrors{}
			Validate_StorageConfig(tt.cfg, verrs, "spec.storage")

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
