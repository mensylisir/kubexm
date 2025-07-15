package v1alpha1

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1/helpers"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/errors/validation"
	"path"
	"strings"
)

type Etcd struct {
	Type              string                 `json:"type,omitempty" yaml:"type,omitempty"`
	Version           string                 `json:"version,omitempty" yaml:"version,omitempty"`
	External          *ExternalEtcdConfig    `json:"external,omitempty" yaml:"external,omitempty"`
	ClusterConfig     *EtcdClusterConfig     `json:"cluster,omitempty" yaml:"cluster,omitempty"`
	BackupConfig      *EtcdBackupConfig      `json:"backup,omitempty" yaml:"backup,omitempty"`
	PerformanceTuning *EtcdPerformanceTuning `json:"performance,omitempty" yaml:"performance,omitempty"`
}

type ExternalEtcdConfig struct {
	Endpoints []string `json:"endpoints" yaml:"endpoints"`
	CAFile    string   `json:"caFile,omitempty" yaml:"caFile,omitempty"`
	CertFile  string   `json:"certFile,omitempty" yaml:"certFile,omitempty"`
	KeyFile   string   `json:"keyFile,omitempty" yaml:"keyFile,omitempty"`
}

type EtcdClusterConfig struct {
	ClientPort   *int              `json:"clientPort,omitempty" yaml:"clientPort,omitempty"`
	PeerPort     *int              `json:"peerPort,omitempty" yaml:"peerPort,omitempty"`
	DataDir      *string           `json:"dataDir,omitempty" yaml:"dataDir,omitempty"`
	ClusterToken string            `json:"clusterToken,omitempty" yaml:"clusterToken,omitempty"`
	ExtraArgs    map[string]string `json:"extraArgs,omitempty" yaml:"extraArgs,omitempty"`
}

type EtcdBackupConfig struct {
	BackupDir   *string `json:"dir,omitempty" yaml:"dir,omitempty"`
	PeriodHours *int    `json:"periodHours,omitempty" yaml:"periodHours,omitempty"`
	KeepNumber  *int    `json:"keepNumber,omitempty" yaml:"keepNumber,omitempty"`
	ScriptPath  *string `json:"scriptPath,omitempty" yaml:"scriptPath,omitempty"`
}

type EtcdPerformanceTuning struct {
	HeartbeatIntervalMillis *int    `json:"heartbeatIntervalMillis,omitempty" yaml:"heartbeatIntervalMillis,omitempty"`
	ElectionTimeoutMillis   *int    `json:"electionTimeoutMillis,omitempty" yaml:"electionTimeoutMillis,omitempty"`
	SnapshotCount           *uint64 `json:"snapshotCount,omitempty" yaml:"snapshotCount,omitempty"`
	AutoCompactionRetention *int    `json:"autoCompactionRetention,omitempty" yaml:"autoCompactionRetention,omitempty"`
	QuotaBackendBytes       *int64  `json:"quotaBackendBytes,omitempty" yaml:"quotaBackendBytes,omitempty"`
	MaxRequestBytes         *uint   `json:"maxRequestBytes,omitempty" yaml:"maxRequestBytes,omitempty"`
	MaxSnapshots            *uint   `json:"maxSnapshots,omitempty" yaml:"maxSnapshots,omitempty"`
	MaxWALs                 *uint   `json:"maxWals,omitempty" yaml:"maxWals,omitempty"`
	Metrics                 *string `json:"metrics,omitempty" yaml:"metrics,omitempty"`
	LogLevel                *string `json:"logLevel,omitempty" yaml:"logLevel,omitempty"`
}

func SetDefaults_Etcd(cfg *Etcd) {
	if cfg == nil {
		return
	}

	if cfg.Type == "" {
		cfg.Type = string(common.EtcdDeploymentTypeKubexm)
	}

	if cfg.Type == string(common.EtcdDeploymentTypeKubexm) {
		if cfg.ClusterConfig == nil {
			cfg.ClusterConfig = &EtcdClusterConfig{}
		}
		SetDefaults_EtcdClusterConfig(cfg.ClusterConfig)

		if cfg.BackupConfig == nil {
			cfg.BackupConfig = &EtcdBackupConfig{}
		}
		SetDefaults_EtcdBackupConfig(cfg.BackupConfig)

		if cfg.PerformanceTuning == nil {
			cfg.PerformanceTuning = &EtcdPerformanceTuning{}
		}
		SetDefaults_EtcdPerformanceTuning(cfg.PerformanceTuning)
	}

	if cfg.Type == string(common.EtcdDeploymentTypeExternal) && cfg.External == nil {
		cfg.External = &ExternalEtcdConfig{}
	}
}

func SetDefaults_EtcdClusterConfig(cfg *EtcdClusterConfig) {
	if cfg.ClientPort == nil {
		cfg.ClientPort = helpers.IntPtr(common.DefaultEtcdClientPort)
	}
	if cfg.PeerPort == nil {
		cfg.PeerPort = helpers.IntPtr(common.DefaultEtcdPeerPort)
	}
	if cfg.DataDir == nil {
		cfg.DataDir = helpers.StrPtr(common.EtcdDefaultDataDirTarget)
	}
	if cfg.ClusterToken == "" {
		cfg.ClusterToken = common.DefaultEtcdClusterToken
	}
}

