package preflight

import (
	"errors" // For creating new errors
	"fmt"
	"strconv"
	"strings"
	// "time" // No longer needed

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	// spec is no longer needed
)

// CheckMemoryStep checks if the host meets the minimum memory requirement.
type CheckMemoryStep struct {
	MinMemoryMB uint64
	StepName    string
	// Internal field to store result of check from Precheck to Run
	actualMemoryMB uint64
	checkError     error
}

// NewCheckMemoryStep creates a new CheckMemoryStep.
func NewCheckMemoryStep(minMemoryMB uint64, stepName string) step.Step {
	name := stepName
	if name == "" {
		name = fmt.Sprintf("Check Memory (minimum %d MB)", minMemoryMB)
	}
	return &CheckMemoryStep{
		MinMemoryMB: minMemoryMB,
		StepName:    name,
	}
}

func (s *CheckMemoryStep) determineActualMemoryMB(ctx runtime.StepContext, host connector.Host) (uint64, error) {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName())

	facts, errFacts := ctx.GetHostFacts(host)
	if errFacts == nil && facts != nil && facts.TotalMemory > 0 { // TotalMemory in facts is expected to be in MiB
		logger.Debug("Using memory size from facts.", "memoryMB", facts.TotalMemory)
		return facts.TotalMemory, nil
	}
	if errFacts != nil {
		logger.Debug("Failed to get host facts, will try command to determine memory size.", "error", errFacts)
	} else if facts == nil || facts.TotalMemory <= 0 {
		logger.Debug("Memory facts not available or zero, attempting to get memory via command.")
	}

	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		return 0, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), errConn)
	}

	var cmd string
	var isKb, isBytes bool

	osID := "linux" // Default assumption
	// Try to get OS ID from facts if available, to adjust command if necessary
	if facts != nil && facts.OS != nil && facts.OS.ID != "" {
		osID = strings.ToLower(facts.OS.ID)
	} else {
		logger.Debug("OS ID not available from facts, defaulting to Linux for memory check command.")
	}

	if osID == "darwin" { // macOS
		cmd = "sysctl -n hw.memsize"
		isBytes = true
	} else { // Assuming Linux-like for /proc/meminfo
		cmd = "grep MemTotal /proc/meminfo | awk '{print $2}'"
		isKb = true
	}

	stdoutBytes, stderrBytes, execErr := conn.Exec(ctx.GoContext(), cmd, &connector.ExecOptions{})
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
		calculatedMemoryMB = memVal / 1024 // Convert KB to MB
	} else if isBytes {
		calculatedMemoryMB = memVal / (1024 * 1024) // Convert Bytes to MB
	} else {
		// This case should ideally not be reached if osID logic correctly sets isKb or isBytes.
		// If it does, it implies memVal is already in MB or unit is unknown.
		logger.Warn("Memory unit flag (isKb/isBytes) not set after command execution; assuming value is in MB.", "value", memVal)
		calculatedMemoryMB = memVal
	}
	logger.Debug("Determined memory size via command.", "command", cmd, "memoryMB", calculatedMemoryMB)
	return calculatedMemoryMB, nil
}

func (s *CheckMemoryStep) Name() string {
	return s.StepName
}

func (s *CheckMemoryStep) Description() string {
	return fmt.Sprintf("Checks if the host has at least %d MB memory.", s.MinMemoryMB)
}

func (s *CheckMemoryStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Precheck")

	if host == nil {
	    s.checkError = fmt.Errorf("host is nil in Precheck for %s", s.Name())
	    logger.Error(s.checkError.Error())
		return false, s.checkError
	}

	actualMemoryMB, err := s.determineActualMemoryMB(ctx, host)
	s.actualMemoryMB = actualMemoryMB
	s.checkError = err

	if err != nil {
		logger.Error("Error determining memory size during precheck.", "error", err)
		return false, err
	}

	if actualMemoryMB >= s.MinMemoryMB {
		logger.Info("Memory requirement met.", "actualMB", actualMemoryMB, "minimumMB", s.MinMemoryMB)
		return true, nil
	}

	errMsg := fmt.Sprintf("Host has %d MB memory, but minimum requirement is %d MB.", actualMemoryMB, s.MinMemoryMB)
	logger.Info(errMsg + " (Precheck determined failure, Run will report this error)")
	s.checkError = errors.New(errMsg) // Store specific failure error for Run
	return false, nil
}

func (s *CheckMemoryStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Run")

	if s.checkError != nil {
	    logger.Error("Memory check failed.", "reason", s.checkError)
		return s.checkError
	}

	// This case implies Precheck returned (true, nil).
	logger.Info("Memory requirement already met (unexpectedly in Run).", "actualMB", s.actualMemoryMB, "minimumMB", s.MinMemoryMB)
	return nil
}

func (s *CheckMemoryStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	// No action to rollback for a check-only step.
	return nil
}

// Ensure CheckMemoryStep implements the step.Step interface.
var _ step.Step = (*CheckMemoryStep)(nil)
