package runtime

import (
	"fmt"
	"strings"
)

// InitializationError represents an error that occurred during the
// initialization phase of the ClusterRuntime. It can hold multiple
// underlying errors, as host initializations may happen concurrently.
// Deprecated: Error handling is now part of the builder methods (`BuildFromFile`, `BuildFromConfig` in `pkg/runtime/builder.go`).
// These builder methods return a single error, which can be a wrapped error containing multiple issues if necessary.
type InitializationError struct {
	SubErrors []error
}

// Error returns a string representation of the InitializationError.
// Deprecated: Part of the deprecated InitializationError type.
// If there's only one sub-error, it's presented directly.
// If there are multiple, it summarizes the count and lists them.
func (e *InitializationError) Error() string {
	if len(e.SubErrors) == 0 {
		return "no initialization errors"
	}
	if len(e.SubErrors) == 1 {
		return fmt.Sprintf("runtime initialization failed: %v", e.SubErrors[0])
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("runtime initialization failed with %d errors:\n", len(e.SubErrors)))
	for i, err := range e.SubErrors {
		sb.WriteString(fmt.Sprintf("  [%d] %v\n", i+1, err))
	}
	return sb.String()
}

// Add appends a non-nil error to the SubErrors slice.
// Deprecated: Part of the deprecated InitializationError type.
func (e *InitializationError) Add(err error) {
	if err != nil {
		e.SubErrors = append(e.SubErrors, err)
	}
}

// IsEmpty returns true if there are no sub-errors.
// Deprecated: Part of the deprecated InitializationError type.
func (e *InitializationError) IsEmpty() bool {
	return len(e.SubErrors) == 0
}

// Unwrap provides compatibility for `errors.Is` and `errors.As`.
// Deprecated: Part of the deprecated InitializationError type.
// It returns the first error in the list, or nil if the list is empty.
// For more sophisticated error unwrapping with multiple errors, a custom
// implementation or a library might be needed, but this is a common approach
// for basic unwrapping.
func (e *InitializationError) Unwrap() error {
	if len(e.SubErrors) == 0 {
		return nil
	}
	// Return the first error for basic unwrapping.
	// To support unwrapping all errors, one would need to implement an `Unwrap() []error`
	// method, which is not standard but possible if using custom error handling utilities.
	return e.SubErrors[0]
}
