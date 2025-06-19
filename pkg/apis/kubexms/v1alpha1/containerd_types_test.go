package v1alpha1

import (
	"strings"
	"testing"
)

// --- Test SetDefaults_ContainerRuntimeConfig ---
func TestSetDefaults_ContainerRuntimeConfig(t *testing.T) {
	cfg := &ContainerRuntimeConfig{}
	SetDefaults_ContainerRuntimeConfig(cfg)
	if cfg.Type != ContainerRuntimeContainerd {
		t.Errorf("Default Type = %s, want %s", cfg.Type, ContainerRuntimeContainerd)
	}
}

// --- Test Validate_ContainerRuntimeConfig ---
func TestValidate_ContainerRuntimeConfig(t *testing.T) {
	validCfg := &ContainerRuntimeConfig{Type: ContainerRuntimeContainerd}
	verrsValid := &ValidationErrors{}
	Validate_ContainerRuntimeConfig(validCfg, verrsValid, "spec.containerRuntime")
	if !verrsValid.IsEmpty() {
		t.Errorf("Validate_ContainerRuntimeConfig for valid config failed: %v", verrsValid)
	}

	invalidCfg := &ContainerRuntimeConfig{Type: "rkt"}
	verrsInvalid := &ValidationErrors{}
	Validate_ContainerRuntimeConfig(invalidCfg, verrsInvalid, "spec.containerRuntime")
	if verrsInvalid.IsEmpty() || !strings.Contains(verrsInvalid.Errors[0], "invalid type 'rkt'") {
		t.Errorf("Validate_ContainerRuntimeConfig for invalid type failed or wrong message: %v", verrsInvalid)
	}
}

// --- Test SetDefaults_ContainerdConfig ---
func TestSetDefaults_ContainerdConfig(t *testing.T) {
	cfg := &ContainerdConfig{}
	SetDefaults_ContainerdConfig(cfg)

	if cfg.RegistryMirrors == nil {
		t.Error("RegistryMirrors should be initialized")
	}
	if cfg.InsecureRegistries == nil {
		t.Error("InsecureRegistries should be initialized")
	}
	if cfg.UseSystemdCgroup == nil || !*cfg.UseSystemdCgroup {
		t.Errorf("UseSystemdCgroup default = %v, want true", cfg.UseSystemdCgroup)
	}
	if cfg.ConfigPath == nil || *cfg.ConfigPath != "/etc/containerd/config.toml" {
	   t.Errorf("ConfigPath default = %v, want /etc/containerd/config.toml", cfg.ConfigPath)
	}
}

// --- Test Validate_ContainerdConfig ---
func TestValidate_ContainerdConfig_Valid(t *testing.T) {
	cfg := &ContainerdConfig{
		RegistryMirrors:    map[string][]string{"docker.io": {"https://mirror.example.com"}},
		InsecureRegistries: []string{"my.registry:5000"},
		ConfigPath:         pstrContainerdTest("/custom/config.toml"), // Use local helper
	}
	SetDefaults_ContainerdConfig(cfg) // Apply defaults
	verrs := &ValidationErrors{}
	Validate_ContainerdConfig(cfg, verrs, "spec.containerd")
	if !verrs.IsEmpty() {
		t.Errorf("Validate_ContainerdConfig for valid config failed: %v", verrs)
	}
}

func TestValidate_ContainerdConfig_Invalid(t *testing.T) {
   tests := []struct{
	   name string
	   cfg *ContainerdConfig
	   wantErrMsg string
   }{
	   {"empty_mirror_key", &ContainerdConfig{RegistryMirrors: map[string][]string{" ": {"m1"}}}, "registry host key cannot be empty"},
	   {"empty_mirror_list", &ContainerdConfig{RegistryMirrors: map[string][]string{"docker.io": {}}}, "must contain at least one mirror URL"},
	   {"empty_mirror_url", &ContainerdConfig{RegistryMirrors: map[string][]string{"docker.io": {" "}}}, "mirror URL cannot be empty"},
	   {"empty_insecure_reg", &ContainerdConfig{InsecureRegistries: []string{" "}}, "registry host cannot be empty"},
	   {"empty_config_path", &ContainerdConfig{ConfigPath: pstrContainerdTest(" ")}, "configPath: cannot be empty if specified"}, // Use local helper
   }

   for _, tt := range tests {
	   t.Run(tt.name, func(t *testing.T){
		   SetDefaults_ContainerdConfig(tt.cfg)
		   verrs := &ValidationErrors{}
		   Validate_ContainerdConfig(tt.cfg, verrs, "spec.containerd")
		   if verrs.IsEmpty() {
				t.Fatalf("Validate_ContainerdConfig expected error for %s, got none", tt.name)
			}
			if !strings.Contains(verrs.Errors[0], tt.wantErrMsg) {
				t.Errorf("Validate_ContainerdConfig error for %s = %v, want to contain %q", tt.name, verrs.Errors[0], tt.wantErrMsg)
			}
	   })
   }
}
// Local pstr helper for containerd_types_test.go as it might be created independently of etcd_types_test.go
func pstrContainerdTest(s string) *string { return &s }
