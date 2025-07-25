package common

const (
	ModuleExecutionSequential  = "sequential"
	ModuleExecutionParallel    = "parallel"
	ModuleExecutionConditional = "conditional"
)

const (
	ModuleStatusPending   = "pending"
	ModuleStatusRunning   = "running"
	ModuleStatusCompleted = "completed"
	ModuleStatusFailed    = "failed"
	ModuleStatusSkipped   = "skipped"
	ModuleStatusCancelled = "cancelled"
)

const (
	ModulePhaseInfrastructure = "infrastructure"
	ModulePhasePreflight      = "preflight"
	ModulePhaseRuntime        = "runtime"
	ModulePhaseKubernetes     = "kubernetes"
	ModulePhaseNetwork        = "network"
	ModulePhaseAddons         = "addons"
	ModulePhaseCleanup        = "cleanup"
)

const (
	DefaultModuleTimeoutMinutes    = 60
	DefaultModuleRetryAttempts     = 2
	DefaultModuleRetryDelaySeconds = 10
	DefaultModuleMaxConcurrency    = 10
)

const (
	MaxModuleNameLength            = 50
	ModuleNameValidCharacters      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
	ModuleNameInvalidStartEndChars = "-_"
)

const (
	MaxModuleDependencyDepth = 10
	MaxModuleDependencyCount = 20
)
