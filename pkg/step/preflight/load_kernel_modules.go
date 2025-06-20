package preflight

import (
	"context" // Required by runtime.Context
	"fmt"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector" // For connector.ExecOptions
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step"
)

// LoadKernelModulesStepSpec defines parameters for loading kernel modules.
type LoadKernelModulesStepSpec struct {
	Modules  []string
	StepName string // Optional: for a custom name for this specific step instance
}

// GetName returns the step name.
func (s *LoadKernelModulesStepSpec) GetName() string {
	if s.StepName != "" { return s.StepName }
	if len(s.Modules) == 0 {
		return "Load Kernel Modules (none specified)"
	}
	return fmt.Sprintf("Load Kernel Modules (%s)", strings.Join(s.Modules, ", "))
}
var _ spec.StepSpec = &LoadKernelModulesStepSpec{}

// LoadKernelModulesStepExecutor implements the logic for LoadKernelModulesStepSpec.
type LoadKernelModulesStepExecutor struct{}

func init() {
	step.Register(step.GetSpecTypeName(&LoadKernelModulesStepSpec{}), &LoadKernelModulesStepExecutor{})
}

// isModuleLoaded is a helper function, now a private method of the executor.
func (e *LoadKernelModulesStepExecutor) isModuleLoaded(ctx *runtime.Context, moduleName string) (bool, error) {
	if ctx.Host.Runner == nil {
		return false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	// lsmod | awk '{print $1}' | grep -xq <module_name>
	cmd := fmt.Sprintf("lsmod | awk '{print $1}' | grep -xq %s", moduleName)

	// Sudo usually not needed for lsmod or grep.
	// Runner.Check returns (bool_isSuccessExitCode, error_execFailed)
	found, err := ctx.Host.Runner.Check(ctx.GoContext, cmd, false)
	if err != nil {
		// This error means the Check command itself failed to execute, not that grep didn't find the module.
		return false, fmt.Errorf("error executing command to check if module %s is loaded on host %s: %w", moduleName, ctx.Host.Name, err)
	}
	return found, nil // found is true if grep exited 0 (module was listed)
}

// Check determines if all modules are loaded.
func (e *LoadKernelModulesStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	rawSpec, rok := ctx.Step().GetCurrentStepSpec()
	if !rok {
		return false, fmt.Errorf("StepSpec not found in context for LoadKernelModulesStepExecutor Check method")
	}
	spec, ok := rawSpec.(*LoadKernelModulesStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected spec type %T for LoadKernelModulesStepExecutor Check method", rawSpec)
	}

	if len(spec.Modules) == 0 {
		if ctx.Logger != nil { // Check logger for nil to be safe
			ctx.Logger.Debugf("No modules specified for step '%s' on host %s, considering done.", spec.GetName(), ctx.Host.Name)
		}
		return true, nil
	}
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()

	for _, mod := range spec.Modules {
		loaded, checkErr := e.isModuleLoaded(ctx, mod)
		if checkErr != nil {
			hostCtxLogger.Errorf("Error checking module %s: %v", mod, checkErr)
			return false, checkErr
		}
		if !loaded {
			hostCtxLogger.Debugf("Check: Module %s is not loaded.", mod)
			return false, nil
		}
		hostCtxLogger.Debugf("Check: Module %s is already loaded.", mod)
	}
	hostCtxLogger.Infof("All specified modules (%s) are already loaded.", strings.Join(spec.Modules, ", "))
	return true, nil
}

// Execute loads kernel modules.
func (e *LoadKernelModulesStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.now()
	rawSpec, rok := ctx.Step().GetCurrentStepSpec()
	if !rok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for LoadKernelModulesStepExecutor Execute method"))
	}
	spec, ok := rawSpec.(*LoadKernelModulesStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected spec type %T for LoadKernelModulesStepExecutor Execute method", rawSpec))
	}

	res := step.NewResult(ctx, startTime, nil) // Use new NewResult signature
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()
	var errorsCollected []string
	var successesCollected []string

	if len(spec.Modules) == 0 {
		res.Status = step.StatusSucceeded; res.Message = "No kernel modules specified to load."
		hostCtxLogger.Infof(res.Message)
		res.EndTime = time.Now(); return res
	}
	if ctx.Host.Runner == nil {
		res.Error = fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
		res.Status = step.StatusFailed; res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}

	for _, mod := range spec.Modules {
		isLoadedBeforeRun, _ := e.isModuleLoaded(ctx, mod) // Check again before modprobe
		if isLoadedBeforeRun {
			hostCtxLogger.Infof("Module %s is already loaded, skipping modprobe.", mod)
			successesCollected = append(successesCollected, mod)
			continue
		}

		hostCtxLogger.Infof("Attempting to load kernel module: %s", mod)
		// Sudo is often required for modprobe.
		_, stderrBytes, err := ctx.Host.Runner.RunWithOptions(ctx.GoContext, fmt.Sprintf("modprobe %s", mod), &connector.ExecOptions{Sudo: true})
		if err != nil {
			errMsg := fmt.Sprintf("failed to load module %s: %v (stderr: %s)", mod, err, string(stderrBytes))
			hostCtxLogger.Warnf(errMsg); errorsCollected = append(errorsCollected, errMsg); continue
		}

		loadedAfterRun, verifyErr := e.isModuleLoaded(ctx, mod)
		if verifyErr != nil {
			errMsg := fmt.Sprintf("failed to verify module %s after attempting load: %v", mod, verifyErr)
			hostCtxLogger.Warnf(errMsg); errorsCollected = append(errorsCollected, errMsg)
		} else if !loadedAfterRun {
			errMsg := fmt.Sprintf("module %s reported as loaded by modprobe, but not found by lsmod verification", mod)
			hostCtxLogger.Warnf(errMsg); errorsCollected = append(errorsCollected, errMsg)
		} else {
			hostCtxLogger.Successf("Kernel module %s loaded successfully.", mod)
			successesCollected = append(successesCollected, mod)
		}
	}
	res.EndTime = time.Now()

	if len(errorsCollected) > 0 {
		res.Status = step.StatusFailed; res.Error = fmt.Errorf(strings.Join(errorsCollected, "; "))
		res.Message = fmt.Sprintf("Successfully loaded: [%s]. Failed or could not verify: %s", strings.Join(successesCollected, ", "), res.Error.Error())
		hostCtxLogger.Errorf("Step finished with errors: %s", res.Message)
	} else {
		res.Status = step.StatusSucceeded
		res.Message = fmt.Sprintf("All specified kernel modules (%s) were successfully loaded and verified.", strings.Join(spec.Modules, ", "))
		hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	}
	return res
}
var _ step.StepExecutor = &LoadKernelModulesStepExecutor{}
