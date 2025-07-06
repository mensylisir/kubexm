package v1alpha1

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/mensylisir/kubexm/pkg/util/validation"
)

// Helper functions (intPtr, stringPtr, etc.) are expected to be in zz_helpers.go

func TestSetDefaults_EtcdConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    *EtcdConfig
		expected *EtcdConfig
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:  "empty config",
			input: &EtcdConfig{},
			expected: &EtcdConfig{
				Type:                         EtcdTypeKubeXMSInternal,
				ClientPort:                   intPtr(2379),
				PeerPort:                     intPtr(2380),
				DataDir:                      stringPtr("/var/lib/etcd"),
				ClusterToken:                 "kubexm-etcd-default-token",
				External:                     nil,
				ExtraArgs:                    []string{},
				BackupDir:                    stringPtr("/var/backups/etcd"),
				BackupPeriodHours:            intPtr(24),
				KeepBackupNumber:             intPtr(7),
				HeartbeatIntervalMillis:      intPtr(250),
				ElectionTimeoutMillis:        intPtr(5000),
				SnapshotCount:                uint64Ptr(10000),
				AutoCompactionRetentionHours: intPtr(8),
				QuotaBackendBytes:            int64Ptr(2147483648),
				MaxRequestBytes:              uintPtr(1572864),
				Metrics:                      stringPtr("basic"),
				LogLevel:                     stringPtr("info"),
				MaxSnapshotsToKeep:           uintPtr(5),
				MaxWALsToKeep:                uintPtr(5),
			},
		},
		{
			name: "type external, external config initialized",
			input: &EtcdConfig{Type: EtcdTypeExternal},
			expected: &EtcdConfig{
				Type:                         EtcdTypeExternal,
				ClientPort:                   intPtr(2379),
				PeerPort:                     intPtr(2380),
				DataDir:                      stringPtr("/var/lib/etcd"),
				ClusterToken:                 "kubexm-etcd-default-token",
				External:                     &ExternalEtcdConfig{Endpoints: []string{}},
				ExtraArgs:                    []string{},
				BackupDir:                    stringPtr("/var/backups/etcd"),
				BackupPeriodHours:            intPtr(24),
				KeepBackupNumber:             intPtr(7),
				HeartbeatIntervalMillis:      intPtr(250),
				ElectionTimeoutMillis:        intPtr(5000),
				SnapshotCount:                uint64Ptr(10000),
				AutoCompactionRetentionHours: intPtr(8),
				QuotaBackendBytes:            int64Ptr(2147483648),
				MaxRequestBytes:              uintPtr(1572864),
				Metrics:                      stringPtr("basic"),
				LogLevel:                     stringPtr("info"),
				MaxSnapshotsToKeep:           uintPtr(5),
				MaxWALsToKeep:                uintPtr(5),
			},
		},
		{
			name: "some fields pre-set",
			input: &EtcdConfig{
				ClientPort: intPtr(3379),
				DataDir:    stringPtr("/mnt/myetcd"),
				LogLevel:   stringPtr("debug"),
			},
			expected: &EtcdConfig{
				Type:                         EtcdTypeKubeXMSInternal,
				ClientPort:                   intPtr(3379),
				PeerPort:                     intPtr(2380),
				DataDir:                      stringPtr("/mnt/myetcd"),
				ClusterToken:                 "kubexm-etcd-default-token",
				External:                     nil,
				ExtraArgs:                    []string{},
				BackupDir:                    stringPtr("/var/backups/etcd"),
				BackupPeriodHours:            intPtr(24),
				KeepBackupNumber:             intPtr(7),
				HeartbeatIntervalMillis:      intPtr(250),
				ElectionTimeoutMillis:        intPtr(5000),
				SnapshotCount:                uint64Ptr(10000),
				AutoCompactionRetentionHours: intPtr(8),
				QuotaBackendBytes:            int64Ptr(2147483648),
				MaxRequestBytes:              uintPtr(1572864),
				Metrics:                      stringPtr("basic"),
				LogLevel:                     stringPtr("debug"),
				MaxSnapshotsToKeep:           uintPtr(5),
				MaxWALsToKeep:                uintPtr(5),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_EtcdConfig(tt.input)
			if !reflect.DeepEqual(tt.input, tt.expected) {
				assert.Equal(t, tt.expected, tt.input)
			}
		})
	}
}

