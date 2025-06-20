package preflight

import (
	// "context" // No longer directly used if runtime.StepContext is used
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector" // For connector.ExecOptions
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
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
func (e *CheckMemoryStepExecutor) runCheckLogic(s *CheckMemoryStepSpec, ctx runtime.StepContext) (currentMemoryMB uint64, met bool, err error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	if currentHost == nil {
		return 0, false, fmt.Errorf("current host not found in context for runCheckLogic")
	}
	logger = logger.With("host", currentHost.GetName(), "step", s.GetName())

	facts, errFacts := ctx.GetHostFacts(currentHost)
	if errFacts == nil && facts != nil && facts.TotalMemory > 0 { // TotalMemory is in MiB
		currentMemoryMB = facts.TotalMemory
		logger.Debug("Using memory size from facts.", "memoryMB", currentMemoryMB)
	} else {
		if errFacts != nil {
			logger.Debug("Failed to get host facts, will try command.", "error", errFacts)
		} else if facts == nil || facts.TotalMemory <= 0 {
			logger.Debug("Memory facts not available or zero, attempting to get memory via command.")
		}

		conn, errConn := ctx.GetConnectorForHost(currentHost)
		if errConn != nil {
			return 0, false, fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), errConn)
		}

		var cmd string
		var isKb bool

		osID := "linux"
		if facts != nil && facts.OS != nil && facts.OS.ID != "" {
			osID = strings.ToLower(facts.OS.ID)
		}

		if osID == "darwin" {
			cmd = "sysctl -n hw.memsize"
			isKb = false // hw.memsize returns bytes
		} else {
			cmd = "grep MemTotal /proc/meminfo | awk '{print $2}'"
			isKb = true // MemTotal from /proc/meminfo is in KB
		}

		stdoutBytes, stderrBytes, execErr := conn.RunCommand(goCtx, cmd, &connector.ExecOptions{Sudo: false}) // Use connector
		if execErr != nil {
			return 0, false, fmt.Errorf("failed to execute command on host %s to get memory info ('%s'): %w (stderr: %s)", currentHost.GetName(), cmd, execErr, string(stderrBytes))
		}

		memStr := strings.TrimSpace(string(stdoutBytes))
		memVal, parseErr := strconv.ParseUint(memStr, 10, 64)
		if parseErr != nil {
			return 0, false, fmt.Errorf("failed to parse memory from command output '%s' on host %s: %w", memStr, currentHost.GetName(), parseErr)
		}

		if isKb {
			currentMemoryMB = memVal / 1024
		} else {
			currentMemoryMB = memVal / (1024 * 1024)
		}
		logger.Debug("Determined memory size via command.", "command", cmd, "memoryMB", currentMemoryMB)
	}

	if currentMemoryMB >= s.MinMemoryMB {
		return currentMemoryMB, true, nil
	}
	return currentMemoryMB, false, nil
}

// Check determines if the memory requirement is met.
func (e *CheckMemoryStepExecutor) Check(ctx runtime.StepContext) (isDone bool, err error) { // Changed to StepContext
	logger := ctx.GetLogger()
	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context for CheckMemoryStepExecutor Check method")
		return false, fmt.Errorf("StepSpec not found in context for CheckMemoryStepExecutor Check method")
	}
	spec, ok := rawSpec.(*CheckMemoryStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		return false, fmt.Errorf("unexpected spec type %T for CheckMemoryStepExecutor Check method", rawSpec)
	}

	_, met, errLogic := e.runCheckLogic(spec, ctx)
	if errLogic != nil {
		return false, errLogic
	}
	return met, nil
}

// Execute performs the memory check and returns a result.
func (e *CheckMemoryStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.Now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()

	res := step.NewResult(ctx, currentHost, startTime, nil)

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context for CheckMemoryStepExecutor Execute method")
		res.Error = fmt.Errorf("StepSpec not found in context for CheckMemoryStepExecutor Execute method")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*CheckMemoryStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected spec type %T for CheckMemoryStepExecutor Execute method", rawSpec)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	// Logger will be contextualized further by runCheckLogic if currentHost is not nil
	currentMemoryMB, met, checkErr := e.runCheckLogic(spec, ctx)
	res.EndTime = time.Now()

	// Re-acquire logger which would be contextualized by runCheckLogic
	logger = ctx.GetLogger()
	if currentHost != nil { logger = logger.With("host", currentHost.GetName()) }
	logger = logger.With("step", spec.GetName())


	if checkErr != nil {
		res.Error = checkErr; res.Status = step.StatusFailed
		res.Message = fmt.Sprintf("Error checking memory: %v", checkErr)
		logger.Error("Step failed.", "error", checkErr)
		return res
	}

	if met {
		res.Status = step.StatusSucceeded
		res.Message = fmt.Sprintf("Host has %d MB memory, which meets the minimum requirement of %d MB.", currentMemoryMB, spec.MinMemoryMB)
		logger.Info("Step succeeded.", "message", res.Message)
	} else {
		res.Status = step.StatusFailed
		res.Error = fmt.Errorf("host has %d MB memory, but minimum requirement is %d MB", currentMemoryMB, spec.MinMemoryMB)
		res.Message = res.Error.Error()
		logger.Error("Step failed.", "message", res.Message)
	}
	return res
}
var _ step.StepExecutor = &CheckMemoryStepExecutor{}
