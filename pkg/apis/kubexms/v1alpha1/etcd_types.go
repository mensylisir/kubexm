package v1alpha1

import (
	"fmt"
	"strings"
)

const (
	EtcdTypeKubeXMSInternal = "stacked"
	EtcdTypeExternal        = "external"
)

// EtcdConfig defines the configuration for the Etcd cluster.
type EtcdConfig struct {
	Type                string              `json:"type,omitempty"`    // "stacked" or "external"
	Version             string              `json:"version,omitempty"` // Etcd version for managed setup
	External            *ExternalEtcdConfig `json:"external,omitempty"`// Config for external etcd

	ClientPort          *int                `json:"clientPort,omitempty"` // Default: 2379
	PeerPort            *int                `json:"peerPort,omitempty"`   // Default: 2380
	DataDir             *string             `json:"dataDir,omitempty"`    // Default: "/var/lib/etcd"

	// ExtraArgs for etcd process, as a list of strings (e.g., "--initial-cluster-token=mytoken").
	// Changed from map[string]string to []string for flag flexibility.
	ExtraArgs           []string            `json:"extraArgs,omitempty"`

	// Backup configuration
	BackupDir           *string `json:"backupDir,omitempty"`           // Directory to store etcd backups
	BackupPeriodHours   *int    `json:"backupPeriodHours,omitempty"`   // Interval in hours for backups
	KeepBackupNumber    *int    `json:"keepBackupNumber,omitempty"`    // Number of backups to retain
	BackupScriptPath    *string `json:"backupScriptPath,omitempty"`    // Path to a custom backup script

	// Performance and tuning
	HeartbeatIntervalMillis *int    `json:"heartbeatIntervalMillis,omitempty"` // Leader heartbeat interval in milliseconds
	ElectionTimeoutMillis   *int    `json:"electionTimeoutMillis,omitempty"`   // Election timeout in milliseconds
	SnapshotCount           *uint64 `json:"snapshotCount,omitempty"`           // Number of committed transactions to trigger a snapshot
	AutoCompactionRetentionHours *int `json:"autoCompactionRetentionHours,omitempty"` // Auto compaction retention in hours (0 to disable)


	// Resource management
	QuotaBackendBytes   *int64 `json:"quotaBackendBytes,omitempty"` // Etcd storage quota in bytes (0 for no quota)
	MaxRequestBytes     *uint  `json:"maxRequestBytes,omitempty"`   // Maximum client request size in bytes

	// Operational settings
	Metrics             *string `json:"metrics,omitempty"`             // Metrics exposure level: "basic" or "extensive"
	LogLevel            *string `json:"logLevel,omitempty"`            // Log level: "debug", "info", "warn", "error", "panic", "fatal"
	MaxSnapshotsToKeep  *uint   `json:"maxSnapshotsToKeep,omitempty"`  // Maximum number of snapshot files to keep (0 for unlimited)
	MaxWALsToKeep       *uint   `json:"maxWALsToKeep,omitempty"`       // Maximum number of WAL files to keep (0 for unlimited)
}

// ExternalEtcdConfig describes how to connect to an external etcd cluster.
type ExternalEtcdConfig struct {
	Endpoints []string `json:"endpoints"`
	CAFile string `json:"caFile,omitempty"`
	CertFile string `json:"certFile,omitempty"`
	KeyFile string `json:"keyFile,omitempty"`
}

