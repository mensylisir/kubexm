package preflight

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// CheckKernelModuleLoadedStepSpec defines parameters for checking if specified kernel modules are loaded.
type CheckKernelModuleLoadedStepSpec struct {
	spec.StepMeta `json:",inline"`

	ModulesToCheck            []string `json:"modulesToCheck,omitempty"` // Required
	OutputLoadedModulesCacheKey string   `json:"outputLoadedModulesCacheKey,omitempty"`
	OutputAllLoadedCacheKey   string   `json:"outputAllLoadedCacheKey,omitempty"`
	Sudo                      bool     `json:"sudo,omitempty"` // For lsmod, typically not needed.
}

// NewCheckKernelModuleLoadedStepSpec creates a new CheckKernelModuleLoadedStepSpec.
func NewCheckKernelModuleLoadedStepSpec(name, description string, modulesToCheck []string) *CheckKernelModuleLoadedStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Check Kernel Modules Loaded"
	}
	finalDescription := description
	// Description refined in populateDefaults

	if len(modulesToCheck) == 0 {
		// This is a required field for the step to be meaningful.
	}

	return &CheckKernelModuleLoadedStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		ModulesToCheck: modulesToCheck,
	}
}

// Name returns the step's name.
func (s *CheckKernelModuleLoadedStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *CheckKernelModuleLoadedStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *CheckKernelModuleLoadedStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *CheckKernelModuleLoadedStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *CheckKernelModuleLoadedStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *CheckKernelModuleLoadedStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *CheckKernelModuleLoadedStepSpec) populateDefaults(logger runtime.Logger) {
	// Sudo defaults to false (Go zero value) for lsmod.
	if s.StepMeta.Description == "" {
		if len(s.ModulesToCheck) > 0 {
			s.StepMeta.Description = fmt.Sprintf("Checks if kernel modules [%s] are loaded.", strings.Join(s.ModulesToCheck, ", "))
		} else {
			s.StepMeta.Description = "Checks if specified kernel modules are loaded (none specified)."
		}
	}
}

// isModuleLoadedInternal checks a single module.
func (s *CheckKernelModuleLoadedStepSpec) isModuleLoadedInternal(ctx runtime.StepContext, conn connector.Connector, moduleName string) (bool, error) {
	// lsmod | awk '{print $1}' | grep -xq <moduleName>
	// The -x flag makes grep match the whole line. -q makes it quiet.
	// Exit status 0 if found, 1 if not.
	cmd := fmt.Sprintf("lsmod | awk '{print $1}' | grep -xq %s", moduleName)
	execOpts := &connector.ExecOptions{Sudo: s.Sudo} // Sudo for lsmod, usually not needed.
	_, _, err := conn.Exec(ctx.GoContext(), cmd, execOpts)

	if err == nil {
		return true, nil // Found
	}
	var cmdErr *connector.CommandError
	if errors.As(err, &cmdErr) {
		if cmdErr.ExitCode == 1 { // Grep not found
			return false, nil
		}
	}
	// Other errors
	return false, fmt.Errorf("error checking module %s: %w", moduleName, err)
}

// Precheck attempts to use cached information if available.
func (s *CheckKernelModuleLoadedStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if len(s.ModulesToCheck) == 0 {
		logger.Info("No modules specified in ModulesToCheck. Precheck done.")
		return true, nil
	}

	if s.OutputAllLoadedCacheKey != "" {
		cachedAllLoadedVal, found := ctx.StepCache().Get(s.OutputAllLoadedCacheKey)
		if found {
			allLoaded, ok := cachedAllLoadedVal.(bool)
			if !ok {
				logger.Warn("Invalid cached 'all loaded' status type, re-running check.", "key", s.OutputAllLoadedCacheKey)
				return false, nil
			}
			if allLoaded {
				// To be fully idempotent, we should also check if OutputLoadedModulesCacheKey matches the current ModulesToCheck.
				// For simplicity, if all were loaded according to cache, assume done.
				logger.Info("All specified modules cached as loaded. Precheck done.")
				return true, nil
			}
			// If cached as not all loaded, Run needs to re-evaluate.
			logger.Info("Cached status indicates not all modules were loaded. Re-running check.")
			return false, nil
		}
	}
	return false, nil // Default to run the check
}

// Run performs the kernel module checks.
func (s *CheckKernelModuleLoadedStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	if len(s.ModulesToCheck) == 0 {
		logger.Info("No modules specified in ModulesToCheck. Nothing to do.")
		if s.OutputAllLoadedCacheKey != "" {
			ctx.StepCache().Set(s.OutputAllLoadedCacheKey, true) // No modules to check means all (zero) are loaded.
		}
		if s.OutputLoadedModulesCacheKey != "" {
			ctx.StepCache().Set(s.OutputLoadedModulesCacheKey, []string{})
		}
		return nil
	}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	loadedModules := []string{}
	allAreLoaded := true

	for _, moduleName := range s.ModulesToCheck {
		isLoaded, checkErr := s.isModuleLoadedInternal(ctx, conn, moduleName)
		if checkErr != nil {
			// Log the error for this specific module but continue checking others.
			// The step itself won't fail unless this is critical.
			// For a "check" step, an error during the check of one item might mean we can't confirm all.
			logger.Error("Error checking status of module.", "module", moduleName, "error", checkErr)
			allAreLoaded = false // Cannot confirm this module, so not all confirmed loaded.
			// Optionally, this could return the error and stop:
			// return fmt.Errorf("failed to check status for module %s on host %s: %w", moduleName, host.GetName(), checkErr)
			continue // Or let it proceed to check others and report overall.
		}

		if isLoaded {
			logger.Info("Kernel module is loaded.", "module", moduleName)
			loadedModules = append(loadedModules, moduleName)
		} else {
			logger.Warn("Kernel module is NOT loaded.", "module", moduleName)
			allAreLoaded = false
		}
	}

	if s.OutputLoadedModulesCacheKey != "" {
		ctx.StepCache().Set(s.OutputLoadedModulesCacheKey, loadedModules)
		logger.Debug("Stored list of loaded modules in cache.", "key", s.OutputLoadedModulesCacheKey, "modules", loadedModules)
	}
	if s.OutputAllLoadedCacheKey != "" {
		ctx.StepCache().Set(s.OutputAllLoadedCacheKey, allAreLoaded)
		logger.Debug("Stored 'all loaded' status in cache.", "key", s.OutputAllLoadedCacheKey, "status", allAreLoaded)
	}

	// This step's purpose is to check and report/cache. It doesn't fail if modules are not loaded.
	// If specific modules *must* be loaded, that should be enforced by a LoadKernelModulesStepSpec
	// or by logic in a Task that consumes the output of this check.
	logger.Info("Kernel module check completed.", "allSpecifiedModulesLoaded", allAreLoaded, "loadedCheckedModules", loadedModules)
	return nil
}

// Rollback for CheckKernelModuleLoadedStep is a no-op.
func (s *CheckKernelModuleLoadedStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Debug("CheckKernelModuleLoadedStep has no rollback action.")
	// Clear cache keys if they were set by this step instance?
	// For now, no, as rollback is no-op.
	// if s.OutputLoadedModulesCacheKey != "" { ctx.StepCache().Delete(s.OutputLoadedModulesCacheKey) }
	// if s.OutputAllLoadedCacheKey != "" { ctx.StepCache().Delete(s.OutputAllLoadedCacheKey) }
	return nil
}

var _ step.Step = (*CheckKernelModuleLoadedStepSpec)(nil)
