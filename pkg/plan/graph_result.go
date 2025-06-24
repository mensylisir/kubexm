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
	// ErrorMessage provides a summary if the overall graph execution failed (e.g., engine error, validation error).
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// NewGraphExecutionResult creates a new result object for a graph execution.
func NewGraphExecutionResult(graphName string) *GraphExecutionResult {
	return &GraphExecutionResult{
		GraphName:   graphName,
		StartTime:   time.Now(),      // Set StartTime on creation
		Status:      StatusPending,   // Default to Pending
		NodeResults: make(map[NodeID]*NodeResult),
	}
}

// Finalize sets the end time and overall status of the graph execution.
func (ger *GraphExecutionResult) Finalize(status Status, message string) {
	ger.EndTime = time.Now()
	ger.Status = status
	ger.ErrorMessage = message // Aligning with existing field name
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

// NewNodeResult creates a new NodeResult with default values.
func NewNodeResult(nodeName, stepName string) *NodeResult {
	return &NodeResult{
		NodeName:    nodeName,
		StepName:    stepName,
		Status:      StatusPending, // Default to Pending
		StartTime:   time.Now(),    // Set StartTime on creation for active nodes
		HostResults: make(map[string]*HostResult),
	}
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

// NewHostResult creates a new HostResult with default values.
func NewHostResult(hostName string) *HostResult {
	return &HostResult{
		HostName:  hostName,
		Status:    StatusPending, // Default to Pending
		StartTime: time.Now(),    // Set StartTime on creation
		Skipped:   false,
	}
}