// SetDefaults_EtcdConfig sets default values for EtcdConfig.
func SetDefaults_EtcdConfig(cfg *EtcdConfig) {
	if cfg == nil {
		return
	}
	if cfg.Type == "" {
		cfg.Type = EtcdTypeKubeXMSInternal
	}
	if cfg.ClientPort == nil {
		defaultPort := 2379
		cfg.ClientPort = &defaultPort
	}
	if cfg.PeerPort == nil {
		defaultPort := 2380
		cfg.PeerPort = &defaultPort
	}
	if cfg.DataDir == nil {
		defaultDataDir := "/var/lib/etcd"
		cfg.DataDir = &defaultDataDir
	}
	if cfg.Type == EtcdTypeExternal && cfg.External == nil {
		cfg.External = &ExternalEtcdConfig{}
	}
	if cfg.External != nil && cfg.External.Endpoints == nil {
	   cfg.External.Endpoints = []string{}
	}
	if cfg.ExtraArgs == nil { cfg.ExtraArgs = []string{} }

	// Default backup settings (can be adjusted)
	if cfg.BackupDir == nil { defaultBackupDir := "/var/backups/etcd"; cfg.BackupDir = &defaultBackupDir }
	if cfg.BackupPeriodHours == nil { defaultBackupPeriod := 24; cfg.BackupPeriodHours = &defaultBackupPeriod } // e.g., daily
	if cfg.KeepBackupNumber == nil { defaultKeepBackups := 7; cfg.KeepBackupNumber = &defaultKeepBackups }

	// Default performance/tuning (values from etcd defaults or common practice)
	if cfg.HeartbeatIntervalMillis == nil { hb := 100; cfg.HeartbeatIntervalMillis = &hb }
	if cfg.ElectionTimeoutMillis == nil { et := 1000; cfg.ElectionTimeoutMillis = &et }
	if cfg.SnapshotCount == nil { var sc uint64 = 100000; cfg.SnapshotCount = &sc } // etcd default
	if cfg.AutoCompactionRetentionHours == nil { acr := 0; cfg.AutoCompactionRetentionHours = &acr } // etcd default is 0 (off for periodic) or 1 for hour-based if mode is 'periodic'

	// Resource management defaults (0 usually means disabled or etcd default)
	if cfg.QuotaBackendBytes == nil { var qbb int64 = 0; cfg.QuotaBackendBytes = &qbb } // 0 = no quota, etcd default 2GB
	// MaxRequestBytes default is 1.5 MiB in etcd

	// Operational defaults
	if cfg.Metrics == nil { m := "basic"; cfg.Metrics = &m }
	if cfg.LogLevel == nil { l := "info"; cfg.LogLevel = &l }
	if cfg.MaxSnapshotsToKeep == nil { var ms uint = 5; cfg.MaxSnapshotsToKeep = &ms } // etcd default
	if cfg.MaxWALsToKeep == nil { var mw uint = 5; cfg.MaxWALsToKeep = &mw }          // etcd default
}

// Validate_EtcdConfig validates EtcdConfig.
func Validate_EtcdConfig(cfg *EtcdConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	validTypes := []string{EtcdTypeKubeXMSInternal, EtcdTypeExternal}
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
	// SnapshotCount is uint64, typically positive or etcd default.
	if cfg.AutoCompactionRetentionHours != nil && *cfg.AutoCompactionRetentionHours < 0 {
		verrs.Add("%s.autoCompactionRetentionHours: cannot be negative, got %d", pathPrefix, *cfg.AutoCompactionRetentionHours)
	}
	if cfg.QuotaBackendBytes != nil && *cfg.QuotaBackendBytes < 0 { // 0 means default/no quota by some tools
		verrs.Add("%s.quotaBackendBytes: cannot be negative, got %d", pathPrefix, *cfg.QuotaBackendBytes)
	}
	if cfg.MaxRequestBytes != nil && *cfg.MaxRequestBytes <= 0 { // MaxRequestBytes is uint, so only check for > 0 if set
        // verrs.Add("%s.maxRequestBytes: must be positive if set, got %d", pathPrefix, *cfg.MaxRequestBytes)
        // A value of 0 might be invalid for MaxRequestBytes depending on etcd, typically it's positive.
        // For now, allow if set. Validation can be stricter if needed.
	}

	if cfg.Metrics != nil && *cfg.Metrics != "" { // Allow empty for etcd default
		validMetrics := []string{"basic", "extensive"}
		if !contains(validMetrics, *cfg.Metrics) { // Assumes contains() helper exists or is added
			verrs.Add("%s.metrics: invalid value '%s', must be 'basic' or 'extensive'", pathPrefix, *cfg.Metrics)
		}
	}
	if cfg.LogLevel != nil && *cfg.LogLevel != "" { // Allow empty for etcd default
		validLogLevels := []string{"debug", "info", "warn", "error", "panic", "fatal"}
		if !contains(validLogLevels, *cfg.LogLevel) {
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