func SetDefaults_EtcdBackupConfig(cfg *EtcdBackupConfig) {
	if cfg.BackupDir == nil {
		cfg.BackupDir = helpers.StrPtr(common.DefaultEtcdBackupDir)
	}
	if cfg.PeriodHours == nil {
		cfg.PeriodHours = helpers.IntPtr(common.DefaultBackupInterval)
	}
	if cfg.KeepNumber == nil {
		cfg.KeepNumber = helpers.IntPtr(common.DefaultEtcdKeepBackups)
	}
	if cfg.ScriptPath == nil {
		cfg.ScriptPath = helpers.StrPtr(common.DefaultEtcdScriptPath)
	}
}

func SetDefaults_EtcdPerformanceTuning(cfg *EtcdPerformanceTuning) {
	if cfg.HeartbeatIntervalMillis == nil {
		cfg.HeartbeatIntervalMillis = helpers.IntPtr(common.DefaultEtcdHeartbeatInterval)
	}
	if cfg.ElectionTimeoutMillis == nil {
		cfg.ElectionTimeoutMillis = helpers.IntPtr(common.DefaultEtcdElectionTimeout)
	}
	if cfg.SnapshotCount == nil {
		cfg.SnapshotCount = helpers.Uint64Ptr(common.DefaultEtcdSnapshotCount)
	}
	if cfg.AutoCompactionRetention == nil {
		cfg.AutoCompactionRetention = helpers.IntPtr(common.DefaultEtcdAutoCompactionRetention)
	}
	if cfg.QuotaBackendBytes == nil {
		cfg.QuotaBackendBytes = helpers.Int64Ptr(common.DefaultEtcdQuotaBackendBytes)
	}
	if cfg.MaxRequestBytes == nil {
		cfg.MaxRequestBytes = helpers.UintPtr(common.DefaultEtcdMaxRequestBytes)
	}
	if cfg.Metrics == nil {
		cfg.Metrics = helpers.StrPtr(common.DefaultEtcdMetrics)
	}
	if cfg.LogLevel == nil {
		cfg.LogLevel = helpers.StrPtr(common.DefaultEtcdLogLevel)
	}
	if cfg.MaxSnapshots == nil {
		cfg.MaxSnapshots = helpers.UintPtr(common.DefaultEtcdMaxSnapshots)
	}
	if cfg.MaxWALs == nil {
		cfg.MaxWALs = helpers.UintPtr(common.DefaultEtcdMaxWALs)
	}
}

func Validate_Etcd(cfg *Etcd, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}
	p := path.Join(pathPrefix)

	if !helpers.ContainsString(common.SupportedEtcdDeploymentTypes, cfg.Type) {
		verrs.Add(fmt.Sprintf("%s.type: invalid type '%s', must be one of %v",
			p, cfg.Type, common.SupportedEtcdDeploymentTypes))
	}

	if cfg.Type == string(common.EtcdDeploymentTypeExternal) {
		if cfg.External == nil {
			verrs.Add(path.Join(p, "external") + ": must be defined if etcd.type is 'external'")
		} else {
			Validate_ExternalEtcdConfig(cfg.External, verrs, path.Join(p, "external"))
		}
		if cfg.ClusterConfig != nil {
			verrs.Add(path.Join(p, "cluster") + ": cannot be set when etcd.type is 'external'")
		}
		if cfg.BackupConfig != nil {
			verrs.Add(path.Join(p, "backup") + ": cannot be set when etcd.type is 'external'")
		}
		if cfg.PerformanceTuning != nil {
			verrs.Add(path.Join(p, "performance") + ": cannot be set when etcd.type is 'external'")
		}

	} else if cfg.Type == string(common.EtcdDeploymentTypeKubexm) {
		if cfg.External != nil {
			verrs.Add(path.Join(p, "external") + ": cannot be set when etcd.type is not 'external'")
		}
		if cfg.ClusterConfig != nil {
			Validate_EtcdClusterConfig(cfg.ClusterConfig, verrs, path.Join(p, "cluster"))
		}
		if cfg.BackupConfig != nil {
			Validate_EtcdBackupConfig(cfg.BackupConfig, verrs, path.Join(p, "backup"))
		}
		if cfg.PerformanceTuning != nil {
			Validate_EtcdPerformanceTuning(cfg.PerformanceTuning, verrs, path.Join(p, "performance"))
		}
	}
}

func Validate_ExternalEtcdConfig(cfg *ExternalEtcdConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if len(cfg.Endpoints) == 0 {
		verrs.Add(pathPrefix + ".endpoints: must contain at least one endpoint")
	}
	for i, ep := range cfg.Endpoints {
		if strings.TrimSpace(ep) == "" {
			verrs.Add(fmt.Sprintf("%s.endpoints[%d]: endpoint cannot be empty", pathPrefix, i))
		}
	}
	if (cfg.CertFile != "" && cfg.KeyFile == "") || (cfg.CertFile == "" && cfg.KeyFile != "") {
		verrs.Add(pathPrefix + ": certFile and keyFile must be specified together for mTLS")
	}
}

