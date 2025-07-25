package common

const (
	PipelineTypeClusterCreate  = "cluster-create"
	PipelineTypeClusterDelete  = "cluster-delete"
	PipelineTypeClusterUpgrade = "cluster-upgrade"
	PipelineTypeClusterScale   = "cluster-scale"
	PipelineTypeNodeAdd        = "node-add"
	PipelineTypeNodeRemove     = "node-remove"
	PipelineTypeMaintenance    = "maintenance"
	PipelineTypeBackup         = "backup"
	PipelineTypeRestore        = "restore"
)

const (
	PipelineExecutionSequential  = "sequential"
	PipelineExecutionParallel    = "parallel"
	PipelineExecutionConditional = "conditional"
	PipelineExecutionPhased      = "phased"
)

const (
	PipelineStatusPending     = "pending"
	PipelineStatusRunning     = "running"
	PipelineStatusCompleted   = "completed"
	PipelineStatusFailed      = "failed"
	PipelineStatusSkipped     = "skipped"
	PipelineStatusCancelled   = "cancelled"
	PipelineStatusRollingBack = "rolling-back"
	PipelineStatusRolledBack  = "rolled-back"
)

const (
	PipelineModeDryRun     = "dry-run"
	PipelineModeExecution  = "execution"
	PipelineModeValidation = "validation"
	PipelineModeRollback   = "rollback"
)

const (
	DefaultPipelineTimeoutMinutes    = 120
	DefaultPipelineRetryAttempts     = 1
	DefaultPipelineRetryDelaySeconds = 30
	DefaultPipelineMaxConcurrency    = 5
)

const (
	MaxPipelineNameLength            = 100
	PipelineNameValidCharacters      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_."
	PipelineNameInvalidStartEndChars = "-_."
)

const (
	MaxPipelineDependencyDepth = 5
	MaxPipelineDependencyCount = 10
)

const (
	PipelineProgressPlanningPhase   = "planning"
	PipelineProgressExecutionPhase  = "execution"
	PipelineProgressValidationPhase = "validation"
	PipelineProgressCleanupPhase    = "cleanup"
)

const (
	PipelineConfigAssumeYes      = "assume-yes"
	PipelineConfigDryRun         = "dry-run"
	PipelineConfigMaxConcurrency = "max-concurrency"
	PipelineConfigTimeout        = "timeout"
	PipelineConfigRetryAttempts  = "retry-attempts"
	PipelineConfigSkipValidation = "skip-validation"
	PipelineConfigForceExecution = "force-execution"
	PipelineConfigVerbose        = "verbose"
)

const (
	PipelineResourceMultiplier          = 1.5
	PipelineOverheadPercentage          = 20
	PipelineMinimumExecutionTimeMinutes = 5
)
