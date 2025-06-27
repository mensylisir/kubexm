package connector

import (
	"errors"
	"fmt" // Added back as it's used by errors.As test section
	"testing"
)

// specificError is a custom error type for testing errors.As with ConnectionError.
type specificError struct{ msg string }

func (se *specificError) Error() string { return se.msg }

func TestCommandError(t *testing.T) {
	underlyingErr := errors.New("underlying issue")
	cmdErr := &CommandError{
		Cmd:        "ls -l",
		ExitCode:   1,
		Stdout:     "some output",
		Stderr:     "permission denied",
		Underlying: underlyingErr,
	}

	expectedMsg := "command 'ls -l' failed with exit code 1: permission denied (underlying error: underlying issue)"
	if cmdErr.Error() != expectedMsg {
		t.Errorf("CommandError.Error() got %q, want %q", cmdErr.Error(), expectedMsg)
	}

	if !errors.Is(cmdErr, underlyingErr) {
		t.Errorf("errors.Is(cmdErr, underlyingErr) was false, expected true")
	}

	// Test without underlying error and without stderr
	cmdErrNoDetails := &CommandError{
		Cmd:      "echo hello",
		ExitCode: 0,
	}
	expectedMsgNoDetails := "command 'echo hello' failed with exit code 0"
	if cmdErrNoDetails.Error() != expectedMsgNoDetails {
		t.Errorf("CommandError.Error() without details got %q, want %q", cmdErrNoDetails.Error(), expectedMsgNoDetails)
	}
	if cmdErrNoDetails.Unwrap() != nil {
		t.Errorf("CommandError.Unwrap() without underlying error got %v, want nil", cmdErrNoDetails.Unwrap())
	}

	// Test with underlying error but no stderr
	cmdErrNoStderr := &CommandError{
		Cmd:        "cat file",
		ExitCode:   2,
		Underlying: underlyingErr,
	}
	expectedMsgNoStderr := "command 'cat file' failed with exit code 2 (underlying error: underlying issue)"
	if cmdErrNoStderr.Error() != expectedMsgNoStderr {
		t.Errorf("CommandError.Error() with underlying but no stderr got %q, want %q", cmdErrNoStderr.Error(), expectedMsgNoStderr)
	}

	// Test with stderr but no underlying error
	cmdErrNoUnderlying := &CommandError{
		Cmd:      "rm /nonexistent",
		ExitCode: 1,
		Stderr:   "No such file or directory",
	}
	expectedMsgNoUnderlying := "command 'rm /nonexistent' failed with exit code 1: No such file or directory"
	if cmdErrNoUnderlying.Error() != expectedMsgNoUnderlying {
		t.Errorf("CommandError.Error() with stderr but no underlying got %q, want %q", cmdErrNoUnderlying.Error(), expectedMsgNoUnderlying)
	}
}

func TestConnectionError(t *testing.T) {
	originalErr := errors.New("network timeout")
	connErr := &ConnectionError{
		Host: "example.com",
		Err:  originalErr,
	}

	expectedMsg := "failed to connect to host example.com: network timeout"
	if connErr.Error() != expectedMsg {
		t.Errorf("ConnectionError.Error() got %q, want %q", connErr.Error(), expectedMsg)
	}

	if !errors.Is(connErr, originalErr) {
		t.Errorf("errors.Is(connErr, originalErr) was false, expected true")
	}

	// Test with a wrapped error to ensure Unwrap works as expected with errors.As
	specificInstance := &specificError{msg: "specific connection problem"}
	wrappedSpecificErr := fmt.Errorf("connection layer: %w", specificInstance)
	connErrWithSpecific := &ConnectionError{
		Host: "another.host",
		Err:  wrappedSpecificErr,
	}

	var targetSpecificError *specificError
	if !errors.As(connErrWithSpecific, &targetSpecificError) {
		t.Errorf("errors.As(connErrWithSpecific, &targetSpecificError) was false, expected true")
	} else {
		if targetSpecificError == nil {
			t.Errorf("targetSpecificError was nil after errors.As")
		} else {
			if targetSpecificError.msg != "specific connection problem" {
				t.Errorf("unwrapped specific error message got %q, want %q", targetSpecificError.msg, "specific connection problem")
			}
		}
	}

	// Test Unwrap with nil underlying error
	connErrNilUnderlying := &ConnectionError{
		Host: "nil.underlying.host",
		Err:  nil,
	}
	if connErrNilUnderlying.Unwrap() != nil {
		t.Errorf("ConnectionError.Unwrap() with nil underlying error got %v, want nil", connErrNilUnderlying.Unwrap())
	}
	expectedMsgNilUnderlying := "failed to connect to host nil.underlying.host: <nil>"
	if connErrNilUnderlying.Error() != expectedMsgNilUnderlying {
		t.Errorf("ConnectionError.Error() with nil underlying got %q, want %q", connErrNilUnderlying.Error(), expectedMsgNilUnderlying)
	}
}
