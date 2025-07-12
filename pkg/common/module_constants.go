package common

// Module-related constants for categorization and execution

// Module execution strategies
const (
	ModuleExecutionSequential = "sequential"
	ModuleExecutionParallel   = "parallel"
	ModuleExecutionConditional = "conditional"
)

// Module status constants
const (
	ModuleStatusPending   = "pending"
	ModuleStatusRunning   = "running"
	ModuleStatusCompleted = "completed"
	ModuleStatusFailed    = "failed"
	ModuleStatusSkipped   = "skipped"
	ModuleStatusCancelled = "cancelled"
)

// Module phases - represents major phases in cluster lifecycle
const (
	ModulePhaseInfrastructure = "infrastructure"
	ModulePhasePreflight      = "preflight"
	ModulePhaseRuntime        = "runtime"
	ModulePhaseKubernetes     = "kubernetes"
	ModulePhaseNetwork        = "network"
	ModulePhaseAddons         = "addons"
	ModulePhaseCleanup        = "cleanup"
)

// Default module timeouts and resource limits
const (
	DefaultModuleTimeoutMinutes   = 60
	DefaultModuleRetryAttempts    = 2
	DefaultModuleRetryDelaySeconds = 10
	DefaultModuleMaxConcurrency   = 10
)

// Module naming and validation constants
const (
	MaxModuleNameLength             = 50
	ModuleNameValidCharacters       = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
	ModuleNameInvalidStartEndChars  = "-_"
)

// Module dependency validation
const (
	MaxModuleDependencyDepth = 10
	MaxModuleDependencyCount = 20
)