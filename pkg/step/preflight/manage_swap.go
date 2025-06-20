package preflight

import (
	"errors"
	"fmt"
	"strings"
	"time" // For backup filename timestamp

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for spec.StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	StatePresent = "present" // or "enabled"
	StateAbsent  = "absent"  // or "disabled"
	FstabPath    = "/etc/fstab"
	SwapCommentMarker = "# KUBEXMS-MANAGED-SWAP:"
)

// ManageSwapStepSpec manages swap state on the target host.
type ManageSwapStepSpec struct {
	spec.StepMeta `json:",inline"`
	DesiredState  string `json:"desiredState,omitempty"` // "disabled" or "enabled"
	// Internal field to store backup path for rollback, not part of the spec for serialization.
	// This field is problematic for a pure spec. For now, keep as runtime state for step.Step impl.
	fstabBackupPath string `json:"-"`
	Sudo            bool   `json:"sudo,omitempty"` // Added Sudo field
}

// NewManageSwapStepSpec creates a new ManageSwapStepSpec.
func NewManageSwapStepSpec(name, description, desiredState string) *ManageSwapStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Manage Swap State"
	}
	finalDescription := description
	// Description will be refined in populateDefaults.

	return &ManageSwapStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		DesiredState: desiredState,
		Sudo:         true, // Default Sudo to true
	}
}

func (s *ManageSwapStepSpec) populateDefaults(logger runtime.Logger) {
	if s.DesiredState == "" {
		s.DesiredState = StateAbsent // Default to "disabled"
		logger.Debug("DesiredState defaulted to 'absent' (disabled).")
	}
	s.DesiredState = strings.ToLower(s.DesiredState)
	// Normalize common terms
	if s.DesiredState == "disabled" { s.DesiredState = StateAbsent }
	if s.DesiredState == "enabled" { s.DesiredState = StatePresent }


	// Sudo is defaulted in factory, but ensure it's logged if logic changes
	// if !s.Sudo { s.Sudo = true; logger.Debug("Sudo defaulted to true.") }


	if s.StepMeta.Description == "" {
		action := "Ensures swap is"
		if s.DesiredState == StateAbsent {
			action += " disabled"
		} else if s.DesiredState == StatePresent {
			action += " enabled"
		} else {
			action = fmt.Sprintf("Manages swap to an unknown state '%s'", s.DesiredState)
		}
		s.StepMeta.Description = action + " (runtime and fstab)."
	}
}


func (s *ManageSwapStepSpec) isSwapOnRuntime(ctx runtime.StepContext, host connector.Host, conn connector.Connector) (bool, string, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "operation", "isSwapOnRuntimeCheck")

	// conn is passed in now
	if conn == nil {
		var errConn error
		conn, errConn = ctx.GetConnectorForHost(host)
		if errConn != nil {
			return false, "", fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), errConn)
		}
	}
		return false, "", fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), errConn)
	}

	// Try 'swapon --summary --noheadings' first
	// Using Sudo:false initially as swapon --summary might be readable by non-root.
	// If it fails due to permissions and needs sudo, that's a configuration issue for the user or a need for sudo:true here.
	// For now, assuming it might work without sudo for read-only info.
	stdoutBytes, stderrBytes, err := conn.Exec(ctx.GoContext(), "swapon --summary --noheadings", &connector.ExecOptions{Sudo: false})
	if err != nil {
		var cmdErr *connector.CommandError
		// If --noheadings is invalid (older swapon), try without it
		if errors.As(err, &cmdErr) && (strings.Contains(strings.ToLower(string(stderrBytes)), "invalid option") || strings.Contains(strings.ToLower(string(stderrBytes)), "bad usage")) {
			logger.Debug("`swapon --summary --noheadings` failed, trying `swapon --summary`.", "stderr", string(stderrBytes))
			stdoutBytes, stderrBytes, err = conn.Exec(ctx.GoContext(), "swapon --summary", &connector.ExecOptions{Sudo: false})
		}
	}

	if err != nil {
		var cmdErr *connector.CommandError
		isCmdNotFoundErr := errors.As(err, &cmdErr) && cmdErr.ExitCode == 127 // Specific exit code for "command not found"

		facts, _ := ctx.GetHostFacts(host) // Best effort to get facts for OS ID
		osID := "unknown"
		if facts != nil && facts.OS != nil && facts.OS.ID != "" {
			osID = strings.ToLower(facts.OS.ID)
		}

		if osID == "linux" { // Fallback for Linux if swapon command fails for other reasons (e.g. permissions, but not not-found)
			logger.Warn("`swapon --summary` command failed, attempting to read /proc/swaps.", "error", err, "stderr", string(stderrBytes))
			procSwapsContentBytes, readErr := conn.ReadFile(ctx.GoContext(), "/proc/swaps")
			if readErr != nil {
				return false, "", fmt.Errorf("failed to run 'swapon --summary' and also failed to read /proc/swaps on host %s: %w", host.GetName(), readErr)
			}
			content := strings.TrimSpace(string(procSwapsContentBytes))
			lines := strings.Split(content, "\n")
			// /proc/swaps has a header line. More than 1 line means swap is configured.
			return len(lines) > 1, content, nil
		}

		if isCmdNotFoundErr { // If swapon not found and not Linux (no /proc/swaps fallback)
			return false, "", fmt.Errorf("`swapon` command not found and OS ('%s') is not Linux with /proc/swaps fallback, cannot determine swap status on host %s", osID, host.GetName())
		}
		// For other errors with swapon when not on Linux (no /proc/swaps fallback)
		return false, "", fmt.Errorf("failed to execute 'swapon --summary' on host %s (stderr: %s): %w", host.GetName(), string(stderrBytes), err)
	}

	// Process output of 'swapon --summary'
	trimmedStdout := strings.TrimSpace(string(stdoutBytes))
	if trimmedStdout == "" { // No output means no swap
		return false, string(stdoutBytes), nil
	}
	lines := strings.Split(trimmedStdout, "\n")
	// If only header line is present (common when --noheadings failed and fallback ran, or if swapon output is just header)
	if len(lines) == 1 && strings.Contains(lines[0], "Filename") && strings.Contains(lines[0], "Type") && strings.Contains(lines[0], "Size") {
		return false, string(stdoutBytes), nil
	}
	// Any other non-empty output (especially with --noheadings) implies swap is configured and likely on.
	return true, string(stdoutBytes), nil
}

