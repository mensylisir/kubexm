package preflight

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/runner" // Assuming runner.Runner is defined here
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
)

// CheckCPUStep verifies that the host has a minimum number of CPU cores.
type CheckCPUStep struct {
	MinCores int
}

// Name returns a human-readable name for the step.
func (s *CheckCPUStep) Name() string {
	return fmt.Sprintf("Check CPU Cores (minimum %d)", s.MinCores)
}

func (s *CheckCPUStep) runCheckLogic(ctx *runtime.Context) (currentCores int, met bool, err error) {
	if ctx.Host.Runner == nil {
		return 0, false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	if ctx.Host.Runner.Facts == nil || ctx.Host.Runner.Facts.TotalCPU == 0 {
		// Fallback to executing command if facts are not populated or zero (which might indicate fact gathering issue)
		// This provides robustness if NewRunner had issues with nproc but the command might still work.
		ctx.Logger.Debugf("CPU facts not available or zero for host %s, attempting to get CPU count via command 'nproc'.", ctx.Host.Name)

		cmdToRun := "nproc"
		// Check OS from facts if available, even if TotalCPU is zero
		osID := "linux" // Default assumption
		if ctx.Host.Runner.Facts != nil && ctx.Host.Runner.Facts.OS != nil && ctx.Host.Runner.Facts.OS.ID != "" {
			osID = ctx.Host.Runner.Facts.OS.ID
		}

		if osID == "darwin" {
			cmdToRun = "sysctl -n hw.ncpu"
		}

		stdout, stderr, execErr := ctx.Host.Runner.Run(ctx.GoContext, cmdToRun, false)
		if execErr != nil {
			// If the primary command failed, and we haven't already tried the alternative based on OS ID,
			// this is where a more complex fallback logic for other OS specific commands could go.
			// For now, we assume one attempt based on OS (or default nproc).
			return 0, false, fmt.Errorf("failed to execute command to get CPU count ('%s') on host %s: %w (stderr: %s)", cmdToRun, ctx.Host.Name, execErr, stderr)
		}

		coresStr := strings.TrimSpace(string(stdout))
		currentCores, err = strconv.Atoi(coresStr)
		if err != nil {
			return 0, false, fmt.Errorf("failed to parse CPU count from command output '%s' on host %s: %w", coresStr, ctx.Host.Name, err)
		}
	} else {
		currentCores = ctx.Host.Runner.Facts.TotalCPU
		ctx.Logger.Debugf("Using CPU count from facts for host %s: %d", ctx.Host.Name, currentCores)
	}

	if currentCores >= s.MinCores {
		return currentCores, true, nil
	}
	return currentCores, false, nil
}


// Check determines if the CPU core requirement is already met.
func (s *CheckCPUStep) Check(ctx *runtime.Context) (isDone bool, err error) {
	_, met, err := s.runCheckLogic(ctx)
	if err != nil {
		// If there was an error determining the state (e.g., command failed),
		// we consider the check not "done" and propagate the error.
		return false, err
	}
	// If met is true, then the condition is satisfied, so the step is "done".
	return met, nil
}

// Run executes the check for CPU cores.
func (s *CheckCPUStep) Run(ctx *runtime.Context) *step.Result {
	startTime := time.Now()
	res := step.NewResult(s.Name(), ctx.Host.Name, startTime, nil)

	currentCores, met, checkErr := s.runCheckLogic(ctx)
	res.EndTime = time.Now()

	if checkErr != nil {
		res.Error = checkErr
		res.Status = "Failed"
		res.Message = fmt.Sprintf("Error checking CPU cores on host %s: %v", ctx.Host.Name, checkErr)
		ctx.Logger.Errorf("Step '%s' on host %s failed: %v", s.Name(), ctx.Host.Name, checkErr)
		return res
	}

	if met {
		res.Status = "Succeeded"
		res.Message = fmt.Sprintf("Host %s has %d CPU cores, which meets the minimum requirement of %d cores.", ctx.Host.Name, currentCores, s.MinCores)
		ctx.Logger.Successf("Step '%s' on host %s succeeded: %s", s.Name(), ctx.Host.Name, res.Message)
	} else {
		res.Status = "Failed" // Failed because condition not met
		res.Error = fmt.Errorf("host %s has %d CPU cores, but minimum requirement is %d cores", ctx.Host.Name, currentCores, s.MinCores)
		res.Message = res.Error.Error()
		ctx.Logger.Errorf("Step '%s' on host %s failed: %s", s.Name(), ctx.Host.Name, res.Message)
	}
	return res
}

var _ step.Step = &CheckCPUStep{}
