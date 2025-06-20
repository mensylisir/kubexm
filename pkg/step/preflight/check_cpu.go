package preflight

import (
	"errors" // For errors.As
	"fmt"
	"strconv"
	"strings"
	// "time" // No longer needed

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for spec.StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
)

// CheckCPUStepSpec checks if the host meets the minimum CPU core requirement.
type CheckCPUStepSpec struct {
	spec.StepMeta `json:",inline"`
	MinCores      int `json:"minCores,omitempty"`
}

// NewCheckCPUStepSpec creates a new CheckCPUStepSpec.
func NewCheckCPUStepSpec(minCores int, name, description string) *CheckCPUStepSpec {
	finalName := name
	if finalName == "" {
		finalName = fmt.Sprintf("Check CPU Cores (minimum %d)", minCores)
	}
	finalDescription := description
	if finalDescription == "" {
		finalDescription = fmt.Sprintf("Checks if the host has at least %d CPU cores.", minCores)
	}
	return &CheckCPUStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		MinCores: minCores,
	}
}

func (s *CheckCPUStepSpec) determineActualCores(ctx runtime.StepContext, host connector.Host) (int, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName())

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

// Name returns the step's name (implementing step.Step).
func (s *CheckCPUStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description (implementing step.Step).
func (s *CheckCPUStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *CheckCPUStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *CheckCPUStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *CheckCPUStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *CheckCPUStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *CheckCPUStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")

	if host == nil {
	    errMsg := fmt.Errorf("host is nil in Precheck for %s", s.GetName())
	    logger.Error(errMsg.Error())
		return false, errMsg // Return error as check cannot be performed
	}

	actualCores, err := s.determineActualCores(ctx, host)
	if err != nil {
		logger.Error("Error determining CPU cores during precheck.", "error", err)
		// Return error because the check itself failed, not that the condition is unmet.
		return false, fmt.Errorf("failed to determine CPU cores for step %s on host %s: %w", s.GetName(), host.GetName(), err)
	}

	if actualCores >= s.MinCores {
		logger.Info("CPU core requirement met.", "actual", actualCores, "minimum", s.MinCores)
		return true, nil // Done = true
	}

	logger.Info("CPU core requirement not met.", "actual", actualCores, "minimum", s.MinCores)
	return false, nil // Done = false, but no error from precheck itself. Run will report the failure.
}

func (s *CheckCPUStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")

	if host == nil {
		return fmt.Errorf("host is nil in Run for %s", s.GetName())
	}

	actualCores, err := s.determineActualCores(ctx, host)
	if err != nil {
		// Error determining cores is a failure of the step's execution.
		return fmt.Errorf("failed to determine CPU cores for step %s on host %s: %w", s.GetName(), host.GetName(), err)
	}

	if actualCores < s.MinCores {
		errMsg := fmt.Errorf("host has %d CPU cores, but minimum requirement is %d cores for step %s on host %s", actualCores, s.MinCores, s.GetName(), host.GetName())
		logger.Error(errMsg.Error())
		return errMsg // This is the actual failure of the check.
	}

	logger.Info("CPU core requirement met.", "actual", actualCores, "minimum", s.MinCores)
	return nil // Success
}

func (s *CheckCPUStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	// No action to rollback for a check-only step.
	return nil
}

// Ensure CheckCPUStepSpec implements the step.Step interface.
var _ step.Step = (*CheckCPUStepSpec)(nil)
