package preflight

import (
	"errors" // For errors.As
	"fmt"
	"strings"
	// "time" // No longer needed

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	// spec is no longer needed
)

// LoadKernelModulesStep loads specified kernel modules on the host.
type LoadKernelModulesStep struct {
	Modules  []string
	StepName string
}

// NewLoadKernelModulesStep creates a new LoadKernelModulesStep.
func NewLoadKernelModulesStep(modules []string, stepName string) step.Step {
	name := stepName
	if name == "" {
		if len(modules) == 0 {
			name = "Load Kernel Modules (none specified)"
		} else {
			name = fmt.Sprintf("Load Kernel Modules (%s)", strings.Join(modules, ", "))
		}
	}
	return &LoadKernelModulesStep{
		Modules:  modules,
		StepName: name,
	}
}

func (s *LoadKernelModulesStep) isModuleLoaded(ctx runtime.StepContext, host connector.Host, moduleName string) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "module", moduleName, "operation", "isModuleLoadedCheck")

	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), errConn)
	}

	// Check with lsmod, awk to get first column (module name), and grep.
	// grep -x matches whole line, -q for quiet.
	cmd := fmt.Sprintf("lsmod | awk '{print $1}' | grep -xq %s", moduleName)
	_, stderrBytes, err := conn.Exec(ctx.GoContext(), cmd, &connector.ExecOptions{Sudo: false}) // lsmod usually doesn't need sudo

	if err == nil {
		return true, nil // Exit code 0 from grep -q means found
	}

	var cmdErr *connector.CommandError
	if errors.As(err, &cmdErr) {
		if cmdErr.ExitCode == 1 { // Grep returns 1 if not found
			return false, nil
		}
		// For other exit codes from grep, or if awk/lsmod failed before grep.
		logger.Error("Command to check module loaded status failed with unexpected exit code.", "exit_code", cmdErr.ExitCode, "stderr", string(stderrBytes), "error", err)
		return false, fmt.Errorf("command '%s' execution failed for module %s on host %s (exit code %d, stderr: %s): %w", cmd, moduleName, host.GetName(), cmdErr.ExitCode, string(stderrBytes), err)
	}

	// Non-CommandError type, e.g., context canceled, connection issue
	logger.Error("Error executing command to check if module is loaded (non-CommandError).", "stderr", string(stderrBytes), "error", err)
	return false, fmt.Errorf("error executing command '%s' for module %s on host %s: %w", cmd, moduleName, host.GetName(), err)
}

func (s *LoadKernelModulesStep) Name() string {
	return s.StepName
}

func (s *LoadKernelModulesStep) Description() string {
	if len(s.Modules) == 0 {
		return "Ensures no specific kernel modules are loaded (as none were specified)."
	}
	return fmt.Sprintf("Ensures kernel modules are loaded: %s.", strings.Join(s.Modules, ", "))
}

func (s *LoadKernelModulesStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Precheck")
    if host == nil { return false, fmt.Errorf("host is nil in Precheck for %s", s.Name())}

	if len(s.Modules) == 0 {
		logger.Debug("No modules specified to load, precheck considered done.")
		return true, nil
	}

	for _, mod := range s.Modules {
		loaded, checkErr := s.isModuleLoaded(ctx, host, mod)
		if checkErr != nil {
			logger.Error("Error checking module loaded status.", "module", mod, "error", checkErr)
			return false, checkErr // Propagate error: if we can't check, we can't be sure it's done.
		}
		if !loaded {
			logger.Info("Module not loaded, Run phase is required.", "module", mod)
			return false, nil // At least one module not loaded
		}
	}
	logger.Info("All specified modules are already loaded.")
	return true, nil // All modules are loaded
}

func (s *LoadKernelModulesStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Run")
    if host == nil { return fmt.Errorf("host is nil in Run for %s", s.Name())}

	if len(s.Modules) == 0 {
		logger.Info("No modules specified to load.")
		return nil
	}

	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), errConn)
	}

	var failedModules []string
	for _, mod := range s.Modules {
		// Check if module is already loaded before attempting modprobe
		// This avoids unnecessary modprobe calls if Precheck was skipped or state changed.
		loaded, checkErr := s.isModuleLoaded(ctx, host, mod)
		if checkErr != nil {
			// If we can't even check, log it and add to failed.
			logger.Error("Failed to check module status before attempting load.", "module", mod, "error", checkErr)
			failedModules = append(failedModules, fmt.Sprintf("%s (pre-load check error: %v)", mod, checkErr))
			continue
		}
		if loaded {
		    logger.Info("Module already loaded, skipping modprobe.", "module", mod)
		    continue
		}

		logger.Info("Attempting to load kernel module with modprobe.", "module", mod)
		// modprobe usually needs sudo.
		_, stderrBytes, err := conn.Exec(ctx.GoContext(), fmt.Sprintf("modprobe %s", mod), &connector.ExecOptions{Sudo: true})
		if err != nil {
			logger.Error("Failed to execute modprobe for module.", "module", mod, "stderr", string(stderrBytes), "error", err)
			failedModules = append(failedModules, fmt.Sprintf("%s (modprobe error: %v, stderr: %s)", mod, err, string(stderrBytes)))
			continue
		}

		// Verify after attempting load
		loadedAfter, verifyErr := s.isModuleLoaded(ctx, host, mod)
		if verifyErr != nil {
			logger.Error("Failed to verify module status after modprobe.", "module", mod, "error", verifyErr)
			failedModules = append(failedModules, fmt.Sprintf("%s (post-load verification error: %v)", mod, verifyErr))
		} else if !loadedAfter {
			// modprobe might exit 0 even if module is blacklisted or fails to load for other reasons.
			logger.Error("Module load attempted with modprobe, but lsmod verification failed.", "module", mod)
			failedModules = append(failedModules, fmt.Sprintf("%s (not found after modprobe attempt)", mod))
		} else {
			logger.Info("Kernel module loaded successfully.", "module", mod)
		}
	}

	if len(failedModules) > 0 {
		return fmt.Errorf("failed to load/verify kernel module(s) for step %s on host %s: %s", s.Name(), host.GetName(), strings.Join(failedModules, "; "))
	}
	return nil
}

func (s *LoadKernelModulesStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for LoadKernelModules is a no-op by default to avoid system instability from unloading modules.")
	return nil
}

// Ensure LoadKernelModulesStep implements the step.Step interface.
var _ step.Step = (*LoadKernelModulesStep)(nil)
