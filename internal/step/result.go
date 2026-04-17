package step

// ===================================================================
// Step Result Types
// ===================================================================
//
// This file re-exports types from internal/types for backward compatibility.
// All step-related types should be defined in internal/types/result.go
//
// Import path: github.com/mensylisir/kubexm/internal/types
// ===================================================================

import (
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/types"
)

// Re-export StepResult and related types for backward compatibility
type StepResult = types.StepResult
type StepStatus = types.StepStatus
type StepMetrics = types.StepMetrics
type StepCategory = types.StepCategory
type StepPriority = types.StepPriority
type StepExecutionOptions = types.StepExecutionOptions

// Re-export constants
const (
	StepStatusPending    = types.StepStatusPending
	StepStatusRunning    = types.StepStatusRunning
	StepStatusCompleted  = types.StepStatusCompleted
	StepStatusFailed     = types.StepStatusFailed
	StepStatusSkipped    = types.StepStatusSkipped
	StepStatusRolledBack = types.StepStatusRolledBack

	CategoryPreparation   = types.CategoryPreparation
	CategoryInstallation  = types.CategoryInstallation
	CategoryConfiguration = types.CategoryConfiguration
	CategoryValidation    = types.CategoryValidation
	CategoryCleanup       = types.CategoryCleanup
	CategoryMaintenance   = types.CategoryMaintenance

	PriorityLow      = types.PriorityLow
	PriorityNormal   = types.PriorityNormal
	PriorityHigh     = types.PriorityHigh
	PriorityCritical = types.PriorityCritical
)

// Re-export constructor
func NewStepResult(stepName, executionID string, host remotefw.Host) *StepResult {
	return types.NewStepResult(stepName, executionID, host)
}
