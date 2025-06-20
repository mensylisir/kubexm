package preflight

import (
	// "context" // No longer directly used if runtime.StepContext is used
	"errors"  // For errors.As
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector" // For connector.CommandError
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
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
func (e *CheckCPUStepExecutor) runCheckLogic(s *CheckCPUStepSpec, ctx runtime.StepContext) (currentCores int, met bool, err error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	if currentHost == nil {
		return 0, false, fmt.Errorf("current host not found in context for runCheckLogic")
	}
	logger = logger.With("host", currentHost.GetName(), "step", s.GetName())

	facts, errFacts := ctx.GetHostFacts(currentHost)
	if errFacts == nil && facts != nil && facts.TotalCPU > 0 {
		currentCores = facts.TotalCPU
		logger.Debug("Using CPU count from facts.", "cores", currentCores)
	} else {
		if errFacts != nil {
			logger.Debug("Failed to get host facts, will try command.", "error", errFacts)
		} else if facts == nil || facts.TotalCPU <=0 {
			logger.Debug("CPU facts not available or zero, attempting to get CPU count via command.")
		}

		conn, errConn := ctx.GetConnectorForHost(currentHost)
		if errConn != nil {
			return 0, false, fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), errConn)
		}

		cmdToRun := "nproc"
		osID := "linux" // Default assumption
		if facts != nil && facts.OS != nil && facts.OS.ID != "" {
			osID = strings.ToLower(facts.OS.ID)
		}

		if osID == "darwin" {
			cmdToRun = "sysctl -n hw.ncpu"
		}

		stdoutBytes, stderrBytes, execErr := conn.RunCommand(goCtx, cmdToRun, &connector.ExecOptions{}) // Use connector
		if execErr != nil {
			return 0, false, fmt.Errorf("failed to execute command to get CPU count ('%s') on host %s: %w (stderr: %s)", cmdToRun, currentHost.GetName(), execErr, string(stderrBytes))
		}

		coresStr := strings.TrimSpace(string(stdoutBytes))
		parsedCores, parseErr := strconv.Atoi(coresStr)
		if parseErr != nil {
			return 0, false, fmt.Errorf("failed to parse CPU count from command output '%s' on host %s: %w", coresStr, currentHost.GetName(), parseErr)
		}
		currentCores = parsedCores
		logger.Debug("Determined CPU count via command.", "command", cmdToRun, "cores", currentCores)
	}

	if currentCores >= s.MinCores {
		return currentCores, true, nil
	}
	return currentCores, false, nil
}

// Check determines if the CPU core requirement is already met.
func (e *CheckCPUStepExecutor) Check(ctx runtime.StepContext) (isDone bool, err error) { // Changed to StepContext
	logger := ctx.GetLogger() // Initial logger
	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context for CheckCPUStepExecutor Check method")
		return false, fmt.Errorf("StepSpec not found in context for CheckCPUStepExecutor Check method")
	}
	spec, ok := rawSpec.(*CheckCPUStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		return false, fmt.Errorf("unexpected spec type %T for CheckCPUStepExecutor Check method", rawSpec)
	}

	_, met, errLogic := e.runCheckLogic(spec, ctx)
	if errLogic != nil {
		return false, errLogic
	}
	return met, nil
}

// Execute performs the check for CPU cores and returns a result.
func (e *CheckCPUStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.Now()
	logger := ctx.GetLogger() // Initial logger
	currentHost := ctx.GetHost()

	res := step.NewResult(ctx, currentHost, startTime, nil)

	// Logger will be further contextualized in runCheckLogic if currentHost is not nil

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context for CheckCPUStepExecutor Execute method")
		res.Error = fmt.Errorf("StepSpec not found in context for CheckCPUStepExecutor Execute method")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*CheckCPUStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected spec type %T for CheckCPUStepExecutor Execute method", rawSpec)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	// If currentHost is nil, runCheckLogic will error out, which is handled below.
	// The logger passed to runCheckLogic (via ctx) will be the base one if currentHost is nil.
	currentCores, met, checkErr := e.runCheckLogic(spec, ctx)
	res.EndTime = time.Now()

	// Re-acquire logger that might have been contextualized by runCheckLogic (if host was available)
	// or use the initial one.
	logger = ctx.GetLogger()
	if currentHost != nil { logger = logger.With("host", currentHost.GetName()) }
	logger = logger.With("step", spec.GetName())


	if checkErr != nil {
		res.Error = checkErr
		res.Status = step.StatusFailed
		res.Message = fmt.Sprintf("Error checking CPU cores: %v", checkErr)
		logger.Error("Step failed.", "error", checkErr)
		return res
	}

	if met {
		res.Status = step.StatusSucceeded
		res.Message = fmt.Sprintf("Host has %d CPU cores, which meets the minimum requirement of %d cores.", currentCores, spec.MinCores)
		logger.Info("Step succeeded.", "message", res.Message)
	} else {
		res.Status = step.StatusFailed
		res.Error = fmt.Errorf("host has %d CPU cores, but minimum requirement is %d cores", currentCores, spec.MinCores)
		res.Message = res.Error.Error()
		logger.Error("Step failed.", "message", res.Message)
	}
	return res
}

// Ensure CheckCPUStepExecutor implements step.StepExecutor
var _ step.StepExecutor = &CheckCPUStepExecutor{}
