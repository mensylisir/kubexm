package preflight

import (
	// "context" // No longer directly used
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// DisableSwapStepSpec defines parameters for disabling swap. (No parameters needed for this version)
type DisableSwapStepSpec struct {
	StepName string // Optional: for a custom name for this specific step instance
}

// GetName returns the step name.
func (s *DisableSwapStepSpec) GetName() string {
	if s.StepName != "" { return s.StepName }
	return "Disable Swap"
}
var _ spec.StepSpec = &DisableSwapStepSpec{}

// DisableSwapStepExecutor implements the logic for DisableSwapStepSpec.
type DisableSwapStepExecutor struct{}

func init() {
	step.Register(step.GetSpecTypeName(&DisableSwapStepSpec{}), &DisableSwapStepExecutor{})
}

// isSwapOn is a helper function moved into the executor context or kept package-level if only used here.
// For executor pattern, it's better as a private method of the executor if it needs access to `e` or takes `s`.
// If it only needs `ctx`, it can be a package-level helper or private method.
func (e *DisableSwapStepExecutor) isSwapOn(ctx runtime.StepContext) (bool, string, error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	if currentHost == nil {
		return false, "", fmt.Errorf("current host not found in context for isSwapOn check")
	}
	logger = logger.With("host", currentHost.GetName(), "operation", "isSwapOnCheck")

	conn, errConn := ctx.GetConnectorForHost(currentHost)
	if errConn != nil {
		return false, "", fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), errConn)
	}

	stdoutBytes, stderrBytes, err := conn.RunCommand(goCtx, "swapon --summary --noheadings", &connector.ExecOptions{Sudo: false})
	if err != nil {
		var cmdErr *connector.CommandError
		if errors.As(err, &cmdErr) && (strings.Contains(strings.ToLower(string(stderrBytes)), "invalid option") || strings.Contains(strings.ToLower(string(stderrBytes)), "bad usage")) {
			logger.Debug("`swapon --summary --noheadings` failed, trying `swapon --summary`.")
			stdoutBytes, stderrBytes, err = conn.RunCommand(goCtx, "swapon --summary", &connector.ExecOptions{Sudo: false})
		}
	}

	if err != nil {
		var cmdErr *connector.CommandError
		isCmdNotFoundErr := errors.As(err, &cmdErr) && cmdErr.ExitCode == 127

		facts, _ := ctx.GetHostFacts(currentHost) // Best effort to get facts for OS ID
		osID := "unknown"
		if facts != nil && facts.OS != nil && facts.OS.ID != "" {
			osID = strings.ToLower(facts.OS.ID)
		}

		if osID == "linux" {
			logger.Warn("`swapon --summary` command failed, attempting to read /proc/swaps.", "error", err, "stderr", string(stderrBytes))
			procSwapsContentBytes, readErr := conn.ReadFile(goCtx, "/proc/swaps") // Use connector
			if readErr != nil {
				return false, "", fmt.Errorf("failed to run 'swapon --summary' and also failed to read /proc/swaps on host %s: %w", currentHost.GetName(), readErr)
			}
			lines := strings.Split(strings.TrimSpace(string(procSwapsContentBytes)), "\n")
			return len(lines) > 1, string(procSwapsContentBytes), nil
		}

		if isCmdNotFoundErr {
			return false, "", fmt.Errorf("`swapon` command not found and OS ('%s') is not Linux with /proc/swaps fallback, cannot determine swap status on host %s", osID, currentHost.GetName())
		}
		return false, "", fmt.Errorf("failed to execute 'swapon --summary' on host %s: %w (stderr: %s)", currentHost.GetName(), err, string(stderrBytes))
	}

	trimmedStdout := strings.TrimSpace(string(stdoutBytes))
	if trimmedStdout == "" {
		return false, string(stdoutBytes), nil
	}
	lines := strings.Split(trimmedStdout, "\n")
	if len(lines) == 1 && strings.Contains(lines[0], "Filename") && strings.Contains(lines[0], "Type") {
		return false, string(stdoutBytes), nil
	}
	return true, string(stdoutBytes), nil
}

// Check determines if swap is already disabled.
func (e *DisableSwapStepExecutor) Check(ctx runtime.StepContext) (isDone bool, err error) { // Changed to StepContext
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost() // Needed for logging and potentially by isSwapOn

	if currentHost == nil {
		// This preflight check is inherently host-specific.
		logger.Error("Current host not found in context for Check")
		return false, fmt.Errorf("current host is required for DisableSwap Check")
	}

	// Get step name for logging even if isSwapOn has its own.
	stepName := "DisableSwap Check"
	if rawSpec, specOk := ctx.StepCache().GetCurrentStepSpec(); specOk {
		if s, castOk := rawSpec.(*DisableSwapStepSpec); castOk {
			stepName = s.GetName()
		}
	}
	logger = logger.With("host", currentHost.GetName(), "step", stepName)

	swapOn, swapOutput, checkErr := e.isSwapOn(ctx)
	if checkErr != nil {
		logger.Error("Error checking swap status.", "output", swapOutput, "error", checkErr)
		return false, fmt.Errorf("error checking swap status on host %s: %w. Output: %s", currentHost.GetName(), checkErr, swapOutput)
	}
	return !swapOn, nil
}

