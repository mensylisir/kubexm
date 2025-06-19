package v1alpha1

import (
	"strings"
	"testing"
)

// --- Test SetDefaults_EtcdConfig ---
func TestSetDefaults_EtcdConfig(t *testing.T) {
	cfg := &EtcdConfig{}
	SetDefaults_EtcdConfig(cfg)

	if cfg.Type != EtcdTypeKubeXMSInternal {
		t.Errorf("Default Type = %s, want %s", cfg.Type, EtcdTypeKubeXMSInternal)
	}
	if cfg.ClientPort == nil || *cfg.ClientPort != 2379 {
		t.Errorf("Default ClientPort = %v, want 2379", cfg.ClientPort)
	}
	if cfg.PeerPort == nil || *cfg.PeerPort != 2380 {
		t.Errorf("Default PeerPort = %v, want 2380", cfg.PeerPort)
	}
	if cfg.DataDir == nil || *cfg.DataDir != "/var/lib/etcd" {
		t.Errorf("Default DataDir = %v, want /var/lib/etcd", cfg.DataDir)
	}
	if cfg.ExtraArgs == nil || cap(cfg.ExtraArgs) == 0 { // Check for non-nil empty slice
		t.Error("ExtraArgs should be initialized as an empty slice, not nil")
	}
	if cfg.BackupDir == nil || *cfg.BackupDir != "/var/backups/etcd" { t.Errorf("Default BackupDir failed: %v", cfg.BackupDir) }
	if cfg.BackupPeriodHours == nil || *cfg.BackupPeriodHours != 24 { t.Errorf("Default BackupPeriodHours failed: %v", cfg.BackupPeriodHours) }
	if cfg.KeepBackupNumber == nil || *cfg.KeepBackupNumber != 7 { t.Errorf("Default KeepBackupNumber failed: %v", cfg.KeepBackupNumber) }

	if cfg.HeartbeatIntervalMillis == nil || *cfg.HeartbeatIntervalMillis != 100 { t.Errorf("Default HeartbeatIntervalMillis failed: %v", cfg.HeartbeatIntervalMillis) }
	if cfg.ElectionTimeoutMillis == nil || *cfg.ElectionTimeoutMillis != 1000 { t.Errorf("Default ElectionTimeoutMillis failed: %v", cfg.ElectionTimeoutMillis) }
	if cfg.SnapshotCount == nil || *cfg.SnapshotCount != 100000 { t.Errorf("Default SnapshotCount failed: %v", cfg.SnapshotCount) }
	if cfg.AutoCompactionRetentionHours == nil || *cfg.AutoCompactionRetentionHours != 0 { t.Errorf("Default AutoCompactionRetentionHours failed: %v", cfg.AutoCompactionRetentionHours) }

	if cfg.QuotaBackendBytes == nil || *cfg.QuotaBackendBytes != 0 { t.Errorf("Default QuotaBackendBytes failed: %v", cfg.QuotaBackendBytes) }
	// MaxRequestBytes is not defaulted in SetDefaults_EtcdConfig, so no check here.

	if cfg.Metrics == nil || *cfg.Metrics != "basic" { t.Errorf("Default Metrics failed: %v", cfg.Metrics) }
	if cfg.LogLevel == nil || *cfg.LogLevel != "info" { t.Errorf("Default LogLevel failed: %v", cfg.LogLevel) }
	if cfg.MaxSnapshotsToKeep == nil || *cfg.MaxSnapshotsToKeep != 5 { t.Errorf("Default MaxSnapshotsToKeep failed: %v", cfg.MaxSnapshotsToKeep) }
	if cfg.MaxWALsToKeep == nil || *cfg.MaxWALsToKeep != 5 { t.Errorf("Default MaxWALsToKeep failed: %v", cfg.MaxWALsToKeep) }

	cfgExternal := &EtcdConfig{Type: EtcdTypeExternal}
	SetDefaults_EtcdConfig(cfgExternal)
	if cfgExternal.External == nil {
		t.Error("External should be initialized if Type is external")
	}
	if cfgExternal.External != nil && cfgExternal.External.Endpoints == nil {
	   t.Error("External.Endpoints should be initialized if External is not nil")
	}
}

