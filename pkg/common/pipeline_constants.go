package common

// Pipeline-related constants for categorization and execution

// Pipeline types and categories
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

// Pipeline execution strategies
const (
	PipelineExecutionSequential   = "sequential"
	PipelineExecutionParallel     = "parallel"
	PipelineExecutionConditional  = "conditional"
	PipelineExecutionPhased       = "phased"
)

// Pipeline status constants
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

// Pipeline execution modes
const (
	PipelineModeDryRun     = "dry-run"
	PipelineModeExecution  = "execution"
	PipelineModeValidation = "validation"
	PipelineModeRollback   = "rollback"
)

// Default pipeline timeouts and resource limits
const (
	DefaultPipelineTimeoutMinutes   = 120 // 2 hours
	DefaultPipelineRetryAttempts    = 1
	DefaultPipelineRetryDelaySeconds = 30
	DefaultPipelineMaxConcurrency   = 5
)

// Pipeline naming and validation constants
const (
	MaxPipelineNameLength             = 100
	PipelineNameValidCharacters       = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_."
	PipelineNameInvalidStartEndChars  = "-_."
)

// Pipeline dependency validation
const (
	MaxPipelineDependencyDepth = 5
	MaxPipelineDependencyCount = 10
)

// Pipeline progress tracking
const (
	PipelineProgressPlanningPhase   = "planning"
	PipelineProgressExecutionPhase  = "execution"
	PipelineProgressValidationPhase = "validation"
	PipelineProgressCleanupPhase    = "cleanup"
)

// Pipeline configuration keys
const (
	PipelineConfigAssumeYes       = "assume-yes"
	PipelineConfigDryRun          = "dry-run"
	PipelineConfigMaxConcurrency  = "max-concurrency"
	PipelineConfigTimeout         = "timeout"
	PipelineConfigRetryAttempts   = "retry-attempts"
	PipelineConfigSkipValidation  = "skip-validation"
	PipelineConfigForceExecution  = "force-execution"
	PipelineConfigVerbose         = "verbose"
)

// Pipeline resource estimation factors
const (
	PipelineResourceMultiplier        = 1.5  // Factor to multiply module resources
	PipelineOverheadPercentage        = 20   // Percentage overhead for pipeline coordination
	PipelineMinimumExecutionTimeMinutes = 5  // Minimum estimated execution time
)