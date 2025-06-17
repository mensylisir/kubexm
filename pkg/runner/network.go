package runner

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector" // Assuming this is the correct path
)

// IsPortOpen checks if a TCP port is listening on the remote host.
// Uses `ss -ltn` or fallback to `netstat -ltn`.
func (r *Runner) IsPortOpen(ctx context.Context, port int) (bool, error) {
	if r.Conn == nil {
		return false, fmt.Errorf("runner has no valid connector")
	}
	if port <= 0 || port > 65535 {
		return false, fmt.Errorf("invalid port number: %d", port)
	}

	// Try with `ss` first as it's generally preferred and faster.
	// `ss -ltn` lists listening TCP sockets with numeric port numbers.
	// `grep -q` makes it quiet and exits 0 if pattern is found.
	// Pattern looks for ":<port> " or "LISTEN ... *:<port>" or "[::]:<port>" etc.
	// A more precise grep might be needed depending on ss/netstat output variations.
	// Example: `grep -q ':%d\b'` to match port at word boundary.
	cmdSs := fmt.Sprintf("ss -ltn | grep -q ':%d '", port)

	// Check if 'ss' command exists
	_, errLookPathSs := r.LookPath(ctx, "ss")

	cmdToRun := cmdSs
	if errLookPathSs != nil { // ss not found, fallback to netstat
		_, errLookPathNetstat := r.LookPath(ctx, "netstat")
		if errLookPathNetstat != nil {
			return false, fmt.Errorf("neither ss nor netstat found on the remote host: ss_err=%v, netstat_err=%v", errLookPathSs, errLookPathNetstat)
		}
		// `netstat -ltn` lists listening TCP sockets with numeric port numbers.
		cmdNetstat := fmt.Sprintf("netstat -ltn | grep -q ':%d\b.*LISTEN'", port) // More specific grep for netstat
		cmdToRun = cmdNetstat
	}

	// Sudo is typically not required to check listening ports.
	return r.Check(ctx, cmdToRun, false)
}

// WaitForPort waits for a TCP port to become open on the remote host, with a timeout.
func (r *Runner) WaitForPort(ctx context.Context, port int, timeout time.Duration) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}

	// Overall timeout for the WaitForPort operation.
	opCtx, opCancel := context.WithTimeout(ctx, timeout)
	defer opCancel()

	// Ticker for polling interval.
	// The interval should not be too aggressive. 1-2 seconds is usually fine.
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-opCtx.Done():
			// This means the overall timeout for WaitForPort has been reached.
			return fmt.Errorf("timed out waiting for port %d to open after %s: %w", port, timeout, opCtx.Err())
		case <-ticker.C:
			// Check if the port is open in the current tick.
			// Use a short timeout for each individual IsPortOpen check within the loop,
			// or rely on the main context's timeout for the check command itself.
			// For simplicity, the IsPortOpen's internal command execution will respect opCtx.
			isOpen, err := r.IsPortOpen(opCtx, port)
			if err != nil {
				// Don't return immediately on error from IsPortOpen, as the check command
				// itself might fail transiently (e.g., temporary network hiccup to host for the check).
				// Log the error or handle it if it's persistent.
				// For now, we continue polling unless the main context opCtx is done.
				// A proper logger associated with the runner would be useful here.
				// fmt.Fprintf(os.Stderr, "Warning: IsPortOpen check for port %d failed: %v\n", port, err)
				// If the error is due to opCtx being done (e.g. timeout during the check command),
				// the select case for opCtx.Done() will catch it.
			}
			if isOpen {
				return nil // Port is open
			}
			// If not open and no error, continue ticking.
		}
	}
}

