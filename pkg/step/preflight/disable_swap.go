package preflight

import (
	"errors"
	"fmt"
	"strings"
	"time" // For backup filename timestamp

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/step"
	// spec is no longer needed
)

// DisableSwapStep disables swap on the target host.
type DisableSwapStep struct {
	StepName string
	// Internal field to store backup path for rollback
	fstabBackupPath string
}

// NewDisableSwapStep creates a new DisableSwapStep.
func NewDisableSwapStep(stepName string) step.Step {
	name := stepName
	if name == "" {
		name = "Disable Swap"
	}
	return &DisableSwapStep{
		StepName: name,
	}
}

func (s *DisableSwapStep) isSwapOn(ctx runtime.StepContext, host connector.Host) (bool, string, error) {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "operation", "isSwapOnCheck")

	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
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


func (s *DisableSwapStep) Name() string {
	return s.StepName
}

func (s *DisableSwapStep) Description() string {
	return "Disables swap memory on the host by running 'swapoff -a' and commenting swap entries in /etc/fstab."
}

func (s *DisableSwapStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Precheck")

	if host == nil { return false, fmt.Errorf("host is nil in Precheck for %s", s.Name())}

	swapOn, _, checkErr := s.isSwapOn(ctx, host)
	if checkErr != nil {
		logger.Error("Error checking swap status during precheck.", "error", checkErr)
		// Don't fail precheck for check error, let Run attempt to disable.
		// But if it's a critical error (e.g., cannot connect), it should be returned.
		// For now, assume Run should proceed if check itself fails.
		return false, nil
	}
	if !swapOn {
		logger.Info("Swap is already disabled.")
		return true, nil
	}
	logger.Info("Swap is currently enabled.")
	return false, nil
}

func (s *DisableSwapStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Run")
    if host == nil { return fmt.Errorf("host is nil in Run for %s", s.Name())}

	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), errConn)
	}

	execOptsSudo := &connector.ExecOptions{Sudo: true}

	logger.Info("Attempting to turn off active swap with 'swapoff -a'.")
	_, stderrSwapoff, errSwapoff := conn.Exec(ctx.GoContext(), "swapoff -a", execOptsSudo)
	if errSwapoff != nil {
		// 'swapoff -a' can fail if no swap is active, which is not an error for our goal.
		// However, it can also fail for permission issues.
		logger.Warn("Command 'swapoff -a' finished with an error. This might be okay if no swap was on, or it could be a permission issue.", "stderr", string(stderrSwapoff), "error", errSwapoff)
		// We proceed to fstab modification regardless, as that's the persistent part.
	} else {
		logger.Info("'swapoff -a' completed successfully.")
	}

	fstabPath := "/etc/fstab"
	// Store backup path in the struct field for potential rollback
	s.fstabBackupPath = fmt.Sprintf("%s.bak-kubexms-%d", fstabPath, time.Now().UnixNano())

	logger.Info("Backing up fstab.", "source", fstabPath, "backup", s.fstabBackupPath)
	backupCmd := fmt.Sprintf("cp -f %s %s", fstabPath, s.fstabBackupPath)
	_, stderrBackup, errBackup := conn.Exec(ctx.GoContext(), backupCmd, execOptsSudo)
	if errBackup != nil {
		s.fstabBackupPath = "" // Clear backup path if backup failed, so rollback doesn't use a potentially partial/failed backup
		return fmt.Errorf("failed to backup %s to %s (stderr: %s): %w", fstabPath, s.fstabBackupPath, string(stderrBackup), errBackup)
	}

	logger.Info("Attempting to comment out swap entries in fstab.", "fstab", fstabPath)
	// Using sed -i without suffix to modify in place after successful backup.
	sedCmd := fmt.Sprintf("sed -E -i '/^[^#].*\\bswap\\b/s/^/#/' %s", fstabPath)
	_, stderrSed, errSed := conn.Exec(ctx.GoContext(), sedCmd, execOptsSudo)
	if errSed != nil {
	    logger.Error("Failed to comment out swap entries in fstab using sed. Attempting to restore from backup.", "fstab", fstabPath, "stderr", string(stderrSed), "error", errSed)
	    restoreCmd := fmt.Sprintf("mv -f %s %s", s.fstabBackupPath, fstabPath) // Use mv to restore
	    _, stderrRestore, errRestore := conn.Exec(ctx.GoContext(), restoreCmd, execOptsSudo)
	    if errRestore != nil {
	        logger.Error("CRITICAL: Failed to restore fstab after sed failure. Manual intervention may be required on host.", "backup", s.fstabBackupPath, "fstab", fstabPath, "restoreStderr", string(stderrRestore), "restoreError", errRestore)
	    } else {
	        logger.Info("Successfully restored fstab from backup after sed failure.", "backup", s.fstabBackupPath, "fstab", fstabPath)
	    }
		return fmt.Errorf("failed to comment out swap entries in %s using sed (stderr: %s): %w", fstabPath, string(stderrSed), errSed)
	}
	logger.Info("Swap entries in fstab commented out.", "fstab", fstabPath)

	// Final verification
	swapOn, finalState, verifyErr := s.isSwapOn(ctx, host)
	if verifyErr != nil {
		// Log this as a warning because the primary actions (swapoff, fstab edit) might have succeeded.
		logger.Warn("Failed to verify swap status after attempting disable. Manual check recommended.", "lastSwapOutput", finalState, "error", verifyErr)
		return nil // Don't fail the step if modification was likely successful but verification failed
	}
	if swapOn {
		return fmt.Errorf("failed to disable swap for step %s on host %s. 'swapon --summary' still shows active swap: %s", s.Name(), host.GetName(), finalState)
	}
	logger.Info("Swap successfully disabled and verified.")
	return nil
}

func (s *DisableSwapStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.Name(), "host", host.GetName(), "phase", "Rollback")
    if host == nil { return fmt.Errorf("host is nil in Rollback for %s", s.Name())}

	if s.fstabBackupPath == "" {
		logger.Warn("No fstab backup path recorded, cannot restore fstab. Skipping fstab restoration.")
		// We could try `swapon -a` if fstab was our only change, but if sed failed, fstab might be original.
		// If Run completed, fstab was modified. If Run failed before fstab mod, backup path might be empty.
		return nil
	}

	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		// If no connector, can't do much.
		logger.Error("Failed to get connector for host during rollback, cannot restore fstab.", "error", errConn)
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), errConn)
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

// Ensure DisableSwapStep implements the step.Step interface.
var _ step.Step = (*DisableSwapStep)(nil)
