package common

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// CheckServiceStatusStepSpec defines parameters for checking systemd service status.
type CheckServiceStatusStepSpec struct {
	spec.StepMeta `json:",inline"`

	ServiceName           string `json:"serviceName,omitempty"` // Required
	DesiredActiveState    string `json:"desiredActiveState,omitempty"` // "active", "inactive"
	DesiredEnabledState   string `json:"desiredEnabledState,omitempty"`// "enabled", "disabled"
	OutputIsActiveCacheKey  string `json:"outputIsActiveCacheKey,omitempty"`
	OutputIsEnabledCacheKey string `json:"outputIsEnabledCacheKey,omitempty"`
	Sudo                  bool   `json:"sudo,omitempty"` // For systemctl commands, usually true if not root
}

// NewCheckServiceStatusStepSpec creates a new CheckServiceStatusStepSpec.
func NewCheckServiceStatusStepSpec(name, description, serviceName string) *CheckServiceStatusStepSpec {
	finalName := name
	if finalName == "" {
		finalName = fmt.Sprintf("Check Service Status: %s", serviceName)
	}
	finalDescription := description
	// Description refined in populateDefaults

	if serviceName == "" {
		// This is a required field.
	}

	return &CheckServiceStatusStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		ServiceName: serviceName,
	}
}

// Name returns the step's name.
func (s *CheckServiceStatusStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *CheckServiceStatusStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *CheckServiceStatusStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *CheckServiceStatusStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *CheckServiceStatusStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *CheckServiceStatusStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *CheckServiceStatusStepSpec) populateDefaults(logger runtime.Logger) {
	// Sudo defaults to false for status checks as per prompt.
	// The boolean zero value is false, so if not set by user, it's already false.
	// No explicit defaulting to true needed here unless it was set true by factory.
	// The factory does not set Sudo.

	s.DesiredActiveState = strings.ToLower(s.DesiredActiveState)
	s.DesiredEnabledState = strings.ToLower(s.DesiredEnabledState)


	if s.StepMeta.Description == "" {
		desc := fmt.Sprintf("Checks status of service '%s'", s.ServiceName)
		if s.DesiredActiveState != "" {
			desc += fmt.Sprintf(", desiring active state '%s'", s.DesiredActiveState)
		}
		if s.DesiredEnabledState != "" {
			desc += fmt.Sprintf(", desiring enabled state '%s'", s.DesiredEnabledState)
		}
		s.StepMeta.Description = desc + "."
	}
}

// Precheck attempts to use cached information if available.
func (s *CheckServiceStatusStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if s.ServiceName == "" {
		return false, fmt.Errorf("ServiceName must be specified for %s", s.GetName())
	}

	activeStateMatches := true // Assume matches if not specified
	enabledStateMatches := true // Assume matches if not specified

	if s.DesiredActiveState != "" && s.OutputIsActiveCacheKey != "" {
		cachedActiveVal, found := ctx.StepCache().Get(s.OutputIsActiveCacheKey)
		if found {
			cachedIsActive, ok := cachedActiveVal.(bool)
			if !ok {
				logger.Warn("Invalid cached active status type, re-running check.", "key", s.OutputIsActiveCacheKey)
				return false, nil // Need to re-run
			}
			expectedActive := s.DesiredActiveState == "active"
			if cachedIsActive != expectedActive {
				activeStateMatches = false
			}
		} else {
			activeStateMatches = false // Not in cache, need to run
		}
	} else if s.DesiredActiveState != "" { // Desired state specified, but no cache key to check
		activeStateMatches = false
	}


	if s.DesiredEnabledState != "" && s.OutputIsEnabledCacheKey != "" {
		cachedEnabledVal, found := ctx.StepCache().Get(s.OutputIsEnabledCacheKey)
		if found {
			cachedIsEnabled, ok := cachedEnabledVal.(bool)
			if !ok {
				logger.Warn("Invalid cached enabled status type, re-running check.", "key", s.OutputIsEnabledCacheKey)
				return false, nil // Need to re-run
			}
			expectedEnabled := s.DesiredEnabledState == "enabled"
			if cachedIsEnabled != expectedEnabled {
				enabledStateMatches = false
			}
		} else {
			enabledStateMatches = false // Not in cache, need to run
		}
	} else if s.DesiredEnabledState != "" { // Desired state specified, but no cache key to check
		enabledStateMatches = false
	}

	if activeStateMatches && enabledStateMatches {
		logger.Info("Service status already matches desired and cached state(s). Precheck done.")
		return true, nil
	}

	logger.Debug("Service status needs to be checked or cache updated.", "activeMatch", activeStateMatches, "enabledMatch", enabledStateMatches)
	return false, nil // Default to run the check
}

