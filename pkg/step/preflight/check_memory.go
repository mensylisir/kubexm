package preflight

import (
	"errors" // For creating new errors
	"fmt"
	"strconv"
	"strings"
	// "time" // No longer needed

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
)

// CheckMemoryStep checks if the host meets the minimum memory requirement.
type CheckMemoryStep struct {
	meta           spec.StepMeta
	MinMemoryMB    uint64
	Sudo           bool // For running commands, though usually not needed for memory checks.
	actualMemoryMB uint64
	checkError     error
}

// NewCheckMemoryStep creates a new CheckMemoryStep.
func NewCheckMemoryStep(instanceName string, minMemoryMB uint64, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = fmt.Sprintf("CheckMemoryMinimum%dMB", minMemoryMB)
	}
	return &CheckMemoryStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Checks if the host has at least %d MB memory.", minMemoryMB),
		},
		MinMemoryMB: minMemoryMB,
		Sudo:        sudo,
	}
}

// Meta returns the step's metadata.
func (s *CheckMemoryStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *CheckMemoryStep) determineActualMemoryMB(ctx runtime.StepContext, host connector.Host) (uint64, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())

	facts, errFacts := ctx.GetHostFacts(host)
	if errFacts == nil && facts != nil && facts.TotalMemory > 0 {
		logger.Debug("Using memory size from facts.", "memoryMB", facts.TotalMemory)
		return facts.TotalMemory, nil
	}
	if errFacts != nil {
		logger.Debug("Failed to get host facts, will try command to determine memory size.", "error", errFacts)
	} else if facts == nil || facts.TotalMemory <= 0 {
		logger.Debug("Memory facts not available or zero, attempting to get memory via command.")
	}

	runnerSvc := ctx.GetRunner()
	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		return 0, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), errConn)
	}

	var cmd string
	var isKb, isBytes bool

	osID := "linux"
	if facts != nil && facts.OS != nil && facts.OS.ID != "" {
		osID = strings.ToLower(facts.OS.ID)
	} else {
		logger.Debug("OS ID not available from facts, defaulting to Linux for memory check command.")
	}

	if osID == "darwin" {
		cmd = "sysctl -n hw.memsize"
		isBytes = true
	} else {
		cmd = "grep MemTotal /proc/meminfo | awk '{print $2}'"
		isKb = true
	}

	execOpts := &connector.ExecOptions{Sudo: s.Sudo}
	stdoutBytes, stderrBytes, execErr := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmd, execOpts)
	if execErr != nil {
		return 0, fmt.Errorf("failed to execute command ('%s') on host %s to get memory info (stderr: %s): %w", cmd, host.GetName(), string(stderrBytes), execErr)
	}

	memStr := strings.TrimSpace(string(stdoutBytes))
	memVal, parseErr := strconv.ParseUint(memStr, 10, 64)
	if parseErr != nil {
		return 0, fmt.Errorf("failed to parse memory from command output '%s' on host %s: %w", memStr, host.GetName(), parseErr)
	}

	var calculatedMemoryMB uint64
	if isKb {
		calculatedMemoryMB = memVal / 1024
	} else if isBytes {
		calculatedMemoryMB = memVal / (1024 * 1024)
	} else {
		logger.Warn("Memory unit flag (isKb/isBytes) not set after command execution; assuming value is in MB.", "value", memVal)
		calculatedMemoryMB = memVal
	}
	logger.Debug("Determined memory size via command.", "command", cmd, "memoryMB", calculatedMemoryMB)
	return calculatedMemoryMB, nil
}

func (s *CheckMemoryStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	if host == nil {
		s.checkError = fmt.Errorf("host is nil in Precheck for %s", s.meta.Name)
		logger.Error(s.checkError.Error())
		return false, s.checkError
	}

	actualMemoryMB, err := s.determineActualMemoryMB(ctx, host)
	s.actualMemoryMB = actualMemoryMB
	s.checkError = err

	if err != nil {
		logger.Error("Error determining memory size during precheck.", "error", err)
		return false, nil // Let Run report the error from determineActualMemoryMB
	}

	if actualMemoryMB >= s.MinMemoryMB {
		logger.Info("Memory requirement met.", "actualMB", actualMemoryMB, "minimumMB", s.MinMemoryMB)
		s.checkError = nil // Clear any previous non-fatal error
		return true, nil
	}

	errMsg := fmt.Sprintf("Host has %d MB memory, but minimum requirement is %d MB.", actualMemoryMB, s.MinMemoryMB)
	logger.Info(errMsg + " (Precheck determined failure, Run will report this error)")
	s.checkError = errors.New(errMsg)
	return false, nil
}

func (s *CheckMemoryStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	if s.checkError != nil {
		logger.Error("Memory check failed.", "reason", s.checkError.Error()) // Use Error() for consistent message
		return s.checkError
	}

	// This case implies Precheck returned (true, nil) or Precheck didn't set s.checkError to a failure.
	// If Precheck was true, Run shouldn't be called. If it is, it's a no-op success.
	if s.actualMemoryMB >= s.MinMemoryMB {
		logger.Info("Memory requirement already met (Run called after Precheck returned true or did not set failure).", "actualMB", s.actualMemoryMB, "minimumMB", s.MinMemoryMB)
		return nil
	}
	// Fallback if logic error in Precheck and s.checkError was not set for failure
	return fmt.Errorf("host has %d MB memory, but minimum requirement is %d MB for step %s on host %s", s.actualMemoryMB, s.MinMemoryMB, s.meta.Name, host.GetName())
}

func (s *CheckMemoryStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	return nil
}

func (s *CheckMemoryStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	// No action to rollback for a check-only step.
	return nil
}

// Ensure CheckMemoryStep implements the step.Step interface.
var _ step.Step = (*CheckMemoryStep)(nil)
