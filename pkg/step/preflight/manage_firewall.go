package preflight

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// ManageFirewallStepSpec defines parameters for managing common firewall services.
type ManageFirewallStepSpec struct {
	spec.StepMeta `json:",inline"`

	DesiredState    string   `json:"desiredState,omitempty"` // "disabled" or "enabled"
	CommonFirewalls []string `json:"commonFirewalls,omitempty"`
	Sudo            bool     `json:"sudo,omitempty"`
}

// NewManageFirewallStepSpec creates a new ManageFirewallStepSpec.
func NewManageFirewallStepSpec(name, description string) *ManageFirewallStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Manage Common Firewalls"
	}
	finalDescription := description
	// Description will be refined in populateDefaults.

	return &ManageFirewallStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		// Defaults will be set in populateDefaults
	}
}

// Name returns the step's name.
func (s *ManageFirewallStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *ManageFirewallStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *ManageFirewallStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *ManageFirewallStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *ManageFirewallStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ManageFirewallStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *ManageFirewallStepSpec) populateDefaults(logger runtime.Logger) {
	if s.DesiredState == "" {
		s.DesiredState = "disabled"
		logger.Debug("DesiredState defaulted to 'disabled'.")
	}
	if len(s.CommonFirewalls) == 0 {
		s.CommonFirewalls = []string{"firewalld", "ufw"}
		logger.Debug("CommonFirewalls defaulted.", "firewalls", s.CommonFirewalls)
	}
	// Sudo is true by default for systemctl/firewall commands
	if !s.Sudo && (s.DesiredState == "disabled" || s.DesiredState == "enabled") { // Only force sudo if not already set and action is required
		// The prompt said "Sudo defaults to true", but it's a bool.
		// A common pattern is to default it true if not explicitly set false.
		// However, the struct field will be false by default.
		// Let's make it explicitly true here if no value was provided (which means it's false).
		// This logic is a bit circular. Better: factory sets it true, or user sets it.
		// For now, if it's false (zero value), set it to true.
		s.Sudo = true // Explicitly default to true here if not set by user (as bool default is false)
		logger.Debug("Sudo defaulted to true as it's typical for firewall management.")
	}


	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Ensures common firewalls (%s) are in '%s' state.",
			strings.Join(s.CommonFirewalls, ", "), s.DesiredState)
	}
}

// isFirewallActive checks if a given firewall service is active.
// Returns (isActive, isPresent, error)
func isFirewallActive(ctx runtime.StepContext, host connector.Host, conn connector.Connector, firewallService string, useSudo bool) (bool, bool, error) {
	logger := ctx.GetLogger().With("firewall", firewallService)
	execOpts := &connector.ExecOptions{Sudo: useSudo}
	isPresent := true

	// Check if service exists first (systemctl list-units can be slow)
	// A lighter check: systemctl status <service> will error if not found.
	statusCmd := fmt.Sprintf("systemctl status %s", firewallService)
	_, _, statusErr := conn.Exec(ctx.GoContext(), statusCmd, execOpts)
	if statusErr != nil {
		// Heuristic: if "could not be found" or "not found", then not present.
		errMsg := strings.ToLower(statusErr.Error())
		if strings.Contains(errMsg, "could not be found") || strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "no such file") {
			logger.Debug("Firewall service not found.", "error", statusErr)
			isPresent = false
			return false, isPresent, nil // Not active because not present
		}
		// Other error checking status (e.g. permission denied if sudo was false but needed)
		logger.Warn("Error checking firewall status, cannot determine presence accurately.", "error", statusErr)
		// Can't determine presence, assume it might be there and active for safety if desired state is disabled.
		// Or, assume not active if desired state is enabled. This is tricky.
		// For now, let error propagate if it's not a clear "not found".
		// However, for precheck, we might want to be less strict.
		// Let's assume for precheck, if status check fails unexpectedly, it's not 'active'.
		return false, true, fmt.Errorf("error checking status of %s: %w", firewallService, statusErr)
	}


	isActiveCmd := fmt.Sprintf("systemctl is-active %s", firewallService)
	stdout, _, err := conn.Exec(ctx.GoContext(), isActiveCmd, execOpts)
	if err != nil {
		// is-active returns non-zero if not active. This is not an execution error.
		logger.Debug("Firewall service is not active (is-active returned non-zero).", "output", strings.TrimSpace(string(stdout)))
		return false, isPresent, nil
	}
	// If command succeeds (exit 0), output is "active" or "inactive" or "activating"
	return strings.TrimSpace(string(stdout)) == "active", isPresent, nil
}


