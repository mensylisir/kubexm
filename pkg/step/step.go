package step

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	// "github.com/kubexms/kubexms/pkg/connector" // For step.Result if it references CommandError - not directly needed here
	"github.com/kubexms/kubexms/pkg/runtime" // Now directly used in StepExecutor interface
	"github.com/kubexms/kubexms/pkg/spec"    // Import the new spec package
)

// Status types for Step Result
const (
	StatusSucceeded = "Succeeded"
	StatusFailed    = "Failed"
	StatusSkipped   = "Skipped" // If Check returns true, or other skip conditions
)

// Result encapsulates the complete execution result of a single Step on a single host.
type Result struct {
	StepName  string
	HostName  string
	Status    string // "Succeeded", "Failed", "Skipped"
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
	if ok {
		stepName = stepSpec.GetName()
	}

	hostName := "localhost" // Default if not a remote execution context
	if ctx.Host != nil && ctx.Host.Name != "" {
		hostName = ctx.Host.Name
	}

	// If ctx itself or essential parts for logging/identification are nil,
	// a more robust fallback might be needed, or panic if context integrity is critical.
	// For now, proceeding with defaults.

	return &Result{
		StepName:  stepName,
		HostName:  hostName,
		StartTime: startTime,
		EndTime:   time.Now(), // EndTime is set when result is created
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
// This is typically called from the init() function of the package defining the StepExecutor
// and its corresponding StepSpec.
// The specTypeName should be a unique string identifier for the StepSpec type.
// Using GetSpecTypeName(new(ConcreteStepSpecType)) is a recommended way to generate this name.
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
// It returns nil if no executor is registered for that type.
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
// This is commonly used as the key for the executor registry.
// It uses the pointer type name (e.g., "*command.CommandStepSpec") to ensure uniqueness
// across packages, assuming StepSpec instances are typically pointers to structs.
// If a StepSpec is a value type, reflect.TypeOf(spec).String() would be "command.CommandStepSpec".
// Using pointer type name is generally safer for registry keys if specs are passed as pointers.
func GetSpecTypeName(s spec.StepSpec) string {
	if s == nil {
		return ""
	}
	// reflect.TypeOf(s).String() will give e.g., "*command.CommandStepSpec" if s is a pointer,
	// or "command.CommandStepSpec" if s is a value.
	// Using the string representation of the type is a common pattern for type registries.
	return reflect.TypeOf(s).String()
}
