package v1alpha1

import (
	"strings"
	"testing"
)

// Helper for Registry tests
func pstrRegistryTest(s string) *string { return &s }

func TestSetDefaults_RegistryConfig(t *testing.T) {
	cfg := &RegistryConfig{}
	SetDefaults_RegistryConfig(cfg)

	// RegistryMirrors and InsecureRegistries are removed from RegistryConfig.
	// if cfg.RegistryMirrors == nil || cap(cfg.RegistryMirrors) != 0 {
	// 	t.Error("RegistryMirrors should be initialized to empty slice")
	// }
	// if cfg.InsecureRegistries == nil || cap(cfg.InsecureRegistries) != 0 {
	// 	t.Error("InsecureRegistries should be initialized to empty slice")
	// }
	if cfg.Auths == nil {
		t.Error("Auths should be initialized to empty map")
	}
	// PrivateRegistry default is no longer "dockerhub.kubexm.local" in SetDefaults_RegistryConfig
	// if cfg.PrivateRegistry != "dockerhub.kubexm.local" {
	// 	t.Errorf("Expected PrivateRegistry default 'dockerhub.kubexm.local', got '%s'", cfg.PrivateRegistry)
	// }

	// Test DataRoot default when Type is set
	cfgWithType := &RegistryConfig{Type: pstrRegistryTest("harbor")}
	SetDefaults_RegistryConfig(cfgWithType)
	if cfgWithType.DataRoot == nil || *cfgWithType.DataRoot != "/mnt/registry" {
		t.Errorf("Expected DataRoot default '/mnt/registry' when Type is set, got %v", cfgWithType.DataRoot)
	}

	// Test DataRoot is not defaulted if Type is not set
	cfgNoType := &RegistryConfig{}
	SetDefaults_RegistryConfig(cfgNoType) // PrivateRegistry will be defaulted here again, that's fine
	if cfgNoType.DataRoot != nil {
		t.Errorf("DataRoot should remain nil if Type is not set, got %v", *cfgNoType.DataRoot)
	}

	// Test NamespaceRewrite initialization
	if cfg.NamespaceRewrite == nil {
		t.Error("NamespaceRewrite should be initialized")
	}
	if cfg.NamespaceRewrite != nil && cfg.NamespaceRewrite.Rules == nil {
		t.Error("NamespaceRewrite.Rules should be initialized to an empty slice")
	}

}

func TestValidate_RegistryConfig(t *testing.T) {
	validAuth := make(map[string]RegistryAuth)
	validAuth["docker.io"] = RegistryAuth{Username: "user", Password: "password"}

	validCfg := &RegistryConfig{
		// RegistryMirrors and InsecureRegistries removed
		PrivateRegistry:    "myprivatereg.com",
		Auths:              validAuth,
	}
	SetDefaults_RegistryConfig(validCfg) // Apply defaults
	verrsValid := &ValidationErrors{}
	Validate_RegistryConfig(validCfg, verrsValid, "spec.registry")
	if !verrsValid.IsEmpty() {
		t.Errorf("Validate_RegistryConfig for valid config failed: %v", verrsValid)
	}

	tests := []struct {
		name       string
		cfg        *RegistryConfig
		wantErrMsg string
	}{
		// {"empty_mirror", &RegistryConfig{RegistryMirrors: []string{" "}}, ".registryMirrors[0]: mirror URL cannot be empty"}, // Field removed
		// {"invalid_mirror_url", &RegistryConfig{RegistryMirrors: []string{"not a url"}}, ".registryMirrors[0]: invalid URL format 'not a url'"}, // Field removed
		// {"empty_insecure", &RegistryConfig{InsecureRegistries: []string{" "}}, ".insecureRegistries[0]: registry host cannot be empty"}, // Field removed
		{"auth_empty_key", &RegistryConfig{Auths: map[string]RegistryAuth{" ": {Username: "u", Password: "p"}}}, ".auths: registry address key cannot be empty"},
		{"auth_no_creds", &RegistryConfig{Auths: map[string]RegistryAuth{"test.com": {}}}, "auths[\"test.com\"]: either username/password or auth string must be provided"},
		{"auth_bad_base64", &RegistryConfig{Auths: map[string]RegistryAuth{"test.com": {Auth: "!!!"}}}, ".auths[\"test.com\"].auth: failed to decode base64 auth string"},
		{"type_empty_if_set", &RegistryConfig{Type: pstrRegistryTest(" ")}, ".type: cannot be empty if specified"},
		{"dataroot_empty_if_set", &RegistryConfig{DataRoot: pstrRegistryTest(" ")}, "spec.registry.registryDataDir (dataRoot): cannot be empty if specified"}, // Exact first error
		// {"type_set_dataroot_missing", &RegistryConfig{Type: pstrRegistryTest("harbor")}, ".dataRoot: must be specified if registry type is set"}, // Defaulting handles this
		{"dataroot_set_type_missing", &RegistryConfig{DataRoot: pstrRegistryTest("/mnt/registry")}, "spec.registry.type: must be specified if registryDataDir (dataRoot) is set for local deployment"}, // Exact error
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_RegistryConfig(tt.cfg) // Apply defaults, though test cases are specific
			verrs := &ValidationErrors{}
			Validate_RegistryConfig(tt.cfg, verrs, "spec.registry")
			if verrs.IsEmpty() {
				t.Fatalf("Validate_RegistryConfig expected error for %s, got none", tt.name)
			}

			// For dataroot_empty_if_set, we expect two errors. Check the first one exactly.
			// For other cases, check if the verrs.Error() (joined string) contains the specific message.
			// This handles cases where a single specific validation is targeted.
			found := false
			if tt.name == "dataroot_empty_if_set" {
				if len(verrs.Errors) > 0 {
					t.Logf("dataroot_empty_if_set: Error[0]: |%s|, wantErrMsg: |%s|", verrs.Errors[0], tt.wantErrMsg)
					if verrs.Errors[0] == tt.wantErrMsg {
						found = true
					}
				}
			} else if tt.name == "dataroot_set_type_missing" { // This also expects an exact match for a single error
				if len(verrs.Errors) == 1 && verrs.Errors[0] == tt.wantErrMsg {
					found = true
				}
			} else {
				if strings.Contains(verrs.Error(), tt.wantErrMsg) {
					found = true
				}
			}

			if !found {
				t.Errorf("Validate_RegistryConfig error for %s. Expected '%s', got %v", tt.name, tt.wantErrMsg, verrs.Errors)
			}
		})
	}
}
