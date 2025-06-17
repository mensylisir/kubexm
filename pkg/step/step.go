package step

import (
	"fmt"
	"time"

	"github.com/kubexms/kubexms/pkg/connector" // For connector.CommandError
	"github.com/kubexms/kubexms/pkg/runtime"
)

// Result encapsulates the complete execution result of a single Step on a single host.
type Result struct {
	StepName  string        // Name of the step that was executed.
	HostName  string        // Name of the host where the step was executed.
	Status    string        // Execution status: "Succeeded", "Failed", "Skipped".
	Stdout    string        // Standard output from the step's execution (if any).
	Stderr    string        // Standard error from the step's execution (if any).
	Error     error         // Go-native error if the step failed. This might wrap a connector.CommandError.
	StartTime time.Time     // Timestamp when the step execution began.
	EndTime   time.Time     // Timestamp when the step execution finished.
	Message   string        // Optional additional message from the step.
}

// NewResult is a helper function to create and initialize a Step Result.
// It automatically sets EndTime and determines Status based on the provided error.
// The Step's Run method is responsible for populating Stdout, Stderr, and Message.
func NewResult(stepName, hostName string, startTime time.Time, runErr error) *Result {
	res := &Result{
		StepName:  stepName,
		HostName:  hostName,
		StartTime: startTime,
		EndTime:   time.Now(),
		Error:     runErr,
	}

	if runErr != nil {
		res.Status = "Failed"
		// Note: res.Stdout and res.Stderr should be populated by the specific Step's Run method,
		// as it has direct access to the output from the runner.Command or other operations.
		// This helper primarily sets status based on the error.
	} else {
		res.Status = "Succeeded"
	}
	return res
}

// Step defines the interface for an atomic, idempotent operation within a task.
// Each Step represents a single, well-defined action to be performed on a host.
type Step interface {
	// Name returns a descriptive name for the step.
	// This name is used for logging and identification.
	// E.g., "Install etcd binaries", "Check CPU core count".
	Name() string

	// Check determines if the Step's intended state has already been achieved.
	// If Check returns (true, nil), the Step's Run method will be skipped (idempotency).
	// If Check returns (false, nil), the Run method will be executed.
	// If Check returns an error, it indicates a problem determining the state,
	// and the execution flow might be halted or the error handled by the Task engine.
	Check(ctx *runtime.Context) (isDone bool, err error)

	// Run executes the primary logic of the Step.
	// It should perform the action to reach the desired state.
	// It returns a Result object detailing the outcome of the execution,
	// including status, stdout/stderr (if applicable), and any Go error encountered.
	// The Step's Run method is responsible for populating Result.Stdout and Result.Stderr.
	Run(ctx *runtime.Context) *Result

	// TODO: Consider adding an `IgnoreError() bool` method to the interface
	// if some steps' failures can be non-critical to a task.
	// Or this can be a property of the Step struct itself.
}
