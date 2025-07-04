package v1alpha1

import (
	"net/url"
	"strings"
)

const (
	EtcdTypeKubeXMSInternal = "kubexm"   // 表示要使用二进制部署etcd
	EtcdTypeExternal        = "external" // 表示外部已经有现成的etcd
	EtcdTypeInternal        = "kubeadm"  // 表示kubeadm部署etcd,即etcd是以静态pod的形式启动的
)

// EtcdConfig defines the configuration for the Etcd cluster.
type EtcdConfig struct {
	Type                string              `json:"type,omitempty" yaml:"type,omitempty"`    // "stacked" or "external"
	Version             string              `json:"version,omitempty" yaml:"version,omitempty"` // Etcd version for managed setup
	Arch                string              `json:"arch,omitempty" yaml:"arch,omitempty"`       // Architecture for etcd binaries
	External            *ExternalEtcdConfig `json:"external,omitempty" yaml:"external,omitempty"`// Config for external etcd

	ClientPort          *int                `json:"clientPort,omitempty" yaml:"clientPort,omitempty"` // Default: 2379
	PeerPort            *int                `json:"peerPort,omitempty" yaml:"peerPort,omitempty"`   // Default: 2380
	DataDir             *string             `json:"dataDir,omitempty" yaml:"dataDir,omitempty"`    // Default: "/var/lib/etcd". This is the main data directory.
	ClusterToken        string              `json:"clusterToken,omitempty" yaml:"clusterToken,omitempty"` // Token for etcd cluster initialization

	// ExtraArgs for etcd process, as a list of strings (e.g., "--initial-cluster-token=mytoken").
	ExtraArgs           []string            `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`

	// Backup configuration
	BackupDir           *string `json:"backupDir,omitempty" yaml:"backupDir,omitempty"`
	BackupPeriodHours   *int    `json:"backupPeriodHours,omitempty" yaml:"backupPeriodHours,omitempty"`
	KeepBackupNumber    *int    `json:"keepBackupNumber,omitempty" yaml:"keepBackupNumber,omitempty"`
	BackupScriptPath    *string `json:"backupScriptPath,omitempty" yaml:"backupScriptPath,omitempty"`

	// Performance and tuning - tags match YAML fields
	HeartbeatIntervalMillis      *int    `json:"heartbeatIntervalMillis,omitempty" yaml:"heartbeatInterval,omitempty"` // YAML: heartbeatInterval
	ElectionTimeoutMillis        *int    `json:"electionTimeoutMillis,omitempty" yaml:"electionTimeout,omitempty"`   // YAML: electionTimeout
	SnapshotCount                *uint64 `json:"snapshotCount,omitempty" yaml:"snapshotCount,omitempty"`             // YAML: snapshotCount
	AutoCompactionRetentionHours *int    `json:"autoCompactionRetentionHours,omitempty" yaml:"autoCompactionRetention,omitempty"` // YAML: autoCompactionRetention

	// Resource management
	QuotaBackendBytes *int64 `json:"quotaBackendBytes,omitempty" yaml:"quotaBackendBytes,omitempty"` // YAML: quotaBackendBytes
	MaxRequestBytes   *uint  `json:"maxRequestBytes,omitempty" yaml:"maxRequestBytes,omitempty"`     // YAML: maxRequestBytes

	// Operational settings
	Metrics            *string `json:"metrics,omitempty" yaml:"metrics,omitempty"`                         // YAML: metrics
	LogLevel           *string `json:"logLevel,omitempty" yaml:"logLevel,omitempty"`                       // YAML: logLevel
	MaxSnapshotsToKeep *uint   `json:"maxSnapshotsToKeep,omitempty" yaml:"maxSnapshots,omitempty"`         // YAML: maxSnapshots
	MaxWALsToKeep      *uint   `json:"maxWALsToKeep,omitempty" yaml:"maxWals,omitempty"`                   // YAML: maxWals
}

// ExternalEtcdConfig describes how to connect to an external etcd cluster.
// Corresponds to etcd.external in YAML.
type ExternalEtcdConfig struct {
	Endpoints []string `json:"endpoints" yaml:"endpoints"`
	CAFile    string   `json:"caFile,omitempty" yaml:"caFile,omitempty"`
	CertFile  string   `json:"certFile,omitempty" yaml:"certFile,omitempty"`
	KeyFile   string   `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`
}

