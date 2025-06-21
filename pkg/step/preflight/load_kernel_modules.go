package preflight

import (
	"errors" // For errors.As
	"fmt"
	"strings"
	// "time" // No longer needed

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
)

// LoadKernelModulesStep loads specified kernel modules on the host.
type LoadKernelModulesStep struct {
	meta    spec.StepMeta
	Modules []string
	Sudo    bool // For modprobe command
}

// NewLoadKernelModulesStep creates a new LoadKernelModulesStep.
func NewLoadKernelModulesStep(instanceName string, modules []string, sudo bool) step.Step {
	name := instanceName
	description := ""
	if name == "" {
		if len(modules) == 0 {
			name = "LoadKernelModulesNoneSpecified"
			description = "Ensures no specific kernel modules are loaded (as none were specified)."
		} else {
			name = fmt.Sprintf("LoadKernelModules-%s", strings.ReplaceAll(strings.Join(modules, "_"), ".", "-"))
			description = fmt.Sprintf("Ensures kernel modules are loaded: %s.", strings.Join(modules, ", "))
		}
	} else {
		// If instanceName is provided, generate a description if not part of meta passed in
		description = fmt.Sprintf("Loads kernel modules: %s (instance: %s)", strings.Join(modules, ", "), name)
	}

	return &LoadKernelModulesStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: description,
		},
		Modules: modules,
		Sudo:    sudo,
	}
}

// Meta returns the step's metadata.
func (s *LoadKernelModulesStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *LoadKernelModulesStep) isModuleLoaded(ctx runtime.StepContext, host connector.Host, moduleName string) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "module", moduleName, "operation", "isModuleLoadedCheck")

	runnerSvc := ctx.GetRunner()
	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), errConn)
	}

	cmd := fmt.Sprintf("lsmod | awk '{print $1}' | grep -xq %s", moduleName)
	// lsmod and grep usually don't need sudo. Pass s.Sudo with false for this specific command.
	// If runner.RunWithOptions had a way to specify sudo per command, that would be better.
	// For now, assuming 'false' for sudo for this check.
	_, stderrBytes, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, &connector.ExecOptions{Sudo: false, Check: true}) // Check:true important for grep

	if err == nil {
		return true, nil // Exit code 0 from grep -q means found
	}

	var cmdErr *connector.CommandError
	if errors.As(err, &cmdErr) {
		if cmdErr.ExitCode == 1 { // Grep returns 1 if not found
			return false, nil
		}
		logger.Error("Command to check module loaded status failed with unexpected exit code.", "exit_code", cmdErr.ExitCode, "stderr", string(stderrBytes), "error", err)
		return false, fmt.Errorf("command '%s' execution failed for module %s on host %s (exit code %d, stderr: %s): %w", cmd, moduleName, host.GetName(), cmdErr.ExitCode, string(stderrBytes), err)
	}

	logger.Error("Error executing command to check if module is loaded (non-CommandError).", "stderr", string(stderrBytes), "error", err)
	return false, fmt.Errorf("error executing command '%s' for module %s on host %s: %w", cmd, moduleName, host.GetName(), err)
}

func (s *LoadKernelModulesStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	if host == nil {
		return false, fmt.Errorf("host is nil in Precheck for %s", s.meta.Name)
	}

	if len(s.Modules) == 0 {
		logger.Debug("No modules specified to load, precheck considered done.")
		return true, nil
	}

	for _, mod := range s.Modules {
		loaded, checkErr := s.isModuleLoaded(ctx, host, mod)
		if checkErr != nil {
			logger.Error("Error checking module loaded status.", "module", mod, "error", checkErr)
			return false, checkErr
		}
		if !loaded {
			logger.Info("Module not loaded, Run phase is required.", "module", mod)
			return false, nil
		}
	}
	logger.Info("All specified modules are already loaded.")
	return true, nil
}

func (s *LoadKernelModulesStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	if host == nil {
		return fmt.Errorf("host is nil in Run for %s", s.meta.Name)
	}

	if len(s.Modules) == 0 {
		logger.Info("No modules specified to load.")
		return nil
	}

	runnerSvc := ctx.GetRunner()
	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), errConn)
	}

	var failedModules []string
	for _, mod := range s.Modules {
		loaded, checkErr := s.isModuleLoaded(ctx, host, mod)
		if checkErr != nil {
			logger.Error("Failed to check module status before attempting load.", "module", mod, "error", checkErr)
			failedModules = append(failedModules, fmt.Sprintf("%s (pre-load check error: %v)", mod, checkErr))
			continue
		}
		if loaded {
			logger.Info("Module already loaded, skipping modprobe.", "module", mod)
			continue
		}

		logger.Info("Attempting to load kernel module with modprobe.", "module", mod)
		modprobeCmd := fmt.Sprintf("modprobe %s", mod)
		if _, err := runnerSvc.Run(ctx.GoContext(), conn, modprobeCmd, s.Sudo); err != nil {
			// runner.Run itself should return a CommandError with stderr if applicable
			logger.Error("Failed to execute modprobe for module.", "module", mod, "error", err)
			failedModules = append(failedModules, fmt.Sprintf("%s (modprobe error: %v)", mod, err))
			continue
		}

		loadedAfter, verifyErr := s.isModuleLoaded(ctx, host, mod)
		if verifyErr != nil {
			logger.Error("Failed to verify module status after modprobe.", "module", mod, "error", verifyErr)
			failedModules = append(failedModules, fmt.Sprintf("%s (post-load verification error: %v)", mod, verifyErr))
		} else if !loadedAfter {
			logger.Error("Module load attempted with modprobe, but lsmod verification failed.", "module", mod)
			failedModules = append(failedModules, fmt.Sprintf("%s (not found after modprobe attempt)", mod))
		} else {
			logger.Info("Kernel module loaded successfully.", "module", mod)
		}
	}

	if len(failedModules) > 0 {
		return fmt.Errorf("failed to load/verify kernel module(s) for step %s on host %s: %s", s.meta.Name, host.GetName(), strings.Join(failedModules, "; "))
	}
	return nil
}

func (s *LoadKernelModulesStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for LoadKernelModules is a no-op by default to avoid system instability from unloading modules.")
	return nil
}

// Ensure LoadKernelModulesStep implements the step.Step interface.
var _ step.Step = (*LoadKernelModulesStep)(nil)
