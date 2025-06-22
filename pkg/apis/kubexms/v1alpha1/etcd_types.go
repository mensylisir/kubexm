package v1alpha1

import (
	// "fmt" // Removed as unused
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
	External            *ExternalEtcdConfig `json:"external,omitempty" yaml:"external,omitempty"`// Config for external etcd

	ClientPort          *int                `json:"clientPort,omitempty" yaml:"clientPort,omitempty"` // Default: 2379
	PeerPort            *int                `json:"peerPort,omitempty" yaml:"peerPort,omitempty"`   // Default: 2380
	DataDir             *string             `json:"dataDir,omitempty" yaml:"dataDir,omitempty"`    // Default: "/var/lib/etcd"

	// ExtraArgs for etcd process, as a list of strings (e.g., "--initial-cluster-token=mytoken").
	// Changed from map[string]string to []string for flag flexibility.
	ExtraArgs           []string            `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`

	// Backup configuration
	BackupDir           *string `json:"backupDir,omitempty" yaml:"backupDir,omitempty"`           // Directory to store etcd backups
	BackupPeriodHours   *int    `json:"backupPeriodHours,omitempty" yaml:"backupPeriodHours,omitempty"`   // Interval in hours for backups
	KeepBackupNumber    *int    `json:"keepBackupNumber,omitempty" yaml:"keepBackupNumber,omitempty"`    // Number of backups to retain
	BackupScriptPath    *string `json:"backupScriptPath,omitempty" yaml:"backupScriptPath,omitempty"`    // Path to a custom backup script

	// Performance and tuning
	HeartbeatIntervalMillis      *int    `json:"heartbeatIntervalMillis,omitempty" yaml:"heartbeatInterval,omitempty"`
	ElectionTimeoutMillis        *int    `json:"electionTimeoutMillis,omitempty" yaml:"electionTimeout,omitempty"`
	SnapshotCount                *uint64 `json:"snapshotCount,omitempty" yaml:"snapshotCount,omitempty"`
	AutoCompactionRetentionHours *int    `json:"autoCompactionRetentionHours,omitempty" yaml:"autoCompactionRetention,omitempty"`

	// Resource management
	QuotaBackendBytes *int64 `json:"quotaBackendBytes,omitempty" yaml:"quotaBackendBytes,omitempty"`
	MaxRequestBytes   *uint  `json:"maxRequestBytes,omitempty" yaml:"maxRequestBytes,omitempty"`

	// Operational settings
	Metrics            *string `json:"metrics,omitempty" yaml:"metrics,omitempty"`
	LogLevel           *string `json:"logLevel,omitempty" yaml:"logLevel,omitempty"`
	MaxSnapshotsToKeep *uint   `json:"maxSnapshotsToKeep,omitempty" yaml:"maxSnapshots,omitempty"`
	MaxWALsToKeep      *uint   `json:"maxWALsToKeep,omitempty" yaml:"maxWals,omitempty"`
}

// ExternalEtcdConfig describes how to connect to an external etcd cluster.
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
	if cfg.HeartbeatIntervalMillis == nil { hb := 250; cfg.HeartbeatIntervalMillis = &hb } // YAML: heartbeatInterval: 250
	if cfg.ElectionTimeoutMillis == nil { et := 5000; cfg.ElectionTimeoutMillis = &et } // YAML: electionTimeout: 5000
	if cfg.SnapshotCount == nil { var sc uint64 = 10000; cfg.SnapshotCount = &sc } // YAML: snapshotCount: 10000
	if cfg.AutoCompactionRetentionHours == nil { acr := 8; cfg.AutoCompactionRetentionHours = &acr } // YAML: autoCompactionRetention: 8

	// Resource management defaults
	if cfg.QuotaBackendBytes == nil { var qbb int64 = 2147483648; cfg.QuotaBackendBytes = &qbb } // YAML: quotaBackendBytes: 2147483648 (2GB)
	if cfg.MaxRequestBytes == nil { var mrb uint = 1572864; cfg.MaxRequestBytes = &mrb } // YAML: maxRequestBytes: 1572864 (1.5MB)

	// Operational defaults
	if cfg.Metrics == nil { m := "basic"; cfg.Metrics = &m } // YAML: metrics: basic
	if cfg.LogLevel == nil { l := "info"; cfg.LogLevel = &l }
	if cfg.MaxSnapshotsToKeep == nil { var ms uint = 5; cfg.MaxSnapshotsToKeep = &ms } // etcd default
	if cfg.MaxWALsToKeep == nil { var mw uint = 5; cfg.MaxWALsToKeep = &mw }          // etcd default
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
	if cfg.MaxRequestBytes != nil && *cfg.MaxRequestBytes == 0 { // MaxRequestBytes is uint. 0 is generally invalid.
		verrs.Add("%s.maxRequestBytes: must be positive if set, got %d", pathPrefix, *cfg.MaxRequestBytes)
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