// Precheck determines if firewalls are in the desired state.
func (s *ManageFirewallStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	allInDesiredState := true
	foundAnyFirewall := false

	for _, fwService := range s.CommonFirewalls {
		var isActive, isPresent bool
		var checkErr error

		if fwService == "ufw" {
			// Check if ufw command exists first
			if _, errLookPath := conn.LookPath(ctx.GoContext(), "ufw"); errLookPath != nil {
				logger.Debug("ufw command not found, skipping ufw checks.")
				isPresent = false
				isActive = false
			} else {
				isPresent = true
				// `ufw status` output: "Status: active" or "Status: inactive"
				// Sudo might be needed for ufw status depending on system.
				stdout, _, errUfw := conn.Exec(ctx.GoContext(), "ufw status", &connector.ExecOptions{Sudo: s.Sudo})
				if errUfw != nil {
					// If ufw status errors (e.g. ufw not loaded), assume inactive for safety if desired is "disabled".
					logger.Warn("`ufw status` command failed. Assuming ufw is inactive for precheck.", "error", errUfw)
					isActive = false
				} else {
					isActive = strings.Contains(strings.ToLower(string(stdout)), "status: active")
				}
			}
		} else { // Assume systemd service like firewalld
			isActive, isPresent, checkErr = isFirewallActive(ctx, host, conn, fwService, s.Sudo)
			if checkErr != nil {
				// If checking status failed for a known service, we can't be sure. Let Run proceed.
				logger.Warn("Could not determine active state for service, precheck will indicate run is needed.", "service", fwService, "error", checkErr)
				return false, nil
			}
		}

		if isPresent {
			foundAnyFirewall = true
			logger.Debug("Firewall status checked.", "firewall", fwService, "isActive", isActive, "isPresent", isPresent)
			if s.DesiredState == "disabled" && isActive {
				allInDesiredState = false
				logger.Info("Firewall is active, but desired state is disabled.", "firewall", fwService)
				break
			}
			if s.DesiredState == "enabled" && !isActive {
				allInDesiredState = false
				logger.Info("Firewall is inactive, but desired state is enabled.", "firewall", fwService)
				// For "enabled", if any of the common firewalls is not active, we might need to run.
				// The run logic will try to enable one.
				break
			}
		}
	}

	if !foundAnyFirewall && s.DesiredState == "disabled" {
	    logger.Info("No common firewalls found on the system. Desired state 'disabled' is met.")
	    return true, nil
	}


	if allInDesiredState {
		logger.Info("All checked firewalls are already in the desired state.", "state", s.DesiredState)
		return true, nil
	}

	logger.Info("Firewall state not as desired, step needs to run.")
	return false, nil
}

// Run manages the state of common firewalls.
func (s *ManageFirewallStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	for _, fwService := range s.CommonFirewalls {
		// Check if the service/command exists before trying to manage it
		var manageCmds []string
		serviceExists := false
		if fwService == "ufw" {
			if _, errLookPath := conn.LookPath(ctx.GoContext(), "ufw"); errLookPath == nil {
				serviceExists = true
				if s.DesiredState == "disabled" {
					manageCmds = append(manageCmds, "ufw disable") // ufw disable is idempotent
				} else if s.DesiredState == "enabled" {
					// `yes | ufw enable` or `ufw --force enable` might be needed for non-interactive.
					// This is risky. For now, use standard enable.
					// If this fails due to interactivity, the step needs adjustment or manual pre-configuration.
					manageCmds = append(manageCmds, "ufw enable")
				}
			}
		} else { // systemd services like firewalld
			// A lightweight check for service existence (e.g. systemctl list-unit-files) could be done.
			// For simplicity, we rely on errors from systemctl commands if service is not found.
			// `systemctl status` check in `isFirewallActive` can give an idea, but might not be run if precheck skipped.
			// Let's assume if we reach here, we attempt the action.
			serviceExists = true // Assume systemd services might exist if listed
			if s.DesiredState == "disabled" {
				manageCmds = append(manageCmds, fmt.Sprintf("systemctl stop %s", fwService))
				manageCmds = append(manageCmds, fmt.Sprintf("systemctl disable %s", fwService))
			} else if s.DesiredState == "enabled" {
				manageCmds = append(manageCmds, fmt.Sprintf("systemctl enable %s", fwService))
				manageCmds = append(manageCmds, fmt.Sprintf("systemctl start %s", fwService))
			}
		}

		if !serviceExists {
			logger.Debug("Firewall service/command not found, skipping management.", "firewall", fwService)
			continue
		}

		for _, cmd := range manageCmds {
			logger.Info("Executing firewall management command.", "command", cmd)
			_, stderr, errCmd := conn.Exec(ctx.GoContext(), cmd, execOpts)
			if errCmd != nil {
				// For disabling, if service is not found or not active, errors are often ignorable.
				errMsg := strings.ToLower(string(stderr) + errCmd.Error())
				isNotFound := strings.Contains(errMsg, "not found") || strings.Contains(errMsg, "no such file")
				isNotActive := strings.Contains(errMsg, "is not active") || strings.Contains(errMsg, "inactive")

				if s.DesiredState == "disabled" && (isNotFound || (strings.Contains(cmd, "stop") && isNotActive))) {
					logger.Info("Command to stop/disable firewall indicated service was not found or already inactive. Considered successful.", "command", cmd, "output", string(stderr))
				} else {
					// For enabling, or other errors during disabling, log as warning as other firewalls might be handled.
					logger.Warn("Firewall management command failed (best-effort).", "command", cmd, "stderr", string(stderr), "error", errCmd)
				}
			} else {
				logger.Info("Firewall management command executed successfully.", "command", cmd)
			}
		}
	}
	logger.Info("Firewall management actions completed.")
	return nil
}

// Rollback for firewall management is typically not performed.
func (s *ManageFirewallStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for firewall state management is not automatically performed to avoid unintended security changes.")
	return nil
}

var _ step.Step = (*ManageFirewallStepSpec)(nil)
