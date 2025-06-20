package plan

import (
	"sync"
	"time"
	"github.com/mensylisir/kubexm/pkg/connector" // Updated import path
	"github.com/mensylisir/kubexm/pkg/step"    // Updated import path
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
	Step  step.Step
	Hosts []connector.Host // Assuming connector.Host is the intended type
}

type Status string

const (
	StatusPending Status = "Pending"
	StatusRunning Status = "Running"
	StatusSuccess Status = "Success"
	StatusFailed  Status = "Failed"
	StatusSkipped Status = "Skipped"
)

type ExecutionResult struct {
	sync.RWMutex
	StartTime    time.Time
	EndTime      time.Time
	Status       Status
	PhaseResults []*PhaseResult
}

type PhaseResult struct {
	sync.RWMutex
	PhaseName     string
	Status        Status
	ActionResults []*ActionResult
}

type ActionResult struct {
	// No RWMutex here, typically managed by PhaseResult or ExecutionResult if needed at this level
	ActionName  string
	Status      Status
	HostResults map[string]*HostResult // Keyed by host name
}

type HostResult struct {
	// No RWMutex here, typically managed by ActionResult
	HostName string // Redundant if key in map, but can be useful for direct access
	Status   Status
	Message  string
	Stdout   string
	Stderr   string
	Skipped  bool // Explicit field for skipped status often helps in UI/reporting
}

// Helper methods for ExecutionResult (optional, but can be useful)

func NewExecutionResult() *ExecutionResult {
	return &ExecutionResult{
		StartTime:    time.Now(),
		Status:       StatusPending,
		PhaseResults: []*PhaseResult{},
	}
}

func (er *ExecutionResult) AddPhaseResult(pr *PhaseResult) {
	er.Lock()
	defer er.Unlock()
	er.PhaseResults = append(er.PhaseResults, pr)
}

func (er *ExecutionResult) SetStatus(s Status) {
	er.Lock()
	defer er.Unlock()
	er.Status = s
}

func (er *ExecutionResult) Finalize() {
	er.Lock()
	defer er.Unlock()
	er.EndTime = time.Now()
	// Determine overall status based on phase results if not already failed
	if er.Status != StatusFailed {
		overallStatus := StatusSuccess
		for _, pr := range er.PhaseResults {
			if pr.Status == StatusFailed {
				overallStatus = StatusFailed
				break
			}
			if pr.Status == StatusSkipped && overallStatus != StatusFailed {
				// If something succeeded and something else was skipped, it's still success overall.
				// If all were skipped, it's skipped. This logic might need refinement based on exact needs.
			}
		}
		er.Status = overallStatus
	}
}

// Helper methods for PhaseResult

func NewPhaseResult(name string) *PhaseResult {
	return &PhaseResult{
		PhaseName:     name,
		Status:        StatusPending,
		ActionResults: []*ActionResult{},
	}
}

func (pr *PhaseResult) AddActionResult(ar *ActionResult) {
	pr.Lock()
	defer pr.Unlock()
	pr.ActionResults = append(pr.ActionResults, ar)
}

func (pr *PhaseResult) SetStatus(s Status) {
	pr.Lock()
	defer pr.Unlock()
	pr.Status = s
}

func (pr *PhaseResult) Finalize() {
	pr.Lock()
	defer pr.Unlock()
	// Determine phase status based on action results if not already failed
	if pr.Status != StatusFailed {
		currentStatus := StatusSuccess // Assume success unless an action failed
		allSkipped := true
		for _, ar := range pr.ActionResults {
			if ar.Status == StatusFailed {
				currentStatus = StatusFailed
				allSkipped = false
				break
			}
			if ar.Status != StatusSkipped {
				allSkipped = false
			}
		}
		if currentStatus != StatusFailed && allSkipped && len(pr.ActionResults) > 0 {
			pr.Status = StatusSkipped
		} else {
			pr.Status = currentStatus
		}
	}
}


// Helper for ActionResult

func NewActionResult(name string) *ActionResult {
	return &ActionResult{
		ActionName:  name,
		Status:      StatusPending,
		HostResults: make(map[string]*HostResult),
	}
}

func (ar *ActionResult) SetHostResult(hostName string, hr *HostResult) {
	// Assuming ActionResult itself is not locked, but its parent (PhaseResult) is.
	// If direct concurrent access to ActionResult.HostResults is possible, it needs a mutex.
	// For simplicity here, assuming coarse-grained locking at PhaseResult or ExecutionResult level
	// when adding these. If fine-grained is needed, ActionResult would also need a RWMutex.
	ar.HostResults[hostName] = hr
}

func (ar *ActionResult) Finalize() {
    // Determine action status based on host results if not already failed
    if ar.Status != StatusFailed {
        currentStatus := StatusSuccess
        allSkipped := true
        for _, hr := range ar.HostResults {
            if hr.Status == StatusFailed {
                currentStatus = StatusFailed
                allSkipped = false
                break
            }
            if !hr.Skipped { // or hr.Status != StatusSkipped
                allSkipped = false
            }
        }
        if currentStatus != StatusFailed && allSkipped && len(ar.HostResults) > 0 {
            ar.Status = StatusSkipped
        } else {
            ar.Status = currentStatus
        }
    }
}