// SetHostname sets the hostname of the remote machine.
// This typically requires sudo.
func (r *Runner) SetHostname(ctx context.Context, hostname string) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}
	if strings.TrimSpace(hostname) == "" {
		return fmt.Errorf("hostname cannot be empty")
	}

	// `hostnamectl set-hostname <hostname>` is the standard way on systemd systems.
	// Need to check if hostnamectl exists.
	cmd := ""
	if _, err := r.LookPath(ctx, "hostnamectl"); err == nil {
		cmd = fmt.Sprintf("hostnamectl set-hostname %s", hostname)
	} else {
		// Fallback for systems without hostnamectl (e.g., older systems, or non-systemd)
		// 1. Set hostname in /etc/hostname
		// 2. Use `hostname <hostname>` command for current session
		// This is more complex and might require multiple steps.
		// For now, we'll rely on hostnamectl or fail.
		// A more robust solution would handle these fallbacks.
		return fmt.Errorf("hostnamectl command not found. Automatic fallback not implemented. Please set hostname manually or ensure hostnamectl is available. Error: %w", err)
	}

	_, stderr, err := r.RunWithOptions(ctx, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to set hostname to %s using command '%s': %w (stderr: %s)", hostname, cmd, err, string(stderr))
	}

	// Additionally, ensure the new hostname is immediately reflected in the current session if possible
	// and also update /etc/hosts if 127.0.1.1 entry for old hostname exists.
	// For simplicity, these steps are omitted here but are important for full hostname change.
	// For example, after `hostnamectl set-hostname`, the `hostname` command might still show old name
	// until a new login or `hostname $(cat /etc/hostname)`.
	// And /etc/hosts might have an entry like `127.0.1.1 old-hostname` which should be updated.

	// A simple way to apply hostname for current session on some systems:
	applyCmd := fmt.Sprintf("hostname %s", hostname)
	// This might fail or not be necessary depending on the system and how hostnamectl works.
	// Run with sudo true, as `hostname` command often needs it.
	_, _, _ = r.RunWithOptions(ctx, applyCmd, &connector.ExecOptions{Sudo: true})
	// Ignore error for this best-effort application. The primary change is via hostnamectl.


	return nil
}

// AddHostEntry adds an entry to /etc/hosts, ensuring it doesn't already exist (idempotent).
// This typically requires sudo.
func (r *Runner) AddHostEntry(ctx context.Context, ip, fqdn string, hostnames ...string) error {
	if r.Conn == nil {
		return fmt.Errorf("runner has no valid connector")
	}
	if strings.TrimSpace(ip) == "" || strings.TrimSpace(fqdn) == "" {
		return fmt.Errorf("IP and FQDN cannot be empty for AddHostEntry")
	}

	allHostnames := []string{fqdn}
	allHostnames = append(allHostnames, hostnames...)
	entryLine := fmt.Sprintf("%s %s", ip, strings.Join(allHostnames, " "))

	// Check if entry (or a conflicting entry for the FQDN) exists
	// `grep -qP` for Perl-compatible regex to match whole words might be better.
	// Example: `grep -qP '(\s|^)%s(\s|$)' /etc/hosts`
	// For simplicity, using basic grep.
	// This check is not perfectly robust for all cases of existing entries (e.g. different IP for same hostname).
	// A more robust solution would parse /etc/hosts.

	// Check if the exact line already exists
	checkCmdExact := fmt.Sprintf("grep -Fxq '%s' /etc/hosts", entryLine)
	exactExists, _ := r.Check(ctx, checkCmdExact, false) // Sudo not needed for grep
	if exactExists {
		return nil // Entry already exists exactly as specified, operation is idempotent.
	}

	// More complex check: if any of the hostnames in the new entry already exist with a *different* IP.
	// Or if the IP exists with *different* hostnames. This is harder to make perfectly idempotent
	// without more complex parsing of /etc/hosts.
	// For now, we proceed to add if the exact line isn't there.
	// A production tool might offer options to overwrite or fail on conflicts.

	// Append entry. Using `tee -a` with sudo is a common way to append to a root-owned file.
	// `echo 'entry' | sudo tee -a /etc/hosts`
	// Ensure the echo command itself handles special characters in entryLine correctly if any.
	// For safety, ensure entryLine doesn't have characters that `echo` or shell would misinterpret.
	// A safer way is to upload a temp file and use `sudo cp` or `sudo cat >>`.
	// Using a simple echo for now.
	appendCmd := fmt.Sprintf("echo '%s' >> /etc/hosts", entryLine)

	_, stderr, err := r.RunWithOptions(ctx, appendCmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to add host entry '%s' to /etc/hosts: %w (stderr: %s)", entryLine, err, string(stderr))
	}
	return nil
}
