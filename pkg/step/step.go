package step

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime" // Ensure this is the correct path
	"github.com/mensylisir/kubexm/pkg/spec"
)

// Status types for Step Result
const (
	StatusSucceeded = "Succeeded"
	StatusFailed    = "Failed"
	StatusSkipped   = "Skipped"
)

// Result encapsulates the complete execution result of a single Step on a single host.
type Result struct {
	StepName  string
	HostName  string
	Status    string
	Stdout    string
	Stderr    string
	Error     error
	StartTime time.Time
	EndTime   time.Time
	Message   string
}

// determineStatus is a helper to set status based on error.
func determineStatus(err error) string {
	if err == nil {
		return StatusSucceeded
	}
	return StatusFailed
}

// NewResult is a helper function to create and initialize a Step Result.
// It now derives StepName and HostName from the runtime.Context.
func NewResult(ctx runtime.Context, startTime time.Time, executionError error) *Result {
	stepSpec, ok := ctx.Step().GetCurrentStepSpec()
	stepName := "UnknownStep (Spec not found in context)"
	if ok && stepSpec != nil { // Added nil check for stepSpec
		stepName = stepSpec.GetName()
	}

	hostName := "localhost" // Default if not a remote execution context
	if ctx.Host != nil && ctx.Host.Name != "" {
		hostName = ctx.Host.Name
	}

	return &Result{
		StepName:  stepName,
		HostName:  hostName,
		StartTime: startTime,
		EndTime:   time.Now(),
		Error:     executionError,
		Status:    determineStatus(executionError),
	}
}

// StepExecutor defines the interface for a step that can be executed.
type StepExecutor interface {
	// Check determines if the step needs to be executed.
	// It should be idempotent.
	// The step's specific configuration (Spec) can be retrieved from ctx.Step().GetCurrentStepSpec().
	Check(ctx runtime.Context) (isDone bool, err error)

	// Execute performs the action of the step.
	// The step's specific configuration (Spec) can be retrieved from ctx.Step().GetCurrentStepSpec().
	Execute(ctx runtime.Context) *Result
}

// --- Step Executor Registry ---

var (
	executorsMu sync.RWMutex
	executors   = make(map[string]StepExecutor)
)

// Register associates a StepExecutor with a specific StepSpec type name.
func Register(specTypeName string, executor StepExecutor) {
	executorsMu.Lock()
	defer executorsMu.Unlock()
	if executor == nil {
		panic("step: Register executor is nil")
	}
	if specTypeName == "" {
		panic("step: Register specTypeName cannot be empty")
	}
	if _, dup := executors[specTypeName]; dup {
		panic("step: Register called twice for executor " + specTypeName)
	}
	executors[specTypeName] = executor
}

// GetExecutor retrieves the StepExecutor registered for the given specTypeName.
func GetExecutor(specTypeName string) StepExecutor {
	executorsMu.RLock()
	defer executorsMu.RUnlock()
	executor, ok := executors[specTypeName]
	if !ok {
		return nil
	}
	return executor
}

// GetSpecTypeName generates a string representation for a StepSpec type.
func GetSpecTypeName(s spec.StepSpec) string {
	if s == nil {
		return ""
	}
	return reflect.TypeOf(s).String()
}
