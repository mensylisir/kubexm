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
	return fmt.Sprintf("command '%s' failed with exit code %d: %s", e.Cmd, e.ExitCode, e.Stderr)
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
