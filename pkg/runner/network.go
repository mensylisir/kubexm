package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
)

// IsPortOpen checks if a TCP port is listening on the remote host.
func (r *defaultRunner) IsPortOpen(ctx context.Context, conn connector.Connector, facts *Facts, port int) (bool, error) {
	if conn == nil {
		return false, fmt.Errorf("connector cannot be nil")
	}
	if port <= 0 || port > 65535 {
		return false, fmt.Errorf("invalid port number: %d", port)
	}

	// Determine command based on OS (from facts) or availability
	cmdToRun := ""
	// useSS := false // Removed unused variable

	// Prefer ss if available
	if _, err := r.LookPath(ctx, conn, "ss"); err == nil {
		cmdToRun = fmt.Sprintf("ss -ltn | grep -q ':%d '", port)
		// useSS = true // Removed unused assignment
	} else {
		// Fallback to netstat
		if _, errNetstat := r.LookPath(ctx, conn, "netstat"); errNetstat == nil {
			cmdToRun = fmt.Sprintf("netstat -ltn | grep -q ':%d\\b.*LISTEN'", port)
		} else {
			return false, fmt.Errorf("neither ss nor netstat found on the remote host")
		}
	}

	// Sudo is typically not required to check listening ports.
	// The Check method itself handles interpreting the exit code.
	return r.Check(ctx, conn, cmdToRun, false)
}

// WaitForPort waits for a TCP port to become open on the remote host, with a timeout.
func (r *defaultRunner) WaitForPort(ctx context.Context, conn connector.Connector, facts *Facts, port int, timeout time.Duration) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}

	opCtx, opCancel := context.WithTimeout(ctx, timeout)
	defer opCancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-opCtx.Done():
			return fmt.Errorf("timed out waiting for port %d to open after %s: %w", port, timeout, opCtx.Err())
		case <-ticker.C:
			isOpen, err := r.IsPortOpen(opCtx, conn, facts, port) // Pass facts
			if err != nil {
				// If the error indicates a fundamental problem like tools not found, fail fast.
				if strings.Contains(err.Error(), "neither ss nor netstat found") {
					return fmt.Errorf("cannot wait for port %d, required tools (ss/netstat) not found: %w", port, err)
				}
				// For other transient errors, continue polling.
				// A log line here could be useful: fmt.Fprintf(os.Stderr, "Debug: IsPortOpen check for port %d returned error during wait: %v\n", port, err)
			}
			if isOpen {
				return nil // Port is open
			}
		}
	}
}

// SetHostname sets the hostname of the remote machine.
func (r *defaultRunner) SetHostname(ctx context.Context, conn connector.Connector, facts *Facts, hostname string) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(hostname) == "" {
		return fmt.Errorf("hostname cannot be empty")
	}

	cmd := ""
	// Use facts to determine the best command if available
	// This is a simplified example; a real implementation would use facts.OS.ID, facts.InitSystem.Type etc.
	if _, err := r.LookPath(ctx, conn, "hostnamectl"); err == nil {
		cmd = fmt.Sprintf("hostnamectl set-hostname %s", hostname)
	} else if _, err := r.LookPath(ctx, conn, "hostname"); err == nil {
		// Basic fallback - this might not persist across reboots on all systems
		cmd = fmt.Sprintf("hostname %s", hostname)
		// Additionally, on non-hostnamectl systems, one might need to update /etc/hostname and /etc/hosts
		// For this refactor, we'll keep it simpler and assume hostnamectl or basic hostname command.
	} else {
		return fmt.Errorf("no suitable command found to set hostname (checked hostnamectl, hostname)")
	}

	_, _, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to set hostname to %s using command '%s': %w", hostname, cmd, err)
	}

	// Best-effort: attempt to apply hostname to current session if not using hostnamectl
	// (hostnamectl usually handles this better)
	if !strings.HasPrefix(cmd, "hostnamectl") {
		applyCmd := fmt.Sprintf("hostname %s", hostname)
		_, _, _ = r.RunWithOptions(ctx, conn, applyCmd, &connector.ExecOptions{Sudo: true})
	}
	return nil
}

// AddHostEntry adds an entry to /etc/hosts, ensuring it doesn't already exist (idempotent).
func (r *defaultRunner) AddHostEntry(ctx context.Context, conn connector.Connector, ip, fqdn string, hostnames ...string) error {
	if conn == nil {
		return fmt.Errorf("connector cannot be nil")
	}
	if strings.TrimSpace(ip) == "" || strings.TrimSpace(fqdn) == "" {
		return fmt.Errorf("IP and FQDN cannot be empty for AddHostEntry")
	}

	allHostnames := []string{fqdn}
	allHostnames = append(allHostnames, hostnames...)
	entryLine := fmt.Sprintf("%s %s", ip, strings.Join(allHostnames, " ")) // entryLine is used below

	// Check if the exact line already exists
	// Using WriteFile with a temporary script for robust check and append
	/*
	scriptContent := fmt.Sprintf(`  // This block was for an old approach
set -e
HOSTS_FILE="/etc/hosts"
ENTRY_LINE="%s"
if grep -Fxq -- "${ENTRY_LINE}" "${HOSTS_FILE}"; then
  echo "Entry already exists."
  exit 0
else
  echo "${ENTRY_LINE}" >> "${HOSTS_FILE}"
  echo "Entry added."
  exit 0
fi
`, entryLine)
	*/ // Closed the block comment

	// Define a temporary path for the script on the remote host
	// This path should ideally be in a directory writable by the SSH user,
	// or a directory that can be created by the SSH user. /tmp is common.
	// The script itself will use sudo for the echo >> /etc/hosts part if WriteFile uses sudo.
	// However, it's better to make the echo command itself use sudo via `tee`.
	// Let's adjust to use `echo | sudo tee -a` for the append part.

	// Revised idempotent add:
	// 1. Check if entry exists (no sudo).
	// 2. If not, append with sudo.
	checkCmdExact := fmt.Sprintf("grep -Fxq '%s' /etc/hosts", entryLine)
	exactExists, errCheck := r.Check(ctx, conn, checkCmdExact, false)
	if errCheck != nil {
		// This means the grep command itself failed, not that it didn't find the entry.
		return fmt.Errorf("failed to check /etc/hosts for existing entry '%s': %w", entryLine, errCheck)
	}
	if exactExists {
		return nil // Idempotent: entry already there.
	}

	// Append entry. Sudo is required.
	// Using `sh -c "echo '...' >> /etc/hosts"` with sudo on `sh`
	// Or better: `echo '...' | sudo tee -a /etc/hosts`
	// For simplicity with RunWithOptions:
	appendCmd := fmt.Sprintf("echo '%s' >> /etc/hosts", entryLine)
	_, _, errAppend := r.RunWithOptions(ctx, conn, appendCmd, &connector.ExecOptions{Sudo: true})
	if errAppend != nil {
		return fmt.Errorf("failed to add host entry '%s' to /etc/hosts: %w", entryLine, errAppend)
	}
	return nil
}
