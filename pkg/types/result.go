package types

import (
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// ===================================================================
// Step Result Types
// ===================================================================

// StepStatus represents the execution status of a step
type StepStatus string

const (
	StepStatusPending    StepStatus = "pending"
	StepStatusRunning    StepStatus = "running"
	StepStatusCompleted  StepStatus = "completed"
	StepStatusFailed     StepStatus = "failed"
	StepStatusSkipped    StepStatus = "skipped"
	StepStatusRolledBack StepStatus = "rolled_back"
)

type StepResult struct {
	StepName     string                 `json:"step_name"`
	ExecutionID  string                 `json:"execution_id"`
	Status       StepStatus             `json:"status"`
	StartTime    time.Time              `json:"start_time"`
	EndTime      time.Time              `json:"end_time"`
	Duration     time.Duration          `json:"duration"`
	Host         connector.Host         `json:"-"`
	HostName     string                 `json:"host_name"`
	Message      string                 `json:"message,omitempty"`
	Error        string                 `json:"error,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Artifacts    []string               `json:"artifacts,omitempty"`
	PreCheckDone bool                   `json:"precheck_done"`
	RollbackDone bool                   `json:"rollback_done,omitempty"`
}

func NewStepResult(stepName, executionID string, host connector.Host) *StepResult {
	return &StepResult{
		StepName:    stepName,
		ExecutionID: executionID,
		Status:      StepStatusPending,
		StartTime:   time.Now(),
		Host:        host,
		HostName:    host.GetName(),
		Metadata:    make(map[string]interface{}),
		Artifacts:   make([]string, 0),
	}
}

func (r *StepResult) MarkCompleted(message string) {
	r.Status = StepStatusCompleted
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime)
	r.Message = message
}

func (r *StepResult) MarkFailed(err error, message string) {
	r.Status = StepStatusFailed
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime)
	r.Error = err.Error()
	r.Message = message
}

func (r *StepResult) MarkSkipped(reason string) {
	r.Status = StepStatusSkipped
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime)
	r.Message = reason
	r.PreCheckDone = true
}

func (r *StepResult) MarkRunning() {
	r.Status = StepStatusRunning
}

func (r *StepResult) MarkRolledBack(message string) {
	r.Status = StepStatusRolledBack
	r.EndTime = time.Now()
	r.Duration = r.EndTime.Sub(r.StartTime)
	r.Message = message
	r.RollbackDone = true
}

func (r *StepResult) AddArtifact(path string) {
	r.Artifacts = append(r.Artifacts, path)
}

func (r *StepResult) SetMetadata(key string, value interface{}) {
	r.Metadata[key] = value
}

func (r *StepResult) GetMetadata(key string) (interface{}, bool) {
	value, exists := r.Metadata[key]
	return value, exists
}

type StepMetrics struct {
	ExecutionCount    int64         `json:"execution_count"`
	TotalDuration     time.Duration `json:"total_duration"`
	AverageDuration   time.Duration `json:"average_duration"`
	SuccessCount      int64         `json:"success_count"`
	FailureCount      int64         `json:"failure_count"`
	SkippedCount      int64         `json:"skipped_count"`
	LastExecutionTime time.Time     `json:"last_execution_time"`
}

type StepCategory string

const (
	CategoryPreparation   StepCategory = "preparation"
	CategoryInstallation  StepCategory = "installation"
	CategoryConfiguration StepCategory = "configuration"
	CategoryValidation    StepCategory = "validation"
	CategoryCleanup       StepCategory = "cleanup"
	CategoryMaintenance   StepCategory = "maintenance"
)

type StepPriority int

const (
	PriorityLow      StepPriority = 0
	PriorityNormal   StepPriority = 1
	PriorityHigh     StepPriority = 2
	PriorityCritical StepPriority = 3
)

type StepExecutionOptions struct {
	MaxRetries        int           `json:"max_retries"`
	RetryDelay        time.Duration `json:"retry_delay"`
	Timeout           time.Duration `json:"timeout"`
	ContinueOnFailure bool          `json:"continue_on_failure"`

	MaxConcurrency    int  `json:"max_concurrency"`
	RequiresExclusive bool `json:"requires_exclusive"`

	Dependencies  []string `json:"dependencies"`
	ConflictsWith []string `json:"conflicts_with"`

	RequiredMemoryMB    int64 `json:"required_memory_mb"`
	RequiredDiskSpaceMB int64 `json:"required_disk_space_mb"`
	RequiredNetworkMbps int64 `json:"required_network_mbps"`
}
