package preflight

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kubexms/kubexms/pkg/runner"
	"github.com/kubexms/kubexms/pkg/runtime"
	"github.com/kubexms/kubexms/pkg/step"
)

// CheckMemoryStep verifies that the host has a minimum amount of memory.
type CheckMemoryStep struct {
	MinMemoryMB uint64
}

// Name returns a human-readable name for the step.
func (s *CheckMemoryStep) Name() string {
	return fmt.Sprintf("Check Memory (minimum %d MB)", s.MinMemoryMB)
}

func (s *CheckMemoryStep) runCheckLogic(ctx *runtime.Context) (currentMemoryMB uint64, met bool, err error) {
	if ctx.Host.Runner == nil {
		return 0, false, fmt.Errorf("runner not available in context for host %s", ctx.Host.Name)
	}
	if ctx.Host.Runner.Facts == nil || ctx.Host.Runner.Facts.TotalMemory == 0 {
		ctx.Logger.Debugf("Memory facts not available or zero for host %s, attempting to get memory via command.", ctx.Host.Name)

		var cmd string
		var isKb bool // True if command output is in KB, false if in bytes

		osID := "linux" // Default assumption
		if ctx.Host.Runner.Facts != nil && ctx.Host.Runner.Facts.OS != nil && ctx.Host.Runner.Facts.OS.ID != "" {
			osID = ctx.Host.Runner.Facts.OS.ID
		}

		if osID == "darwin" {
			cmd = "sysctl -n hw.memsize"
			isKb = false
		} else { // Assume Linux-like /proc/meminfo
			cmd = "grep MemTotal /proc/meminfo | awk '{print $2}'"
			isKb = true
		}

		stdout, stderr, execErr := ctx.Host.Runner.Run(ctx.GoContext, cmd, false)
		if execErr != nil {
			return 0, false, fmt.Errorf("failed to execute command to get memory info ('%s') on host %s: %w (stderr: %s)", cmd, ctx.Host.Name, execErr, stderr)
		}

		memStr := strings.TrimSpace(string(stdout))
		memVal, parseErr := strconv.ParseUint(memStr, 10, 64)
		if parseErr != nil {
			return 0, false, fmt.Errorf("failed to parse memory from command output '%s' on host %s: %w", memStr, ctx.Host.Name, parseErr)
		}

		if isKb {
			currentMemoryMB = memVal / 1024 // Convert KB to MB
		} else {
			currentMemoryMB = memVal / (1024 * 1024) // Convert Bytes to MB
		}
	} else {
		currentMemoryMB = ctx.Host.Runner.Facts.TotalMemory // Facts.TotalMemory is already in MiB
		ctx.Logger.Debugf("Using memory size from facts for host %s: %d MB", ctx.Host.Name, currentMemoryMB)
	}

	if currentMemoryMB >= s.MinMemoryMB {
		return currentMemoryMB, true, nil
	}
	return currentMemoryMB, false, nil
}

// Check determines if the memory requirement is already met.
func (s *CheckMemoryStep) Check(ctx *runtime.Context) (isDone bool, err error) {
	_, met, err := s.runCheckLogic(ctx)
	if err != nil {
		return false, err
	}
	return met, nil
}

// Run executes the check for memory size.
func (s *CheckMemoryStep) Run(ctx *runtime.Context) *step.Result {
	startTime := time.Now()
	res := step.NewResult(s.Name(), ctx.Host.Name, startTime, nil)

	currentMemoryMB, met, checkErr := s.runCheckLogic(ctx)
	res.EndTime = time.Now()

	if checkErr != nil {
		res.Error = checkErr
		res.Status = "Failed"
		res.Message = fmt.Sprintf("Error checking memory on host %s: %v", ctx.Host.Name, checkErr)
		ctx.Logger.Errorf("Step '%s' on host %s failed: %v", s.Name(), ctx.Host.Name, checkErr)
		return res
	}

	if met {
		res.Status = "Succeeded"
		res.Message = fmt.Sprintf("Host %s has %d MB memory, which meets the minimum requirement of %d MB.", ctx.Host.Name, currentMemoryMB, s.MinMemoryMB)
		ctx.Logger.Successf("Step '%s' on host %s succeeded: %s", s.Name(), ctx.Host.Name, res.Message)
	} else {
		res.Status = "Failed"
		res.Error = fmt.Errorf("host %s has %d MB memory, but minimum requirement is %d MB", ctx.Host.Name, currentMemoryMB, s.MinMemoryMB)
		res.Message = res.Error.Error()
		ctx.Logger.Errorf("Step '%s' on host %s failed: %s", s.Name(), ctx.Host.Name, res.Message)
	}
	return res
}

var _ step.Step = &CheckMemoryStep{}