// --- Test Validate_EtcdConfig ---
func TestValidate_EtcdConfig_Valid(t *testing.T) {
	cfgStacked := &EtcdConfig{Type: EtcdTypeKubeXMSInternal, ClientPort: pint(2379), PeerPort: pint(2380), DataDir: pstr("/var/lib/etcd")}
	SetDefaults_EtcdConfig(cfgStacked) // Apply defaults to fill other fields if necessary
	verrsStacked := &ValidationErrors{}
	Validate_EtcdConfig(cfgStacked, verrsStacked, "spec.etcd")
	if !verrsStacked.IsEmpty() {
		t.Errorf("Validate_EtcdConfig for valid stacked config failed: %v", verrsStacked)
	}

	cfgExternal := &EtcdConfig{
		Type: EtcdTypeExternal,
		External: &ExternalEtcdConfig{Endpoints: []string{"http://etcd1:2379"}},
	}
	SetDefaults_EtcdConfig(cfgExternal)
	verrsExternal := &ValidationErrors{}
	Validate_EtcdConfig(cfgExternal, verrsExternal, "spec.etcd")
	if !verrsExternal.IsEmpty() {
		t.Errorf("Validate_EtcdConfig for valid external config failed: %v", verrsExternal)
	}
}

func TestValidate_EtcdConfig_Invalid(t *testing.T) {
	tests := []struct {
		name       string
		cfg        *EtcdConfig
		wantErrMsg string
	}{
		{"invalid type", &EtcdConfig{Type: "invalid"}, "invalid type 'invalid'"},
		{"external_no_config", &EtcdConfig{Type: EtcdTypeExternal, External: nil}, ".external: must be defined"},
		{"external_no_endpoints", &EtcdConfig{Type: EtcdTypeExternal, External: &ExternalEtcdConfig{Endpoints: []string{}}}, ".external.endpoints: must contain at least one endpoint"},
		{"external_empty_endpoint", &EtcdConfig{Type: EtcdTypeExternal, External: &ExternalEtcdConfig{Endpoints: []string{""}}}, ".external.endpoints[0]: endpoint cannot be empty"},
		{"external_mismatched_tls_cert", &EtcdConfig{Type: EtcdTypeExternal, External: &ExternalEtcdConfig{Endpoints: []string{"h"}, CertFile: "cert"}}, "certFile and keyFile must be specified together"},
		{"external_mismatched_tls_key", &EtcdConfig{Type: EtcdTypeExternal, External: &ExternalEtcdConfig{Endpoints: []string{"h"}, KeyFile: "key"}}, "certFile and keyFile must be specified together"},
		{"invalid_client_port_low", &EtcdConfig{Type: EtcdTypeKubeXMSInternal, ClientPort: pint(0)}, ".clientPort: invalid port 0"},
		{"invalid_client_port_high", &EtcdConfig{Type: EtcdTypeKubeXMSInternal, ClientPort: pint(70000)}, ".clientPort: invalid port 70000"},
		{"invalid_peer_port", &EtcdConfig{Type: EtcdTypeKubeXMSInternal, PeerPort: pint(0)}, ".peerPort: invalid port 0"},
		{"empty_datadir", &EtcdConfig{Type: EtcdTypeKubeXMSInternal, DataDir: pstr(" ")}, ".dataDir: cannot be empty if specified"},
		{"negative_backup_period", &EtcdConfig{Type: EtcdTypeKubeXMSInternal, BackupPeriodHours: pint(-1)}, ".backupPeriodHours: cannot be negative"},
		{"negative_keep_backup", &EtcdConfig{Type: EtcdTypeKubeXMSInternal, KeepBackupNumber: pint(-1)}, ".keepBackupNumber: cannot be negative"},
		{"zero_heartbeat", &EtcdConfig{Type: EtcdTypeKubeXMSInternal, HeartbeatIntervalMillis: pint(0)}, ".heartbeatIntervalMillis: must be positive"},
		{"zero_election_timeout", &EtcdConfig{Type: EtcdTypeKubeXMSInternal, ElectionTimeoutMillis: pint(0)}, ".electionTimeoutMillis: must be positive"},
		{"negative_autocompaction", &EtcdConfig{Type: EtcdTypeKubeXMSInternal, AutoCompactionRetentionHours: pint(-1)}, ".autoCompactionRetentionHours: cannot be negative"},
		{"negative_quota", &EtcdConfig{Type: EtcdTypeKubeXMSInternal, QuotaBackendBytes: pint64(-100)}, ".quotaBackendBytes: cannot be negative"},
		{"zero_max_request_bytes", &EtcdConfig{Type: EtcdTypeKubeXMSInternal, MaxRequestBytes: puint(0)}, ".maxRequestBytes: must be positive if set"}, // MaxRequestBytes uses *uint, 0 is invalid if set
		{"invalid_metrics", &EtcdConfig{Type: EtcdTypeKubeXMSInternal, Metrics: pstr("detailed")}, ".metrics: invalid value 'detailed'"},
		{"invalid_loglevel", &EtcdConfig{Type: EtcdTypeKubeXMSInternal, LogLevel: pstr("trace")}, ".logLevel: invalid value 'trace'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_EtcdConfig(tt.cfg) // Apply defaults before validation
			verrs := &ValidationErrors{}
			Validate_EtcdConfig(tt.cfg, verrs, "spec.etcd")
			if verrs.IsEmpty() {
				t.Fatalf("Validate_EtcdConfig expected error for %s, got none", tt.name)
			}
			found := false
			for _, eStr := range verrs.Errors {
				if strings.Contains(eStr, tt.wantErrMsg) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Validate_EtcdConfig error for %s = %v, want to contain %q", tt.name, verrs, tt.wantErrMsg)
			}
		})
	}
}

