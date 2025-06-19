package v1alpha1

import (
	"strings"
	"testing"
)

// Helper for Storage tests
func pboolStorageTestHelper(b bool) *bool { return &b } // Renamed to avoid conflict if merged
// func pstrStorageTest(s string) *string { return &s } // If needed for version etc.

func TestSetDefaults_StorageConfig(t *testing.T) {
	cfg := &StorageConfig{}
	SetDefaults_StorageConfig(cfg)

	// DefaultStorageClass is not defaulted, so it should be nil
	if cfg.DefaultStorageClass != nil {
		t.Errorf("DefaultStorageClass should be nil by default, got %v", *cfg.DefaultStorageClass)
	}
	// OpenEBS is not initialized by default in SetDefaults_StorageConfig itself,
	// but SetDefaults_Cluster initializes Spec.Storage to &StorageConfig{}, then calls SetDefaults_StorageConfig.
	// SetDefaults_StorageConfig then calls SetDefaults_OpenEBSConfig if OpenEBS is not nil.
	// So, if OpenEBS is intended to be defaulted when storage: {} is present,
	// then SetDefaults_StorageConfig should initialize cfg.OpenEBS = &OpenEBSConfig{}
	// For this test, we assume it's not initialized if openebs: {} is missing.
	if cfg.OpenEBS != nil { // Test what happens if it was nil
		t.Errorf("OpenEBS should be nil if not specified in YAML, got %v", cfg.OpenEBS)
	}

	// Test with OpenEBS explicitly present but empty
	cfgWithEmptyOpenEBS := &StorageConfig{OpenEBS: &OpenEBSConfig{}}
	SetDefaults_StorageConfig(cfgWithEmptyOpenEBS)
	if cfgWithEmptyOpenEBS.OpenEBS == nil {
		t.Fatal("OpenEBS should remain initialized if passed as empty struct")
	}
	// Defaults for OpenEBS sub-fields are tested in TestSetDefaults_OpenEBSConfig
}

func TestSetDefaults_OpenEBSConfig(t *testing.T) {
	cfg := &OpenEBSConfig{}
	SetDefaults_OpenEBSConfig(cfg)

	if cfg.Enabled == nil || *cfg.Enabled != false {
		t.Errorf("Default OpenEBS Enabled = %v, want false", cfg.Enabled)
	}
	// BasePath should not be defaulted if Enabled is false (or nil defaulted to false)
	if cfg.BasePath != "" {
		t.Errorf("Default OpenEBS BasePath = %s, want \"\" when not enabled", cfg.BasePath)
	}

	// Test when Enabled is true
	cfgEnabled := &OpenEBSConfig{Enabled: pboolStorageTestHelper(true)}
	SetDefaults_OpenEBSConfig(cfgEnabled)
	if cfgEnabled.BasePath != "/var/openebs/local" {
		t.Errorf("Default OpenEBS BasePath for enabled = %s, want /var/openebs/local", cfgEnabled.BasePath)
	}
	if cfgEnabled.Engines == nil { t.Fatal("OpenEBS Engines should be initialized when enabled") }
	if cfgEnabled.Engines.LocalHostPath == nil || cfgEnabled.Engines.LocalHostPath.Enabled == nil || !*cfgEnabled.Engines.LocalHostPath.Enabled {
		t.Error("OpenEBS Engines.LocalHostPath should be enabled by default when OpenEBS is enabled")
	}
    if cfgEnabled.Engines.Mayastor == nil || cfgEnabled.Engines.Mayastor.Enabled == nil || *cfgEnabled.Engines.Mayastor.Enabled != false {
        t.Error("OpenEBS Engines.Mayastor should default to disabled")
    }
    // Version is not defaulted
    if cfgEnabled.Version != nil {
        t.Errorf("OpenEBS Version should be nil by default, got %v", *cfgEnabled.Version)
    }
}

func TestValidate_StorageConfig(t *testing.T) {
	validCfg := &StorageConfig{
		OpenEBS: &OpenEBSConfig{Enabled: pboolStorageTestHelper(true), BasePath: "/data/openebs"},
		DefaultStorageClass: pstrStorageTest("my-default-sc"),
	}
	SetDefaults_StorageConfig(validCfg) // Ensure defaults are applied before validation
	verrsValid := &ValidationErrors{}
	Validate_StorageConfig(validCfg, verrsValid, "spec.storage")
	if !verrsValid.IsEmpty() {
		t.Errorf("Validate_StorageConfig for valid config failed: %v", verrsValid)
	}

	tests := []struct {
		name       string
		cfg        *StorageConfig
		wantErrMsg string
	}{
		{"openebs_enabled_empty_basepath",
			&StorageConfig{OpenEBS: &OpenEBSConfig{Enabled: pboolStorageTestHelper(true), BasePath: " "}},
			".openebs.basePath: cannot be empty if OpenEBS is enabled"},
		{"empty_default_storage_class",
			&StorageConfig{DefaultStorageClass: pstrStorageTest(" ")},
			".defaultStorageClass: cannot be empty if specified"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Defaults are not re-applied here as the test is about validating specific states
			// SetDefaults_StorageConfig(tt.cfg)
			verrs := &ValidationErrors{}
			Validate_StorageConfig(tt.cfg, verrs, "spec.storage")
			if verrs.IsEmpty() {
				t.Fatalf("Validate_StorageConfig expected error for %s, got none", tt.name)
			}
			if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
				t.Errorf("Validate_StorageConfig error for %s = %v, want to contain %q", tt.name, verrs, tt.wantErrMsg)
			}
		})
	}
}

// pstrStorageTest is a local helper, assuming pstr might be defined elsewhere or to avoid conflict
func pstrStorageTest(s string) *string { return &s }
