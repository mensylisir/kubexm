package runner

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Value   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error for field '%s' with value '%s': %s", e.Field, e.Value, e.Message)
}

// InputValidator provides validation for runner inputs
type InputValidator struct {
	// Regular expressions for common validation patterns
	hostnameRegex    *regexp.Regexp
	pathRegex        *regexp.Regexp
	packageNameRegex *regexp.Regexp
	serviceNameRegex *regexp.Regexp
	permissionRegex  *regexp.Regexp
	ipAddressRegex   *regexp.Regexp
}

// NewInputValidator creates a new input validator
func NewInputValidator() *InputValidator {
	return &InputValidator{
		hostnameRegex:    regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`),
		pathRegex:        regexp.MustCompile(`^[a-zA-Z0-9/._-]+$`),
		packageNameRegex: regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9+._-]*$`),
		serviceNameRegex: regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`),
		permissionRegex:  regexp.MustCompile(`^[0-7]{3,4}$`),
		ipAddressRegex:   regexp.MustCompile(`^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`),
	}
}

// ValidatePath validates file paths for security
func (v *InputValidator) ValidatePath(path string) error {
	if path == "" {
		return ValidationError{Field: "path", Value: path, Message: "path cannot be empty"}
	}
	
	// Check for directory traversal attempts
	if strings.Contains(path, "..") {
		return ValidationError{Field: "path", Value: path, Message: "path contains directory traversal patterns"}
	}
	
	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return ValidationError{Field: "path", Value: path, Message: "path contains null bytes"}
	}
	
	// Check path length
	if len(path) > 4096 {
		return ValidationError{Field: "path", Value: path, Message: "path is too long (max 4096 characters)"}
	}
	
	// Normalize path
	normalized := filepath.Clean(path)
	if normalized != path && !strings.HasPrefix(path, "./") && !strings.HasPrefix(path, "/") {
		return ValidationError{Field: "path", Value: path, Message: "path contains suspicious patterns"}
	}
	
	return nil
}

// ValidateCommand validates commands for injection attacks
func (v *InputValidator) ValidateCommand(cmd string) error {
	if cmd == "" {
		return ValidationError{Field: "command", Value: cmd, Message: "command cannot be empty"}
	}
	
	// Check command length
	if len(cmd) > 8192 {
		return ValidationError{Field: "command", Value: cmd, Message: "command is too long (max 8192 characters)"}
	}
	
	// Check for null bytes
	if strings.Contains(cmd, "\x00") {
		return ValidationError{Field: "command", Value: cmd, Message: "command contains null bytes"}
	}
	
	// Check for potentially dangerous patterns
	dangerousPatterns := []string{
		"; rm -rf",
		"&& rm -rf",
		"| rm -rf",
		"; dd if=",
		"&& dd if=",
		"| dd if=",
		":(){ :|:& };:",  // Fork bomb
		"/dev/random",
		"/dev/urandom",
		"mkfs.",
		"fdisk",
		"parted",
	}
	
	cmdLower := strings.ToLower(cmd)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(cmdLower, pattern) {
			return ValidationError{Field: "command", Value: cmd, Message: fmt.Sprintf("command contains potentially dangerous pattern: %s", pattern)}
		}
	}
	
	return nil
}

// ValidatePackageName validates package names
func (v *InputValidator) ValidatePackageName(name string) error {
	if name == "" {
		return ValidationError{Field: "package_name", Value: name, Message: "package name cannot be empty"}
	}
	
	if len(name) > 255 {
		return ValidationError{Field: "package_name", Value: name, Message: "package name is too long (max 255 characters)"}
	}
	
	if !v.packageNameRegex.MatchString(name) {
		return ValidationError{Field: "package_name", Value: name, Message: "package name contains invalid characters"}
	}
	
	return nil
}

// ValidateServiceName validates service names
func (v *InputValidator) ValidateServiceName(name string) error {
	if name == "" {
		return ValidationError{Field: "service_name", Value: name, Message: "service name cannot be empty"}
	}
	
	if len(name) > 255 {
		return ValidationError{Field: "service_name", Value: name, Message: "service name is too long (max 255 characters)"}
	}
	
	if !v.serviceNameRegex.MatchString(name) {
		return ValidationError{Field: "service_name", Value: name, Message: "service name contains invalid characters"}
	}
	
	return nil
}

// ValidatePermissions validates file permissions
func (v *InputValidator) ValidatePermissions(perms string) error {
	if perms == "" {
		return ValidationError{Field: "permissions", Value: perms, Message: "permissions cannot be empty"}
	}
	
	if !v.permissionRegex.MatchString(perms) {
		return ValidationError{Field: "permissions", Value: perms, Message: "permissions must be in octal format (e.g., 644, 0755)"}
	}
	
	return nil
}

// ValidateHostname validates hostnames
func (v *InputValidator) ValidateHostname(hostname string) error {
	if hostname == "" {
		return ValidationError{Field: "hostname", Value: hostname, Message: "hostname cannot be empty"}
	}
	
	if len(hostname) > 253 {
		return ValidationError{Field: "hostname", Value: hostname, Message: "hostname is too long (max 253 characters)"}
	}
	
	if !v.hostnameRegex.MatchString(hostname) {
		return ValidationError{Field: "hostname", Value: hostname, Message: "hostname contains invalid characters"}
	}
	
	return nil
}

// ValidateIPAddress validates IP addresses
func (v *InputValidator) ValidateIPAddress(ip string) error {
	if ip == "" {
		return ValidationError{Field: "ip_address", Value: ip, Message: "IP address cannot be empty"}
	}
	
	if !v.ipAddressRegex.MatchString(ip) {
		return ValidationError{Field: "ip_address", Value: ip, Message: "invalid IP address format"}
	}
	
	return nil
}

// ValidateUser validates usernames
func (v *InputValidator) ValidateUser(username string) error {
	if username == "" {
		return ValidationError{Field: "username", Value: username, Message: "username cannot be empty"}
	}
	
	if len(username) > 32 {
		return ValidationError{Field: "username", Value: username, Message: "username is too long (max 32 characters)"}
	}
	
	// Check for valid username characters (alphanumeric, underscore, hyphen)
	for _, char := range username {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '_' && char != '-' {
			return ValidationError{Field: "username", Value: username, Message: "username contains invalid characters"}
		}
	}
	
	// Username should not start with a digit or hyphen
	if unicode.IsDigit(rune(username[0])) || username[0] == '-' {
		return ValidationError{Field: "username", Value: username, Message: "username cannot start with a digit or hyphen"}
	}
	
	return nil
}

// ValidateGroup validates group names
func (v *InputValidator) ValidateGroup(groupname string) error {
	if groupname == "" {
		return ValidationError{Field: "groupname", Value: groupname, Message: "group name cannot be empty"}
	}
	
	if len(groupname) > 32 {
		return ValidationError{Field: "groupname", Value: groupname, Message: "group name is too long (max 32 characters)"}
	}
	
	// Check for valid group name characters (alphanumeric, underscore, hyphen)
	for _, char := range groupname {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '_' && char != '-' {
			return ValidationError{Field: "groupname", Value: groupname, Message: "group name contains invalid characters"}
		}
	}
	
	return nil
}

// SanitizeInput sanitizes input strings
func (v *InputValidator) SanitizeInput(input string) string {
	// Remove null bytes
	input = strings.ReplaceAll(input, "\x00", "")
	
	// Remove control characters except tab, newline, and carriage return
	result := make([]rune, 0, len(input))
	for _, char := range input {
		if unicode.IsControl(char) && char != '\t' && char != '\n' && char != '\r' {
			continue
		}
		result = append(result, char)
	}
	
	return string(result)
}

// SanitizePath sanitizes file paths
func (v *InputValidator) SanitizePath(path string) string {
	// Remove null bytes and control characters
	path = v.SanitizeInput(path)
	
	// Clean the path
	path = filepath.Clean(path)
	
	return path
}

// ValidateEnvironmentVariable validates environment variable names and values
func (v *InputValidator) ValidateEnvironmentVariable(name, value string) error {
	if name == "" {
		return ValidationError{Field: "env_name", Value: name, Message: "environment variable name cannot be empty"}
	}
	
	if len(name) > 255 {
		return ValidationError{Field: "env_name", Value: name, Message: "environment variable name is too long (max 255 characters)"}
	}
	
	// Environment variable names should start with letter or underscore
	if !unicode.IsLetter(rune(name[0])) && name[0] != '_' {
		return ValidationError{Field: "env_name", Value: name, Message: "environment variable name must start with letter or underscore"}
	}
	
	// Check for valid characters in name
	for _, char := range name {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '_' {
			return ValidationError{Field: "env_name", Value: name, Message: "environment variable name contains invalid characters"}
		}
	}
	
	if len(value) > 8192 {
		return ValidationError{Field: "env_value", Value: value, Message: "environment variable value is too long (max 8192 characters)"}
	}
	
	// Check for null bytes in value
	if strings.Contains(value, "\x00") {
		return ValidationError{Field: "env_value", Value: value, Message: "environment variable value contains null bytes"}
	}
	
	return nil
}

// ValidatedRunner wraps a runner with input validation
type ValidatedRunner struct {
	Runner
	validator *InputValidator
}

// NewValidatedRunner creates a new validated runner
func NewValidatedRunner(runner Runner) *ValidatedRunner {
	return &ValidatedRunner{
		Runner:    runner,
		validator: NewInputValidator(),
	}
}

// GetValidator returns the input validator
func (r *ValidatedRunner) GetValidator() *InputValidator {
	return r.validator
}