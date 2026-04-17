package errors

import (
	"errors"
	"fmt"

	"github.com/mensylisir/kubexm/internal/connector"
)

// Kind classifies the severity and recoverability of an error.
// This drives how the Runner and Pipeline handle the error.
type Kind string

const (
	// KindFatal means the error is unrecoverable and should halt execution immediately.
	// Examples: invalid configuration, missing required binaries, SSH handshake failure.
	KindFatal Kind = "fatal"

	// KindRetryable means the operation can be retried and may succeed on subsequent attempts.
	// Examples: network timeout, service not yet started, transient resource contention.
	KindRetryable Kind = "retryable"

	// KindSkip means the step was not applicable in the current context (e.g., already done,
	// feature disabled, wrong environment). This is not an error condition.
	// Examples: service already running, firewall already disabled, package already installed.
	KindSkip Kind = "skip"

	// KindValidation means the error is a configuration or input validation failure.
	// Examples: missing required field, invalid IP address format, unsupported OS version.
	KindValidation Kind = "validation"
)

// String implements fmt.Stringer for Kind.
func (k Kind) String() string { return string(k) }

// ErrorKind returns KindRetryable if the error chain contains a retryable sentinel,
// KindSkip if the chain contains a skip sentinel, KindValidation for validation errors,
// and KindFatal by default.
func ErrorKind(err error) Kind {
	if err == nil {
		return KindFatal // nil is treated as fatal
	}

	// Check for sentinel skip errors (unwrap chain)
	var skiperr *skipError
	if errors.As(err, &skiperr) {
		return KindSkip
	}

	// Check for sentinel retryable errors
	var retryerr *retryError
	if errors.As(err, &retryerr) {
		return KindRetryable
	}

	// Validation errors use errors.ValidationError or custom validators
	var validationErr *ValidationError
	if errors.As(err, &validationErr) {
		return KindValidation
	}

	// Command errors with specific exit codes
	var cmdErr *connector.CommandError
	if errors.As(err, &cmdErr) {
		// Exit code 0 with non-zero expected means "already done" — treat as skip
		if cmdErr.ExitCode == 0 {
			return KindSkip
		}
		// Non-zero exit codes are generally fatal (command failed), but could be
		// retryable if the command is idempotent (e.g., apt-get install, service start)
		return KindFatal
	}

	// Connection errors are typically fatal
	var connErr *connector.ConnectionError
	if errors.As(err, &connErr) {
		return KindFatal
	}

	return KindFatal
}

// --- Sentinel Errors ---

// ErrSkipped is a sentinel error indicating the step was not applicable.
var ErrSkipped = &skipError{msg: "step skipped: not applicable"}

// skipError is a sentinel error type for skip conditions.
type skipError struct{ msg string }

func (e *skipError) Error() string   { return e.msg }
func (e *skipError) Is(target error) bool {
	// Make skipError match ErrSkipped
	_, ok := target.(*skipError)
	return ok
}

// ErrRetryable is a sentinel error indicating the operation can be retried.
var ErrRetryable = &retryError{msg: "operation is retryable"}

// retryError is a sentinel error type for retryable conditions.
type retryError struct{ msg string }

func (e *retryError) Error() string   { return e.msg }
func (e *retryError) Is(target error) bool {
	_, ok := target.(*retryError)
	return ok
}

// NewSkip creates a skip error with a custom message.
func NewSkip(format string, args ...interface{}) error {
	return &skipError{msg: fmt.Sprintf(format, args...)}
}

// NewRetryable creates a retryable error with a custom message.
func NewRetryable(format string, args ...interface{}) error {
	return &retryError{msg: fmt.Sprintf(format, args...)}
}

// ValidationError wraps validation failures with field context.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation error on '%s': %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation error: %s", e.Message)
}

// NewValidationError creates a new validation error.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{Field: field, Message: message}
}

// StepError wraps an error with step execution context.
// Use this when reporting errors from step.Run() to include which step and host failed.
type StepError struct {
	StepName  string
	Host      string
	Cmd       string
	ExitCode  int
	Kind      Kind
	Underlying error
}

func (e *StepError) Error() string {
	base := fmt.Sprintf("step %q on host %q failed", e.StepName, e.Host)
	if e.Cmd != "" {
		base += fmt.Sprintf(" (command: %s)", e.Cmd)
	}
	if e.ExitCode != 0 {
		base += fmt.Sprintf(" [exit %d]", e.ExitCode)
	}
	base += fmt.Sprintf(": %v", e.Underlying)
	return base
}

func (e *StepError) Unwrap() error { return e.Underlying }

// NewStepError wraps an error with step context.
// The Kind is determined automatically by Classify, or can be set explicitly.
func NewStepError(stepName, host string, err error) *StepError {
	kind := KindFatal
	if err != nil {
		if cmdErr, ok := err.(*connector.CommandError); ok {
			kind = KindFatal
			return &StepError{
				StepName:  stepName,
				Host:      host,
				Cmd:       cmdErr.Cmd,
				ExitCode:  cmdErr.ExitCode,
				Kind:      kind,
				Underlying: cmdErr,
			}
		}
		kind = ErrorKind(err)
	}
	return &StepError{
		StepName:  stepName,
		Host:      host,
		Kind:      kind,
		Underlying: err,
	}
}

// IsFatal returns true if the error should halt the entire pipeline.
func IsFatal(err error) bool { return ErrorKind(err) == KindFatal }

// IsRetryable returns true if the error can be retried.
func IsRetryable(err error) bool { return ErrorKind(err) == KindRetryable }

// IsSkip returns true if the error indicates the step was not applicable.
func IsSkip(err error) bool { return ErrorKind(err) == KindSkip }

// IsValidation returns true if the error is a validation failure.
func IsValidation(err error) bool { return ErrorKind(err) == KindValidation }

// IsRetryableExitCode returns true if the given exit code should trigger a retry.
// Returns false for idempotent operations (status checks) but true for
// install/start operations that may have transient failures.
func IsRetryableExitCode(cmd string, exitCode int) bool {
	if exitCode == 0 {
		return false
	}
	// Idempotent status checks — non-zero means "not ready yet", retryable
	idempotentPrefixes := []string{
		"systemctl is-active",
		"systemctl is-enabled",
		"systemctl status",
		"docker ps",
		"crictl ps",
		"kubectl get",
		"nc ",
		"curl ",
		"ping ",
	}
	for _, prefix := range idempotentPrefixes {
		if len(cmd) >= len(prefix) && cmd[:len(prefix)] == prefix {
			return true
		}
	}
	// Destructive/modifying commands with non-zero exit codes are fatal
	return false
}
