package connector

import "fmt"

type CommandError struct {
	Cmd        string
	ExitCode   int
	Stdout     string
	Stderr     string
	Underlying error
}

func (e *CommandError) Error() string {
	errMsg := fmt.Sprintf("command '%s' failed with exit code %d", e.Cmd, e.ExitCode)
	if e.Stderr != "" {
		errMsg = fmt.Sprintf("%s, stderr: %s", errMsg, e.Stderr)
	} else {
		errMsg = fmt.Sprintf("%s (no stderr)", errMsg)
	}
	if e.Underlying != nil {
		errMsg = fmt.Sprintf("%s, underlying error: %v", errMsg, e.Underlying)
	}
	return errMsg
}

func (e *CommandError) Unwrap() error {
	return e.Underlying
}

type ConnectionError struct {
	Host string
	Err  error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("failed to connect to host %s: %v", e.Host, e.Err)
}

func (e *ConnectionError) Unwrap() error {
	return e.Err
}
