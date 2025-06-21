package preflight

import (
	"errors"
	"fmt"
	"strings"
	"time" // For backup filename timestamp

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec" // Added for StepMeta
	"github.com/mensylisir/kubexm/pkg/step"
)

// DisableSwapStep disables swap on the target host.
type DisableSwapStep struct {
	meta spec.StepMeta
	Sudo bool // Sudo for system commands
	// Internal field to store backup path for rollback
	fstabBackupPath string
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
			Description: "Disables swap memory on the host by running 'swapoff -a' and commenting swap entries in /etc/fstab.",
		},
		Sudo: sudo,
	}
}

// Meta returns the step's metadata.
func (s *DisableSwapStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *DisableSwapStep) isSwapOn(ctx runtime.StepContext, host connector.Host) (bool, string, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "operation", "isSwapOnCheck")

	runnerSvc := ctx.GetRunner()
	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		return false, "", fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), errConn)
	}

	// Try 'swapon --summary --noheadings' first
	// Using Sudo:s.Sudo for consistency, though summary might not always need it.
	execOpts := &connector.ExecOptions{Sudo: s.Sudo, Check: true} // Check:true allows non-zero exit for parsing
	stdoutBytes, stderrBytes, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, "swapon --summary --noheadings", execOpts)

	if err != nil {
		var cmdErr *connector.CommandError
		// If --noheadings is invalid (older swapon), try without it
		if errors.As(err, &cmdErr) && (strings.Contains(strings.ToLower(string(stderrBytes)), "invalid option") || strings.Contains(strings.ToLower(string(stderrBytes)), "bad usage")) {
			logger.Debug("`swapon --summary --noheadings` failed, trying `swapon --summary`.", "stderr", string(stderrBytes))
			stdoutBytes, stderrBytes, err = runnerSvc.RunWithOptions(ctx.GoContext(), conn, "swapon --summary", execOpts)
		}
	}

	if err != nil {
		var cmdErr *connector.CommandError
		isCmdNotFoundErr := errors.As(err, &cmdErr) && cmdErr.ExitCode == 127

		facts, _ := ctx.GetHostFacts(host) // Best effort to get facts for OS ID
		osID := "unknown"
		if facts != nil && facts.OS != nil && facts.OS.ID != "" {
			osID = strings.ToLower(facts.OS.ID)
		}

		if osID == "linux" {
			logger.Warn("`swapon --summary` command failed, attempting to read /proc/swaps.", "error", err, "stderr", string(stderrBytes))
			// runnerSvc.ReadFile does not take sudo option, /proc/swaps is generally readable.
			procSwapsContentBytes, readErr := runnerSvc.ReadFile(ctx.GoContext(), conn, "/proc/swaps")
			if readErr != nil {
				return false, "", fmt.Errorf("failed to run 'swapon --summary' and also failed to read /proc/swaps on host %s: %w", host.GetName(), readErr)
			}
			content := strings.TrimSpace(string(procSwapsContentBytes))
			lines := strings.Split(content, "\n")
			return len(lines) > 1, content, nil
		}

		if isCmdNotFoundErr {
			return false, "", fmt.Errorf("`swapon` command not found and OS ('%s') is not Linux with /proc/swaps fallback, cannot determine swap status on host %s", osID, host.GetName())
		}
		return false, "", fmt.Errorf("failed to execute 'swapon --summary' on host %s (stderr: %s): %w", host.GetName(), string(stderrBytes), err)
	}

	trimmedStdout := strings.TrimSpace(string(stdoutBytes))
	if trimmedStdout == "" {
		return false, string(stdoutBytes), nil
	}
	lines := strings.Split(trimmedStdout, "\n")
	if len(lines) == 1 && strings.Contains(lines[0], "Filename") && strings.Contains(lines[0], "Type") && strings.Contains(lines[0], "Size") {
		return false, string(stdoutBytes), nil
	}
	return true, string(stdoutBytes), nil
}

func (s *DisableSwapStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")

	if host == nil {
		return false, fmt.Errorf("host is nil in Precheck for %s", s.meta.Name)
	}

	swapOn, _, checkErr := s.isSwapOn(ctx, host)
	if checkErr != nil {
		logger.Error("Error checking swap status during precheck.", "error", checkErr)
		return false, nil // Let Run attempt.
	}
	if !swapOn {
		logger.Info("Swap is already disabled.")
		return true, nil
	}
	logger.Info("Swap is currently enabled.")
	return false, nil
}

