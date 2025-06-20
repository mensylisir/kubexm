package preflight

import (
	"context" // Required by runtime.Context
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
func (e *DisableSwapStepExecutor) isSwapOn(ctx *runtime.Context) (bool, string, error) {
	if ctx.Host.Runner == nil {
		return false, "", fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "operation", "isSwapOnCheck").Sugar()

	// Try `swapon --summary --noheadings` first.
	stdoutBytes, stderrBytes, err := ctx.Host.Runner.RunWithOptions(ctx.GoContext, "swapon --summary --noheadings", &connector.ExecOptions{Sudo: false})
	if err != nil {
		var cmdErr *connector.CommandError
		// Check if it's an error due to --noheadings being unsupported
		if errors.As(err, &cmdErr) && (strings.Contains(strings.ToLower(string(stderrBytes)), "invalid option") || strings.Contains(strings.ToLower(string(stderrBytes)), "bad usage")) {
			hostCtxLogger.Debugf("`swapon --summary --noheadings` failed, trying `swapon --summary`.")
			stdoutBytes, stderrBytes, err = ctx.Host.Runner.RunWithOptions(ctx.GoContext, "swapon --summary", &connector.ExecOptions{Sudo: false})
		}
	}

	if err != nil {
		var cmdErr *connector.CommandError
		isCmdNotFoundErr := errors.As(err, &cmdErr) && cmdErr.ExitCode == 127

		osID := "unknown"
		if ctx.Host.Runner.Facts != nil && ctx.Host.Runner.Facts.OS != nil && ctx.Host.Runner.Facts.OS.ID != "" {
			osID = strings.ToLower(ctx.Host.Runner.Facts.OS.ID)
		}

		if osID == "linux" { // Only attempt /proc/swaps on Linux
			hostCtxLogger.Warnf("`swapon --summary` command failed (Error: %v, Stderr: %s), attempting to read /proc/swaps.", err, string(stderrBytes))
			// Runner's ReadFile uses Exec("cat ...")
			procSwapsContentBytes, readErr := ctx.Host.Runner.ReadFile(ctx.GoContext, "/proc/swaps")
			if readErr != nil {
				return false, "", fmt.Errorf("failed to run 'swapon --summary' and also failed to read /proc/swaps on host %s: %w", ctx.Host.Name, readErr)
			}
			lines := strings.Split(strings.TrimSpace(string(procSwapsContentBytes)), "\n")
			// Header line + data lines means swap is on
			return len(lines) > 1, string(procSwapsContentBytes), nil
		}

		if isCmdNotFoundErr {
			return false, "", fmt.Errorf("`swapon` command not found and OS ('%s') is not Linux with /proc/swaps fallback, cannot determine swap status on host %s", osID, ctx.Host.Name)
		}
		return false, "", fmt.Errorf("failed to execute 'swapon --summary' on host %s: %w (stderr: %s)", ctx.Host.Name, err, string(stderrBytes))
	}

	trimmedStdout := strings.TrimSpace(string(stdoutBytes))
	if trimmedStdout == "" { // No output from `swapon --summary --noheadings` means no swap
		return false, string(stdoutBytes), nil
	}
	lines := strings.Split(trimmedStdout, "\n")
	// If `swapon --summary` (no --noheadings) was used, check if it's just the header.
	if len(lines) == 1 && strings.Contains(lines[0], "Filename") && strings.Contains(lines[0], "Type") {
		return false, string(stdoutBytes), nil
	}
	// Otherwise, any output (from --noheadings) or multiple lines (from default summary) indicates swap is on.
	return true, string(stdoutBytes), nil
}

// Check determines if swap is already disabled.
func (e *DisableSwapStepExecutor) Check(ctx runtime.Context) (isDone bool, err error) {
	// Spec not strictly needed for logic but can be retrieved for consistent logging if desired.
	// For now, isSwapOn's logger is sufficient.
	swapOn, swapOutput, checkErr := e.isSwapOn(ctx)
	if checkErr != nil {
		// Log using the step's configured name if available, or a default.
		stepName := "DisableSwap Check"
		if rawSpec, specOk := ctx.Step().GetCurrentStepSpec(); specOk {
			if s, castOk := rawSpec.(*DisableSwapStepSpec); castOk {
				stepName = s.GetName()
			}
		}
		ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", stepName).Errorf("Error checking swap status: %v. Output: %s", checkErr, swapOutput)
		return false, fmt.Errorf("error checking swap status on host %s: %w. Output: %s", ctx.Host.Name, checkErr, swapOutput)
	}
	return !swapOn, nil // isDone is true if swap is OFF
}

// Execute disables swap.
func (e *DisableSwapStepExecutor) Execute(ctx runtime.Context) *step.Result {
	startTime := time.Now()
	rawSpec, rok := ctx.Step().GetCurrentStepSpec()
	if !rok {
		return step.NewResult(ctx, startTime, fmt.Errorf("StepSpec not found in context for DisableSwapStepExecutor Execute method"))
	}
	spec, ok := rawSpec.(*DisableSwapStepSpec)
	if !ok {
		return step.NewResult(ctx, startTime, fmt.Errorf("unexpected spec type %T for DisableSwapStepExecutor Execute method: %T", rawSpec, rawSpec))
	}

	res := step.NewResult(ctx, startTime, nil) // Use new NewResult signature
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step_spec", spec.GetName()).Sugar()

	hostCtxLogger.Infof("Attempting to turn off active swap with 'swapoff -a'...")
	_, stderrSwapoff, errSwapoff := ctx.Host.Runner.RunWithOptions(ctx.GoContext, "swapoff -a", &connector.ExecOptions{Sudo: true})
	if errSwapoff != nil {
		hostCtxLogger.Warnf("Command 'swapoff -a' finished with error (this might be okay if no swap was active): %v. Stderr: %s", errSwapoff, string(stderrSwapoff))
	} else {
		hostCtxLogger.Successf("'swapoff -a' completed.")
	}
	res.Stdout += fmt.Sprintf("swapoff -a stderr: %s\n", string(stderrSwapoff))

	fstabPath := "/etc/fstab"
	hostCtxLogger.Infof("Attempting to comment out swap entries in %s...", fstabPath)

	backupCmd := fmt.Sprintf("cp %s %s.bak-kubexms-%d", fstabPath, fstabPath, time.Now().UnixNano())
	_, stderrBackup, errBackup := ctx.Host.Runner.RunWithOptions(ctx.GoContext, backupCmd, &connector.ExecOptions{Sudo: true})
	if errBackup != nil {
		res.Error = fmt.Errorf("failed to backup %s: %w (stderr: %s)", fstabPath, errBackup, string(stderrBackup))
		res.Status = step.StatusFailed; hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	res.Stdout += fmt.Sprintf("Backup %s: OK\n", fstabPath)

	sedCmd := fmt.Sprintf("sed -E -i.kubexms_fstab_prev '/^[^#].*\\bswap\\b/s/^/#/' %s", fstabPath)
	_, stderrSed, errSed := ctx.Host.Runner.RunWithOptions(ctx.GoContext, sedCmd, &connector.ExecOptions{Sudo: true})
	if errSed != nil {
		res.Error = fmt.Errorf("failed to comment out swap entries in %s using sed: %w (stderr: %s)", fstabPath, errSed, string(stderrSed))
		res.Status = step.StatusFailed; hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	res.Stdout += fmt.Sprintf("Commented swap in %s: OK. Stderr: %s\n", fstabPath, string(stderrSed))
	hostCtxLogger.Successf("Swap entries in %s commented out.", fstabPath)

	swapOn, finalState, verifyErr := e.isSwapOn(ctx)
	res.EndTime = time.Now()
	if verifyErr != nil {
		res.Error = fmt.Errorf("failed to verify swap status after attempting disable: %w. Last state: %s", verifyErr, finalState)
		res.Status = step.StatusFailed; hostCtxLogger.Errorf("Step failed: %v", res.Error); return res
	}
	if !swapOn {
		res.Status = step.StatusSucceeded; res.Message = "Swap is successfully disabled."
		hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	} else {
		res.Status = step.StatusFailed; res.Error = fmt.Errorf("failed to disable swap. Current swap status output: %s", finalState)
		res.Message = res.Error.Error(); hostCtxLogger.Errorf("Step failed: %s", res.Message)
	}
	return res
}
var _ step.StepExecutor = &DisableSwapStepExecutor{}
