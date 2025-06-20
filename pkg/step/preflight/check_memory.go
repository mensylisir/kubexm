package preflight

import (
	"context" // Required by runtime.Context
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector" // For connector.ExecOptions
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step"
)

// CheckMemoryStepSpec defines parameters for checking system memory.
type CheckMemoryStepSpec struct {
	MinMemoryMB uint64
	StepName    string // Optional: for a custom name for this specific step instance
}

// GetName returns the step name.
func (s *CheckMemoryStepSpec) GetName() string {
	if s.StepName != "" {
		return s.StepName
	}
	return fmt.Sprintf("Check Memory (minimum %d MB)", s.MinMemoryMB)
}
var _ spec.StepSpec = &CheckMemoryStepSpec{}

// CheckMemoryStepExecutor implements the logic for CheckMemoryStepSpec.
type CheckMemoryStepExecutor struct{}

func init() {
	step.Register(step.GetSpecTypeName(&CheckMemoryStepSpec{}), &CheckMemoryStepExecutor{})
}

// runCheckLogic contains the core logic for checking memory, shared by Check and Execute.
func (e *CheckMemoryStepExecutor) runCheckLogic(s *CheckMemoryStepSpec, ctx *runtime.Context) (currentMemoryMB uint64, met bool, err error) {
	if ctx.Host.Runner == nil {
		return 0, false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", s.GetName()).Sugar()

	// Prefer facts if available and seems valid (TotalMemory > 0)
	if ctx.Host.Runner.Facts != nil && ctx.Host.Runner.Facts.TotalMemory > 0 {
		currentMemoryMB = ctx.Host.Runner.Facts.TotalMemory // Facts.TotalMemory is already in MiB
		hostCtxLogger.Debugf("Using memory size from facts: %d MB", currentMemoryMB)
	} else {
		hostCtxLogger.Debugf("Memory facts not available or zero, attempting to get memory via command.")

		var cmd string
		var isKb bool // True if command output is in KB, false if in bytes (for Bytes output by sysctl on darwin)

		osID := "linux" // Default assumption
		if ctx.Host.Runner.Facts != nil && ctx.Host.Runner.Facts.OS != nil && ctx.Host.Runner.Facts.OS.ID != "" {
			osID = strings.ToLower(ctx.Host.Runner.Facts.OS.ID)
		}

		if osID == "darwin" {
			cmd = "sysctl -n hw.memsize" // This returns bytes on macOS
			isKb = false
		} else { // Assume Linux-like /proc/meminfo, which reports in KB for MemTotal
			cmd = "grep MemTotal /proc/meminfo | awk '{print $2}'"
			isKb = true
		}

		// Sudo false, no specific options needed for these read-only commands.
		stdoutBytes, stderrBytes, execErr := ctx.Host.Runner.RunWithOptions(ctx.GoContext, cmd, &connector.ExecOptions{Sudo: false})
		if execErr != nil {
			return 0, false, fmt.Errorf("failed to execute command on host %s to get memory info ('%s'): %w (stderr: %s)", ctx.Host.Name, cmd, execErr, string(stderrBytes))
		}

		memStr := strings.TrimSpace(string(stdoutBytes))
		memVal, parseErr := strconv.ParseUint(memStr, 10, 64)
		if parseErr != nil {
			return 0, false, fmt.Errorf("failed to parse memory from command output '%s' on host %s: %w", memStr, ctx.Host.Name, parseErr)
		}

		if isKb {
			currentMemoryMB = memVal / 1024 // Convert KB to MB
		} else { // Value is in Bytes (e.g. from macOS sysctl hw.memsize)
			currentMemoryMB = memVal / (1024 * 1024) // Convert Bytes to MB
		}
		hostCtxLogger.Debugf("Determined memory size via command '%s': %d MB", cmd, currentMemoryMB)
	}

	if currentMemoryMB >= s.MinMemoryMB {
		return currentMemoryMB, true, nil
	}
	return currentMemoryMB, false, nil
}

// Check determines if the memory requirement is met.
func (e *CheckMemoryStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	rawSpec, rok := ctx.Step().GetCurrentStepSpec()
	if !rok {
		return false, fmt.Errorf("StepSpec not found in context for CheckMemoryStepExecutor Check method")
	}
	spec, ok := rawSpec.(*CheckMemoryStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected spec type %T for CheckMemoryStepExecutor Check method", rawSpec)
	}

	_, met, errLogic := e.runCheckLogic(spec, ctx)
	// If runCheckLogic itself had an error (e.g. command execution failed), propagate that.
	if errLogic != nil {
		return false, errLogic
	}
	return met, nil
}

// Execute performs the memory check and returns a result.
func (e *CheckMemoryStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	rawSpec, rok := ctx.Step().GetCurrentStepSpec()
	if !rok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for CheckMemoryStepExecutor Execute method"))
	}
	spec, ok := rawSpec.(*CheckMemoryStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected spec type %T for CheckMemoryStepExecutor Execute method", rawSpec))
	}

	res := step.NewResult(ctx, startTime, nil) // Use new NewResult signature
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()

	currentMemoryMB, met, checkErr := e.runCheckLogic(spec, ctx)
	res.EndTime = time.Now()

	if checkErr != nil {
		res.Error = checkErr; res.Status = step.StatusFailed
		res.Message = fmt.Sprintf("Error checking memory: %v", checkErr)
		hostCtxLogger.Errorf("Step failed: %v", checkErr)
		return res
	}

	if met {
		res.Status = step.StatusSucceeded
		res.Message = fmt.Sprintf("Host has %d MB memory, which meets the minimum requirement of %d MB.", currentMemoryMB, spec.MinMemoryMB)
		hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	} else {
		res.Status = step.StatusFailed
		res.Error = fmt.Errorf("host has %d MB memory, but minimum requirement is %d MB", currentMemoryMB, spec.MinMemoryMB)
		res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %s", res.Message)
	}
	return res
}
var _ step.StepExecutor = &CheckMemoryStepExecutor{}
