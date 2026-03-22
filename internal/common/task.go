package common

type TaskCategory string

const (
	TaskCategoryCore       TaskCategory = "core"
	TaskCategoryResource   TaskCategory = "resource"
	TaskCategoryConfig     TaskCategory = "config"
	TaskCategoryService    TaskCategory = "service"
	TaskCategoryValidation TaskCategory = "validation"
	TaskCategoryCleanup    TaskCategory = "cleanup"
)

type TaskPriority int

const (
	TaskPriorityLow      TaskPriority = 1
	TaskPriorityNormal   TaskPriority = 5
	TaskPriorityHigh     TaskPriority = 10
	TaskPriorityCritical TaskPriority = 15
)

type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusRunning    TaskStatus = "running"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusSkipped    TaskStatus = "skipped"
	TaskStatusCancelled  TaskStatus = "cancelled"
	TaskStatusProcessing TaskStatus = "Processing"
	TaskStatusSuccess    TaskStatus = "Success"
)

const (
	DefaultTaskTimeoutMinutes    = 30
	DefaultTaskRetryAttempts     = 3
	DefaultTaskRetryDelaySeconds = 5
	DefaultTaskMemoryMB          = 100
	DefaultTaskCPUPercent        = 10
	DefaultTaskDiskMB            = 50
	DefaultTaskNetworkMBps       = 1
	DefaultTaskMaxConcurrency    = 5
)

const (
	MaxTaskNameLength            = 100
	TaskNameValidCharacters      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_."
	TaskNameInvalidStartEndChars = "-_."
)
