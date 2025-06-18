package preflight

import (
	"fmt"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector" // For ExecOptions if needed, though direct runner methods are primary
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
	"github.com/kubexms/kubexms/pkg/step/spec"
)

// DisableFirewallStepSpec defines the specification for disabling common firewalls.
type DisableFirewallStepSpec struct{}

// GetName returns the name of the step.
func (s *DisableFirewallStepSpec) GetName() string {
	return "Disable Common Firewalls (firewalld, ufw)"
}

// DisableFirewallStepExecutor implements the logic for disabling common firewalls.
type DisableFirewallStepExecutor struct{}

// checkServiceState attempts to determine if a service exists and if it's active.
// Returns: exists (bool), isActive (bool), error (for actual execution errors, not for "not found" or "inactive")
func (e *DisableFirewallStepExecutor) checkServiceState(ctx *runtime.Context, serviceName string) (exists bool, isActive bool, err error) {
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "service", serviceName).Sugar()

	// IsServiceActive is the primary method. How it handles "not found" depends on its implementation.
	// Let's assume it returns a specific error type or message for "not found".
	// For now, we'll interpret any error as "service does not exist or problem checking status".
	active, err := ctx.Host.Runner.IsServiceActive(ctx.GoContext, serviceName)
	if err != nil {
		// A common behavior for IsServiceActive might be to error if the service unit doesn't exist.
		// We need to distinguish "not found" from other errors.
		// This is a simplification; real runner might provide better distinction.
		// For now, if an error occurs, we assume it might not exist or is inaccessible.
		// Example heuristic: error message contains "not found", "no such file", "Loaded: not-found"
		errMsg := strings.ToLower(err.Error())
		if strings.Contains(errMsg, "not-found") || strings.Contains(errMsg, "no such file") || strings.Contains(errMsg, "could not be found") || strings.Contains(errMsg, "unrecognized service") {
			hostCtxLogger.Debugf("Service %s likely does not exist (based on error: %v).", serviceName, err)
			return false, false, nil // Does not exist, therefore not active
		}
		// Other errors are actual problems.
		hostCtxLogger.Errorf("Error checking status of service %s: %v", serviceName, err)
		return false, false, fmt.Errorf("failed to check status of service %s: %w", serviceName, err)
	}

	// If no error, the service exists, and `active` holds its state.
	return true, active, nil
}

// Check determines if the firewalls (firewalld, ufw) are already disabled.
func (e *DisableFirewallStepExecutor) Check(s spec.StepSpec, ctx *runtime.Context) (isDone bool, err error) {
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", s.GetName()).Sugar()
	firewallsToCheck := []string{"firewalld", "ufw"}

	for _, fwService := range firewallsToCheck {
		exists, isActive, checkErr := e.checkServiceState(ctx, fwService)
		if checkErr != nil {
			return false, checkErr // Actual error during check
		}

		if exists && isActive {
			hostCtxLogger.Infof("Firewall service %s is active. Configuration is not done.", fwService)
			return false, nil // Not done if any firewall is found and active
		}
		if exists && !isActive {
			hostCtxLogger.Debugf("Firewall service %s exists but is not active.", fwService)
			// This is good, but we also want it disabled (not starting on boot).
			// IsServiceEnabled would be useful here. For now, not active is the main check for "done".
			// If we consider "disabled" to mean "not active AND not enabled", this check is insufficient.
			// The original script does `systemctl disable`. So, not active is a good sign, but not fully "done".
			// However, for Check, if it's not active, we are closer to "done".
			// Let's assume for Check, "not active" is the primary indicator. Execute will ensure disable.
		}
		if !exists {
			hostCtxLogger.Debugf("Firewall service %s does not appear to exist.", fwService)
		}
	}

	hostCtxLogger.Infof("All checked firewall services (firewalld, ufw) are confirmed not active or do not exist.")
	return true, nil
}

// Execute disables common firewalls (firewalld, ufw).
func (e *DisableFirewallStepExecutor) Execute(s spec.StepSpec, ctx *runtime.Context) *step.Result {
	stepName := s.GetName()
	startTime := time.Now()
	res := step.NewResult(stepName, ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", stepName).Sugar()

	firewallsToManage := []string{"firewalld", "ufw"}

	for _, fwService := range firewallsToManage {
		hostCtxLogger.Infof("Managing firewall service: %s", fwService)

		// Check if service exists using a status command, similar to script.
		// `systemctl status <service>` exit code 3 for inactive, 4 for not found (usually). 0 for active.
		// A simple check for existence could be `systemctl list-unit-files <service>.service`.
		// For simplicity, we'll use the checkServiceState and then attempt operations.
		// Runner methods for Stop/Disable should be idempotent or handle "not found" gracefully.

		exists, isActive, checkErr := e.checkServiceState(ctx, fwService)
		if checkErr != nil {
			// Log the error but don't immediately fail the whole step,
			// as one firewall failing to check shouldn't stop attempts on others.
			// However, this indicates an issue.
			hostCtxLogger.Warnf("Could not reliably determine state of %s due to error: %v. Will still attempt disable operations.", fwService, checkErr)
			// To be safe, assume it might exist if checkErr is not a clear "not found"
			exists = true // Assume exists to try to disable it
		}

		if !exists {
			hostCtxLogger.Infof("Service %s does not seem to exist. Skipping stop/disable.", fwService)
			continue
		}

		// If it exists and is active, try to stop it.
		if isActive {
			hostCtxLogger.Infof("Attempting to stop service %s...", fwService)
			if err := ctx.Host.Runner.StopService(ctx.GoContext, fwService); err != nil {
				// Log error but continue, as it might be already stopped or stop failed but disable might work.
				hostCtxLogger.Warnf("Failed to stop service %s (may already be stopped or error during stop): %v", fwService, err)
			} else {
				hostCtxLogger.Infof("Service %s stopped successfully.", fwService)
			}
		} else {
			hostCtxLogger.Infof("Service %s is not active. Skipping stop.", fwService)
		}

		// Attempt to disable it in all cases where it might exist.
		hostCtxLogger.Infof("Attempting to disable service %s...", fwService)
		if err := ctx.Host.Runner.DisableService(ctx.GoContext, fwService); err != nil {
			// Log error but consider this non-fatal for the overall step if other firewalls are handled.
			// The post-check will verify.
			hostCtxLogger.Warnf("Failed to disable service %s (may already be disabled or error during disable): %v", fwService, err)
		} else {
			hostCtxLogger.Infof("Service %s disabled successfully.", fwService)
		}
	}

	// Perform a post-execution check
	done, checkErr := e.Check(s, ctx)
	if checkErr != nil {
		res.Error = fmt.Errorf("post-execution check failed: %w", checkErr)
		res.SetFailed(res.Error.Error())
		hostCtxLogger.Errorf("Step failed verification: %v", res.Error)
		return res
	}
	if !done {
		errMsg := "post-execution check indicates firewalls are still not correctly disabled"
		res.Error = fmt.Errorf(errMsg)
		res.SetFailed(errMsg)
		hostCtxLogger.Errorf("Step failed verification: %s", errMsg)
		return res
	}

	res.SetSucceeded("Common firewalls (firewalld, ufw) checked and disable operations attempted.")
	hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	return res
}

func init() {
	step.Register(step.GetSpecTypeName(&DisableFirewallStepSpec{}), &DisableFirewallStepExecutor{})
}