// SetDefaults_EtcdConfig sets default values for EtcdConfig.
func SetDefaults_EtcdConfig(cfg *EtcdConfig) {
	if cfg == nil {
		return
	}
	if cfg.Type == "" {
		cfg.Type = EtcdTypeKubeXMSInternal // Default to KubeXM deploying etcd as binaries
	}
	if cfg.ClientPort == nil {
		cfg.ClientPort = intPtr(2379)
	}
	if cfg.PeerPort == nil {
		cfg.PeerPort = intPtr(2380)
	}
	if cfg.DataDir == nil {
		cfg.DataDir = stringPtr("/var/lib/etcd") // This is the base directory for etcd data.
	}
	// Arch defaults handled by HostSpec or runtime fact gathering.
	if cfg.ClusterToken == "" {
		cfg.ClusterToken = "kubexm-etcd-default-token" // Default token
	}
	if cfg.Type == EtcdTypeExternal && cfg.External == nil {
		cfg.External = &ExternalEtcdConfig{}
	}
	if cfg.External != nil && cfg.External.Endpoints == nil {
		cfg.External.Endpoints = []string{}
	}
	if cfg.ExtraArgs == nil { cfg.ExtraArgs = []string{} }

	// Default backup settings (can be adjusted)
	if cfg.BackupDir == nil { cfg.BackupDir = stringPtr("/var/backups/etcd") }
	if cfg.BackupPeriodHours == nil { cfg.BackupPeriodHours = intPtr(24) } // e.g., daily
	if cfg.KeepBackupNumber == nil { cfg.KeepBackupNumber = intPtr(7) }

	// Default performance/tuning (values from etcd defaults or common practice)
	if cfg.HeartbeatIntervalMillis == nil { cfg.HeartbeatIntervalMillis = intPtr(250) } // YAML: heartbeatInterval: 250
	if cfg.ElectionTimeoutMillis == nil { cfg.ElectionTimeoutMillis = intPtr(5000) } // YAML: electionTimeout: 5000
	if cfg.SnapshotCount == nil { cfg.SnapshotCount = uint64Ptr(10000) } // YAML: snapshotCount: 10000
	if cfg.AutoCompactionRetentionHours == nil { cfg.AutoCompactionRetentionHours = intPtr(8) } // YAML: autoCompactionRetention: 8

	// Resource management defaults
	if cfg.QuotaBackendBytes == nil { cfg.QuotaBackendBytes = int64Ptr(2147483648) } // YAML: quotaBackendBytes: 2147483648 (2GB)
	if cfg.MaxRequestBytes == nil { cfg.MaxRequestBytes = uintPtr(1572864) } // YAML: maxRequestBytes: 1572864 (1.5MB)

	// Operational defaults
	if cfg.Metrics == nil { cfg.Metrics = stringPtr("basic") } // YAML: metrics: basic
	if cfg.LogLevel == nil { cfg.LogLevel = stringPtr("info") }
	if cfg.MaxSnapshotsToKeep == nil { cfg.MaxSnapshotsToKeep = uintPtr(5) } // etcd default
	if cfg.MaxWALsToKeep == nil { cfg.MaxWALsToKeep = uintPtr(5) }          // etcd default
}