// Name returns the step's name (implementing step.Step).
func (s *ManageSwapStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description (implementing step.Step).
func (s *ManageSwapStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *ManageSwapStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *ManageSwapStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *ManageSwapStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *ManageSwapStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

// readFstabAndCheckSwapEntries checks fstab for active swap entries.
// Returns true if active swap entries are found, false otherwise.
func (s *ManageSwapStepSpec) readFstabAndCheckSwapEntries(ctx runtime.StepContext, conn connector.Connector) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "operation", "readFstabAndCheckSwapEntries")
	fstabContentBytes, err := conn.ReadFile(ctx.GoContext(), FstabPath)
	if err != nil {
		// If fstab is not readable, we can't determine persistent state.
		// Depending on strictness, this could be an error or a signal to proceed with runtime changes.
		logger.Warn("Failed to read fstab. Cannot determine persistent swap state.", "path", FstabPath, "error", err)
		return false, fmt.Errorf("failed to read fstab at %s: %w", FstabPath, err)
	}
	fstabContent := string(fstabContentBytes)
	scanner := bufio.NewScanner(strings.NewReader(fstabContent))
	hasActiveSwapEntryInFstab := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "#") && strings.Contains(line, "swap") {
			// Further check if it's a valid swap entry, e.g., by checking fields.
			// For simplicity, contains "swap" and not commented out is sufficient here.
			fields := strings.Fields(line)
			if len(fields) >= 3 && fields[2] == "swap" {
				logger.Debug("Found active swap entry in fstab.", "line", line)
				hasActiveSwapEntryInFstab = true
				break
			}
		}
	}
	return hasActiveSwapEntryInFstab, nil
}


func (s *ManageSwapStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	if host == nil { return false, fmt.Errorf("host is nil in Precheck for %s", s.GetName())}

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runtimeSwapOn, _, runtimeCheckErr := s.isSwapOnRuntime(ctx, host, conn)
	if runtimeCheckErr != nil {
		logger.Error("Error checking runtime swap status during precheck.", "error", runtimeCheckErr)
		return false, runtimeCheckErr // If we can't check, assume action is needed.
	}

	fstabHasActiveSwap, fstabCheckErr := s.readFstabAndCheckSwapEntries(ctx, conn)
	if fstabCheckErr != nil {
		logger.Error("Error checking fstab for swap entries during precheck.", "error", fstabCheckErr)
		return false, fstabCheckErr // If we can't check, assume action is needed.
	}

	if s.DesiredState == StateAbsent { // "disabled"
		if !runtimeSwapOn && !fstabHasActiveSwap {
			logger.Info("Swap is already disabled (runtime and fstab).")
			return true, nil
		}
		logger.Info("Swap needs to be disabled.", "runtimeOn", runtimeSwapOn, "fstabHasActive", fstabHasActiveSwap)
		return false, nil
	} else if s.DesiredState == StatePresent { // "enabled"
		if runtimeSwapOn && fstabHasActiveSwap {
			logger.Info("Swap is already enabled (runtime and fstab).")
			return true, nil
		}
		// If fstab has active swap but runtime is off, 'swapon -a' in Run will handle it.
		// If runtime is on but fstab is not (e.g. manual swapon), Run should fix fstab.
		// If both are off, Run enables both.
		logger.Info("Swap needs to be enabled or fstab corrected.", "runtimeOn", runtimeSwapOn, "fstabHasActive", fstabHasActiveSwap)
		return false, nil
	}
	return false, fmt.Errorf("unknown DesiredState: %s for step %s", s.DesiredState, s.GetName())
}

