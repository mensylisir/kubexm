package connector

import "fmt"

// CommandError encapsulates detailed information about a command execution failure.
type CommandError struct {
	Cmd        string
	ExitCode   int
	Stdout     string
	Stderr     string
	Underlying error
}

// Error returns a string representation of the CommandError.
func (e *CommandError) Error() string {
	errMsg := fmt.Sprintf("command '%s' failed with exit code %d", e.Cmd, e.ExitCode)
	if e.Stderr != "" {
		errMsg = fmt.Sprintf("%s: %s", errMsg, e.Stderr)
	}
	if e.Underlying != nil {
		errMsg = fmt.Sprintf("%s (underlying error: %v)", errMsg, e.Underlying)
	}
	return errMsg
}

// Unwrap returns the underlying error for errors.Is and errors.As support.
func (e *CommandError) Unwrap() error {
	return e.Underlying
}

// ConnectionError represents a failure to establish a connection.
type ConnectionError struct {
	Host string
	Err  error
}

// Error returns a string representation of the ConnectionError.
func (e *ConnectionError) Error() string {
	return fmt.Sprintf("failed to connect to host %s: %v", e.Host, e.Err)
}

// Unwrap returns the underlying error for errors.Is and errors.As support.
func (e *ConnectionError) Unwrap() error {
	return e.Err
}