func (s *DisableSwapStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	if host == nil {
		return fmt.Errorf("host is nil in Run for %s", s.meta.Name)
	}

	runnerSvc := ctx.GetRunner()
	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), errConn)
	}

	logger.Info("Attempting to turn off active swap with 'swapoff -a'.")
	// Use runnerSvc.Run with s.Sudo
	if _, errSwapoff := runnerSvc.Run(ctx.GoContext(), conn, "swapoff -a", s.Sudo); errSwapoff != nil {
		logger.Warn("Command 'swapoff -a' finished with an error. This might be okay if no swap was on.", "error", errSwapoff)
	} else {
		logger.Info("'swapoff -a' completed successfully.")
	}

	fstabPath := "/etc/fstab"
	s.fstabBackupPath = fmt.Sprintf("%s.bak-kubexms-%d", fstabPath, time.Now().UnixNano())

	logger.Info("Backing up fstab.", "source", fstabPath, "backup", s.fstabBackupPath)
	backupCmd := fmt.Sprintf("cp -f %s %s", fstabPath, s.fstabBackupPath)
	if _, errBackup := runnerSvc.Run(ctx.GoContext(), conn, backupCmd, s.Sudo); errBackup != nil {
		s.fstabBackupPath = ""
		return fmt.Errorf("failed to backup %s to %s: %w", fstabPath, s.fstabBackupPath, errBackup)
	}

	logger.Info("Attempting to comment out swap entries in fstab.", "fstab", fstabPath)
	sedCmd := fmt.Sprintf("sed -E -i '/^[^#].*\\bswap\\b/s/^/#/' %s", fstabPath)
	if _, errSed := runnerSvc.Run(ctx.GoContext(), conn, sedCmd, s.Sudo); errSed != nil {
		logger.Error("Failed to comment out swap entries in fstab using sed. Attempting to restore from backup.", "fstab", fstabPath, "error", errSed)
		restoreCmd := fmt.Sprintf("mv -f %s %s", s.fstabBackupPath, fstabPath)
		if _, errRestore := runnerSvc.Run(ctx.GoContext(), conn, restoreCmd, s.Sudo); errRestore != nil {
			logger.Error("CRITICAL: Failed to restore fstab after sed failure. Manual intervention may be required on host.", "backup", s.fstabBackupPath, "fstab", fstabPath, "restoreError", errRestore)
		} else {
			logger.Info("Successfully restored fstab from backup after sed failure.", "backup", s.fstabBackupPath, "fstab", fstabPath)
		}
		return fmt.Errorf("failed to comment out swap entries in %s using sed: %w", fstabPath, errSed)
	}
	logger.Info("Swap entries in fstab commented out.", "fstab", fstabPath)

	swapOn, finalState, verifyErr := s.isSwapOn(ctx, host)
	if verifyErr != nil {
		logger.Warn("Failed to verify swap status after attempting disable. Manual check recommended.", "lastSwapOutput", finalState, "error", verifyErr)
		return nil
	}
	if swapOn {
		return fmt.Errorf("failed to disable swap for step %s on host %s. 'swapon --summary' still shows active swap: %s", s.meta.Name, host.GetName(), finalState)
	}
	logger.Info("Swap successfully disabled and verified.")
	return nil
}

func (s *DisableSwapStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	if host == nil {
		return fmt.Errorf("host is nil in Rollback for %s", s.meta.Name)
	}

	if s.fstabBackupPath == "" {
		logger.Warn("No fstab backup path recorded, cannot restore fstab. Skipping fstab restoration.")
		return nil
	}

	runnerSvc := ctx.GetRunner()
	conn, errConn := ctx.GetConnectorForHost(host)
	if errConn != nil {
		logger.Error("Failed to get connector for host during rollback, cannot restore fstab.", "error", errConn)
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), errConn)
	}

	fstabPath := "/etc/fstab"
	logger.Info("Attempting to restore fstab from backup.", "backup", s.fstabBackupPath, "fstab", fstabPath)
	restoreCmd := fmt.Sprintf("mv -f %s %s", s.fstabBackupPath, fstabPath)
	if _, errRestore := runnerSvc.Run(ctx.GoContext(), conn, restoreCmd, s.Sudo); errRestore != nil {
		logger.Error("Failed to restore fstab from backup during rollback. Manual intervention may be required.", "backup", s.fstabBackupPath, "fstab", fstabPath, "error", errRestore)
	} else {
		logger.Info("Successfully restored fstab from backup.", "fstab", fstabPath)
		logger.Info("Attempting 'swapon -a' to re-enable swaps from restored fstab.")
		if _, errSwapon := runnerSvc.Run(ctx.GoContext(), conn, "swapon -a", s.Sudo); errSwapon != nil {
			logger.Warn("`swapon -a` after fstab restore failed. Swaps may need to be manually re-enabled.", "error", errSwapon)
		} else {
			logger.Info("`swapon -a` executed after fstab restore.")
		}
	}
	return nil
}

// Ensure DisableSwapStep implements the step.Step interface.
var _ step.Step = (*DisableSwapStep)(nil)
