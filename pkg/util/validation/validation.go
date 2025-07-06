package validation

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/mensylisir/kubexm/pkg/common" // Import common package
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

// validChartVersionRegex is the compiled regular expression for chart versions.
// It's kept private as it's only used by IsValidChartVersion.
var validChartVersionRegex = regexp.MustCompile(common.ValidChartVersionRegexString)

// IsValidChartVersion checks if the version string matches common chart version patterns.
// Allows "latest", "stable", or versions like "1.2.3", "v1.2.3", "1.2", "v1.0", "1", "v2"
// as defined by common.ValidChartVersionRegexString.
func IsValidChartVersion(version string) bool {
	if version == "latest" || version == "stable" {
		return true
	}
	return validChartVersionRegex.MatchString(version)
}
