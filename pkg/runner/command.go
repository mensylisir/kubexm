package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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

// RunInBackground executes a command in the background on the remote host.
// It uses "nohup" and output redirection to /dev/null to ensure the command detaches.
func (r *defaultRunner) RunInBackground(ctx context.Context, conn connector.Connector, cmd string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil for RunInBackground")
	}
	if strings.TrimSpace(cmd) == "" {
		return fmt.Errorf("command cannot be empty for RunInBackground")
	}

	// Ensure nohup is available. If not, this method might not work as expected or at all.
	// A simple check without failing immediately, as some minimal systems might lack it,
	// and the user might have a command that backgrounds itself.
	// However, for a reliable detach, nohup is good.
	var backgroundCmd string
	nohupPath, err := r.LookPath(ctx, conn, "nohup")
	if err != nil {
		// If nohup is not found, we could try running without it, but it's less reliable.
		// For now, let's proceed but be aware that behavior might differ.
		// A stricter approach would be to return an error here.
		// Alternative: cmd = fmt.Sprintf("(%s) > /dev/null 2>&1 &", cmd)
		// Using sh -c 'command &' is also common.
		// Let's use a common pattern that works on most systems:
		// sh -c "your_command > /dev/null 2>&1 &"
		// This doesn't require nohup explicitly.
		// The command needs to be properly escaped if it contains special shell characters.
		// For simplicity, assuming 'cmd' is a relatively simple command string.
		// A robust solution would involve more complex shell escaping for 'cmd'.
		escapedCmd := strings.ReplaceAll(cmd, "'", `\'`) // Basic escaping for single quotes within the command
		backgroundCmd = fmt.Sprintf("sh -c '%s > /dev/null 2>&1 &' ", escapedCmd)
		// nohupPath = "" // This line is not needed as nohupPath is already declared and not used further in this branch
	} else {
		// If nohup is available, use it.
		// nohup sh -c 'actual_command' > /dev/null 2>&1 &
		// This structure is generally robust.
		escapedCmd := strings.ReplaceAll(cmd, "'", `\'`)
		backgroundCmd = fmt.Sprintf("%s sh -c '%s' > /dev/null 2>&1 &", nohupPath, escapedCmd)
	}


	// Execute the command. We expect this to return quickly.
	// The output of the backgrounded command itself is not captured here.
	// We are only interested in whether the command to launch it in the background succeeded.
	_, stderr, execErr := r.RunWithOptions(ctx, conn, backgroundCmd, &connector.ExecOptions{Sudo: sudo})
	if execErr != nil {
		return fmt.Errorf("failed to launch command '%s' in background using '%s': %w (stderr: %s)", cmd, backgroundCmd, execErr, string(stderr))
	}
	return nil
}


// RunRetry executes a command and retries it a specified number of times if it fails,
// with a delay between retries.
// `retries` is the number of additional attempts after the first one fails.
// So, total attempts = 1 (initial) + retries.
func (r *defaultRunner) RunRetry(ctx context.Context, conn connector.Connector, cmd string, sudo bool, retries int, delay time.Duration) (string, error) {
	if conn == nil {
		return "", fmt.Errorf("connector cannot be nil for RunRetry")
	}
	if strings.TrimSpace(cmd) == "" {
		return "", fmt.Errorf("command cannot be empty for RunRetry")
	}
	if retries < 0 {
		retries = 0 // Ensure retries is not negative
	}

	var lastErr error
	var output string

	totalAttempts := 1 + retries

	for attempt := 0; attempt < totalAttempts; attempt++ {
		select {
		case <-ctx.Done(): // Check context before each attempt and before delay
			if lastErr != nil {
				return output, fmt.Errorf("context cancelled during retries for command '%s': %w (last error: %s)", cmd, ctx.Err(), lastErr.Error())
			}
			return "", fmt.Errorf("context cancelled before command '%s' could complete: %w", cmd, ctx.Err())
		default:
		}

		output, lastErr = r.Run(ctx, conn, cmd, sudo)
		if lastErr == nil {
			return output, nil // Success
		}

		// If it's the last attempt, don't delay, just return the error.
		if attempt == totalAttempts-1 {
			break
		}

		if delay > 0 {
			select {
			case <-time.After(delay):
				// Continue to next attempt
			case <-ctx.Done():
				return output, fmt.Errorf("context cancelled during delay for command '%s': %w (last error: %s)", cmd, ctx.Err(), lastErr.Error())
			}
		}
	}

	return output, fmt.Errorf("command '%s' failed after %d attempts: %w (last output: %s)", cmd, totalAttempts, lastErr, output)
}