func TestValidate_EtcdConfig(t *testing.T) {
	validCases := []struct {
		name  string
		input *EtcdConfig
	}{
		{
			name:  "minimal valid internal etcd (after defaults)",
			input: &EtcdConfig{Type: EtcdTypeKubeXMSInternal},
		},
		{
			name: "valid external etcd",
			input: &EtcdConfig{
				Type:         EtcdTypeExternal,
				External:     &ExternalEtcdConfig{Endpoints: []string{"http://etcd1:2379", "http://etcd2:2379"}},
				ClusterToken: "some-token",
			},
		},
		{
			name: "valid internal etcd with all fields",
			input: &EtcdConfig{
				Type:                         EtcdTypeKubeXMSInternal,
				Version:                      "v3.5.0",
				Arch:                         "arm64",
				ClientPort:                   intPtr(2379),
				PeerPort:                     intPtr(2380),
				DataDir:                      stringPtr("/var/lib/etcd-data"),
				ClusterToken:                 "securetoken",
				ExtraArgs:                    []string{"--debug"},
				BackupDir:                    stringPtr("/backups/etcd"),
				BackupPeriodHours:            intPtr(12),
				KeepBackupNumber:             intPtr(10),
				BackupScriptPath:             stringPtr("/usr/local/bin/backup-etcd.sh"),
				HeartbeatIntervalMillis:      intPtr(100),
				ElectionTimeoutMillis:        intPtr(1000),
				SnapshotCount:                uint64Ptr(5000),
				AutoCompactionRetentionHours: intPtr(1),
				QuotaBackendBytes:            int64Ptr(4 * 1024 * 1024 * 1024),
				MaxRequestBytes:              uintPtr(2 * 1024 * 1024),
				Metrics:                      stringPtr("extensive"),
				LogLevel:                     stringPtr("debug"),
				MaxSnapshotsToKeep:           uintPtr(10),
				MaxWALsToKeep:                uintPtr(10),
			},
		},
	}

	for _, tt := range validCases {
		t.Run("Valid_"+tt.name, func(t *testing.T) {
			SetDefaults_EtcdConfig(tt.input)
			verrs := &validation.ValidationErrors{}
			Validate_EtcdConfig(tt.input, verrs, "spec.etcd")
			assert.False(t, verrs.HasErrors(), "Expected no validation errors for '%s', but got: %s", tt.name, verrs.Error())
		})
	}

	invalidCases := []struct {
		name        string
		cfgBuilder  func() *EtcdConfig
		errContains []string
	}{
		{"invalid type", func() *EtcdConfig { return &EtcdConfig{Type: "invalid-type"} }, []string{"invalid type 'invalid-type'"}},
		{"external_no_config_struct_becomes_no_endpoints_after_defaulting", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeExternal, External: nil} }, []string{".external.endpoints: must contain at least one endpoint"}},
		{"external_no_endpoints", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeExternal, External: &ExternalEtcdConfig{Endpoints: []string{}}} }, []string{".external.endpoints: must contain at least one endpoint"}},
		{"external_empty_endpoint", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeExternal, External: &ExternalEtcdConfig{Endpoints: []string{""}}} }, []string{".external.endpoints[0]: endpoint cannot be empty"}},
		{"external_invalid_endpoint_url", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeExternal, External: &ExternalEtcdConfig{Endpoints: []string{"http://invalid domain/"}}} }, []string{".external.endpoints[0]: invalid URL format for endpoint"}},
		{
			"external_invalid_endpoint_scheme",
			func() *EtcdConfig {
				return &EtcdConfig{Type: EtcdTypeExternal, External: &ExternalEtcdConfig{Endpoints: []string{"ftp://etcd.example.com:2379"}}}
			},
			[]string{".external.endpoints[0]: URL scheme for endpoint 'ftp://etcd.example.com:2379' must be http or https"},
		},
		{"external_mismatched_tls_cert", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeExternal, External: &ExternalEtcdConfig{Endpoints: []string{"http://valid.com"}, CertFile: "cert"}} }, []string{"certFile and keyFile must be specified together"}},
		{"external_mismatched_tls_key", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeExternal, External: &ExternalEtcdConfig{Endpoints: []string{"http://valid.com"}, KeyFile: "key"}} }, []string{"certFile and keyFile must be specified together"}},
		{"invalid_client_port_low", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeKubeXMSInternal, ClientPort: intPtr(0)} }, []string{".clientPort: invalid port 0"}},
		{"invalid_client_port_high", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeKubeXMSInternal, ClientPort: intPtr(70000)} }, []string{".clientPort: invalid port 70000"}},
		{"invalid_peer_port", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeKubeXMSInternal, PeerPort: intPtr(0)} }, []string{".peerPort: invalid port 0"}},
		{"empty_datadir", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeKubeXMSInternal, DataDir: stringPtr(" ")} }, []string{".dataDir: cannot be empty if specified"}},
		{"empty_clustertoken", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeKubeXMSInternal, ClusterToken: " "} }, []string{".clusterToken: cannot be empty"}},
		{"negative_backup_period", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeKubeXMSInternal, BackupPeriodHours: intPtr(-1)} }, []string{".backupPeriodHours: cannot be negative"}},
		{"negative_keep_backup", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeKubeXMSInternal, KeepBackupNumber: intPtr(-1)} }, []string{".keepBackupNumber: cannot be negative"}},
		{"zero_heartbeat", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeKubeXMSInternal, HeartbeatIntervalMillis: intPtr(0)} }, []string{".heartbeatIntervalMillis: must be positive"}},
		{"zero_election_timeout", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeKubeXMSInternal, ElectionTimeoutMillis: intPtr(0)} }, []string{".electionTimeoutMillis: must be positive"}},
		{
			"election_timeout_not_greater_than_heartbeat",
			func() *EtcdConfig {
				return &EtcdConfig{Type: EtcdTypeKubeXMSInternal, HeartbeatIntervalMillis: intPtr(100), ElectionTimeoutMillis: intPtr(500)}
			},
			[]string{"electionTimeoutMillis (500) should be significantly greater than heartbeatIntervalMillis (100)"},
		},
		{
			"election_timeout_equal_to_5x_heartbeat (edge case, should fail)",
			func() *EtcdConfig {
				return &EtcdConfig{Type: EtcdTypeKubeXMSInternal, HeartbeatIntervalMillis: intPtr(100), ElectionTimeoutMillis: intPtr(500)}
			},
			[]string{"electionTimeoutMillis (500) should be significantly greater than heartbeatIntervalMillis (100)"},
		},
		{"negative_autocompaction", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeKubeXMSInternal, AutoCompactionRetentionHours: intPtr(-1)} }, []string{".autoCompactionRetentionHours: cannot be negative"}},
		{"negative_quota", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeKubeXMSInternal, QuotaBackendBytes: int64Ptr(-100)} }, []string{".quotaBackendBytes: cannot be negative"}},
		{"zero_max_request_bytes", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeKubeXMSInternal, MaxRequestBytes: uintPtr(0)} }, []string{".maxRequestBytes: must be positive if set"}},
		{"invalid_metrics", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeKubeXMSInternal, Metrics: stringPtr("detailed")} }, []string{".metrics: invalid value 'detailed'"}},
		{"invalid_loglevel", func() *EtcdConfig { return &EtcdConfig{Type: EtcdTypeKubeXMSInternal, LogLevel: stringPtr("trace")} }, []string{".logLevel: invalid value 'trace'"}},
	}

	for _, tt := range invalidCases {
		t.Run("Invalid_"+tt.name, func(t *testing.T) {
			cfg := tt.cfgBuilder()
			SetDefaults_EtcdConfig(cfg)
			verrs := &validation.ValidationErrors{}
			Validate_EtcdConfig(cfg, verrs, "spec.etcd")
			assert.True(t, verrs.HasErrors(), "Expected validation errors for '%s', but got none", tt.name)
			if len(tt.errContains) > 0 {
				fullError := verrs.Error()
				for _, errStr := range tt.errContains {
					assert.Contains(t, fullError, errStr, "Error message for '%s' does not contain expected substring '%s'. Full error: %s", tt.name, errStr, fullError)
				}
			}
		})
	}
}

