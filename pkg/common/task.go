package common

// Task-related constants for categorization and scheduling

// TaskCategory defines the type of task for classification and scheduling
type TaskCategory string

const (
	// Core system tasks that must run sequentially
	TaskCategoryCore TaskCategory = "core"
	// Resource provisioning tasks (downloads, extracts)
	TaskCategoryResource TaskCategory = "resource"
	// Configuration tasks (files, templates)
	TaskCategoryConfig TaskCategory = "config"
	// Service management tasks (start, stop, restart)
	TaskCategoryService TaskCategory = "service"
	// Validation and verification tasks
	TaskCategoryValidation TaskCategory = "validation"
	// Cleanup and maintenance tasks
	TaskCategoryCleanup TaskCategory = "cleanup"
)

// TaskPriority defines execution priority for scheduling
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

// Task naming and validation constants
const (
	MaxTaskNameLength            = 100
	TaskNameValidCharacters      = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_."
	TaskNameInvalidStartEndChars = "-_."
)
