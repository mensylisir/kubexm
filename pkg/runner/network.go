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

	cmdToRun := ""
	if _, err := r.LookPath(ctx, conn, "ss"); err == nil {
		cmdToRun = fmt.Sprintf("ss -ltn | grep -q ':%d '", port)
	} else {
		if _, errNetstat := r.LookPath(ctx, conn, "netstat"); errNetstat == nil {
			cmdToRun = fmt.Sprintf("netstat -ltn | grep -q ':%d\\b.*LISTEN'", port)
		} else {
			return false, fmt.Errorf("neither ss nor netstat found on the remote host")
		}
	}
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

	isOpenInitial, errInitial := r.IsPortOpen(opCtx, conn, facts, port)
	if errInitial != nil {
		if strings.Contains(errInitial.Error(), "neither ss nor netstat found") {
			return fmt.Errorf("cannot wait for port %d, required tools (ss/netstat) not found: %w", port, errInitial)
		}
	}
	if isOpenInitial {
		return nil
	}

	for {
		select {
		case <-opCtx.Done():
			return fmt.Errorf("timed out waiting for port %d to open after %s: %w", port, timeout, opCtx.Err())
		case <-ticker.C:
			isOpen, err := r.IsPortOpen(opCtx, conn, facts, port)
			if err != nil {
				if strings.Contains(err.Error(), "neither ss nor netstat found") {
					return fmt.Errorf("cannot wait for port %d, required tools (ss/netstat) not found: %w", port, err)
				}
			}
			if isOpen {
				return nil
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
	if _, err := r.LookPath(ctx, conn, "hostnamectl"); err == nil {
		cmd = fmt.Sprintf("hostnamectl set-hostname %s", hostname)
	} else if _, errHostname := r.LookPath(ctx, conn, "hostname"); errHostname == nil {
		cmd = fmt.Sprintf("hostname %s", hostname)
	} else {
		return fmt.Errorf("no suitable command found to set hostname (checked hostnamectl, hostname)")
	}
	_, _, err := r.RunWithOptions(ctx, conn, cmd, &connector.ExecOptions{Sudo: true})
	if err != nil {
		return fmt.Errorf("failed to set hostname to %s using command '%s': %w", hostname, cmd, err)
	}
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
	entryLine := fmt.Sprintf("%s %s", ip, strings.Join(allHostnames, " "))

	checkCmdExact := fmt.Sprintf("grep -Fxq '%s' /etc/hosts", entryLine)
	exactExists, errCheck := r.Check(ctx, conn, checkCmdExact, false)
	if errCheck != nil {
		return fmt.Errorf("failed to check /etc/hosts for existing entry '%s': %w", entryLine, errCheck)
	}
	if exactExists {
		return nil
	}
	appendCmd := fmt.Sprintf("echo '%s' >> /etc/hosts", entryLine)
	_, _, errAppend := r.RunWithOptions(ctx, conn, appendCmd, &connector.ExecOptions{Sudo: true})
	if errAppend != nil {
		return fmt.Errorf("failed to add host entry '%s' to /etc/hosts: %w", entryLine, errAppend)
	}
	return nil
}

// --- Stubs for new network methods from enriched interface ---

func (r *defaultRunner) DisableFirewall(ctx context.Context, conn connector.Connector, facts *Facts) error {
	return fmt.Errorf("DisableFirewall not implemented")
}
