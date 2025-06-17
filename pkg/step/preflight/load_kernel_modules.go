package preflight

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/runner"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
)

// LoadKernelModulesStep ensures specified kernel modules are loaded.
type LoadKernelModulesStep struct {
	Modules []string
}

// Name returns a human-readable name for the step.
func (s *LoadKernelModulesStep) Name() string {
	if len(s.Modules) == 0 {
		return "Load Kernel Modules (none specified)"
	}
	return fmt.Sprintf("Load Kernel Modules (%s)", strings.Join(s.Modules, ", "))
}

// isModuleLoaded checks if a single module is loaded using lsmod.
func isModuleLoaded(ctx *runtime.Context, moduleName string) (bool, error) {
	if ctx.Host.Runner == nil {
		return false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	// lsmod | awk '{print $1}' | grep -xq <module_name>
	// -x: match whole line exactly, -q: quiet (status code indicates match)
	cmd := fmt.Sprintf("lsmod | awk '{print $1}' | grep -xq %s", moduleName)

	// Sudo usually not needed for lsmod or grep.
	found, err := ctx.Host.Runner.Check(ctx.GoContext, cmd, false)
	if err != nil {
		// This error means the Check command itself failed to execute, not that grep didn't find the module.
		return false, fmt.Errorf("error executing command to check if module %s is loaded on host %s: %w", moduleName, ctx.Host.Name, err)
	}
	return found, nil // found is true if grep exited 0 (module was listed)
}

// Check determines if all specified kernel modules are already loaded.
func (s *LoadKernelModulesStep) Check(ctx *runtime.Context) (isDone bool, err error) {
	if len(s.Modules) == 0 {
		ctx.Logger.Debugf("No modules specified for step '%s' on host %s, considering done.", s.Name(), ctx.Host.Name)
		return true, nil
	}
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", s.Name()).Sugar()

	for _, mod := range s.Modules {
		loaded, checkErr := isModuleLoaded(ctx, mod)
		if checkErr != nil {
			hostCtxLogger.Errorf("Error checking module %s: %v", mod, checkErr)
			return false, checkErr // Error during check for this module
		}
		if !loaded {
			hostCtxLogger.Debugf("Check: Module %s is not loaded.", mod)
			return false, nil // At least one module is not loaded, so not done.
		}
		hostCtxLogger.Debugf("Check: Module %s is already loaded.", mod)
	}
	hostCtxLogger.Infof("All specified modules (%s) are already loaded.", strings.Join(s.Modules, ", "))
	return true, nil // All modules are loaded
}

// Run attempts to load all specified kernel modules using modprobe.
func (s *LoadKernelModulesStep) Run(ctx *runtime.Context) *step.Result {
	startTime := time.Now()
	res := step.NewResult(s.Name(), ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", s.Name()).Sugar()
	var errorsCollected []string
	var successesCollected []string

	if len(s.Modules) == 0 {
		res.Status = "Succeeded"
		res.Message = "No kernel modules specified to load."
		hostCtxLogger.Infof(res.Message)
		res.EndTime = time.Now()
		return res
	}

	for _, mod := range s.Modules {
		// Re-check if module is loaded before attempting to load, in case Check wasn't called or state changed.
		// This makes Run more robust if called directly.
		isLoadedBeforeRun, _ := isModuleLoaded(ctx, mod)
		if isLoadedBeforeRun {
			hostCtxLogger.Infof("Module %s is already loaded, skipping modprobe.", mod)
			successesCollected = append(successesCollected, mod)
			continue
		}

		hostCtxLogger.Infof("Attempting to load kernel module: %s", mod)
		// Sudo is often required for modprobe.
		_, stderr, err := ctx.Host.Runner.Run(ctx.GoContext, fmt.Sprintf("modprobe %s", mod), true)
		if err != nil {
			errMsg := fmt.Sprintf("failed to load module %s: %v (stderr: %s)", mod, err, string(stderr))
			hostCtxLogger.Warnf(errMsg)
			errorsCollected = append(errorsCollected, errMsg)
			continue // Try next module
		}

		// Verify if loaded after modprobe
		loadedAfterRun, verifyErr := isModuleLoaded(ctx, mod)
		if verifyErr != nil {
			errMsg := fmt.Sprintf("failed to verify module %s after attempting load: %v", mod, verifyErr)
			hostCtxLogger.Warnf(errMsg)
			errorsCollected = append(errorsCollected, errMsg)
		} else if !loadedAfterRun {
			errMsg := fmt.Sprintf("module %s reported as loaded by modprobe, but not found by lsmod verification", mod)
			hostCtxLogger.Warnf(errMsg)
			errorsCollected = append(errorsCollected, errMsg)
		} else {
			hostCtxLogger.Successf("Kernel module %s loaded successfully.", mod)
			successesCollected = append(successesCollected, mod)
		}
	}
	res.EndTime = time.Now()

	if len(errorsCollected) > 0 {
		res.Status = "Failed"
		res.Error = fmt.Errorf(strings.Join(errorsCollected, "; "))
		res.Message = fmt.Sprintf("Successfully loaded: [%s]. Failed or could not verify: %s", strings.Join(successesCollected, ", "), res.Error.Error())
		hostCtxLogger.Errorf("Step finished with errors: %s", res.Message)
	} else {
		res.Status = "Succeeded"
		res.Message = fmt.Sprintf("All specified kernel modules (%s) were successfully loaded and verified.", strings.Join(s.Modules, ", "))
		hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	}
	return res
}

var _ step.Step = &LoadKernelModulesStep{}
