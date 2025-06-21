package plan

import (
	"sync"
	"time"
	// Adjust these import paths based on your actual project structure
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/step"
)

type ExecutionPlan struct {
	Phases []Phase
}

type Phase struct {
	Name    string
	Actions []Action
}

type Action struct {
	Name  string
	Step  step.Step // This refers to the step.Step interface
	Hosts []connector.Host // This refers to the connector.Host interface
}

type Status string

const (
	StatusPending   Status = "Pending"
	StatusRunning   Status = "Running"
	StatusSuccess   Status = "Success"
	StatusFailed    Status = "Failed"
	StatusSkipped   Status = "Skipped"
)

type ExecutionResult struct {
	sync.RWMutex // RWMutex for concurrent access if top-level result is shared
	StartTime    time.Time
	EndTime      time.Time
	Status       Status
	PhaseResults []*PhaseResult
}

type PhaseResult struct {
	sync.RWMutex // RWMutex for concurrent updates to this phase (e.g., actions finishing)
	PhaseName     string
	Status        Status
	ActionResults []*ActionResult
}

type ActionResult struct {
	// No RWMutex here by default; updates are typically serialized by PhaseResult's lock
	// or handled if multiple hosts for one action update this concurrently (then it would need a lock).
	// For V11 spec, assuming no lock here.
	ActionName  string
	Status      Status
	HostResults map[string]*HostResult // Key: host.GetName()
}

type HostResult struct {
	// No RWMutex here by default.
	HostName string // Should match host.GetName()
	Status   Status
	Message  string // Error message or success details
	Stdout   string
	Stderr   string
	Skipped  bool   // Explicitly true if the step was skipped on this host
}