func (s *ManageSwapStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)
    if host == nil { return fmt.Errorf("host is nil in Run for %s", s.GetName())}

	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), errConn)
	}
	execOptsSudo := &connector.ExecOptions{Sudo: s.Sudo}

	if s.DesiredState == StateAbsent { // "disabled"
		logger.Info("Attempting to turn off active swap with 'swapoff -a'.")
		_, stderrSwapoff, errSwapoff := conn.Exec(ctx.GoContext(), "swapoff -a", execOptsSudo)
		if errSwapoff != nil {
			logger.Warn("Command 'swapoff -a' finished with an error. This might be okay if no swap was on.", "stderr", string(stderrSwapoff), "error", errSwapoff)
		} else {
			logger.Info("'swapoff -a' completed successfully.")
		}

		// Comment out swap entries in fstab
		s.fstabBackupPath = fmt.Sprintf("%s.bak-kubexms-swap-%d", FstabPath, time.Now().UnixNano())
		logger.Info("Backing up fstab.", "source", FstabPath, "backup", s.fstabBackupPath)
		backupCmd := fmt.Sprintf("cp -f %s %s", FstabPath, s.fstabBackupPath)
		if _, _, err := conn.Exec(ctx.GoContext(), backupCmd, execOptsSudo); err != nil {
			s.fstabBackupPath = "" // Clear on backup failure
			return fmt.Errorf("failed to backup %s: %w", FstabPath, err)
		}

		logger.Info("Commenting out swap entries in fstab.", "fstab", FstabPath)
		// Add marker only if not already present to avoid double marking on re-runs
		sedCmd := fmt.Sprintf("sed -E -i.prev -e '/^[^#].*\\bswap\\b/s/^/%s /' %s", SwapCommentMarker, FstabPath)
		_, stderrSed, errSed := conn.Exec(ctx.GoContext(), sedCmd, execOptsSudo)
		if errSed != nil { // Attempt to restore backup if sed fails
			logger.Error("Failed to comment out swap entries in fstab. Attempting to restore backup.", "error", errSed, "stderr", string(stderrSed))
			restoreCmd := fmt.Sprintf("mv -f %s.prev %s", FstabPath, FstabPath) // Use .prev created by sed -i.prev
			if _, _, errRestore := conn.Exec(ctx.GoContext(), restoreCmd, execOptsSudo); errRestore != nil {
				logger.Error("CRITICAL: Failed to restore fstab from .prev backup after sed failure.", "error", errRestore)
			}
			return fmt.Errorf("failed to comment out swap entries in %s: %w", FstabPath, errSed)
		}
		logger.Info("Swap entries in fstab commented out.")

	} else if s.DesiredState == StatePresent { // "enabled"
		logger.Info("Attempting to uncomment KubeXMS-managed swap entries in fstab.", "fstab", FstabPath)
		// Uncomment lines marked by this tool
		sedCmd := fmt.Sprintf("sed -E -i.prev 's/^%s *//' %s", SwapCommentMarker, FstabPath)
		_, stderrSed, errSed := conn.Exec(ctx.GoContext(), sedCmd, execOptsSudo)
		if errSed != nil {
			logger.Warn("Failed to uncomment swap entries in fstab (command may have no effect if no marked lines).", "error", errSed, "stderr", string(stderrSed))
			// Not necessarily fatal, swapon -a might still work if entries are already fine
		} else {
			logger.Info("KubeXMS-managed swap entries in fstab uncommented (if any).")
		}

		logger.Info("Attempting to turn on swap with 'swapon -a'.")
		_, stderrSwapon, errSwapon := conn.Exec(ctx.GoContext(), "swapon -a", execOptsSudo)
		if errSwapon != nil {
			// This can fail if no swap partitions are defined or available.
			logger.Error("Command 'swapon -a' failed. Ensure swap partitions are correctly defined in fstab and available.", "stderr", string(stderrSwapon), "error", errSwapon)
			return fmt.Errorf("failed to execute 'swapon -a': %w", errSwapon)
		}
		logger.Info("'swapon -a' completed successfully.")
	} else {
		return fmt.Errorf("unknown DesiredState: %s for step %s", s.DesiredState, s.GetName())
	}
	return nil
}