// Run performs the service status checks.
func (s *CheckServiceStatusStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if s.ServiceName == "" {
		return fmt.Errorf("ServiceName must be specified for %s", s.GetName())
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// Check Active State
	isActiveCmd := fmt.Sprintf("systemctl is-active %s", s.ServiceName)
	_, _, errActive := conn.Exec(ctx.GoContext(), isActiveCmd, execOpts)
	isActive := errActive == nil // systemctl is-active returns 0 (no error) if active

	logger.Info("Service active status.", "service", s.ServiceName, "isActive", isActive)
	if s.OutputIsActiveCacheKey != "" {
		ctx.StepCache().Set(s.OutputIsActiveCacheKey, isActive)
		logger.Debug("Stored active status in cache.", "key", s.OutputIsActiveCacheKey, "status", isActive)
	}
	if s.DesiredActiveState != "" {
		expectedActive := s.DesiredActiveState == "active"
		if isActive != expectedActive {
			logger.Warn("Service active state does not match desired.", "actual", isActive, "desired", expectedActive)
			// This step only checks and logs/caches. It does not enforce or return an error for mismatch.
		} else {
			logger.Info("Service active state matches desired.", "state", s.DesiredActiveState)
		}
	}

	// Check Enabled State
	isEnabledCmd := fmt.Sprintf("systemctl is-enabled %s", s.ServiceName)
	// systemctl is-enabled returns 0 if enabled, 1 if disabled.
	// Other non-zero codes for static, indirect, etc. We simplify to "enabled" (exit 0) vs "not strictly enabled".
	stdoutEnabled, _, errEnabled := conn.Exec(ctx.GoContext(), isEnabledCmd, execOpts)
	isEnabled := errEnabled == nil
	actualEnabledStatus := strings.TrimSpace(string(stdoutEnabled))


	logger.Info("Service enabled status.", "service", s.ServiceName, "isEnabledByExitCode", isEnabled, "rawOutput", actualEnabledStatus)
	if s.OutputIsEnabledCacheKey != "" {
		// For cache, store the boolean interpretation based on exit code.
		ctx.StepCache().Set(s.OutputIsEnabledCacheKey, isEnabled)
		logger.Debug("Stored enabled status (boolean based on exit code) in cache.", "key", s.OutputIsEnabledCacheKey, "status", isEnabled)
	}
	if s.DesiredEnabledState != "" {
		expectedEnabled := s.DesiredEnabledState == "enabled"
		if isEnabled != expectedEnabled {
			logger.Warn("Service enabled state does not match desired.", "actualBoolean", isEnabled, "rawOutput", actualEnabledStatus, "desired", expectedEnabled)
		} else {
			logger.Info("Service enabled state matches desired.", "state", s.DesiredEnabledState)
		}
	}
	return nil // Step's purpose is to check and cache/log, not to fail on mismatch.
}

// Rollback for CheckServiceStatusStep is a no-op.
func (s *CheckServiceStatusStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Debug("CheckServiceStatusStep has no rollback action.")
	return nil
}

var _ step.Step = (*CheckServiceStatusStepSpec)(nil)
