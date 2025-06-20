package step

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector" // Added for NewResult
	"github.com/mensylisir/kubexm/pkg/runtime"   // Ensure this is the correct path for StepContext
	"github.com/mensylisir/kubexm/pkg/spec"      // Already present, used for StepSpec
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
func NewResult(ctx runtime.StepContext, host connector.Host, startTime time.Time, executionError error) *Result {
	stepName := "UnknownStep (Spec not found or incorrect type in context)"
	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec() // Use StepCache from StepContext
	if ok {
		if typedSpec, assertOK := rawSpec.(spec.StepSpec); assertOK && typedSpec != nil {
			stepName = typedSpec.GetName()
		}
	}

	currentHostName := "localhost" // Default for local steps or if host is nil
	if host != nil {
		currentHostName = host.GetName()
	}

	return &Result{
		StepName:  stepName,
		HostName:  currentHostName,
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
	// The step's specific configuration (Spec) can be retrieved from ctx.StepCache().GetCurrentStepSpec().
	Check(ctx runtime.StepContext) (isDone bool, err error)

	// Execute performs the action of the step.
	// The step's specific configuration (Spec) can be retrieved from ctx.StepCache().GetCurrentStepSpec().
	Execute(ctx runtime.StepContext) *Result
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
