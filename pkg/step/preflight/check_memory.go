package preflight

import (
	"errors" // For creating new errors
	"fmt"
	"strconv"
	"strings"
	// "time" // No longer needed

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for spec.StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
)

// CheckMemoryStepSpec checks if the host meets the minimum memory requirement.
type CheckMemoryStepSpec struct {
	spec.StepMeta `json:",inline"`
	MinMemoryMB   uint64 `json:"minMemoryMB,omitempty"`
}

// NewCheckMemoryStepSpec creates a new CheckMemoryStepSpec.
func NewCheckMemoryStepSpec(minMemoryMB uint64, name, description string) *CheckMemoryStepSpec {
	finalName := name
	if finalName == "" {
		finalName = fmt.Sprintf("Check Memory (minimum %d MB)", minMemoryMB)
	}
	finalDescription := description
	if finalDescription == "" {
		finalDescription = fmt.Sprintf("Checks if the host has at least %d MB memory.", minMemoryMB)
	}
	return &CheckMemoryStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		MinMemoryMB: minMemoryMB,
	}
}

func (s *CheckMemoryStepSpec) determineActualMemoryMB(ctx runtime.StepContext, host connector.Host) (uint64, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName())

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

// Name returns the step's name (implementing step.Step).
func (s *CheckMemoryStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description (implementing step.Step).
func (s *CheckMemoryStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *CheckMemoryStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *CheckMemoryStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *CheckMemoryStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *CheckMemoryStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *CheckMemoryStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")

	if host == nil {
	    errMsg := fmt.Errorf("host is nil in Precheck for %s", s.GetName())
	    logger.Error(errMsg.Error())
		return false, errMsg
	}

	actualMemoryMB, err := s.determineActualMemoryMB(ctx, host)
	if err != nil {
		logger.Error("Error determining memory size during precheck.", "error", err)
		return false, fmt.Errorf("failed to determine memory size for step %s on host %s: %w", s.GetName(), host.GetName(), err)
	}

	if actualMemoryMB >= s.MinMemoryMB {
		logger.Info("Memory requirement met.", "actualMB", actualMemoryMB, "minimumMB", s.MinMemoryMB)
		return true, nil
	}

	logger.Info("Memory requirement not met.", "actualMB", actualMemoryMB, "minimumMB", s.MinMemoryMB)
	return false, nil // Requirement not met, but precheck itself is successful. Run will error.
}

func (s *CheckMemoryStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")

	if host == nil {
		return fmt.Errorf("host is nil in Run for %s", s.GetName())
	}

	actualMemoryMB, err := s.determineActualMemoryMB(ctx, host)
	if err != nil {
		return fmt.Errorf("failed to determine memory size for step %s on host %s: %w", s.GetName(), host.GetName(), err)
	}

	if actualMemoryMB < s.MinMemoryMB {
		errMsg := fmt.Errorf("host has %d MB memory, but minimum requirement is %d MB for step %s on host %s", actualMemoryMB, s.MinMemoryMB, s.GetName(), host.GetName())
		logger.Error(errMsg.Error())
		return errMsg
	}

	logger.Info("Memory requirement met.", "actualMB", actualMemoryMB, "minimumMB", s.MinMemoryMB)
	return nil
}

func (s *CheckMemoryStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	// No action to rollback for a check-only step.
	return nil
}

// Ensure CheckMemoryStepSpec implements the step.Step interface.
var _ step.Step = (*CheckMemoryStepSpec)(nil)
