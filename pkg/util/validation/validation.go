package validation

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	// "github.com/mensylisir/kubexm/pkg/common" // Temporarily removed
)

// Moved from common to here temporarily to resolve build issues.
// Corrected to use PURE Go raw string literal. NO external quotes or concatenation.
const localValidChartVersionRegexString = ` + "`^v?([0-9]+)(\\.[0-9]+){0,2}$`" + `

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
var validChartVersionRegex = regexp.MustCompile(localValidChartVersionRegexString)

// IsValidChartVersion checks if the version string matches common chart version patterns.
func IsValidChartVersion(version string) bool {
	if version == "latest" || version == "stable" {
		return true
	}
	return validChartVersionRegex.MatchString(version)
}

// IsValidDomainName checks if a string is a valid domain name.
// It uses a regex based on RFC 1035 and RFC 1123.
func IsValidDomainName(domain string) bool {
	// Corrected to use PURE Go raw string literal. NO external quotes or concatenation.
	const domainValidationRegexString = ` + "`^([a-zA-Z0-9]([a-zA-Z0-9\\-]{0,61}[a-zA-Z0-9])?\\.)*([a-zA-Z0-9]([a-zA-Z0-9\\-]{0,61}[a-zA-Z0-9])?)$`" + `
	domainRegex := regexp.MustCompile(domainValidationRegexString)
	return domainRegex.MatchString(domain)
}

// IsValidIP checks if the string is a valid IP address.
func IsValidIP(ipStr string) bool {
	return net.ParseIP(ipStr) != nil
}
