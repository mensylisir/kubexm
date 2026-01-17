package runner

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

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

func (r *defaultRunner) OriginRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, string, error) {
	if conn == nil {
		return "", "", fmt.Errorf("connector cannot be nil")
	}

	opts := &connector.ExecOptions{
		Sudo: sudo,
	}

	stdout, stderr, err := conn.Exec(ctx, cmd, opts)

	if err != nil {
		if len(stderr) > 0 {
			return string(stdout), string(stderr), fmt.Errorf("command '%s' failed: %w, stderr: %s", cmd, err, string(stderr))
		}
		return string(stdout), string(stderr), fmt.Errorf("command '%s' failed: %w", cmd, err)
	}

	return string(stdout), string(stderr), nil
}

func (r *defaultRunner) MustRun(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (string, error) {
	output, err := r.Run(ctx, conn, cmd, sudo)
	if err != nil {
		return output, fmt.Errorf("command '%s' (sudo: %v) failed: %w. Output: %s", cmd, sudo, err, output)
	}
	return output, nil
}

func (r *defaultRunner) Check(ctx context.Context, conn connector.Connector, cmd string, sudo bool) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	opts := &connector.ExecOptions{Sudo: sudo}
	_, _, err := conn.Exec(ctx, cmd, opts)

	if err == nil {
		return true, nil
	}
	var cmdError *connector.CommandError
	if errors.As(err, &cmdError) { // Check if the error is a CommandError
		// Non-zero exit code means the command executed but "failed" the check.
		// This is not an operational error of the Check method itself.
		return false, nil
	}
	return false, err
}

func (r *defaultRunner) RunWithOptions(ctx context.Context, conn connector.Connector, cmd string, opts *connector.ExecOptions) (stdout, stderr []byte, err error) {
	if conn == nil {
		return nil, nil, fmt.Errorf("connector cannot be nil")
	}
	if opts == nil {
		opts = &connector.ExecOptions{}
	}
	return conn.Exec(ctx, cmd, opts)
}

func (r *defaultRunner) RunInBackground(ctx context.Context, conn connector.Connector, cmd string, sudo bool) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil for RunInBackground")
	}
	if strings.TrimSpace(cmd) == "" {
		return fmt.Errorf("command cannot be empty for RunInBackground")
	}

	var backgroundCmd string
	nohupPath, err := r.LookPath(ctx, conn, "nohup")
	if err != nil {
		escapedCmd := strings.ReplaceAll(cmd, "'", `\'`)
		backgroundCmd = fmt.Sprintf("sh -c '%s > /dev/null 2>&1 &' ", escapedCmd)
	} else {
		escapedCmd := strings.ReplaceAll(cmd, "'", `\'`)
		backgroundCmd = fmt.Sprintf("%s sh -c '%s' > /dev/null 2>&1 &", nohupPath, escapedCmd)
	}

	_, stderr, execErr := r.RunWithOptions(ctx, conn, backgroundCmd, &connector.ExecOptions{Sudo: sudo})
	if execErr != nil {
		return fmt.Errorf("failed to launch command '%s' in background using '%s': %w (stderr: %s)", cmd, backgroundCmd, execErr, string(stderr))
	}
	return nil
}

func (r *defaultRunner) RunRetry(ctx context.Context, conn connector.Connector, cmd string, sudo bool, retries int, delay time.Duration) (string, error) {
	if conn == nil {
		return "", fmt.Errorf("connector cannot be nil for RunRetry")
	}
	if strings.TrimSpace(cmd) == "" {
		return "", fmt.Errorf("command cannot be empty for RunRetry")
	}
	if retries < 0 {
		retries = 0
	}

	var lastErr error
	var output string

	totalAttempts := 1 + retries

	for attempt := 0; attempt < totalAttempts; attempt++ {
		select {
		case <-ctx.Done():
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

		if attempt == totalAttempts-1 {
			break
		}

		if delay > 0 {
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return output, fmt.Errorf("context cancelled during delay for command '%s': %w (last error: %s)", cmd, ctx.Err(), lastErr.Error())
			}
		}
	}

	return output, fmt.Errorf("command '%s' failed after %d attempts: %w (last output: %s)", cmd, totalAttempts, lastErr, output)
}
