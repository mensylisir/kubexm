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

	if cfg.RegistryMirrors == nil || cap(cfg.RegistryMirrors) != 0 {
		t.Error("RegistryMirrors should be initialized to empty slice")
	}
	if cfg.InsecureRegistries == nil || cap(cfg.InsecureRegistries) != 0 {
		t.Error("InsecureRegistries should be initialized to empty slice")
	}
	if cfg.Auths == nil {
		t.Error("Auths should be initialized to empty map")
	}
	// Type and DataRoot are not defaulted by SetDefaults_RegistryConfig
	if cfg.Type != nil {
		t.Errorf("Type should be nil by default, got %v", *cfg.Type)
	}
	if cfg.DataRoot != nil {
		t.Errorf("DataRoot should be nil by default, got %v", *cfg.DataRoot)
	}
}

func TestValidate_RegistryConfig(t *testing.T) {
	validAuth := make(map[string]RegistryAuth)
	validAuth["docker.io"] = RegistryAuth{Username: "user", Password: "password"}

	validCfg := &RegistryConfig{
		RegistryMirrors:    []string{"https://mirror.example.com"},
		InsecureRegistries: []string{"my.insecure.registry:5000"},
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
		{"empty_mirror", &RegistryConfig{RegistryMirrors: []string{" "}}, ".registryMirrors[0]: mirror URL cannot be empty"},
		{"invalid_mirror_url", &RegistryConfig{RegistryMirrors: []string{"not a url"}}, ".registryMirrors[0]: invalid URL format 'not a url'"},
		{"empty_insecure", &RegistryConfig{InsecureRegistries: []string{" "}}, ".insecureRegistries[0]: registry host cannot be empty"},
		{"auth_empty_key", &RegistryConfig{Auths: map[string]RegistryAuth{" ": {Username: "u", Password: "p"}}}, ".auths: registry address key cannot be empty"},
		{"auth_no_creds", &RegistryConfig{Auths: map[string]RegistryAuth{"test.com": {}}}, "auths[\"test.com\"]: either username/password or auth string must be provided"},
		{"auth_bad_base64", &RegistryConfig{Auths: map[string]RegistryAuth{"test.com": {Auth: "!!!"}}}, ".auths[\"test.com\"].auth: failed to decode base64 auth string"},
		{"type_empty_if_set", &RegistryConfig{Type: pstrRegistryTest(" ")}, ".type: cannot be empty if specified"},
		{"dataroot_empty_if_set", &RegistryConfig{DataRoot: pstrRegistryTest(" ")}, ".dataRoot: cannot be empty if specified"},
		{"type_set_dataroot_missing", &RegistryConfig{Type: pstrRegistryTest("harbor")}, ".dataRoot: must be specified if registry type is set"},
		{"dataroot_set_type_missing", &RegistryConfig{DataRoot: pstrRegistryTest("/mnt/registry")}, ".type: must be specified if dataRoot is set"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_RegistryConfig(tt.cfg) // Apply defaults, though test cases are specific
			verrs := &ValidationErrors{}
			Validate_RegistryConfig(tt.cfg, verrs, "spec.registry")
			if verrs.IsEmpty() {
				t.Fatalf("Validate_RegistryConfig expected error for %s, got none", tt.name)
			}
			if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
				t.Errorf("Validate_RegistryConfig error for %s = %v, want to contain %q", tt.name, verrs, tt.wantErrMsg)
			}
		})
	}
}
