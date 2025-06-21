package preflight

import (
	"errors" // For errors.As
	"fmt"
	"strconv"
	"strings"
	// "time" // No longer needed

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
)

// CheckCPUStep checks if the host meets the minimum CPU core requirement.
type CheckCPUStep struct {
	meta     spec.StepMeta
	MinCores int
	Sudo     bool // For running commands like nproc, though usually not needed.
	// Internal field to store result of check from Precheck to Run
	actualCores int
	checkError  error
}

// NewCheckCPUStep creates a new CheckCPUStep.
func NewCheckCPUStep(instanceName string, minCores int, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = fmt.Sprintf("CheckCPUCoresMinimum%d", minCores)
	}
	return &CheckCPUStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Checks if the host has at least %d CPU cores.", minCores),
		},
		MinCores: minCores,
		Sudo:     sudo,
	}
}

// Meta returns the step's metadata.
func (s *CheckCPUStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *CheckCPUStep) determineActualCores(ctx runtime.StepContext, host connector.Host) (int, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName())

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

	runnerSvc := ctx.GetRunner()
	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		return 0, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), errConn)
	}

	cmdToRun := "nproc"
	osID := "linux"
	if facts != nil && facts.OS != nil && facts.OS.ID != "" {
		osID = strings.ToLower(facts.OS.ID)
	} else {
		logger.Debug("OS ID not available from facts, defaulting to Linux for CPU count command.")
	}

	if osID == "darwin" {
		cmdToRun = "sysctl -n hw.ncpu"
	}

	execOpts := &connector.ExecOptions{Sudo: s.Sudo}
	stdoutBytes, stderrBytes, execErr := runnerSvc.RunWithOptions(ctx.GoContext(), conn, cmdToRun, execOpts)
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

func (s *CheckCPUStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	if host == nil {
		s.checkError = fmt.Errorf("host is nil in Precheck for %s", s.meta.Name)
		logger.Error(s.checkError.Error())
		return false, s.checkError
	}

	actualCores, err := s.determineActualCores(ctx, host)
	s.actualCores = actualCores // Store for Run
	s.checkError = err          // Store error for Run

	if err != nil {
		logger.Error("Error determining CPU cores during precheck.", "error", err)
		// Pass the error to Run phase. Precheck itself doesn't fail due to check logic error,
		// but Run will report it.
		return false, nil
	}

	if actualCores >= s.MinCores {
		logger.Info("CPU core requirement met.", "actual", actualCores, "minimum", s.MinCores)
		s.checkError = nil // Clear any previous non-fatal error if check now passes
		return true, nil
	}

	logger.Info("CPU core requirement not met.", "actual", actualCores, "minimum", s.MinCores)
	s.checkError = fmt.Errorf("host has %d CPU cores, but minimum requirement is %d cores", s.actualCores, s.MinCores)
	return false, nil // Indicate Run should be called to report this failure.
}

func (s *CheckCPUStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	// Precheck should have populated s.checkError if there was an issue or requirement not met.
	if s.checkError != nil {
		logger.Error("CPU check failed.", "reason", s.checkError.Error())
		return s.checkError
	}

	// This case implies Precheck returned (true, nil) OR (false, nil) but s.checkError was not set
	// to a failure (e.g. if actualCores >= s.MinCores but something else was an issue).
	// If Precheck was true, Run shouldn't be called. If it is, it's a no-op success.
	if s.actualCores >= s.MinCores {
		logger.Info("CPU core requirement already met (Run called after Precheck returned true or did not set failure).", "actual", s.actualCores, "minimum", s.MinCores)
		return nil
	}

	// This should ideally not be reached if Precheck correctly sets s.checkError when requirement is not met.
	// This is a fallback error message.
	unknownFailureMsg := fmt.Sprintf("CPU check failed for an unexpected reason for step %s on host %s (actual: %d, min: %d)", s.meta.Name, host.GetName(), s.actualCores, s.MinCores)
	logger.Error(unknownFailureMsg)
	return errors.New(unknownFailureMsg)
}

func (s *CheckCPUStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	// No action to rollback for a check-only step.
	return nil
}

// Ensure CheckCPUStep implements the step.Step interface.
var _ step.Step = (*CheckCPUStep)(nil)
