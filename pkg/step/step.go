package step

import (
	"github.com/mensylisir/kubexm/pkg/connector"
	// engine import removed, StepContext is now local to pkg/step
	"github.com/mensylisir/kubexm/pkg/spec"
)

// NoOpStep provides a base implementation for steps that might not need
// to implement all methods of the Step interface (e.g., no rollback).
// Concrete steps can embed NoOpStep to inherit default no-op behaviors.
type NoOpStep struct{}

// Meta should be implemented by the embedding struct.
// This default implementation returns an empty StepMeta, which is generally not useful.
// It's a placeholder to satisfy the interface if a step forgets to implement it,
// though linters would ideally catch an unimplemented interface method.
func (s *NoOpStep) Meta() *spec.StepMeta {
	return &spec.StepMeta{
		Name:        "NoOpStep",
		Description: "This is a no-operation step.",
	}
}

// Precheck by default indicates the step needs to run.
// Returns false (not done), nil (no error).
func (s *NoOpStep) Precheck(ctx StepContext, host connector.Host) (bool, error) { // Changed to StepContext
	return false, nil
}

// Run by default does nothing and succeeds.
// Returns nil (no error).
func (s *NoOpStep) Run(ctx StepContext, host connector.Host) error { // Changed to StepContext
	return nil
}

// Rollback by default does nothing and succeeds.
// Returns nil (no error).
func (s *NoOpStep) Rollback(ctx StepContext, host connector.Host) error { // Changed to StepContext
	return nil
}

// Ensure NoOpStep implements the Step interface.
var _ Step = (*NoOpStep)(nil)