func TestEtcdConfig_GetPortsAndDataDir(t *testing.T) {
	t.Run("nil config", func(t *testing.T) {
		var nilCfg *EtcdConfig
		assert.Equal(t, 2379, nilCfg.GetClientPort())
		assert.Equal(t, 2380, nilCfg.GetPeerPort())
		assert.Equal(t, "/var/lib/etcd", nilCfg.GetDataDir())
	})

	t.Run("empty config after defaults", func(t *testing.T) {
		emptyCfg := &EtcdConfig{}
		SetDefaults_EtcdConfig(emptyCfg)
		assert.Equal(t, 2379, emptyCfg.GetClientPort())
		assert.Equal(t, 2380, emptyCfg.GetPeerPort())
		assert.Equal(t, "/var/lib/etcd", emptyCfg.GetDataDir())
	})

	t.Run("config with specified values", func(t *testing.T) {
		customClientPort := 2377
		customPeerPort := 2378
		customDataDir := "/mnt/etcd_data"
		specifiedCfg := &EtcdConfig{
			ClientPort: intPtr(customClientPort),
			PeerPort:   intPtr(customPeerPort),
			DataDir:    stringPtr(customDataDir),
		}
		assert.Equal(t, customClientPort, specifiedCfg.GetClientPort())
		assert.Equal(t, customPeerPort, specifiedCfg.GetPeerPort())
		assert.Equal(t, customDataDir, specifiedCfg.GetDataDir())
	})

	t.Run("config with some nil fields (should use getter defaults)", func(t *testing.T) {
		partialCfg := &EtcdConfig{
			ClientPort: nil,
			PeerPort: intPtr(12345),
			DataDir: nil,
		}
		assert.Equal(t, 2379, partialCfg.GetClientPort(), "Client port should fallback to getter's default")
		assert.Equal(t, 12345, partialCfg.GetPeerPort())
		assert.Equal(t, "/var/lib/etcd", partialCfg.GetDataDir(), "DataDir should fallback to getter's default")
	})
}