// Execute disables swap.
func (e *DisableSwapStepExecutor) Execute(ctx runtime.StepContext) *step.Result { // Changed to StepContext
	startTime := time.Now()
	logger := ctx.GetLogger()
	currentHost := ctx.GetHost()
	goCtx := ctx.GoContext()

	res := step.NewResult(ctx, currentHost, startTime, nil)

	if currentHost == nil {
		logger.Error("Current host not found in context for Execute")
		res.Error = fmt.Errorf("current host is required for DisableSwap Execute")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("host", currentHost.GetName())

	rawSpec, ok := ctx.StepCache().GetCurrentStepSpec()
	if !ok {
		logger.Error("StepSpec not found in context")
		res.Error = fmt.Errorf("StepSpec not found in context for DisableSwapStepExecutor Execute method")
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	spec, ok := rawSpec.(*DisableSwapStepSpec)
	if !ok {
		logger.Error("Unexpected StepSpec type", "type", fmt.Sprintf("%T", rawSpec))
		res.Error = fmt.Errorf("unexpected spec type %T for DisableSwapStepExecutor Execute method", rawSpec)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	logger = logger.With("step", spec.GetName())

	conn, errConn := ctx.GetConnectorForHost(currentHost)
	if errConn != nil {
		logger.Error("Failed to get connector for host", "error", errConn)
		res.Error = fmt.Errorf("failed to get connector for host %s: %w", currentHost.GetName(), errConn)
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}

	logger.Info("Attempting to turn off active swap with 'swapoff -a'...")
	_, stderrSwapoff, errSwapoff := conn.RunCommand(goCtx, "swapoff -a", &connector.ExecOptions{Sudo: true})
	if errSwapoff != nil {
		logger.Warn("Command 'swapoff -a' finished with error (this might be okay if no swap was active).", "stderr", string(stderrSwapoff), "error", errSwapoff)
	} else {
		logger.Info("'swapoff -a' completed.")
	}
	res.Stdout += fmt.Sprintf("swapoff -a stderr: %s\n", string(stderrSwapoff))

	fstabPath := "/etc/fstab"
	logger.Info("Attempting to comment out swap entries.", "fstab", fstabPath)

	backupCmd := fmt.Sprintf("cp %s %s.bak-kubexms-%d", fstabPath, fstabPath, time.Now().UnixNano())
	_, stderrBackup, errBackup := conn.RunCommand(goCtx, backupCmd, &connector.ExecOptions{Sudo: true})
	if errBackup != nil {
		logger.Error("Failed to backup fstab.", "fstab", fstabPath, "stderr", string(stderrBackup), "error", errBackup)
		res.Error = fmt.Errorf("failed to backup %s: %w (stderr: %s)", fstabPath, errBackup, string(stderrBackup))
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	res.Stdout += fmt.Sprintf("Backup %s: OK\n", fstabPath)

	sedCmd := fmt.Sprintf("sed -E -i.kubexms_fstab_prev '/^[^#].*\\bswap\\b/s/^/#/' %s", fstabPath)
	_, stderrSed, errSed := conn.RunCommand(goCtx, sedCmd, &connector.ExecOptions{Sudo: true})
	if errSed != nil {
		logger.Error("Failed to comment out swap entries in fstab using sed.", "fstab", fstabPath, "stderr", string(stderrSed), "error", errSed)
		res.Error = fmt.Errorf("failed to comment out swap entries in %s using sed: %w (stderr: %s)", fstabPath, errSed, string(stderrSed))
		res.Status = step.StatusFailed; res.EndTime = time.Now(); return res
	}
	res.Stdout += fmt.Sprintf("Commented swap in %s: OK. Stderr: %s\n", fstabPath, string(stderrSed))
	logger.Info("Swap entries in fstab commented out.", "fstab", fstabPath)

	swapOn, finalState, verifyErr := e.isSwapOn(ctx)
	res.EndTime = time.Now()
	if verifyErr != nil {
		logger.Error("Failed to verify swap status after attempting disable.", "lastOutput", finalState, "error", verifyErr)
		res.Error = fmt.Errorf("failed to verify swap status after attempting disable: %w. Last state: %s", verifyErr, finalState)
		res.Status = step.StatusFailed; return res
	}
	if !swapOn {
		res.Status = step.StatusSucceeded; res.Message = "Swap is successfully disabled."
		logger.Info("Step succeeded.", "message", res.Message)
	} else {
		logger.Error("Failed to disable swap.", "currentSwapOutput", finalState)
		res.Status = step.StatusFailed; res.Error = fmt.Errorf("failed to disable swap. Current swap status output: %s", finalState)
		res.Message = res.Error.Error();
	}
	return res
}
var _ step.StepExecutor = &DisableSwapStepExecutor{}
