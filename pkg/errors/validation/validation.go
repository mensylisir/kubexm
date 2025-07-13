package validation

import (
	"fmt"
	"strings"
)

type ValidationErrors struct {
	errors []string
}

// Add records a new validation error with formatted message.
func (v *ValidationErrors) Add(format string, args ...interface{}) {
	v.errors = append(v.errors, fmt.Sprintf(format, args...))
}

// AddError records a new validation error.
func (v *ValidationErrors) AddError(path, message string) {
	v.errors = append(v.errors, fmt.Sprintf("%s: %s", path, message))
}

// Error returns the formatted error string.
func (v *ValidationErrors) Error() string {
	if len(v.errors) == 0 {
		return ""
	}
	return strings.Join(v.errors, "\n")
}

// HasErrors returns true if there are any validation errors.
func (v *ValidationErrors) HasErrors() bool {
	return len(v.errors) > 0
}

// IsEmpty returns true if there are no validation errors.
func (v *ValidationErrors) IsEmpty() bool {
	return len(v.errors) == 0
}

// Count returns the number of validation errors.
func (v *ValidationErrors) Count() int {
	return len(v.errors)
}

// Clear removes all validation errors.
func (v *ValidationErrors) Clear() {
	v.errors = nil
}

// GetErrors returns all validation errors as a slice.
func (v *ValidationErrors) GetErrors() []string {
	return v.errors
}

// Network validation functions
