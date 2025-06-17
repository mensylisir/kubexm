package runner

import (
	"context"
	"fmt"

	"github.com/kubexms/kubexms/pkg/connector" // Assuming this is the correct path from go.mod
)

// Run executes a command and returns combined stdout/stderr and error.
// This is a general-purpose command execution function.
func (r *Runner) Run(ctx context.Context, cmd string, sudo bool) (string, error) {
	if r.Conn == nil {
		return "", fmt.Errorf("runner has no valid connector")
	}
	opts := &connector.ExecOptions{
		Sudo: sudo,
		// Other defaults like Timeout can be set here or rely on ExecOptions zero values
	}
	stdout, stderr, err := r.Conn.Exec(ctx, cmd, opts)
	output := string(stdout)
	if len(stderr) > 0 {
		if len(output) > 0 {
			output += "\n"
		}
		output += string(stderr) // Append stderr to output for simplicity in this basic Run
	}

	if err != nil {
		// Return the combined output along with the error.
		// The error from Conn.Exec should ideally be a *connector.CommandError
		// which already contains Stdout, Stderr, and ExitCode.
		return output, err
	}
	return output, nil
}

// MustRun executes a command and panics if it fails.
// Useful for critical steps where failure means the entire process is meaningless.
func (r *Runner) MustRun(ctx context.Context, cmd string, sudo bool) string {
	output, err := r.Run(ctx, cmd, sudo)
	if err != nil {
		panic(fmt.Errorf("command '%s' (sudo: %v) failed: %w. Output: %s", cmd, sudo, err, output))
	}
	return output
}

// Check executes a command and returns true if it exits with 0, false otherwise.
// Ideal for conditional checks, e.g., `systemctl is-active my-service`.
func (r *Runner) Check(ctx context.Context, cmd string, sudo bool) (bool, error) {
	if r.Conn == nil {
		return false, fmt.Errorf("runner has no valid connector")
	}
	opts := &connector.ExecOptions{Sudo: sudo}
	_, _, err := r.Conn.Exec(ctx, cmd, opts)

	if err == nil {
		return true, nil // Exit code 0
	}

	// If it's a CommandError, it means the command executed but returned non-zero.
	// This is an expected 'false' outcome for Check.
	if _, ok := err.(*connector.CommandError); ok {
		// You could check cmdErr.ExitCode if needed, but for Check, any non-zero is false.
		// For example, if cmdErr.IsExitCode(0) is false.
		// For now, any CommandError means the command ran but "failed" in the sense of Check.
		return false, nil
	}

	// An actual execution error occurred (e.g., connection failed, command not found by shell).
	return false, err
}

// RunWithOptions provides full control over connector.ExecOptions.
// Useful for scenarios requiring fine-tuned control over timeouts, retries, etc.
func (r *Runner) RunWithOptions(ctx context.Context, cmd string, opts *connector.ExecOptions) (stdout, stderr []byte, err error) {
	if r.Conn == nil {
		return nil, nil, fmt.Errorf("runner has no valid connector")
	}
	if opts == nil { // Ensure opts is not nil to avoid panic in Conn.Exec if it expects it
	    opts = &connector.ExecOptions{}
	}
	return r.Conn.Exec(ctx, cmd, opts)
}
