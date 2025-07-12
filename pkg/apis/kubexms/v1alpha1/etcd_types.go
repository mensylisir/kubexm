package v1alpha1

import (
	"fmt"
	"strings"
)

const (
	EtcdTypeKubeXMSInternal = "kubexm"   // 表示要使用二进制部署etcd
	EtcdTypeExternal        = "external" // 表示外部已经有现成的etcd
	EtcdTypeInternal        = "kubeadm"  // 表示kubeadm部署etcd,即etcd是以静态pod的形式启动的
)

// EtcdConfig defines the configuration for the Etcd cluster.
type EtcdConfig struct {
	Type                string              `json:"type,omitempty" yaml:"type,omitempty"`
	Version             string              `json:"version,omitempty" yaml:"version,omitempty"`
	Arch                string              `json:"arch,omitempty" yaml:"arch,omitempty"`
	External            *ExternalEtcdConfig `json:"external,omitempty" yaml:"external,omitempty"`
	ClientPort          *int                `json:"clientPort,omitempty" yaml:"clientPort,omitempty"`
	PeerPort            *int                `json:"peerPort,omitempty" yaml:"peerPort,omitempty"`
	DataDir             *string             `json:"dataDir,omitempty" yaml:"dataDir,omitempty"`
	ClusterToken        string              `json:"clusterToken,omitempty" yaml:"clusterToken,omitempty"`
	ExtraArgs           []string            `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
	BackupDir           *string             `json:"backupDir,omitempty" yaml:"backupDir,omitempty"`
	BackupPeriodHours   *int                `json:"backupPeriodHours,omitempty" yaml:"backupPeriodHours,omitempty"`
	KeepBackupNumber    *int                `json:"keepBackupNumber,omitempty" yaml:"keepBackupNumber,omitempty"`
	BackupScriptPath    *string             `json:"backupScriptPath,omitempty" yaml:"backupScriptPath,omitempty"`
	HeartbeatIntervalMillis      *int    `json:"heartbeatIntervalMillis,omitempty" yaml:"heartbeatInterval,omitempty"`
	ElectionTimeoutMillis        *int    `json:"electionTimeoutMillis,omitempty" yaml:"electionTimeout,omitempty"`
	SnapshotCount                *uint64 `json:"snapshotCount,omitempty" yaml:"snapshotCount,omitempty"`
	AutoCompactionRetentionHours *int    `json:"autoCompactionRetentionHours,omitempty" yaml:"autoCompactionRetention,omitempty"`
	QuotaBackendBytes *int64 `json:"quotaBackendBytes,omitempty" yaml:"quotaBackendBytes,omitempty"`
	MaxRequestBytes   *uint  `json:"maxRequestBytes,omitempty" yaml:"maxRequestBytes,omitempty"`
	Metrics            *string `json:"metrics,omitempty" yaml:"metrics,omitempty"`
	LogLevel           *string `json:"logLevel,omitempty" yaml:"logLevel,omitempty"`
	MaxSnapshotsToKeep *uint   `json:"maxSnapshotsToKeep,omitempty" yaml:"maxSnapshots,omitempty"`
	MaxWALsToKeep      *uint   `json:"maxWALsToKeep,omitempty" yaml:"maxWals,omitempty"`
	// TLS *ManagedEtcdTLSConfig `json:"tls,omitempty" yaml:"tls,omitempty"` // From suggested improvements
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
	if cfg.ClusterToken == "" {
		cfg.ClusterToken = "kubexm-etcd-default-token"
	}
	if cfg.Type == EtcdTypeExternal && cfg.External == nil {
		cfg.External = &ExternalEtcdConfig{}
	}
	if cfg.External != nil && len(cfg.External.Endpoints) == 0 { // Fixed: check length of Endpoints
	   cfg.External.Endpoints = []string{}
	}
	if cfg.ExtraArgs == nil { cfg.ExtraArgs = []string{} }
	if cfg.BackupDir == nil { defaultBackupDir := "/var/backups/etcd"; cfg.BackupDir = &defaultBackupDir }
	if cfg.BackupPeriodHours == nil { defaultBackupPeriod := 24; cfg.BackupPeriodHours = &defaultBackupPeriod }
	if cfg.KeepBackupNumber == nil { defaultKeepBackups := 7; cfg.KeepBackupNumber = &defaultKeepBackups }
	if cfg.HeartbeatIntervalMillis == nil { hb := 250; cfg.HeartbeatIntervalMillis = &hb }
	if cfg.ElectionTimeoutMillis == nil { et := 5000; cfg.ElectionTimeoutMillis = &et }
	if cfg.SnapshotCount == nil { var sc uint64 = 10000; cfg.SnapshotCount = &sc }
	if cfg.AutoCompactionRetentionHours == nil { acr := 8; cfg.AutoCompactionRetentionHours = &acr }
	if cfg.QuotaBackendBytes == nil { var qbb int64 = 2147483648; cfg.QuotaBackendBytes = &qbb }
	if cfg.MaxRequestBytes == nil { var mrb uint = 1572864; cfg.MaxRequestBytes = &mrb }
	if cfg.Metrics == nil { m := "basic"; cfg.Metrics = &m }
	if cfg.LogLevel == nil { l := "info"; cfg.LogLevel = &l }
	if cfg.MaxSnapshotsToKeep == nil { var ms uint = 5; cfg.MaxSnapshotsToKeep = &ms }
	if cfg.MaxWALsToKeep == nil { var mw uint = 5; cfg.MaxWALsToKeep = &mw }
}

// Validate_EtcdConfig validates EtcdConfig.
func Validate_EtcdConfig(cfg *EtcdConfig, verrs *ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	validTypes := []string{EtcdTypeKubeXMSInternal, EtcdTypeExternal, EtcdTypeInternal, ""} // Allow empty for default
	if !containsString(validTypes, cfg.Type) { // Using local containsString, assuming ValidationErrors is from cluster_types.go
		verrs.Add(pathPrefix + ".type: invalid type '" + cfg.Type + "', must be one of " + fmt.Sprintf("%v", validTypes))
	}
	if cfg.Type == EtcdTypeExternal {
		if cfg.External == nil {
			verrs.Add(pathPrefix + ".external: must be defined if etcd.type is '" + EtcdTypeExternal + "'")
		} else {
			if len(cfg.External.Endpoints) == 0 {
				verrs.Add(pathPrefix + ".external.endpoints: must contain at least one endpoint if etcd.type is '" + EtcdTypeExternal + "'")
			}
			for i, ep := range cfg.External.Endpoints {
				if strings.TrimSpace(ep) == "" {
					verrs.Add(pathPrefix + ".external.endpoints[" + fmt.Sprintf("%d", i) + "]: endpoint cannot be empty")
				}
			}
			if (cfg.External.CertFile != "" && cfg.External.KeyFile == "") || (cfg.External.CertFile == "" && cfg.External.KeyFile != "") {
				verrs.Add(pathPrefix + ".external: certFile and keyFile must be specified together for mTLS")
			}
		}
	}
	if cfg.ClientPort != nil && (*cfg.ClientPort <= 0 || *cfg.ClientPort > 65535) {
	   verrs.Add(pathPrefix + ".clientPort: invalid port " + fmt.Sprintf("%d", *cfg.ClientPort) + ", must be between 1-65535")
	}
	if cfg.PeerPort != nil && (*cfg.PeerPort <= 0 || *cfg.PeerPort > 65535) {
	   verrs.Add(pathPrefix + ".peerPort: invalid port " + fmt.Sprintf("%d", *cfg.PeerPort) + ", must be between 1-65535")
	}
	if cfg.DataDir != nil && strings.TrimSpace(*cfg.DataDir) == "" {
		verrs.Add(pathPrefix + ".dataDir: cannot be empty if specified")
	}
	if strings.TrimSpace(cfg.ClusterToken) == "" {
		verrs.Add(pathPrefix + ".clusterToken: cannot be empty")
	}
	if cfg.BackupPeriodHours != nil && *cfg.BackupPeriodHours < 0 {
		verrs.Add(pathPrefix + ".backupPeriodHours: cannot be negative, got " + fmt.Sprintf("%d", *cfg.BackupPeriodHours))
	}
	if cfg.KeepBackupNumber != nil && *cfg.KeepBackupNumber < 0 {
		verrs.Add(pathPrefix + ".keepBackupNumber: cannot be negative, got " + fmt.Sprintf("%d", *cfg.KeepBackupNumber))
	}
	if cfg.HeartbeatIntervalMillis != nil && *cfg.HeartbeatIntervalMillis <= 0 {
		verrs.Add(pathPrefix + ".heartbeatIntervalMillis: must be positive, got " + fmt.Sprintf("%d", *cfg.HeartbeatIntervalMillis))
	}
	if cfg.ElectionTimeoutMillis != nil && *cfg.ElectionTimeoutMillis <= 0 {
		verrs.Add(pathPrefix + ".electionTimeoutMillis: must be positive, got " + fmt.Sprintf("%d", *cfg.ElectionTimeoutMillis))
	}
	if cfg.AutoCompactionRetentionHours != nil && *cfg.AutoCompactionRetentionHours < 0 {
		verrs.Add(pathPrefix + ".autoCompactionRetentionHours: cannot be negative, got " + fmt.Sprintf("%d", *cfg.AutoCompactionRetentionHours))
	}
	if cfg.QuotaBackendBytes != nil && *cfg.QuotaBackendBytes < 0 {
		verrs.Add(pathPrefix + ".quotaBackendBytes: cannot be negative, got " + fmt.Sprintf("%d", *cfg.QuotaBackendBytes))
	}
	if cfg.MaxRequestBytes != nil && *cfg.MaxRequestBytes == 0 {
		verrs.Add(pathPrefix + ".maxRequestBytes: must be positive if set, got " + fmt.Sprintf("%d", *cfg.MaxRequestBytes))
	}
	if cfg.Metrics != nil && *cfg.Metrics != "" {
		validMetrics := []string{"basic", "extensive"}
		if !containsString(validMetrics, *cfg.Metrics) {
			verrs.Add(pathPrefix + ".metrics: invalid value '" + *cfg.Metrics + "', must be 'basic' or 'extensive'")
		}
	}
	if cfg.LogLevel != nil && *cfg.LogLevel != "" {
		validLogLevels := []string{"debug", "info", "warn", "error", "panic", "fatal"}
		if !containsString(validLogLevels, *cfg.LogLevel) {
			verrs.Add(pathPrefix + ".logLevel: invalid value '" + *cfg.LogLevel + "'")
		}
	}
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

// Assuming ValidationErrors and containsString are defined in cluster_types.go or a shared util.
// type ValidationErrors struct{ Errors []string }
// func (ve *ValidationErrors) Add(format string, args ...interface{}) { ve.Errors = append(ve.Errors, fmt.Sprintf(format, args...)) }
// func containsString(slice []string, item string) bool { for _, s := range slice { if s == item { return true } }; return false }
// NOTE: DeepCopy methods should be generated by controller-gen.
// Removed ManagedEtcdTLSConfig from struct for now as it was a suggested improvement.
// Fixed SetDefaults_EtcdConfig: length check for External.Endpoints.
// Updated Validate_EtcdConfig to use local containsString if ValidationErrors is defined in cluster_types.go
// and ensure pathPrefix is used in Add calls.
// Corrected containsString usage in Validate_EtcdConfig.
// Removed local stubs for ValidationErrors and containsString.
// Added import "strings".