func Validate_EtcdClusterConfig(cfg *EtcdClusterConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg.ClientPort != nil && (*cfg.ClientPort <= 0 || *cfg.ClientPort > 65535) {
		verrs.Add(fmt.Sprintf("%s.clientPort: invalid port %d, must be between 1-65535", pathPrefix, *cfg.ClientPort))
	}
	if cfg.PeerPort != nil && (*cfg.PeerPort <= 0 || *cfg.PeerPort > 65535) {
		verrs.Add(fmt.Sprintf("%s.peerPort: invalid port %d, must be between 1-65535", pathPrefix, *cfg.PeerPort))
	}
	if cfg.DataDir != nil && strings.TrimSpace(*cfg.DataDir) == "" {
		verrs.Add(pathPrefix + ".dataDir: cannot be empty if specified")
	}
	if strings.TrimSpace(cfg.ClusterToken) == "" {
		verrs.Add(pathPrefix + ".clusterToken: cannot be empty")
	}
}

func Validate_EtcdBackupConfig(cfg *EtcdBackupConfig, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg.PeriodHours != nil && *cfg.PeriodHours < 0 {
		verrs.Add(fmt.Sprintf("%s.periodHours: cannot be negative, got %d", pathPrefix, *cfg.PeriodHours))
	}
	if cfg.KeepNumber != nil && *cfg.KeepNumber < 0 {
		verrs.Add(fmt.Sprintf("%s.keepNumber: cannot be negative, got %d", pathPrefix, *cfg.KeepNumber))
	}
}

func Validate_EtcdPerformanceTuning(cfg *EtcdPerformanceTuning, verrs *validation.ValidationErrors, pathPrefix string) {
	if cfg == nil {
		return
	}

	if cfg.HeartbeatIntervalMillis != nil && *cfg.HeartbeatIntervalMillis <= 0 {
		verrs.Add(fmt.Sprintf("%s.heartbeatIntervalMillis: must be positive, got %d",
			pathPrefix, *cfg.HeartbeatIntervalMillis))
	}

	if cfg.ElectionTimeoutMillis != nil && *cfg.ElectionTimeoutMillis <= 0 {
		verrs.Add(fmt.Sprintf("%s.electionTimeoutMillis: must be positive, got %d",
			pathPrefix, *cfg.ElectionTimeoutMillis))
	}

	if cfg.SnapshotCount != nil && *cfg.SnapshotCount == 0 {
		verrs.Add(fmt.Sprintf("%s.snapshotCount: must be a positive value if set, got %d",
			pathPrefix, *cfg.SnapshotCount))
	}

	if cfg.AutoCompactionRetention != nil && *cfg.AutoCompactionRetention < 0 {
		verrs.Add(fmt.Sprintf("%s.autoCompactionRetention: cannot be negative, got %d",
			pathPrefix, *cfg.AutoCompactionRetention))
	}

	if cfg.QuotaBackendBytes != nil && *cfg.QuotaBackendBytes < 0 {
		verrs.Add(fmt.Sprintf("%s.quotaBackendBytes: cannot be negative, got %d",
			pathPrefix, *cfg.QuotaBackendBytes))
	}

	if cfg.MaxRequestBytes != nil && *cfg.MaxRequestBytes == 0 {
		verrs.Add(fmt.Sprintf("%s.maxRequestBytes: must be positive if set, got %d",
			pathPrefix, *cfg.MaxRequestBytes))
	}

	if cfg.MaxSnapshots != nil && *cfg.MaxSnapshots == 0 {
		verrs.Add(fmt.Sprintf("%s.maxSnapshots: must be positive if set, got %d",
			pathPrefix, *cfg.MaxSnapshots))
	}

	if cfg.MaxWALs != nil && *cfg.MaxWALs == 0 {
		verrs.Add(fmt.Sprintf("%s.maxWals: must be positive if set, got %d",
			pathPrefix, *cfg.MaxWALs))
	}

	if cfg.Metrics != nil && *cfg.Metrics != "" && !helpers.ContainsString(common.ValidEtcdMetricsLevels, *cfg.Metrics) {
		verrs.Add(fmt.Sprintf("%s.metrics: invalid value '%s', must be one of %v",
			pathPrefix, *cfg.Metrics, common.ValidEtcdMetricsLevels))
	}

	if cfg.LogLevel != nil && *cfg.LogLevel != "" && !helpers.ContainsString(common.ValidEtcdLogLevels, *cfg.LogLevel) {
		verrs.Add(fmt.Sprintf("%s.logLevel: invalid value '%s', must be one of %v",
			pathPrefix, *cfg.LogLevel, common.ValidEtcdLogLevels))
	}
}
