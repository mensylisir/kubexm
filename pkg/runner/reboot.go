package runner

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// Reboot issues a reboot command to the host and optionally waits for it to become responsive.
// The method attempts to be somewhat resilient to connection drops during the reboot process.
// receiver `r` is of type *defaultRunner.
func (r *defaultRunner) Reboot(ctx context.Context, conn connector.Connector, timeout time.Duration) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil for Reboot")
	}

	// Issue the reboot command.
	// Using a slightly delayed, backgrounded reboot command for robustness.
	rebootCmd := "sh -c 'sleep 2 && reboot > /dev/null 2>&1 &'"

	// Attempt to issue the reboot command.
	// RunWithOptions is a method on defaultRunner which eventually calls conn.Exec.
	// Sudo is true, short timeout for sending the command itself.
	_, _, execErr := r.RunWithOptions(ctx, conn, rebootCmd, &connector.ExecOptions{Sudo: true, Timeout: 10 * time.Second})

	// We don't strictly fail on execErr here if it indicates a connection drop,
	// as that's expected when a reboot command succeeds.
	if execErr != nil {
		// Check if the error is a context deadline exceeded, or if the error message
		// suggests the connection was closed or the session terminated.
		errStr := strings.ToLower(execErr.Error())
		if !(strings.Contains(errStr, "context deadline exceeded") ||
			strings.Contains(errStr, "session channel closed") ||
			strings.Contains(errStr, "connection lost") ||
			strings.Contains(errStr, "eof") || // End Of File, common for abrupt closes
			strings.Contains(errStr, "broken pipe")) { // Can also indicate connection closed
			return fmt.Errorf("failed to issue reboot command: %w", execErr)
		}
		// If it's an expected error type, log it and proceed.
		fmt.Fprintf(os.Stderr, "Reboot command initiated, connection may have dropped as expected: %v\n", execErr)
	}

	// Wait a grace period for the shutdown to initiate properly on the remote host.
	fmt.Fprintf(os.Stderr, "Reboot command sent. Waiting for shutdown to initiate (10s grace period)...\n")
	time.Sleep(10 * time.Second)

	// Context for the overall waiting period.
	rebootWaitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(5 * time.Second) // Poll every 5 seconds.
	defer ticker.Stop()

	fmt.Fprintf(os.Stderr, "Waiting for host to become responsive after reboot (up to %s)...\n", timeout)

	for {
		select {
		case <-rebootWaitCtx.Done():
			return fmt.Errorf("timed out waiting for host to become responsive after reboot: %w", rebootWaitCtx.Err())
		case <-ticker.C:
			// Attempt a simple command to check if the host is back up and responsive.
			// The 'conn' object might be stale. A robust solution would involve re-establishing
			// the connection, but this runner is stateless. We rely on the connector's
			// potential internal reconnect logic or accept this check might fail until a fresh
			// connection is made externally.
			checkCmd := "uptime" // A simple command that should work on a booted system.

			// Use a short timeout for each check itself.
			checkCtx, checkCancel := context.WithTimeout(rebootWaitCtx, 10*time.Second)
			// We directly use conn.Exec here for the check, as r.RunWithOptions might add complexity
			// not needed for a simple check.
			_, _, checkErr := conn.Exec(checkCtx, checkCmd, &connector.ExecOptions{Sudo: false, Timeout: 5 * time.Second})
			checkCancel() // Release resources for this specific check context.

			if checkErr == nil {
				fmt.Fprintf(os.Stderr, "Host is responsive after reboot.\n")
				return nil // Host is back up.
			}
			// Log that the host is not yet responsive to show progress.
			fmt.Fprintf(os.Stderr, "Host not yet responsive: %v. Retrying...\n", checkErr)
		}
	}
}