// Validate_EtcdConfig validates EtcdConfig.
func Validate_EtcdConfig(cfg *EtcdConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	validTypes := []string{EtcdTypeKubeXMSInternal, EtcdTypeExternal, EtcdTypeInternal}
	isValidType := false
	for _, vt := range validTypes {
		if cfg.Type == vt {
			isValidType = true
			break
		}
	}
	if !isValidType {
		verrs.Add("%s.type: invalid type '%s', must be one of %v", pathPrefix, cfg.Type, validTypes)
	}
	if cfg.Type == EtcdTypeExternal {
		if cfg.External == nil {
			verrs.Add("%s.external: must be defined if etcd.type is '%s'", pathPrefix, EtcdTypeExternal)
		} else {
			if len(cfg.External.Endpoints) == 0 {
				verrs.Add("%s.external.endpoints: must contain at least one endpoint if etcd.type is '%s'", pathPrefix, EtcdTypeExternal)
			}
			for i, ep := range cfg.External.Endpoints {
				if strings.TrimSpace(ep) == "" {
					verrs.Add("%s.external.endpoints[%d]: endpoint cannot be empty", pathPrefix, i)
				} else {
					// Validate endpoint URL format
				u, err := url.ParseRequestURI(ep)
					if err != nil {
						verrs.Add("%s.external.endpoints[%d]: invalid URL format for endpoint '%s': %v", pathPrefix, i, ep, err)
				} else if u.Scheme != "http" && u.Scheme != "https" {
					verrs.Add("%s.external.endpoints[%d]: URL scheme for endpoint '%s' must be http or https, got '%s'", pathPrefix, i, ep, u.Scheme)
					}
				}
			}
			if (cfg.External.CertFile != "" && cfg.External.KeyFile == "") || (cfg.External.CertFile == "" && cfg.External.KeyFile != "") {
				verrs.Add("%s.external: certFile and keyFile must be specified together for mTLS", pathPrefix)
			}
		}
	}
	if cfg.ClientPort != nil && (*cfg.ClientPort <= 0 || *cfg.ClientPort > 65535) {
	   verrs.Add("%s.clientPort: invalid port %d, must be between 1-65535", pathPrefix, *cfg.ClientPort)
	}
	if cfg.PeerPort != nil && (*cfg.PeerPort <= 0 || *cfg.PeerPort > 65535) {
	   verrs.Add("%s.peerPort: invalid port %d, must be between 1-65535", pathPrefix, *cfg.PeerPort)
	}
	if cfg.DataDir != nil && strings.TrimSpace(*cfg.DataDir) == "" {
		verrs.Add("%s.dataDir: cannot be empty if specified", pathPrefix)
	}
	if strings.TrimSpace(cfg.ClusterToken) == "" {
		verrs.Add("%s.clusterToken: cannot be empty", pathPrefix)
	}
	// Arch validation (e.g., amd64, arm64) could be added if strict values are known.
	// For now, assume any non-empty string is fine, or it's validated by resource handle.

	if cfg.BackupPeriodHours != nil && *cfg.BackupPeriodHours < 0 {
		verrs.Add("%s.backupPeriodHours: cannot be negative, got %d", pathPrefix, *cfg.BackupPeriodHours)
	}
	if cfg.KeepBackupNumber != nil && *cfg.KeepBackupNumber < 0 {
		verrs.Add("%s.keepBackupNumber: cannot be negative, got %d", pathPrefix, *cfg.KeepBackupNumber)
	}
	if cfg.HeartbeatIntervalMillis != nil && *cfg.HeartbeatIntervalMillis <= 0 {
		verrs.Add("%s.heartbeatIntervalMillis: must be positive, got %d", pathPrefix, *cfg.HeartbeatIntervalMillis)
	}
	if cfg.ElectionTimeoutMillis != nil && *cfg.ElectionTimeoutMillis <= 0 {
		verrs.Add("%s.electionTimeoutMillis: must be positive, got %d", pathPrefix, *cfg.ElectionTimeoutMillis)
	}

	if cfg.HeartbeatIntervalMillis != nil && cfg.ElectionTimeoutMillis != nil &&
		*cfg.ElectionTimeoutMillis <= (*cfg.HeartbeatIntervalMillis*5) { // Typical recommendation: ElectionTimeout should be at least 5x HeartbeatInterval
		verrs.Add("%s: electionTimeoutMillis (%d) should be significantly greater than heartbeatIntervalMillis (%d) (e.g., >= 5x)",
			pathPrefix, *cfg.ElectionTimeoutMillis, *cfg.HeartbeatIntervalMillis)
	}

	// SnapshotCount is uint64, typically positive or etcd default.
	if cfg.AutoCompactionRetentionHours != nil && *cfg.AutoCompactionRetentionHours < 0 {
		verrs.Add("%s.autoCompactionRetentionHours: cannot be negative, got %d", pathPrefix, *cfg.AutoCompactionRetentionHours)
	}
	if cfg.QuotaBackendBytes != nil && *cfg.QuotaBackendBytes < 0 { // 0 means default/no quota by some tools
		verrs.Add("%s.quotaBackendBytes: cannot be negative, got %d", pathPrefix, *cfg.QuotaBackendBytes)
	}
	if cfg.MaxRequestBytes != nil && *cfg.MaxRequestBytes == 0 { // MaxRequestBytes is uint. 0 is generally invalid.
		verrs.Add("%s.maxRequestBytes: must be positive if set, got %d", pathPrefix, *cfg.MaxRequestBytes)
	}

	if cfg.Metrics != nil && *cfg.Metrics != "" { // Allow empty for etcd default
		validMetrics := []string{"basic", "extensive"}
		if !containsString(validMetrics, *cfg.Metrics) {
			verrs.Add("%s.metrics: invalid value '%s', must be 'basic' or 'extensive'", pathPrefix, *cfg.Metrics)
		}
	}
	if cfg.LogLevel != nil && *cfg.LogLevel != "" { // Allow empty for etcd default
		validLogLevels := []string{"debug", "info", "warn", "error", "panic", "fatal"}
		if !containsString(validLogLevels, *cfg.LogLevel) {
			verrs.Add("%s.logLevel: invalid value '%s'", pathPrefix, *cfg.LogLevel)
		}
	}
	// MaxSnapshotsToKeep is uint, no need to check < 0
	// MaxWALsToKeep is uint, no need to check < 0
}

func (e *EtcdConfig) GetClientPort() int {
	if e != nil && e.ClientPort != nil { return *e.ClientPort }
	return 2379
}
func (e *EtcdConfig) GetPeerPort() int {
	if e != nil && e.PeerPort != nil { return *e.PeerPort }
	return 2380
}
func (e *EtcdConfig) GetDataDir() string {
   if e != nil && e.DataDir != nil && *e.DataDir != "" { return *e.DataDir }
   return "/var/lib/etcd"
}
