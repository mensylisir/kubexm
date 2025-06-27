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

}

func TestValidate_StorageConfig(t *testing.T) {
	validCfg := &StorageConfig{
		OpenEBS:             &OpenEBSConfig{Enabled: boolPtr(true), BasePath: "/data/openebs"},
		DefaultStorageClass: stringPtr("my-default-sc"),
	}
	SetDefaults_StorageConfig(validCfg)
	verrsValid := &ValidationErrors{}
	Validate_StorageConfig(validCfg, verrsValid, "spec.storage")
	assert.True(t, verrsValid.IsEmpty(), "Validate_StorageConfig for valid config failed: %v", verrsValid.Error())

	tests := []struct {
		name       string
		cfg        *StorageConfig
		wantErrMsg string
	}{
		{"openebs_enabled_empty_basepath",
			&StorageConfig{OpenEBS: &OpenEBSConfig{Enabled: boolPtr(true), BasePath: " "}},
			".openebs.basePath: cannot be empty if OpenEBS is enabled"},
		{"empty_default_storage_class",
			&StorageConfig{DefaultStorageClass: stringPtr(" ")},
			".defaultStorageClass: cannot be empty if specified"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For these specific validation tests, defaults on the parent StorageConfig
			// don't interfere with the specific invalid field being tested.
			// SetDefaults_StorageConfig(tt.cfg)
			verrs := &ValidationErrors{}
			Validate_StorageConfig(tt.cfg, verrs, "spec.storage")
			assert.False(t, verrs.IsEmpty(), "Expected error for %s, got none", tt.name)
			assert.Contains(t, verrs.Error(), tt.wantErrMsg, "Error for %s = %v, want to contain %q", tt.name, verrs.Error(), tt.wantErrMsg)
		})
	}
}
