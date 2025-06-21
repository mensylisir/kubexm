package os

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	// "github.com/mensylisir/kubexm/pkg/engine" // Removed
	"github.com/mensylisir/kubexm/pkg/runner" // For runner.Runner type
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"   // For step.StepContext
)

// DisableFirewallStep attempts to disable common firewalls like firewalld and ufw.
type DisableFirewallStep struct {
	meta             spec.StepMeta
	Sudo             bool
	TargetFirewalls  []string // e.g., ["firewalld", "ufw"] or empty to try known ones
	originalStatuses map[string]string // For rollback: "firewalld" -> "active"
}

// NewDisableFirewallStep creates a new DisableFirewallStep.
func NewDisableFirewallStep(instanceName string, sudo bool, targetFirewalls []string) step.Step {
	name := instanceName
	if name == "" {
		name = "DisableFirewall"
	}
	if len(targetFirewalls) == 0 {
		targetFirewalls = []string{"firewalld", "ufw"} // Default known firewalls
	}
	return &DisableFirewallStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Disables common firewalls: %v", targetFirewalls),
		},
		Sudo:             sudo,
		TargetFirewalls:  targetFirewalls,
		originalStatuses: make(map[string]string),
	}
}

func (s *DisableFirewallStep) Meta() *spec.StepMeta {
	return &s.meta
}

// getFirewallStatus checks if a specific firewall service is active.
// Returns "active", "inactive", "not_found", or "error".
func (s *DisableFirewallStep) getFirewallStatus(ctx step.StepContext, host connector.Host, runnerSvc runner.Runner, conn connector.Connector, firewallService string) string { // Changed to step.StepContext
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "firewall", firewallService)
	isActiveCmd := fmt.Sprintf("systemctl is-active %s", firewallService)
	// We don't want this check command to fail the step if the service doesn't exist.
	// The 'Check' field was conceptual and doesn't exist in ExecOptions.
	// The command 'systemctl is-active' itself returns specific exit codes for active/inactive/not-found.
	// The runnerSvc.RunWithOptions should return a CommandError that includes the exit code.
	execOpts := &connector.ExecOptions{Sudo: s.Sudo, Retries: 0}

	stdoutBytes, _, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, isActiveCmd, execOpts)
	statusOutput := strings.TrimSpace(string(stdoutBytes))

	if err != nil {
		// Check if error is because service is not found or inactive, which is not a "failure" for this check.
		// A CommandError with specific exit codes for "not found" (e.g., 3 for systemctl) or "inactive" could be checked.
		// For simplicity, if there's an error, we might assume it's not deterministically active.
		// However, `systemctl is-active` returns "inactive" (exit code 3) or "unknown" (exit code 4 for not found)
		// and these are not Go errors from Exec. A Go error means the command itself failed to run.
		logger.Debug("Error checking firewall status. Assuming not active or not found.", "error", err)

		// More robust: Check if service file exists
		// exists, _ := runnerSvc.Exists(ctx.GoContext(), conn, fmt.Sprintf("/lib/systemd/system/%s.service", firewallService))
		// if !exists { return "not_found"}

		// For now, if `is-active` results in an execution error (not specific exit codes for inactive/not-found),
		// treat as error in detection.
		// However, if it's CommandError, the exit code might tell us more.
		// `systemctl is-active` returns exit code 0 for active, 3 for inactive/failed.
		// If `err` is nil, stdout is the status. If `err` is a CommandError with ExitCode 3, it's inactive.
		if cmdErr, ok := err.(*connector.CommandError); ok {
			if cmdErr.ExitCode == 3 { // Typically "inactive" or "failed"
				return "inactive"
			}
			if cmdErr.ExitCode == 4 { // Typically "not found" by systemctl
			    return "not_found"
			}
		}
		// If any other error, it's an issue with the command execution itself.
		logger.Warn("Could not reliably determine firewall status due to command execution error.", "error", err)
		return "error"
	}

	return statusOutput // "active", "inactive", "activating" etc.
}

