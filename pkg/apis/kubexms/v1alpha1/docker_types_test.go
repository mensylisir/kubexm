package v1alpha1

import (
	"strings"
	"testing"
)

// Helpers for Docker tests
func pstrDockerTest(s string) *string { return &s }
func pboolDockerTest(b bool) *bool { return &b }
func pintDockerTest(i int) *int { return &i }

func TestSetDefaults_DockerConfig(t *testing.T) {
	cfg := &DockerConfig{}
	SetDefaults_DockerConfig(cfg)

	if cfg.RegistryMirrors == nil || cap(cfg.RegistryMirrors) != 0 { t.Error("RegistryMirrors default failed") }
	if cfg.InsecureRegistries == nil || cap(cfg.InsecureRegistries) != 0 { t.Error("InsecureRegistries default failed") }
	if cfg.ExecOpts == nil || cap(cfg.ExecOpts) != 0 { t.Error("ExecOpts default failed") }
	if cfg.LogOpts == nil { t.Error("LogOpts default failed") }
	if cfg.DefaultAddressPools == nil || cap(cfg.DefaultAddressPools) != 0 { t.Error("DefaultAddressPools default failed") }
	if cfg.StorageOpts == nil || cap(cfg.StorageOpts) != 0 { t.Error("StorageOpts default failed") }
	if cfg.Runtimes == nil { t.Error("Runtimes default failed") }

	if cfg.LogDriver == nil || *cfg.LogDriver != "json-file" { t.Errorf("LogDriver default failed: %v", cfg.LogDriver) }
	if cfg.IPTables == nil || !*cfg.IPTables { t.Errorf("IPTables default failed: %v", cfg.IPTables) }
	if cfg.IPMasq == nil || !*cfg.IPMasq { t.Errorf("IPMasq default failed: %v", cfg.IPMasq) }
	if cfg.Experimental == nil || *cfg.Experimental != false { t.Errorf("Experimental default failed: %v", cfg.Experimental) }
	if cfg.MaxConcurrentDownloads == nil || *cfg.MaxConcurrentDownloads != 3 { t.Errorf("MaxConcurrentDownloads default failed: %v", cfg.MaxConcurrentDownloads) }
	if cfg.MaxConcurrentUploads == nil || *cfg.MaxConcurrentUploads != 5 { t.Errorf("MaxConcurrentUploads default failed: %v", cfg.MaxConcurrentUploads) }
	if cfg.Bridge == nil || *cfg.Bridge != "docker0" { t.Errorf("Bridge default failed: %v", cfg.Bridge) }
}

func TestValidate_DockerConfig_Valid(t *testing.T) {
	cfg := &DockerConfig{
		LogDriver: pstrDockerTest("journald"),
		DataRoot:  pstrDockerTest("/var/lib/docker-custom"),
		BIP:       pstrDockerTest("172.20.0.1/16"),
	}
	SetDefaults_DockerConfig(cfg)
	verrs := &ValidationErrors{}
	Validate_DockerConfig(cfg, verrs, "spec.containerRuntime.docker")
	if !verrs.IsEmpty() {
		t.Errorf("Validate_DockerConfig for valid config failed: %v", verrs)
	}
}

func TestValidate_DockerConfig_Invalid(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *DockerConfig
		wantErrMsg string
	}{
		{"empty_mirror", &DockerConfig{RegistryMirrors: []string{" "}}, ".registryMirrors[0]: mirror URL cannot be empty"},
		{"empty_insecure", &DockerConfig{InsecureRegistries: []string{" "}}, ".insecureRegistries[0]: registry host cannot be empty"},
		{"empty_dataroot", &DockerConfig{DataRoot: pstrDockerTest(" ")}, ".dataRoot: cannot be empty if specified"},
		{"invalid_logdriver", &DockerConfig{LogDriver: pstrDockerTest("badlog")}, ".logDriver: invalid log driver 'badlog'"},
		{"invalid_bip", &DockerConfig{BIP: pstrDockerTest("invalid")}, ".bip: invalid CIDR format"},
		{"invalid_fixedcidr", &DockerConfig{FixedCIDR: pstrDockerTest("invalid")}, ".fixedCIDR: invalid CIDR format"},
		{"addrpool_bad_base", &DockerConfig{DefaultAddressPools: []DockerAddressPool{{Base: "invalid", Size: 24}}}, ".defaultAddressPools[0].base: invalid CIDR format"},
		{"addrpool_bad_size_low", &DockerConfig{DefaultAddressPools: []DockerAddressPool{{Base: "172.30.0.0/16", Size: 0}}}, ".defaultAddressPools[0].size: invalid subnet size 0"},
		{"addrpool_bad_size_high", &DockerConfig{DefaultAddressPools: []DockerAddressPool{{Base: "172.30.0.0/16", Size: 33}}}, ".defaultAddressPools[0].size: invalid subnet size 33"},
		{"empty_storagedriver", &DockerConfig{StorageDriver: pstrDockerTest(" ")}, ".storageDriver: cannot be empty if specified"},
		{"runtime_empty_name", &DockerConfig{Runtimes: map[string]DockerRuntime{" ": {Path: "/p"}}}, ".runtimes: runtime name key cannot be empty"},
		{"runtime_empty_path", &DockerConfig{Runtimes: map[string]DockerRuntime{"rt1": {Path: " "}}}, ".runtimes['rt1'].path: path cannot be empty"},
		{"mcd_zero", &DockerConfig{MaxConcurrentDownloads: pintDockerTest(0)}, ".maxConcurrentDownloads: must be positive"},
		{"mcu_zero", &DockerConfig{MaxConcurrentUploads: pintDockerTest(0)}, ".maxConcurrentUploads: must be positive"},
		{"empty_bridge", &DockerConfig{Bridge: pstrDockerTest(" ")}, ".bridge: name cannot be empty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_DockerConfig(tt.cfg)
			verrs := &ValidationErrors{}
			Validate_DockerConfig(tt.cfg, verrs, "spec.containerRuntime.docker")
			if verrs.IsEmpty() {
				t.Fatalf("Validate_DockerConfig expected error for %s, got none", tt.name)
			}
			if !strings.Contains(verrs.Error(), tt.wantErrMsg) {
				t.Errorf("Validate_DockerConfig error for '%s' = %v, want to contain %q", tt.name, verrs, tt.wantErrMsg)
			}
		})
	}
}
