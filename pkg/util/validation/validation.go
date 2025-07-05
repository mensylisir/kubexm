package validation

import (
	"fmt"
	"net/url"
	"strings"
)

// ValidationErrors is a collection of validation errors.
type ValidationErrors struct {
	errors []string
}

// Add records a new validation error.
func (v *ValidationErrors) Add(path, message string) {
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

// IsValidURL checks if a string is a valid URL.
func IsValidURL(u string) bool {
	_, err := url.ParseRequestURI(u)
	return err == nil
}

