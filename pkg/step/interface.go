package step

import (
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime" // For StepContext
)

// Step defines the interface that all concrete steps must implement.
// Steps are designed to be idempotent.
type Step interface {
	// Name returns a unique identifier for the step type.
	Name() string

	// Description provides a human-readable summary of what the step does.
	Description() string

	// Precheck determines if the step's conditions are already met or if execution is required.
	// It is called by the Engine before Run.
	// - ctx: The StepContext providing access to runtime services and host-specific info.
	// - host: The target host for this precheck.
	// Returns:
	//   - bool: true if the step is considered done/skipped, false if Run needs to be called.
	//   - error: Any error encountered during the precheck. If an error occurs,
	//            the step is generally considered failed and Run will not be called.
	Precheck(ctx runtime.StepContext, host connector.Host) (bool, error)

	// Run executes the primary logic of the step.
	// It is called by the Engine if Precheck returns false and no error.
	// - ctx: The StepContext providing access to runtime services and host-specific info.
	// - host: The target host for this execution.
	// Returns:
	//   - error: Any error encountered during execution. If an error occurs,
	//            the Engine may attempt to call Rollback.
	Run(ctx runtime.StepContext, host connector.Host) error

	// Rollback attempts to revert any changes made by the Run method.
	// It is called by the Engine if Run returns an error.
	// Implementations should be idempotent.
	// - ctx: The StepContext providing access to runtime services and host-specific info.
	// - host: The target host for this rollback.
	// Returns:
	//   - error: Any error encountered during rollback.
	Rollback(ctx runtime.StepContext, host connector.Host) error
}
