package plan

import (
	"time"
)

// Status remains the same as it's a universal concept.
type Status string

const (
	StatusPending Status = "Pending"
	StatusRunning Status = "Running"
	StatusSuccess Status = "Success"
	StatusFailed  Status = "Failed"
	StatusSkipped Status = "Skipped" // A node can be skipped if its dependencies fail.
)

// GraphExecutionResult is the top-level report for a graph-based execution.
type GraphExecutionResult struct {
	GraphName    string                 `json:"graphName"`
	StartTime    time.Time              `json:"startTime"`
	EndTime      time.Time              `json:"endTime"`
	Status       Status                 `json:"status"`
	NodeResults  map[NodeID]*NodeResult `json:"nodeResults"`
	// Consider adding an overall error message for the graph execution itself
	// ErrorMessage string                 `json:"errorMessage,omitempty"`
}

// NewGraphExecutionResult creates a new GraphExecutionResult with default values.
func NewGraphExecutionResult(graphName string) *GraphExecutionResult {
	return &GraphExecutionResult{
		GraphName:   graphName,
		StartTime:   time.Now(),      // Set StartTime on creation
		Status:      StatusPending,   // Default to Pending
		NodeResults: make(map[NodeID]*NodeResult),
	}
}

// NodeResult captures the outcome of a single ExecutionNode's execution.
// It's equivalent to the old ActionResult.
type NodeResult struct {
	NodeName    string                 `json:"nodeName"` // Should match ExecutionNode.Name
	StepName    string                 `json:"stepName"` // Should match ExecutionNode.StepName
	Status      Status                 `json:"status"`
	StartTime   time.Time              `json:"startTime"`
	EndTime     time.Time              `json:"endTime"`
	Message     string                 `json:"message,omitempty"` // e.g., "Skipped due to failed dependency 'node-X'"
	HostResults map[string]*HostResult `json:"hostResults"`     // Keyed by HostName. This structure remains the same.
}

// HostResult captures the outcome of a single step on a single host.
// This structure is fundamental and does not need to change.
type HostResult struct {
	HostName  string    `json:"hostName"`
	Status    Status    `json:"status"`
	Message   string    `json:"message"` // Can include Stdout/Stderr or a summary
	Stdout    string    `json:"stdout,omitempty"`
	Stderr    string    `json:"stderr,omitempty"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Skipped   bool      `json:"skipped"` // Skipped due to precheck, not dependency failure.
}