// --- Test EtcdConfig Helper Methods ---
func TestEtcdConfig_GetPortsAndDataDir(t *testing.T) {
	// Test with nil config
	var nilCfg *EtcdConfig
	if nilCfg.GetClientPort() != 2379 { t.Error("nilCfg.GetClientPort failed") }
	if nilCfg.GetPeerPort() != 2380 { t.Error("nilCfg.GetPeerPort failed") }
	if nilCfg.GetDataDir() != "/var/lib/etcd" { t.Error("nilCfg.GetDataDir failed") }

	// Test with empty config (defaults should apply)
	emptyCfg := &EtcdConfig{}
	SetDefaults_EtcdConfig(emptyCfg) // Strictly, helpers should work even before SetDefaults if they provide ultimate fallback
	if emptyCfg.GetClientPort() != 2379 { t.Error("emptyCfg.GetClientPort failed") }
	if emptyCfg.GetPeerPort() != 2380 { t.Error("emptyCfg.GetPeerPort failed") }
	if emptyCfg.GetDataDir() != "/var/lib/etcd" { t.Error("emptyCfg.GetDataDir failed") }


	// Test with specified values
	customClientPort := 2377
	customPeerPort := 2378
	customDataDir := "/mnt/etcd_data"
	specifiedCfg := &EtcdConfig{
		ClientPort: &customClientPort,
		PeerPort:   &customPeerPort,
		DataDir:    &customDataDir,
	}
	if specifiedCfg.GetClientPort() != customClientPort { t.Error("specifiedCfg.GetClientPort failed") }
	if specifiedCfg.GetPeerPort() != customPeerPort { t.Error("specifiedCfg.GetPeerPort failed") }
	if specifiedCfg.GetDataDir() != customDataDir { t.Error("specifiedCfg.GetDataDir failed") }
}

// Helper functions to get pointers for basic types in tests
func pint(i int) *int { v := i; return &v }
func pstr(s string) *string { return &s }
func pint64(i int64) *int64 { v := i; return &v }
func puint(u uint) *uint { v := u; return &v }
