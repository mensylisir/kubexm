package preflight

import (
	// "context" // No longer directly used if runtime.StepContext is used
	"errors" // For errors.As
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector" // For connector.ExecOptions and connector.Connector
	"github.com/mensylisir/kubexm/pkg/runtime"   // For runtime.StepContext
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
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
func (e *LoadKernelModulesStepExecutor) isModuleLoaded(ctx runtime.StepContext, moduleName string) (bool, error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	if currentHost == nil {
		return false, fmt.Errorf("current host not found in context for isModuleLoaded check")
	}
	// logger is already contextualized by the caller (Check/Execute methods)
	// logger = logger.With("host", currentHost.GetName())

	conn, errConn := ctx.GetConnectorForHost(currentHost)
	if errConn != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), errConn)
	}

	cmd := fmt.Sprintf("lsmod | awk '{print $1}' | grep -xq %s", moduleName)
	// Sudo usually not needed for lsmod or grep.
	_, _, err := conn.RunCommand(goCtx, cmd, &connector.ExecOptions{Sudo: false}) // Use connector

	if err == nil {
		return true, nil // Exit code 0 from grep -q means found
	}
	var cmdErr *connector.CommandError
	if errors.As(err, &cmdErr) && cmdErr.ExitCode == 1 {
		return false, nil // Exit code 1 from grep -q means not found
	}
	// Any other error means the command execution itself failed.
	logger.Error("Error executing command to check if module is loaded.", "module", moduleName, "error", err)
	return false, fmt.Errorf("error executing command to check if module %s is loaded on host %s: %w", moduleName, currentHost.GetName(), err)
}

// Check determines if all modules are loaded.
func (e *LoadKernelModulesStepExecutor) Check(ctx runtime.StepContext) (isDone bool, err error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()

	if currentHost == nil {
		logger.Error("Current host not found in context for Check")
		return false, fmt.Errorf("current host is required for LoadKernelModulesStep Check")
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		return false, fmt.Errorf("StepSpec not found in context for LoadKernelModulesStepExecutor Check method")
	}
	spec, ok := rawSpec.(*LoadKernelModulesStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		return false, fmt.Errorf("unexpected spec type %T for LoadKernelModulesStepExecutor Check method", rawSpec)
	}
	logger = logger.With("step", spec.GetName())

	if len(spec.Modules) == 0 {
		logger.Debug("No modules specified, considering done.")
		return true, nil
	}

	for _, mod := range spec.Modules {
		loaded, checkErr := e.isModuleLoaded(ctx, mod) // Pass StepContext
		if checkErr != nil {
			logger.Error("Error checking module.", "module", mod, "error", checkErr)
			return false, checkErr
		}
		if !loaded {
			logger.Debug("Module is not loaded.", "module", mod)
			return false, nil
		}
		logger.Debug("Module is already loaded.", "module", mod)
	}
	logger.Info("All specified modules are already loaded.", "modules", strings.Join(spec.Modules, ", "))
	return true, nil
}

// Execute loads kernel modules.
func (e *LoadKernelModulesStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	res := step.NewResult(ctx, currentHost, startTime, nil)

	if currentHost == nil {
		logger.Error("Current host not found in context for Execute")
		res.Error = fmt.Errorf("current host is required for LoadKernelModulesStep Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		res.Error = fmt.Errorf("StepSpec not found in context for LoadKernelModulesStepExecutor Execute method")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*LoadKernelModulesStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected spec type %T for LoadKernelModulesStepExecutor Execute method", rawSpec)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("step", spec.GetName())

	var errorsCollected []string
	var successesCollected []string

	if len(spec.Modules) == 0 {
		res.Status = step.StatusSucceeded; res.Message = "No kernel modules specified to load."
		logger.Info(res.Message)
		res.EndTime = time.Now(); return res
	}

	conn, errConn := ctx.GetConnectorForHost(currentHost)
	if errConn != nil {
		logger.Error("Failed to get connector for host", "error", errConn)
		res.Error = fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), errConn)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	for _, mod := range spec.Modules {
		isLoadedBeforeRun, _ := e.isModuleLoaded(ctx, mod)
		if isLoadedBeforeRun {
			logger.Info("Module is already loaded, skipping modprobe.", "module", mod)
			successesCollected = append(successesCollected, mod)
			continue
		}

		logger.Info("Attempting to load kernel module.", "module", mod)
		_, stderrBytes, err := conn.RunCommand(goCtx, fmt.Sprintf("modprobe %s", mod), &connector.ExecOptions{Sudo: true}) // Use connector
		if err != nil {
			errMsg := fmt.Sprintf("failed to load module %s: %v (stderr: %s)", mod, err, string(stderrBytes))
			logger.Warn(errMsg); errorsCollected = append(errorsCollected, errMsg); continue
		}

		loadedAfterRun, verifyErr := e.isModuleLoaded(ctx, mod)
		if verifyErr != nil {
			errMsg := fmt.Sprintf("failed to verify module %s after attempting load: %v", mod, verifyErr)
			logger.Warn(errMsg); errorsCollected = append(errorsCollected, errMsg)
		} else if !loadedAfterRun {
			errMsg := fmt.Sprintf("module %s reported as loaded by modprobe, but not found by lsmod verification", mod)
			logger.Warn(errMsg); errorsCollected = append(errorsCollected, errMsg)
		} else {
			logger.Info("Kernel module loaded successfully.", "module", mod)
			successesCollected = append(successesCollected, mod)
		}
	}
	res.EndTime = time.Now()

	if len(errorsCollected) > 0 {
		res.Status = step.StatusFailed; res.Error = fmt.Errorf(strings.Join(errorsCollected, "; "))
		res.Message = fmt.Sprintf("Successfully loaded: [%s]. Failed or could not verify: %s", strings.Join(successesCollected, ", "), res.Error.Error())
		logger.Error("Step finished with errors.", "message", res.Message)
	} else {
		res.Status = step.StatusSucceeded
		res.Message = fmt.Sprintf("All specified kernel modules (%s) were successfully loaded and verified.", strings.Join(spec.Modules, ", "))
		logger.Info("Step succeeded.", "message", res.Message)
	}
	return res
}
var _ step.StepExecutor = &LoadKernelModulesStepExecutor{}
