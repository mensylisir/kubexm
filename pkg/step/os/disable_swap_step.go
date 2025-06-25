package os

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	fstabPath = "/etc/fstab"
)

// DisableSwapStep disables swap temporarily (swapoff -a) and persistently (comments out swap in /etc/fstab).
type DisableSwapStep struct {
	meta             spec.StepMeta
	Sudo             bool
	fstabBackupPath  string // Path to the backup of fstab, created during Run
}

// NewDisableSwapStep creates a new DisableSwapStep.
func NewDisableSwapStep(instanceName string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "DisableSwap"
	}
	return &DisableSwapStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: "Disables swap temporarily and persistently.",
		},
		Sudo: sudo,
	}
}

func (s *DisableSwapStep) Meta() *spec.StepMeta {
	return &s.meta
}

// isSwapLine checks if a line from fstab is an active swap entry.
func isSwapLine(line string) bool {
	line = strings.TrimSpace(line)
	if strings.HasPrefix(line, "#") || line == "" {
		return false
	}
	fields := strings.Fields(line)
	// <file system> <mount point> <type> <options> <dump> <pass>
	// For swap, <mount point> is "none" and <type> is "swap".
	return len(fields) >= 3 && fields[1] == "none" && fields[2] == "swap"
}

func (s *DisableSwapStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// 1. Check current swap status (swapon --show or cat /proc/swaps)
	// `swapon --show` is cleaner. If it outputs nothing, swap is off.
	// Sudo might be needed for `swapon -s` on some systems.
	stdoutSwapOn, _, errSwapOn := runnerSvc.RunWithOptions(ctx.GoContext(), conn, "swapon --show", execOpts)
	if errSwapOn != nil {
		// If swapon --show fails, it's hard to tell. Fallback to /proc/swaps.
		// Some systems might not have `swapon --show` or it might behave differently.
		// `cat /proc/swaps` is more universal for checking active swap.
		// Header line + data lines. If only header, swap is off.
		procSwapsContent, _, errProcSwaps := runnerSvc.RunWithOptions(ctx.GoContext(), conn, "cat /proc/swaps", &connector.ExecOptions{Sudo: false}) // cat /proc/swaps usually doesn't need sudo
		if errProcSwaps != nil {
			logger.Warn("Both 'swapon --show' and 'cat /proc/swaps' failed. Assuming swap status check inconclusive, step will run.", "swapon_error", errSwapOn, "proc_swaps_error", errProcSwaps)
			return false, nil
		}
		lines := strings.Split(strings.TrimSpace(string(procSwapsContent)), "\n")
		if len(lines) <= 1 { // Only header line or empty
			logger.Info("No active swap found via /proc/swaps.")
			// Now check fstab for persistent entries
		} else {
			logger.Info("Active swap detected via /proc/swaps. DisableSwapStep needs to run.")
			return false, nil // Active swap found
		}
	} else if strings.TrimSpace(string(stdoutSwapOn)) == "" {
		logger.Info("No active swap found via 'swapon --show'.")
		// Now check fstab for persistent entries
	} else {
		logger.Info("Active swap detected via 'swapon --show'. DisableSwapStep needs to run.", "output", string(stdoutSwapOn))
		return false, nil // Active swap found
	}

	// 2. Check /etc/fstab for persistent swap entries
	fstabContentBytes, errFstab := runnerSvc.ReadFile(ctx.GoContext(), conn, fstabPath)
	if errFstab != nil {
		logger.Warn("Failed to read /etc/fstab for precheck. Step will run to ensure desired state.", "error", errFstab)
		return false, nil
	}
	fstabLines := strings.Split(string(fstabContentBytes), "\n")
	for _, line := range fstabLines {
		if isSwapLine(line) {
			logger.Info("Persistent swap entry found in /etc/fstab. DisableSwapStep needs to run.", "line", line)
			return false, nil // Found an active swap line
		}
	}

	logger.Info("No active swap detected and no persistent swap entries found in /etc/fstab. Swap is disabled.")
	return true, nil
}

