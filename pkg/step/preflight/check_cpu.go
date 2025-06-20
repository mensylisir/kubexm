package preflight

import (
	"errors" // For errors.As
	"fmt"
	"strconv"
	"strings"
	// "time" // No longer needed

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	// spec is no longer needed
)

// CheckCPUStep checks if the host meets the minimum CPU core requirement.
type CheckCPUStep struct {
	MinCores int
	StepName string
	// Internal field to store result of check from Precheck to Run
	actualCores int
	checkError  error
}

// NewCheckCPUStep creates a new CheckCPUStep.
func NewCheckCPUStep(minCores int, stepName string) step.Step {
	name := stepName
	if name == "" {
		name = fmt.Sprintf("Check CPU Cores (minimum %d)", minCores)
	}
	return &CheckCPUStep{
		MinCores: minCores,
		StepName: name,
	}
}

func (s *CheckCPUStep) determineActualCores(ctx runtime.StepContext, host connector.Host) (int, error) {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName())

	facts, errFacts := ctx.GetHostFacts(host)
	if errFacts == nil && facts != nil && facts.TotalCPU > 0 {
		logger.Debug("Using CPU count from facts.", "cores", facts.TotalCPU)
		return facts.TotalCPU, nil
	}
	if errFacts != nil {
		logger.Debug("Failed to get host facts, will try command to determine CPU cores.", "error", errFacts)
	} else if facts == nil || facts.TotalCPU <= 0 {
		logger.Debug("CPU facts not available or zero, attempting to get CPU count via command.")
	}

	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		return 0, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), errConn)
	}

	cmdToRun := "nproc" // Default for Linux
	osID := "linux"     // Default assumption
	// Try to get OS ID from facts if available, to adjust command if necessary
	if facts != nil && facts.OS != nil && facts.OS.ID != "" {
		osID = strings.ToLower(facts.OS.ID)
	} else {
		// As a deeper fallback, try to get OS from connector if facts didn't provide it
		// This is more involved and might be skipped if facts are expected to be reliable.
		// For now, we rely on facts or the default 'linux'.
		logger.Debug("OS ID not available from facts, defaulting to Linux for CPU count command.")
	}

	if osID == "darwin" { // macOS
		cmdToRun = "sysctl -n hw.ncpu"
	}
	// Add other OS specific commands here if needed, e.g., for Windows:
	// else if osID == "windows" { cmdToRun = "echo %NUMBER_OF_PROCESSORS%" } // Or wmic cpu get NumberOfCores

	stdoutBytes, stderrBytes, execErr := conn.Exec(ctx.GoContext(), cmdToRun, &connector.ExecOptions{})
	if execErr != nil {
		return 0, fmt.Errorf("failed to execute command to get CPU count ('%s') on host %s (stderr: %s): %w", cmdToRun, host.GetName(), string(stderrBytes), execErr)
	}

	coresStr := strings.TrimSpace(string(stdoutBytes))
	parsedCores, parseErr := strconv.Atoi(coresStr)
	if parseErr != nil {
		return 0, fmt.Errorf("failed to parse CPU count from command output '%s' on host %s: %w", coresStr, host.GetName(), parseErr)
	}
	logger.Debug("Determined CPU count via command.", "command", cmdToRun, "cores", parsedCores)
	return parsedCores, nil
}

func (s *CheckCPUStep) Name() string {
	return s.StepName
}

func (s *CheckCPUStep) Description() string {
	return fmt.Sprintf("Checks if the host has at least %d CPU cores.", s.MinCores)
}

func (s *CheckCPUStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Precheck")

	if host == nil {
	    s.checkError = fmt.Errorf("host is nil in Precheck for %s", s.Name())
	    logger.Error(s.checkError.Error())
		return false, s.checkError
	}

	actualCores, err := s.determineActualCores(ctx, host)
	s.actualCores = actualCores
	s.checkError = err

	if err != nil {
		logger.Error("Error determining CPU cores during precheck.", "error", err)
		return false, err
	}

	if actualCores >= s.MinCores {
		logger.Info("CPU core requirement met.", "actual", actualCores, "minimum", s.MinCores)
		return true, nil
	}

	logger.Info("CPU core requirement not met.", "actual", actualCores, "minimum", s.MinCores)
	// Store the fact that requirement is not met, but no error in precheck itself.
	// Run will then return the actual error indicating the failure.
	s.checkError = fmt.Errorf("host has %d CPU cores, but minimum requirement is %d cores", s.actualCores, s.MinCores)
	return false, nil
}

func (s *CheckCPUStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Run")

	// If Precheck encountered an error determining cores, s.checkError would be that error.
	// If Precheck determined cores but they were insufficient, s.checkError was set to the failure message.
	if s.checkError != nil {
		// This error is the one determined by Precheck (either a failure to check, or requirement not met)
		logger.Error("CPU check failed.", "reason", s.checkError.Error())
		return s.checkError
	}

	// This case implies Precheck returned (true, nil), meaning requirement was met.
	// Run should ideally not be called by the engine if Precheck is true.
	// If it is, then it's a no-op success.
	if s.actualCores >= s.MinCores {
	    logger.Info("CPU core requirement already met (Run called after Precheck returned true).", "actual", s.actualCores, "minimum", s.MinCores)
		return nil
	}

	// Should not be reached if Precheck logic is correct and s.checkError is set.
	// This is a fallback error.
	unknownFailureMsg := fmt.Sprintf("CPU check failed for an unexpected reason for step %s on host %s (actual: %d, min: %d)", s.Name(), host.GetName(), s.actualCores, s.MinCores)
	logger.Error(unknownFailureMsg)
	return errors.New(unknownFailureMsg)
}

func (s *CheckCPUStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	// No action to rollback for a check-only step.
	return nil
}

// Ensure CheckCPUStep implements the step.Step interface.
var _ step.Step = (*CheckCPUStep)(nil)
