package preflight

import (
	"context" // Not directly used by this file's funcs, but runtime.Context needs it
	"errors"  // For errors.As
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector" // For connector.CommandError
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/spec"
	"github.com/kubexms/kubexms/pkg/step"
)

// CheckCPUStepSpec defines the parameters for checking CPU core count.
type CheckCPUStepSpec struct {
	MinCores int
	StepName string // Optional: for a custom name for this specific step instance
}

// GetName returns a human-readable name for the step spec.
// If StepName is provided in the spec, it's used; otherwise, a default is generated.
func (s *CheckCPUStepSpec) GetName() string {
	if s.StepName != "" {
		return s.StepName
	}
	return fmt.Sprintf("Check CPU Cores (minimum %d)", s.MinCores)
}

// Ensure CheckCPUStepSpec implements spec.StepSpec
var _ spec.StepSpec = &CheckCPUStepSpec{}

// CheckCPUStepExecutor implements the logic for CheckCPUStepSpec.
type CheckCPUStepExecutor struct{}

func init() {
	// Register this executor for the CheckCPUStepSpec type.
	step.Register(step.GetSpecTypeName(&CheckCPUStepSpec{}), &CheckCPUStepExecutor{})
}

// runCheckLogic contains the core logic for checking CPU cores, shared by Check and Execute.
func (e *CheckCPUStepExecutor) runCheckLogic(ctx runtime.Context, spec *CheckCPUStepSpec) (currentCores int, met bool, err error) {
	if ctx.Host.Runner == nil {
		return 0, false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	hostCtxLogger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()


	// Prefer facts if available and seems valid (TotalCPU > 0)
	if ctx.Host.Runner.Facts != nil && ctx.Host.Runner.Facts.TotalCPU > 0 {
		currentCores = ctx.Host.Runner.Facts.TotalCPU
		hostCtxLogger.Debugf("Using CPU count from facts: %d", currentCores)
	} else {
		hostCtxLogger.Debugf("CPU facts not available or zero, attempting to get CPU count via command.")

		cmdToRun := "nproc"
		osID := "linux" // Default assumption
		if ctx.Host.Runner.Facts != nil && ctx.Host.Runner.Facts.OS != nil && ctx.Host.Runner.Facts.OS.ID != "" {
			osID = strings.ToLower(ctx.Host.Runner.Facts.OS.ID)
		}

		if osID == "darwin" { // macOS specific command if nproc is typically not there or facts were missing
			cmdToRun = "sysctl -n hw.ncpu"
		}

		stdoutBytes, stderrBytes, execErr := ctx.Host.Runner.RunWithOptions(ctx.GoContext, cmdToRun, &connector.ExecOptions{})

		if execErr != nil {
			return 0, false, fmt.Errorf("failed to execute command to get CPU count ('%s') on host %s: %w (stderr: %s)", cmdToRun, ctx.Host.Name, execErr, string(stderrBytes))
		}

		coresStr := strings.TrimSpace(string(stdoutBytes))
		parsedCores, parseErr := strconv.Atoi(coresStr)
		if parseErr != nil {
			return 0, false, fmt.Errorf("failed to parse CPU count from command output '%s' on host %s: %w", coresStr, ctx.Host.Name, parseErr)
		}
		currentCores = parsedCores
		hostCtxLogger.Debugf("Determined CPU count via command '%s': %d", cmdToRun, currentCores)
	}

	if currentCores >= spec.MinCores {
		return currentCores, true, nil
	}
	return currentCores, false, nil
}

// Check determines if the CPU core requirement is already met.
func (e *CheckCPUStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return false, fmt.Errorf("StepSpec not found in context for CheckCPUStep Check")
	}
	spec, ok := currentFullSpec.(*CheckCPUStepSpec)
	if !ok {
		return false, fmt.Errorf("unexpected StepSpec type for CheckCPUStep Check: %T", currentFullSpec)
	}

	_, met, err := e.runCheckLogic(ctx, spec)
	if err != nil {
		return false, err
	}
	return met, nil
}

// Execute performs the check for CPU cores and returns a result.
func (e *CheckCPUStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	currentFullSpec, ok := ctx.Step().GetCurrentStepSpec()
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for CheckCPUStep Execute"))
	}
	spec, ok := currentFullSpec.(*CheckCPUStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected StepSpec type for CheckCPUStep Execute: %T", currentFullSpec))
	}

	hostCtxLogger := ctx.Logger.SugaredLogger().With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()
	currentCores, met, checkErr := e.runCheckLogic(ctx, spec)

	// Create result after check logic to pass the error to NewResult
	res := step.NewResult(ctx, startTime, checkErr)
	// res.EndTime is already set by NewResult

	if checkErr != nil {
		// Error already set in res by NewResult
		res.Message = fmt.Sprintf("Error checking CPU cores: %v", checkErr) // Additional message if needed
		hostCtxLogger.Errorf("Step failed: %v", checkErr)
		return res
	}

	if met {
		// StatusSucceeded already set by NewResult if checkErr is nil
		res.Message = fmt.Sprintf("Host has %d CPU cores, which meets the minimum requirement of %d cores.", currentCores, spec.MinCores)
		hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	} else {
		res.Error = fmt.Errorf("host has %d CPU cores, but minimum requirement is %d cores", currentCores, spec.MinCores)
		res.Status = step.StatusFailed // Explicitly set Failed if checkErr was nil but condition not met
		res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %s", res.Message)
	}
	return res
}

// Ensure CheckCPUStepExecutor implements step.StepExecutor
var _ step.StepExecutor = &CheckCPUStepExecutor{}