func (s *ManageSwapStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
    if host == nil { return fmt.Errorf("host is nil in Rollback for %s", s.GetName())}
	s.populateDefaults(logger) // Ensure Sudo is set

	if s.fstabBackupPath != "" { // This implies original action was likely "disabled"
		logger.Info("Attempting to restore fstab from backup.", "backup", s.fstabBackupPath, "fstab", FstabPath)
		conn, errConn := ctx.GetConnectorForHost(host)
		if errConn != nil {
			logger.Error("Failed to get connector for host during rollback, cannot restore fstab.", "error", errConn)
			return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), errConn)
		}
		execOptsSudo := &connector.ExecOptions{Sudo: s.Sudo}
		restoreCmd := fmt.Sprintf("mv -f %s %s", s.fstabBackupPath, FstabPath)
		_, stderrRestore, errRestore := conn.Exec(ctx.GoContext(), restoreCmd, execOptsSudo)
		if errRestore != nil {
			logger.Error("Failed to restore fstab from backup during rollback. Manual intervention may be required.", "stderr", string(stderrRestore), "error", errRestore)
		} else {
			logger.Info("Successfully restored fstab from backup.")
			logger.Info("Attempting 'swapon -a' to re-enable swaps from restored fstab.")
			if _, _, errSwapon := conn.Exec(ctx.GoContext(), "swapon -a", execOptsSudo); errSwapon != nil {
				logger.Warn("`swapon -a` after fstab restore failed.", "error", errSwapon)
			}
		}
	} else if s.DesiredState == StatePresent { // Original action was "enabled"
		// Rollback for "enabled" could be to disable swap. This is a destructive action.
		// For now, just log. A more specific step might be needed for explicit disable on rollback.
		logger.Info("Original action was to enable swap. Rollback will run 'swapoff -a'. Fstab changes (if any made by this tool) are not reverted by this simple rollback.")
		conn, errConn := ctx.GetConnectorForHost(host)
		if errConn != nil {
			logger.Error("Failed to get connector for host during rollback.", "error", errConn)
			return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), errConn)
		}
		execOptsSudo := &connector.ExecOptions{Sudo: s.Sudo}
		if _, _, errSwapoff := conn.Exec(ctx.GoContext(), "swapoff -a", execOptsSudo); errSwapoff != nil {
			logger.Warn("`swapoff -a` during rollback of enable action failed.", "error", errSwapoff)
		}

	} else {
		logger.Info("No specific rollback action taken for current DesiredState or lack of backup.", "desiredState", s.DesiredState)
	}
	return nil
}

// Ensure ManageSwapStepSpec implements the step.Step interface.
var _ step.Step = (*ManageSwapStepSpec)(nil)
	}

    fstabPath := "/etc/fstab"
	logger.Info("Attempting to restore fstab from backup.", "backup", s.fstabBackupPath, "fstab", fstabPath)
	restoreCmd := fmt.Sprintf("mv -f %s %s", s.fstabBackupPath, fstabPath) // Use mv to restore
	_, stderrRestore, errRestore := conn.Exec(ctx.GoContext(), restoreCmd, &connector.ExecOptions{Sudo: true})
	if errRestore != nil {
		logger.Error("Failed to restore fstab from backup during rollback. Manual intervention may be required.", "backup", s.fstabBackupPath, "fstab", fstabPath, "stderr", string(stderrRestore), "error", errRestore)
		// Don't return error, rollback is best-effort
	} else {
		logger.Info("Successfully restored fstab from backup.", "fstab", fstabPath)
		// After restoring fstab, it might be desirable to run 'swapon -a' to re-enable swaps defined there.
		// However, this depends on policy - should rollback fully revert or just undo this step's specific file change?
		// For now, just restoring the file.
		logger.Info("Attempting 'swapon -a' to re-enable swaps from restored fstab.")
		_, stderrSwapon, errSwapon := conn.Exec(ctx.GoContext(), "swapon -a", &connector.ExecOptions{Sudo: true})
		if errSwapon != nil {
			logger.Warn("`swapon -a` after fstab restore failed. Swaps may need to be manually re-enabled.", "stderr", string(stderrSwapon), "error", errSwapon)
		} else {
			logger.Info("`swapon -a` executed after fstab restore.")
		}
	}
	return nil
}

// Ensure DisableSwapStepSpec implements the step.Step interface.
var _ step.Step = (*DisableSwapStepSpec)(nil)
