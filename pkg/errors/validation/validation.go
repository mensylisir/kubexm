package validation

import (
	"fmt"
	"strings"
)

type ValidationErrors struct {
	errors []string
}

func (v *ValidationErrors) Add(format string, args ...interface{}) {
	v.errors = append(v.errors, fmt.Sprintf(format, args...))
}

func (v *ValidationErrors) AddError(path, message string) {
	v.errors = append(v.errors, fmt.Sprintf("%s: %s", path, message))
}

func (v *ValidationErrors) Error() string {
	if len(v.errors) == 0 {
		return ""
	}
	return strings.Join(v.errors, "\n")
}

func (v *ValidationErrors) HasErrors() bool {
	return len(v.errors) > 0
}

func (v *ValidationErrors) IsEmpty() bool {
	return len(v.errors) == 0
}

func (v *ValidationErrors) Count() int {
	return len(v.errors)
}

// Clear removes all  errors.
func (v *ValidationErrors) Clear() {
	v.errors = nil
}

// GetErrors returns all validation errors as a slice.
func (v *ValidationErrors) GetErrors() []string {
	return v.errors
}

// Network validation functions
