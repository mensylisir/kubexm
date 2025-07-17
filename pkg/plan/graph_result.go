package plan

import (
	"time"
)

type Status string

const (
	StatusPending Status = "Pending"
	StatusRunning Status = "Running"
	StatusSuccess Status = "Success"
	StatusFailed  Status = "Failed"
	StatusSkipped Status = "Skipped"
)

type GraphExecutionResult struct {
	GraphName   string                 `json:"graphName,omitempty" yaml:"graphName,omitempty"`
	StartTime   time.Time              `json:"startTime,omitempty" yaml:"startTime,omitempty"`
	EndTime     time.Time              `json:"endTime,omitempty" yaml:"endTime,omitempty"`
	Status      Status                 `json:"status,omitempty" yaml:"status,omitempty"`
	NodeResults map[NodeID]*NodeResult `json:"nodeResults,omitempty" yaml:"nodeResults,omitempty"`
	Message     string                 `json:"message,omitempty" yaml:"message,omitempty"`
}

func NewGraphExecutionResult(graphName string) *GraphExecutionResult {
	return &GraphExecutionResult{
		GraphName:   graphName,
		StartTime:   time.Now(),
		Status:      StatusPending,
		NodeResults: make(map[NodeID]*NodeResult),
	}
}

func (ger *GraphExecutionResult) Finalize(status Status, message string) {
	ger.EndTime = time.Now()
	ger.Status = status
	ger.Message = message
}

type NodeResult struct {
	NodeName    string                 `json:"nodeName,omitempty" yaml:"nodeName,omitempty"`
	StepName    string                 `json:"stepName,omitempty" yaml:"stepName,omitempty"`
	Status      Status                 `json:"status,omitempty" yaml:"status,omitempty"`
	StartTime   time.Time              `json:"startTime,omitempty" yaml:"startTime,omitempty"`
	EndTime     time.Time              `json:"endTime,omitempty" yaml:"endTime,omitempty"`
	Message     string                 `json:"message,omitempty" yaml:"message,omitempty"`
	HostResults map[string]*HostResult `json:"hostResults,omitempty" yaml:"hostResults,omitempty"`
}

func NewNodeResult(nodeName, stepName string) *NodeResult {
	return &NodeResult{
		NodeName:    nodeName,
		StepName:    stepName,
		Status:      StatusPending,
		StartTime:   time.Now(),
		HostResults: make(map[string]*HostResult),
	}
}

func (nr *NodeResult) AggregateStatus() {
	if len(nr.HostResults) == 0 {
		nr.Status = StatusSuccess
		return
	}

	finalStatus := StatusSuccess
	for _, hr := range nr.HostResults {
		if hr.Status == StatusFailed {
			finalStatus = StatusFailed
			break
		}
		if hr.Status == StatusSkipped && finalStatus != StatusFailed {
		}
	}
	nr.Status = finalStatus
}

type HostResult struct {
	HostName  string    `json:"hostName,omitempty" yaml:"hostName"`
	Status    Status    `json:"status,omitempty" yaml:"status,omitempty"`
	Message   string    `json:"message,omitempty" yaml:"message,omitempty"`
	Stdout    string    `json:"stdout,omitempty" yaml:"stdout,omitempty"`
	Stderr    string    `json:"stderr,omitempty" yaml:"stderr,omitempty"`
	StartTime time.Time `json:"startTime,omitempty" yaml:"startTime,omitempty"`
	EndTime   time.Time `json:"endTime,omitempty" yaml:"endTime,omitempty"`
}

func NewHostResult(hostName string) *HostResult {
	return &HostResult{
		HostName:  hostName,
		Status:    StatusPending,
		StartTime: time.Now(),
	}
}
