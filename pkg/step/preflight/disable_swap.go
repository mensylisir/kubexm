package preflight

import (
	"context"
	"errors" // Required for errors.As
	"fmt"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/connector" // For CommandError
	"github.com/kubexms/kubexms/pkg/runner"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
)

// DisableSwapStep disables swap on the host and comments out swap entries in /etc/fstab.
type DisableSwapStep struct{}

// Name returns a human-readable name for the step.
func (s *DisableSwapStep) Name() string {
	return "Disable Swap"
}

// isSwapOn checks if swap is currently active by inspecting `swapon --summary` or `/proc/swaps`.
func isSwapOn(ctx *runtime.Context) (bool, string, error) {
	if ctx.Host.Runner == nil {
		return false, "", fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	// Try `swapon --summary` first.
	stdout, stderr, err := ctx.Host.Runner.Run(ctx.GoContext, "swapon --summary --noheadings", false) // Try with noheadings
	if err != nil {
		// If --noheadings is not supported, swapon might return an error. Try without it.
		var cmdErr *connector.CommandError
		if errors.As(err, &cmdErr) && (strings.Contains(strings.ToLower(string(stderr)), "invalid option") || strings.Contains(strings.ToLower(string(stderr)), "bad usage")) {
			ctx.Logger.Debugf("`swapon --summary --noheadings` failed, trying `swapon --summary` on host %s.", ctx.Host.Name)
			stdout, stderr, err = ctx.Host.Runner.Run(ctx.GoContext, "swapon --summary", false)
		}
	}


	if err != nil {
		var cmdErr *connector.CommandError
		isCmdNotFoundError := false
		if errors.As(err, &cmdErr) && cmdErr.ExitCode == 127 { // 127 for command not found
			isCmdNotFoundError = true
		}

		// Attempt to read /proc/swaps as a fallback on Linux if swapon failed generally or not found
		if ctx.Host.Runner.Facts != nil && ctx.Host.Runner.Facts.OS != nil && ctx.Host.Runner.Facts.OS.ID == "linux" {
			ctx.Logger.Warnf("`swapon --summary` command failed on host %s (Error: %v, Stderr: %s), attempting to read /proc/swaps.", ctx.Host.Name, err, string(stderr))
			procSwapsContent, readErr := ctx.Host.Runner.ReadFile(ctx.GoContext, "/proc/swaps")
			if readErr != nil {
				return false, "", fmt.Errorf("failed to run 'swapon --summary' and also failed to read /proc/swaps on host %s: %w", ctx.Host.Name, readErr)
			}
			lines := strings.Split(strings.TrimSpace(string(procSwapsContent)), "\n")
			if len(lines) > 1 { // Header line + data lines means swap is on
				return true, string(procSwapsContent), nil
			}
			return false, string(procSwapsContent), nil
		}
		// If not Linux and swapon command was not found
		if isCmdNotFoundError {
		    return false, "", fmt.Errorf("`swapon` command not found and OS is not Linux, cannot determine swap status on host %s", ctx.Host.Name)
		}
		// Other execution error for swapon --summary
		return false, "", fmt.Errorf("failed to execute 'swapon --summary' on host %s: %w (stderr: %s)", ctx.Host.Name, err, string(stderr))
	}

	// If `swapon --summary` ran, check its output.
	// If --noheadings worked, any line means swap is on.
	// If --noheadings didn't work, output has a header. More than 1 line means swap is on.
	trimmedStdout := strings.TrimSpace(string(stdout))
	if trimmedStdout == "" { // No output means no swap
		return false, string(stdout), nil
	}
	lines := strings.Split(trimmedStdout, "\n")
	// If the only line is the header (e.g., "Filename Type Size Used Priority")
	if len(lines) == 1 && strings.Contains(lines[0], "Filename") && strings.Contains(lines[0], "Type") {
		return false, string(stdout), nil
	}
	// Otherwise, if there's any content (especially if --noheadings worked, or multiple lines)
	return true, string(stdout), nil
}

// Check determines if swap is already disabled.
func (s *DisableSwapStep) Check(ctx *runtime.Context) (isDone bool, err error) {
	swapOn, swapOutput, checkErr := isSwapOn(ctx)
	if checkErr != nil {
		return false, fmt.Errorf("error checking swap status on host %s: %w. Output: %s", ctx.Host.Name, checkErr, swapOutput)
	}
	// isDone is true if swap is NOT on.
	return !swapOn, nil
}

// Run executes the commands to disable swap.
func (s *DisableSwapStep) Run(ctx *runtime.Context) *step.Result {
	startTime := time.Now()
	res := step.NewResult(s.Name(), ctx.Host.Name, startTime, nil)
	hostCtxLogger := ctx.Logger.SugaredLogger.With("host", ctx.Host.Name, "step", s.Name()).Sugar()


	// 1. Turn off current swap
	hostCtxLogger.Infof("Attempting to turn off active swap with 'swapoff -a'...")
	_, stderrSwapoff, errSwapoff := ctx.Host.Runner.Run(ctx.GoContext, "swapoff -a", true) // Sudo needed
	if errSwapoff != nil {
		hostCtxLogger.Warnf("Command 'swapoff -a' finished with error (this might be okay if no swap was active): %v. Stderr: %s", errSwapoff, string(stderrSwapoff))
	} else {
		hostCtxLogger.Successf("'swapoff -a' completed.")
	}
	res.Stdout += fmt.Sprintf("swapoff -a stderr: %s\n", string(stderrSwapoff))


	// 2. Comment out swap entries in /etc/fstab
	fstabPath := "/etc/fstab"
	hostCtxLogger.Infof("Attempting to comment out swap entries in %s...", fstabPath)

	// Create a backup of fstab first
	backupCmd := fmt.Sprintf("cp %s %s.bak-kubexms-%d", fstabPath, fstabPath, time.Now().UnixNano()) // Ensure unique backup name
	_, stderrBackup, errBackup := ctx.Host.Runner.Run(ctx.GoContext, backupCmd, true)
	if errBackup != nil {
		res.Error = fmt.Errorf("failed to backup %s on host %s: %w (stderr: %s)", fstabPath, ctx.Host.Name, errBackup, string(stderrBackup))
		res.Status = "Failed"
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}
	res.Stdout += fmt.Sprintf("Backup %s: OK\n", fstabPath)

	// Sed command to comment lines containing "swap" that are not already comments.
	// This regex tries to find lines with "swap" as a whole word that don't start with "#".
	sedCmd := fmt.Sprintf("sed -E -i.kubexms_fstab_prev '/^[^#].*\\bswap\\b/s/^/#/' %s", fstabPath)

	_, stderrSed, errSed := ctx.Host.Runner.Run(ctx.GoContext, sedCmd, true)
	if errSed != nil {
		res.Error = fmt.Errorf("failed to comment out swap entries in %s using sed on host %s: %w (stderr: %s)", fstabPath, ctx.Host.Name, errSed, string(stderrSed))
		res.Status = "Failed"
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}
	res.Stdout += fmt.Sprintf("Commented swap in %s: OK. Stderr: %s\n", fstabPath, string(stderrSed))
	hostCtxLogger.Successf("Swap entries in %s commented out.", fstabPath)

	// 3. Verify swap is off now
	swapOn, finalState, verifyErr := isSwapOn(ctx)
	if verifyErr != nil {
		res.Error = fmt.Errorf("failed to verify swap status on host %s after attempting disable: %w. Last state: %s", ctx.Host.Name, verifyErr, finalState)
		res.Status = "Failed"
		hostCtxLogger.Errorf("Step failed: %v", res.Error)
		return res
	}

	res.EndTime = time.Now()
	if !swapOn {
		res.Status = "Succeeded"
		res.Message = "Swap is successfully disabled."
		hostCtxLogger.Successf("Step succeeded: %s", res.Message)
	} else {
		res.Status = "Failed"
		res.Error = fmt.Errorf("failed to disable swap on host %s. Current swap status output: %s", ctx.Host.Name, finalState)
		res.Message = res.Error.Error()
		hostCtxLogger.Errorf("Step failed: %s", res.Message)
	}
	return res
}

var _ step.Step = &DisableSwapStep{}
