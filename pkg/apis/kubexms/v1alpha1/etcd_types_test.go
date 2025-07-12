package v1alpha1

import (
	"testing"

	"github.com/mensylisir/kubexm/pkg/util"
	"github.com/stretchr/testify/assert"
)

func TestSetDefaults_EtcdConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    *EtcdConfig
		expected *EtcdConfig // Simplified expected for brevity, focusing on key defaults
	}{
		{
			name:  "nil input",
			input: nil,
		},
		{
			name: "empty config",
			input: &EtcdConfig{},
			expected: &EtcdConfig{
				Type:                        EtcdTypeKubeXMSInternal,
				ClientPort:                  util.IntPtr(2379),
				PeerPort:                    util.IntPtr(2380),
				DataDir:                     util.StrPtr("/var/lib/etcd"),
				ClusterToken:                "kubexm-etcd-default-token",
				ExtraArgs:                   []string{"--logger=zap", "--log-outputs=stderr", "--auto-compaction-mode=periodic", "--peer-client-cert-auth=true", "--peer-auto-tls=false", "--auto-tls=false", "--client-cert-auth=true"}, // Order might vary
				BackupDir:                   util.StrPtr("/var/backups/etcd"),
				BackupPeriodHours:           util.IntPtr(24),
				KeepBackupNumber:            util.IntPtr(7),
				HeartbeatIntervalMillis:     util.IntPtr(250),
				ElectionTimeoutMillis:       util.IntPtr(5000),
				SnapshotCount:               util.Uint64Ptr(10000),
				AutoCompactionRetentionHours: util.IntPtr(8),
				QuotaBackendBytes:           util.Int64Ptr(2147483648),
				MaxRequestBytes:             util.UintPtr(1572864),
				Metrics:                     util.StrPtr("basic"),
				LogLevel:                    util.StrPtr("info"),
				MaxSnapshotsToKeep:          util.UintPtr(5),
				MaxWALsToKeep:               util.UintPtr(5),
			},
		},
		{
			name: "type external, no TLS in external",
			input: &EtcdConfig{
				Type:     EtcdTypeExternal,
				External: &ExternalEtcdConfig{},
			},
			expected: &EtcdConfig{
				Type:     EtcdTypeExternal,
				External: &ExternalEtcdConfig{Endpoints: []string{}}, // Endpoints defaulted to empty slice
				// Other fields get their standard defaults
				ClientPort:                  util.IntPtr(2379),
				PeerPort:                    util.IntPtr(2380),
				DataDir:                     util.StrPtr("/var/lib/etcd"),
				ClusterToken:                "kubexm-etcd-default-token",
				ExtraArgs:                   []string{"--logger=zap", "--log-outputs=stderr", "--auto-compaction-mode=periodic"}, // No TLS args by default if external certs not set
				BackupDir:                   util.StrPtr("/var/backups/etcd"),
				BackupPeriodHours:           util.IntPtr(24),
				KeepBackupNumber:            util.IntPtr(7),
				HeartbeatIntervalMillis:     util.IntPtr(250),
				ElectionTimeoutMillis:       util.IntPtr(5000),
				SnapshotCount:               util.Uint64Ptr(10000),
				AutoCompactionRetentionHours: util.IntPtr(8),
				QuotaBackendBytes:           util.Int64Ptr(2147483648),
				MaxRequestBytes:             util.UintPtr(1572864),
				Metrics:                     util.StrPtr("basic"),
				LogLevel:                    util.StrPtr("info"),
				MaxSnapshotsToKeep:          util.UintPtr(5),
				MaxWALsToKeep:               util.UintPtr(5),
			},
		},
		{
			name: "type external with TLS",
			input: &EtcdConfig{
				Type: EtcdTypeExternal,
				External: &ExternalEtcdConfig{
					CertFile: "cert", KeyFile: "key", CAFile: "ca", // Presence triggers TLS args
				},
			},
			expected: &EtcdConfig{
				Type: EtcdTypeExternal,
				External: &ExternalEtcdConfig{
					Endpoints: []string{}, CertFile: "cert", KeyFile: "key", CAFile: "ca",
				},
				ClientPort:                  util.IntPtr(2379),
				PeerPort:                    util.IntPtr(2380),
				DataDir:                     util.StrPtr("/var/lib/etcd"),
				ClusterToken:                "kubexm-etcd-default-token",
				ExtraArgs:                   []string{"--logger=zap", "--log-outputs=stderr", "--auto-compaction-mode=periodic", "--client-cert-auth=true"}, // Only client-cert-auth for external with TLS
				BackupDir:                   util.StrPtr("/var/backups/etcd"),
				BackupPeriodHours:           util.IntPtr(24),
				KeepBackupNumber:            util.IntPtr(7),
				HeartbeatIntervalMillis:     util.IntPtr(250),
				ElectionTimeoutMillis:       util.IntPtr(5000),
				SnapshotCount:               util.Uint64Ptr(10000),
				AutoCompactionRetentionHours: util.IntPtr(8),
				QuotaBackendBytes:           util.Int64Ptr(2147483648),
				MaxRequestBytes:             util.UintPtr(1572864),
				Metrics:                     util.StrPtr("basic"),
				LogLevel:                    util.StrPtr("info"),
				MaxSnapshotsToKeep:          util.UintPtr(5),
				MaxWALsToKeep:               util.UintPtr(5),
			},
		},
		{
			name: "some fields pre-set with custom ExtraArgs",
			input: &EtcdConfig{
				ClientPort: util.IntPtr(23790),
				ExtraArgs:  []string{"--custom-arg=true", "--logger=logrus"}, // logger will be preserved
			},
			expected: &EtcdConfig{
				Type:                        EtcdTypeKubeXMSInternal,
				ClientPort:                  util.IntPtr(23790),
				PeerPort:                    util.IntPtr(2380),
				DataDir:                     util.StrPtr("/var/lib/etcd"),
				ClusterToken:                "kubexm-etcd-default-token",
				ExtraArgs:                   []string{"--custom-arg=true", "--logger=logrus", "--log-outputs=stderr", "--auto-compaction-mode=periodic", "--peer-client-cert-auth=true", "--peer-auto-tls=false", "--auto-tls=false", "--client-cert-auth=true"}, // Order might vary
				BackupDir:                   util.StrPtr("/var/backups/etcd"),
				BackupPeriodHours:           util.IntPtr(24),
				KeepBackupNumber:            util.IntPtr(7),
				HeartbeatIntervalMillis:     util.IntPtr(250),
				ElectionTimeoutMillis:       util.IntPtr(5000),
				SnapshotCount:               util.Uint64Ptr(10000),
				AutoCompactionRetentionHours: util.IntPtr(8),
				QuotaBackendBytes:           util.Int64Ptr(2147483648),
				MaxRequestBytes:             util.UintPtr(1572864),
				Metrics:                     util.StrPtr("basic"),
				LogLevel:                    util.StrPtr("info"),
				MaxSnapshotsToKeep:          util.UintPtr(5),
				MaxWALsToKeep:               util.UintPtr(5),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetDefaults_EtcdConfig(tt.input)
			if tt.input == nil {
				assert.Nil(t, tt.expected)
				return
			}
			assert.Equal(t, tt.expected.Type, tt.input.Type)
			assert.Equal(t, tt.expected.ClientPort, tt.input.ClientPort)
			assert.Equal(t, tt.expected.PeerPort, tt.input.PeerPort)
			assert.Equal(t, tt.expected.DataDir, tt.input.DataDir)
			assert.Equal(t, tt.expected.ClusterToken, tt.input.ClusterToken)
			// Compare ExtraArgs as sets because order doesn't matter
			assert.ElementsMatch(t, tt.expected.ExtraArgs, tt.input.ExtraArgs, "ExtraArgs do not match for test: %s. Expected: %v, Actual: %v", tt.name, tt.expected.ExtraArgs, tt.input.ExtraArgs)
			assert.Equal(t, tt.expected.External, tt.input.External)
			// Compare other fields
			assert.Equal(t, tt.expected.BackupDir, tt.input.BackupDir)
			assert.Equal(t, tt.expected.BackupPeriodHours, tt.input.BackupPeriodHours)
			assert.Equal(t, tt.expected.KeepBackupNumber, tt.input.KeepBackupNumber)
			assert.Equal(t, tt.expected.HeartbeatIntervalMillis, tt.input.HeartbeatIntervalMillis)
			assert.Equal(t, tt.expected.ElectionTimeoutMillis, tt.input.ElectionTimeoutMillis)
			assert.Equal(t, tt.expected.SnapshotCount, tt.input.SnapshotCount)
			assert.Equal(t, tt.expected.AutoCompactionRetentionHours, tt.input.AutoCompactionRetentionHours)
			assert.Equal(t, tt.expected.QuotaBackendBytes, tt.input.QuotaBackendBytes)
			assert.Equal(t, tt.expected.MaxRequestBytes, tt.input.MaxRequestBytes)
			assert.Equal(t, tt.expected.Metrics, tt.input.Metrics)
			assert.Equal(t, tt.expected.LogLevel, tt.input.LogLevel)
			assert.Equal(t, tt.expected.MaxSnapshotsToKeep, tt.input.MaxSnapshotsToKeep)
			assert.Equal(t, tt.expected.MaxWALsToKeep, tt.input.MaxWALsToKeep)
		})
	}
}

