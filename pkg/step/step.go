package step

import (
	"fmt"
	"reflect"
	"sync"
	"time"

	// "github.com/kubexms/kubexms/pkg/connector" // For step.Result if it references CommandError - not directly needed here
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/spec"     // Import the new spec package
)

// Result encapsulates the complete execution result of a single Step on a single host.
// (This struct remains largely the same as previously defined)
type Result struct {
	StepName  string
	HostName  string
	Status    string        // "Succeeded", "Failed", "Skipped"
	Stdout    string
	Stderr    string
	Error     error
	StartTime time.Time
	EndTime   time.Time
	Message   string
}

// NewResult is a helper function to create and initialize a Step Result.
// (This function remains largely the same)
// EndTime is typically updated by the executor after the StepExecutor's Execute method finishes.
func NewResult(stepName, hostName string, startTime time.Time, runErr error) *Result {
	res := &Result{
		StepName:  stepName,
		HostName:  hostName,
		StartTime: startTime,
		EndTime:   time.Now(), // Initial EndTime, can be updated by caller
		Error:     runErr,
	}
	if runErr != nil {
		res.Status = "Failed"
	} else {
		res.Status = "Succeeded"
	}
	return res
}


// StepExecutor defines the interface for executing the logic of a specific StepSpec.
// Each type of StepSpec (e.g., command.CommandStepSpec, preflight.CheckCPUStepSpec) will have a
// corresponding implementation of StepExecutor.
type StepExecutor interface {
	// Check determines if the operation defined by the spec has already been completed
	// or if its conditions are already met on the target host.
	//
	// Parameters:
	//   - s: The specific StepSpec instance containing the parameters for this check.
	//   - ctx: The runtime context providing access to the host's runner and logger.
	//
	// Returns:
	//   - isDone: True if the step's goal is already achieved and Execute should be skipped.
	//   - err: An error if the check itself failed (e.g., could not query state).
	Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error)

	// Execute performs the primary action of the step as defined by the spec.
	//
	// Parameters:
	//   - s: The specific StepSpec instance containing the parameters for this execution.
	//   - ctx: The runtime context providing access to the host's runner and logger.
	//
	// Returns:
	//   - *Result: A detailed result of the execution, including status, output, and any errors.
	Execute(s spec.StepSpec, ctx *runtime.Context) *Result
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