func (s *DisableSwapStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// 1. Temporarily disable all swap: swapoff -a
	logger.Info("Attempting to disable all active swap (swapoff -a).")
	_, stderrSwapoff, errSwapoff := runnerSvc.RunWithOptions(ctx.GoContext(), conn, "swapoff -a", execOpts)
	if errSwapoff != nil {
		// swapoff -a might return error if no swap is on, which is fine.
		// Check stderr for common messages like "swapoff: /path/to/swapfile: swapoff failed: Invalid argument" if no swap
		// Or "swapoff: not found" if no swap devices. This is not a failure of the step's goal.
		// A more robust check would be to see if `swapon --show` is empty AFTER this.
		logger.Warn("swapoff -a command finished with an issue (this might be ignorable if no swap was active).", "error", errSwapoff, "stderr", string(stderrSwapoff))
	} else {
		logger.Info("All active swap disabled (swapoff -a).")
	}

	// 2. Persistently disable swap in /etc/fstab
	// Create a backup of fstab first
	s.fstabBackupPath = fstabPath + ".kubexm_bak_" + fmt.Sprintf("%d", ctx.GoContext().Value("pipelineStartTime")) // Include a timestamp or unique ID if multiple runs

	backupCmd := fmt.Sprintf("cp %s %s", fstabPath, s.fstabBackupPath)
	logger.Info("Backing up /etc/fstab.", "to", s.fstabBackupPath)
	_, stderrBackup, errBackup := runnerSvc.RunWithOptions(ctx.GoContext(), conn, backupCmd, execOpts)
	if errBackup != nil {
		logger.Error("Failed to backup /etc/fstab. Aborting persistent swap disable.", "error", errBackup, "stderr", string(stderrBackup))
		return fmt.Errorf("failed to backup %s: %w. Stderr: %s", fstabPath, errBackup, string(stderrBackup))
	}

	logger.Info("Commenting out swap entries in /etc/fstab.")
	// This sed command finds lines with 'swap' as the type and comments them out.
	// It's safer than deleting lines.
	// Regex: match lines that are not comments, contain 'swap' as a word, and typically 'none' as mount point.
	// sed -i.bak -E '/^[[:space:]]*[^#].*\<swap\>/s/^/#/' /etc/fstab
	// A simpler sed: find lines with " swap " and comment them.
	// Using a more careful sed: find lines that are not comments, have "none" as field 2 and "swap" as field 3
	sedCmd := fmt.Sprintf("sed -i -E '/^[[:space:]]*[^#][^[:space:]]+[[:space:]]+none[[:space:]]+swap[[:space:]]+/s/^/#/' %s", fstabPath)

	_, stderrSed, errSed := runnerSvc.RunWithOptions(ctx.GoContext(), conn, sedCmd, execOpts)
	if errSed != nil {
		logger.Error("Failed to comment out swap entries in /etc/fstab.", "command", sedCmd, "error", errSed, "stderr", string(stderrSed))
		// Attempt to restore fstab from backup if sed fails
		restoreCmd := fmt.Sprintf("mv %s %s", s.fstabBackupPath, fstabPath)
		runnerSvc.RunWithOptions(ctx.GoContext(), conn, restoreCmd, execOpts) // Best effort restore
		return fmt.Errorf("failed to modify %s to disable swap: %w. Stderr: %s", fstabPath, errSed, string(stderrSed))
	}

	logger.Info("Swap entries commented out in /etc/fstab. A reboot may be required for changes to fully take effect if swap was in use by certain processes.")
	return nil
}

func (s *DisableSwapStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// 1. Restore /etc/fstab from backup
	if s.fstabBackupPath != "" {
		if exists, _ := runnerSvc.Exists(ctx.GoContext(), conn, s.fstabBackupPath); exists {
			logger.Info("Attempting to restore /etc/fstab from backup.", "from", s.fstabBackupPath)
			restoreCmd := fmt.Sprintf("mv %s %s", s.fstabBackupPath, fstabPath)
			_, stderrRestore, errRestore := runnerSvc.RunWithOptions(ctx.GoContext(), conn, restoreCmd, execOpts)
			if errRestore != nil {
				logger.Error("Failed to restore /etc/fstab from backup during rollback.", "error", errRestore, "stderr", string(stderrRestore))
				// This is a significant issue for rollback.
			} else {
				logger.Info("/etc/fstab restored from backup.")
			}
		} else {
			logger.Warn("fstab backup file not found, cannot restore.", "path", s.fstabBackupPath)
		}
	} else {
		logger.Warn("fstab backup path not recorded, cannot restore /etc/fstab.")
	}

	// 2. Attempt to turn swap back on (swapon -a)
	// This will only work if fstab was successfully restored and contained valid swap entries.
	logger.Info("Attempting to re-enable swap (swapon -a) based on (restored) /etc/fstab.")
	_, stderrSwapon, errSwapon := runnerSvc.RunWithOptions(ctx.GoContext(), conn, "swapon -a", execOpts)
	if errSwapon != nil {
		logger.Warn("swapon -a command failed during rollback (this might be expected if fstab was not restored or had no swap).", "error", errSwapon, "stderr", string(stderrSwapon))
	} else {
		logger.Info("Swap re-enabled via swapon -a.")
	}
	// Note: A reboot might be needed for the system to fully recognize restored swap configurations.
	return nil // Best effort for rollback
}

var _ step.Step = (*DisableSwapStep)(nil)
