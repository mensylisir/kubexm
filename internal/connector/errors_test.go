package connector

import (
	"errors"
	"testing"
)

func TestCommandError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *CommandError
		expected string
	}{
		{
			name: "WithStderr",
			err: &CommandError{
				Cmd:      "ls /nonexistent",
				ExitCode: 2,
				Stdout:   "",
				Stderr:   "No such file or directory",
			},
			expected: "command 'ls /nonexistent' failed with exit code 2, stderr: No such file or directory",
		},
		{
			name: "WithoutStderr",
			err: &CommandError{
				Cmd:      "echo test",
				ExitCode: 1,
				Stdout:   "test",
				Stderr:   "",
			},
			expected: "command 'echo test' failed with exit code 1 (no stderr)",
		},
		{
			name: "WithUnderlying",
			err: &CommandError{
				Cmd:        "timeout cmd",
				ExitCode:   -1,
				Stderr:     "timeout",
				Underlying: errors.New("context deadline exceeded"),
			},
			expected: "command 'timeout cmd' failed with exit code -1, stderr: timeout, underlying error: context deadline exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestCommandError_Unwrap(t *testing.T) {
	underlying := errors.New("test error")
	err := &CommandError{
		Cmd:        "test",
		ExitCode:   1,
		Underlying: underlying,
	}

	unwrapped := err.Unwrap()
	if unwrapped != underlying {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlying)
	}

	// Test nil underlying
	errNoUnderlying := &CommandError{
		Cmd:      "test",
		ExitCode: 1,
	}
	if errNoUnderlying.Unwrap() != nil {
		t.Error("Unwrap() should return nil when no underlying error")
	}
}

func TestConnectionError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *ConnectionError
		expected string
	}{
		{
			name: "BasicConnectionError",
			err: &ConnectionError{
				Host: "192.168.1.1",
				Err:  errors.New("connection refused"),
			},
			expected: "failed to connect to host 192.168.1.1: connection refused",
		},
		{
			name: "TimeoutError",
			err: &ConnectionError{
				Host: "example.com",
				Err:  errors.New("i/o timeout"),
			},
			expected: "failed to connect to host example.com: i/o timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.expected {
				t.Errorf("Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestConnectionError_Unwrap(t *testing.T) {
	underlying := errors.New("test error")
	err := &ConnectionError{
		Host: "test.com",
		Err:  underlying,
	}

	unwrapped := err.Unwrap()
	if unwrapped != underlying {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlying)
	}
}