func TestValidate_EtcdConfig(t *testing.T) {
	validMinimalInternal := &EtcdConfig{}
	SetDefaults_EtcdConfig(validMinimalInternal) // Apply defaults to make it valid

	tests := []struct {
		name        string
		input       *EtcdConfig
		expectError bool
		errorMsg    string
	}{
		{
			name:        "Valid minimal valid internal etcd (after defaults)",
			input:       validMinimalInternal,
			expectError: false,
		},
		{
			name: "Valid external etcd",
			input: &EtcdConfig{
				Type: EtcdTypeExternal,
				External: &ExternalEtcdConfig{
					Endpoints: []string{"https://etcd1:2379"},
					CAFile:    "ca.crt", CertFile: "cert.crt", KeyFile: "key.pem",
				},
				ClusterToken: "token", // Still required even if external
			},
			expectError: false,
		},
		{
			name: "Valid internal etcd with all fields",
			input: &EtcdConfig{
				Type:                        EtcdTypeKubeXMSInternal,
				Version:                     "v3.5.0",
				Arch:                        "amd64",
				ClientPort:                  util.IntPtr(2379),
				PeerPort:                    util.IntPtr(2380),
				DataDir:                     util.StrPtr("/var/lib/myetcd"),
				ClusterToken:                "my-secure-token",
				ExtraArgs:                   []string{"--debug"},
				BackupDir:                   util.StrPtr("/backup/etcd"),
				BackupPeriodHours:           util.IntPtr(12),
				KeepBackupNumber:            util.IntPtr(5),
				BackupScriptPath:            util.StrPtr("/opt/backup.sh"),
				HeartbeatIntervalMillis:     util.IntPtr(100),
				ElectionTimeoutMillis:       util.IntPtr(1000), // 10x heartbeat
				SnapshotCount:               util.Uint64Ptr(5000),
				AutoCompactionRetentionHours: util.IntPtr(1),
				QuotaBackendBytes:           util.Int64Ptr(4 * 1024 * 1024 * 1024), // 4GB
				MaxRequestBytes:             util.UintPtr(2 * 1024 * 1024),      // 2MB
				Metrics:                     util.StrPtr("extensive"),
				LogLevel:                    util.StrPtr("debug"),
				MaxSnapshotsToKeep:          util.UintPtr(10),
				MaxWALsToKeep:               util.UintPtr(10),
			},
			expectError: false,
		},
		{
			name:        "Invalid invalid type",
			input:       &EtcdConfig{Type: "invalid", ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".type: invalid type 'invalid'",
		},
		{
			name:        "Invalid external no config struct (becomes no endpoints after defaulting)",
			input:       &EtcdConfig{Type: EtcdTypeExternal, ClusterToken: "token"}, // External is nil
			expectError: true,
			errorMsg:    ".external.endpoints: must contain at least one endpoint if etcd.type is 'external'",
		},
		{
			name:        "Invalid external no endpoints",
			input:       &EtcdConfig{Type: EtcdTypeExternal, External: &ExternalEtcdConfig{}, ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".external.endpoints: must contain at least one endpoint",
		},
		{
			name:        "Invalid external empty endpoint",
			input:       &EtcdConfig{Type: EtcdTypeExternal, External: &ExternalEtcdConfig{Endpoints: []string{" "}}, ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".external.endpoints[0]: endpoint cannot be empty",
		},
		{
			name:        "Invalid external invalid endpoint url",
			input:       &EtcdConfig{Type: EtcdTypeExternal, External: &ExternalEtcdConfig{Endpoints: []string{"://bad-url"}}, ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".external.endpoints[0]: invalid URL format for endpoint '://bad-url'",
		},
		{
			name:        "Invalid external invalid endpoint scheme",
			input:       &EtcdConfig{Type: EtcdTypeExternal, External: &ExternalEtcdConfig{Endpoints: []string{"ftp://etcd:2379"}}, ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".external.endpoints[0]: URL scheme for endpoint 'ftp://etcd:2379' must be http or https",
		},
		{
			name:        "Invalid external mismatched tls cert",
			input:       &EtcdConfig{Type: EtcdTypeExternal, External: &ExternalEtcdConfig{Endpoints: []string{"https://e:2379"}, CertFile: "cert.pem"}, ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".external: certFile and keyFile must be specified together",
		},
		{
			name:        "Invalid external mismatched tls key",
			input:       &EtcdConfig{Type: EtcdTypeExternal, External: &ExternalEtcdConfig{Endpoints: []string{"https://e:2379"}, KeyFile: "key.pem"}, ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".external: certFile and keyFile must be specified together",
		},
		{
			name:        "Invalid invalid client port low",
			input:       &EtcdConfig{ClientPort: util.IntPtr(0), ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".clientPort: invalid port 0",
		},
		{
			name:        "Invalid invalid client port high",
			input:       &EtcdConfig{ClientPort: util.IntPtr(65536), ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".clientPort: invalid port 65536",
		},
		{
			name:        "Invalid invalid peer port",
			input:       &EtcdConfig{PeerPort: util.IntPtr(0), ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".peerPort: invalid port 0",
		},
		{
			name:        "Invalid empty datadir",
			input:       &EtcdConfig{DataDir: util.StrPtr(" "), ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".dataDir: cannot be empty if specified",
		},
		{
			name:        "Invalid empty clustertoken",
			input:       &EtcdConfig{ClusterToken: " "},
			expectError: true,
			errorMsg:    ".clusterToken: cannot be empty",
		},
		{
			name:        "Invalid negative backup period",
			input:       &EtcdConfig{BackupPeriodHours: util.IntPtr(-1), ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".backupPeriodHours: cannot be negative",
		},
		{
			name:        "Invalid negative keep backup",
			input:       &EtcdConfig{KeepBackupNumber: util.IntPtr(-1), ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".keepBackupNumber: cannot be negative",
		},
		{
			name:        "Invalid zero heartbeat",
			input:       &EtcdConfig{HeartbeatIntervalMillis: util.IntPtr(0), ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".heartbeatIntervalMillis: must be positive",
		},
		{
			name:        "Invalid zero election timeout",
			input:       &EtcdConfig{ElectionTimeoutMillis: util.IntPtr(0), ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".electionTimeoutMillis: must be positive",
		},
		{
			name: "Invalid election timeout not greater than heartbeat",
			input: &EtcdConfig{
				HeartbeatIntervalMillis: util.IntPtr(1000),
				ElectionTimeoutMillis:   util.IntPtr(1000), // Not > 5*heartbeat
				ClusterToken:            "token",
			},
			expectError: true,
			errorMsg:    "electionTimeoutMillis (1000) should be significantly greater than heartbeatIntervalMillis (1000)",
		},
		{
			name: "Invalid election timeout equal to 5x heartbeat (edge case, should fail)",
			input: &EtcdConfig{
				HeartbeatIntervalMillis: util.IntPtr(200),
				ElectionTimeoutMillis:   util.IntPtr(1000), // Exactly 5x, should fail
				ClusterToken:            "token",
			},
			expectError: true,
			errorMsg:    "electionTimeoutMillis (1000) should be significantly greater than heartbeatIntervalMillis (200)",
		},

		{
			name:        "Invalid negative autocompaction",
			input:       &EtcdConfig{AutoCompactionRetentionHours: util.IntPtr(-1), ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".autoCompactionRetentionHours: cannot be negative",
		},
		{
			name:        "Invalid negative quota",
			input:       &EtcdConfig{QuotaBackendBytes: util.Int64Ptr(-1), ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".quotaBackendBytes: cannot be negative",
		},
		{
			name:        "Invalid zero max request bytes",
			input:       &EtcdConfig{MaxRequestBytes: util.UintPtr(0), ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".maxRequestBytes: must be positive if set",
		},
		{
			name:        "Invalid invalid metrics",
			input:       &EtcdConfig{Metrics: util.StrPtr("none"), ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".metrics: invalid value 'none'",
		},
		{
			name:        "Invalid invalid loglevel",
			input:       &EtcdConfig{LogLevel: util.StrPtr("trace"), ClusterToken: "token"},
			expectError: true,
			errorMsg:    ".logLevel: invalid value 'trace'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputToTest := tt.input
			if inputToTest != nil && tt.name != "Invalid external no config struct (becomes no endpoints after defaulting)" {
				// For most tests, apply defaults as validation runs on defaulted structs.
				// The specific test "Invalid external no config struct" needs External to be nil before SetDefaults.
				SetDefaults_EtcdConfig(inputToTest)
			} else if tt.name == "Invalid external no config struct (becomes no endpoints after defaulting)" && inputToTest != nil {
				// For this specific case, we want External to be nil, then call SetDefaults
				inputToTest.External = nil
				SetDefaults_EtcdConfig(inputToTest)
			}


			verrs := &ValidationErrors{}
			Validate_EtcdConfig(inputToTest, verrs, "spec.etcd")
			if tt.expectError {
				assert.True(t, verrs.HasErrors(), "Expected error for test '%s', but got none. Input: %+v, DefaultedOrOriginal: %+v", tt.name, tt.input, inputToTest)
				if tt.errorMsg != "" {
					assert.Contains(t, verrs.Error(), tt.errorMsg, "Error message for test '%s' does not contain '%s'. Full error: %s", tt.name, tt.errorMsg, verrs.Error())
				}
			} else {
				assert.False(t, verrs.HasErrors(), "Unexpected error for test '%s': %s. Input: %+v, DefaultedOrOriginal: %+v", tt.name, verrs.Error(), tt.input, inputToTest)
			}
		})
	}
}

func TestEtcdConfig_GetPortsAndDataDir(t *testing.T) {
	tests := []struct {
		name                string
		cfg                 *EtcdConfig
		expectedClientPort  int
		expectedPeerPort    int
		expectedDataDir     string
	}{
		{
			name:               "nil config",
			cfg:                nil,
			expectedClientPort: 2379,
			expectedPeerPort:   2380,
			expectedDataDir:    "/var/lib/etcd",
		},
		{
			name: "empty config after defaults",
			cfg: func() *EtcdConfig {
				c := &EtcdConfig{}
				SetDefaults_EtcdConfig(c)
				return c
			}(),
			expectedClientPort: 2379,
			expectedPeerPort:   2380,
			expectedDataDir:    "/var/lib/etcd",
		},
		{
			name: "config with specified values",
			cfg: &EtcdConfig{
				ClientPort: util.IntPtr(12379),
				PeerPort:   util.IntPtr(12380),
				DataDir:    util.StrPtr("/my/etcd/data"),
			},
			expectedClientPort: 12379,
			expectedPeerPort:   12380,
			expectedDataDir:    "/my/etcd/data",
		},
		{
			name: "config with some nil fields (should use getter defaults)",
			cfg: &EtcdConfig{
				ClientPort: util.IntPtr(2379), // PeerPort and DataDir are nil
			},
			expectedClientPort: 2379,
			expectedPeerPort:   2380, // Default from getter
			expectedDataDir:    "/var/lib/etcd", // Default from getter
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedClientPort, tt.cfg.GetClientPort())
			assert.Equal(t, tt.expectedPeerPort, tt.cfg.GetPeerPort())
			assert.Equal(t, tt.expectedDataDir, tt.cfg.GetDataDir())
		})
	}
}