func (s *DisableFirewallStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) { // Changed to step.StepContext
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	allDisabledOrNotFound := true
	for _, fw := range s.TargetFirewalls {
		status := s.getFirewallStatus(ctx, host, runnerSvc, conn, fw)
		logger.Debug("Firewall status check.", "firewall", fw, "status", status)
		if status == "active" || status == "activating" {
			logger.Info("Firewall is active, precheck fails.", "firewall", fw)
			allDisabledOrNotFound = false
			break
		}
	}

	if allDisabledOrNotFound {
		logger.Info("All targeted firewalls are already disabled or not found.")
		return true, nil
	}
	return false, nil
}

func (s *DisableFirewallStep) Run(ctx step.StepContext, host connector.Host) error { // Changed to step.StepContext
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	s.originalStatuses = make(map[string]string) // Clear for current run

	for _, fw := range s.TargetFirewalls { // Corrected loop syntax
		status := s.getFirewallStatus(ctx, host, runnerSvc, conn, fw)
		s.originalStatuses[fw] = status // Store for potential rollback
		logger.Debug("Original status recorded for rollback.", "firewall", fw, "status", status)

		if status == "active" || status == "activating" {
			logger.Info("Attempting to disable firewall.", "firewall", fw)

			stopCmd := fmt.Sprintf("systemctl stop %s", fw)
			if _, errStop := runnerSvc.Run(ctx.GoContext(), conn, stopCmd, s.Sudo); errStop != nil {
				logger.Warn("Failed to stop firewall service (continuing to disable).", "firewall", fw, "error", errStop)
			} else {
				logger.Info("Firewall service stopped.", "firewall", fw)
			}

			disableCmd := fmt.Sprintf("systemctl disable %s", fw)
			if _, errDisable := runnerSvc.Run(ctx.GoContext(), conn, disableCmd, s.Sudo); errDisable != nil {
				// Log error but don't fail the whole step, as other firewalls might need disabling.
				// However, if a specific firewall *must* be disabled, this might be an error.
				logger.Error("Failed to disable firewall service.", "firewall", fw, "error", errDisable)
				// Potentially collect errors and return a combined one if strictness is needed.
			} else {
				logger.Info("Firewall service disabled.", "firewall", fw)
			}
		} else {
			logger.Info("Firewall not active or not found, skipping disable.", "firewall", fw, "status", status)
		}
	}
	return nil // Best effort for disabling multiple firewalls
}

func (s *DisableFirewallStep) Rollback(ctx step.StepContext, host connector.Host) error { // Changed to step.StepContext
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}

	for firewall, originalStatus := range s.originalStatuses {
		if originalStatus == "active" || originalStatus == "activating" {
			logger.Info("Attempting to restore firewall state for rollback.", "firewall", firewall, "originalStatus", originalStatus)

			enableCmd := fmt.Sprintf("systemctl enable %s", firewall)
			if _, errEnable := runnerSvc.Run(ctx.GoContext(), conn, enableCmd, s.Sudo); errEnable != nil {
				logger.Warn("Failed to enable firewall service during rollback.", "firewall", firewall, "error", errEnable)
			} else {
				logger.Info("Firewall service enabled (or was already).", "firewall", firewall)
			}

			startCmd := fmt.Sprintf("systemctl start %s", firewall)
			if _, errStart := runnerSvc.Run(ctx.GoContext(), conn, startCmd, s.Sudo); errStart != nil {
				logger.Warn("Failed to start firewall service during rollback.", "firewall", firewall, "error", errStart)
			} else {
				logger.Info("Firewall service started (or was already).", "firewall", firewall)
			}
		} else {
			logger.Info("Firewall was not originally active, no rollback action needed.", "firewall", firewall, "originalStatus", originalStatus)
		}
	}
	return nil // Best effort for rollback
}

var _ step.Step = (*DisableFirewallStep)(nil)
