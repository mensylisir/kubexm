package runner

import (
	"context"
	"errors"
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// Run executes a command and returns combined stdout/stderr and error.
func (r *defaultRunner) Run(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
	if conn == nil {
		return "", fmt.Errorf("connector cannot be nil")
	}
	opts := &connector.ExecOptions{
		Sudo: sudo,
	}
	stdout, stderr, err := conn.Exec(ctx, cmd, opts)
	output := string(stdout)
	if len(stderr) > 0 {
		if len(output) > 0 {
			output += "\n"
		}
		output += string(stderr)
	}

	if err != nil {
		return output, err
	}
	return output, nil
}

// MustRun executes a command and panics if it fails.
func (r *defaultRunner) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) string {
	output, err := r.Run(ctx, conn, cmd, sudo)
	if err != nil {
		panic(fmt.Errorf("command '%s' (sudo: %v) failed: %w. Output: %s", cmd, sudo, err, output))
	}
	return output
}

// Check executes a command and returns true if it exits with 0, false otherwise.
func (r *defaultRunner) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	opts := &connector.ExecOptions{Sudo: sudo}
	_, _, err := conn.Exec(ctx, cmd, opts)

	if err == nil {
		return true, nil
	}
	// Assuming connector.CommandError is the type for non-zero exits
	// from the connector's Exec method.
	var cmdError *connector.CommandError
	if errors.As(err, &cmdError) { // Check if the error is a CommandError
		// Non-zero exit code means the command executed but "failed" the check.
		// This is not an operational error of the Check method itself.
		return false, nil
	}
	// If err is not nil and not a CommandError, it's an operational error (e.g., connection issue, context cancelled).
	return false, err
}

// RunWithOptions provides full control over connector.ExecOptions.
func (r *defaultRunner) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) (stdout, stderr []byte, err error) {
	if conn == nil {
		return nil, nil, fmt.Errorf("connector cannot be nil")
	}
	if opts == nil {
		opts = &connector.ExecOptions{}
	}
	return conn.Exec(ctx, cmd, opts)
}
