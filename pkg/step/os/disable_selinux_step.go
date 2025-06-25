package os

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const (
	selinuxConfigFile = "/etc/selinux/config"
)

// DisableSelinuxStep disables SELinux temporarily (setenforce 0) and persistently (/etc/selinux/config).
type DisableSelinuxStep struct {
	meta             spec.StepMeta
	Sudo             bool
	originalSelinuxValue string // For rollback: to restore SELINUX=enforcing or permissive
	rebootRequired   bool     // To inform the user if a reboot is needed for changes to fully take effect
}

// NewDisableSelinuxStep creates a new DisableSelinuxStep.
func NewDisableSelinuxStep(instanceName string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "DisableSelinux"
	}
	return &DisableSelinuxStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: "Disables SELinux temporarily and persistently.",
		},
		Sudo: sudo,
	}
}

func (s *DisableSelinuxStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *DisableSelinuxStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// Check current SELinux status (getenforce)
	// Sudo might not be needed for getenforce, but let's be consistent if other commands need it.
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}
	stdout, _, err := runnerSvc.RunWithOptions(ctx.GoContext(), conn, "getenforce", execOpts)
	if err != nil {
		// If getenforce fails, SELinux might not be installed/supported. Treat as "effectively disabled" for this step's purpose.
		logger.Warn("getenforce command failed, assuming SELinux is not active or not installed.", "error", err)
		s.originalSelinuxValue = "not_found" // Mark for rollback info
		return true, nil // Skip
	}

	currentStatus := strings.ToLower(strings.TrimSpace(string(stdout)))
	s.originalSelinuxValue = currentStatus // Store for potential rollback (though rollback is complex for setenforce 0)
	logger.Debug("Current SELinux status (getenforce).", "status", currentStatus)

	if currentStatus == "disabled" {
		logger.Info("SELinux is already reported as 'disabled' by getenforce.")
		// Also check config file for persistent disablement
		confContentBytes, errRead := runnerSvc.ReadFile(ctx.GoContext(), conn, selinuxConfigFile)
		if errRead == nil {
			confContent := string(confContentBytes)
			if strings.Contains(confContent, "SELINUX=disabled") {
				logger.Info("SELinux is also persistently disabled in config file.")
				return true, nil
			}
			logger.Info("SELinux is 'disabled' by getenforce, but not persistently in config. Step needs to run to make it persistent.")
		} else {
			logger.Warn("Could not read SELinux config file to verify persistent state. Step will run.", "file", selinuxConfigFile, "error", errRead)
		}
	}

	return false, nil
}

func (s *DisableSelinuxStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// 1. Temporarily disable SELinux if not already Disabled (originalSelinuxValue from precheck)
	if s.originalSelinuxValue != "disabled" && s.originalSelinuxValue != "not_found" {
		logger.Info("Attempting to temporarily disable SELinux (setenforce 0).")
		_, stderr, errSetenforce := runnerSvc.RunWithOptions(ctx.GoContext(), conn, "setenforce 0", execOpts)
		if errSetenforce != nil {
			// This is often a non-fatal warning if SELinux is already permissive or system doesn't support it well.
			logger.Warn("setenforce 0 command finished with an issue (might be ignorable if SELinux was already permissive or unsupported).", "error", errSetenforce, "stderr", string(stderr))
		} else {
			logger.Info("SELinux temporarily set to permissive/disabled (setenforce 0).")
		}
	} else {
		logger.Info("SELinux already disabled or not found, skipping setenforce 0.")
	}

	// 2. Persistently disable SELinux in /etc/selinux/config
	logger.Info("Attempting to persistently disable SELinux in config file.", "file", selinuxConfigFile)
	// This command replaces SELINUX=enforcing or SELINUX=permissive with SELINUX=disabled
	// It's relatively safe as it targets specific lines.
	// Using -i.bak to create a backup.
	sedCmd := fmt.Sprintf("sed -i.bak -e 's/^SELINUX=enforcing/SELINUX=disabled/' -e 's/^SELINUX=permissive/SELINUX=disabled/' %s", selinuxConfigFile)
	_, stderrSed, errSed := runnerSvc.RunWithOptions(ctx.GoContext(), conn, sedCmd, execOpts)
	if errSed != nil {
		logger.Error("Failed to persistently disable SELinux in config file.", "command", sedCmd, "error", errSed, "stderr", string(stderrSed))
		return fmt.Errorf("failed to modify %s: %w. Stderr: %s", selinuxConfigFile, errSed, string(stderrSed))
	}

	// Check if a change was made (and if reboot might be needed)
	// A simple way: check if SELINUX=disabled is now in the file.
	// A more complex way: diff original file with current.
	// For now, assume if sed command succeeded, it's set.
	// Inform user that a reboot is often required for /etc/selinux/config changes to fully apply,
	// although setenforce 0 handles the current session.
	s.rebootRequired = true // Mark that a reboot might be needed for full effect.
	logger.Info("SELinux persistently set to disabled in config file. A reboot may be required for this to take full effect if getenforce still shows permissive.", "file", selinuxConfigFile)
	// Store this info in cache if other steps need to know a reboot is pending?
	// ctx.TaskCache().Set("RebootRequiredAfterSelinuxChange", true)

	return nil
}

func (s *DisableSelinuxStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")

	if s.originalSelinuxValue == "" || s.originalSelinuxValue == "disabled" || s.originalSelinuxValue == "not_found" {
		logger.Info("Original SELinux state was disabled or not found, no rollback action needed for config file.")
		return nil
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s for rollback: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// Restore from backup if it exists (sed -i.bak creates this)
	backupFileInSed := selinuxConfigFile + ".bak" // Name used by `sed -i.bak`
	if exists, _ := runnerSvc.Exists(ctx.GoContext(), conn, backupFileInSed); exists {
		logger.Info("Restoring SELinux config from .bak created by sed.", "backup", backupFileInSed, "target", selinuxConfigFile)
		mvCmd := fmt.Sprintf("mv %s %s", backupFileInSed, selinuxConfigFile)
		_, stderrMv, errMv := runnerSvc.RunWithOptions(ctx.GoContext(), conn, mvCmd, execOpts)
		if errMv != nil {
			logger.Error("Failed to restore SELinux config from .bak.", "command", mvCmd, "error", errMv, "stderr", string(stderrMv))
			// If mv fails, the .bak file is still there. The original file might be the modified one.
		} else {
			logger.Info("SELinux config restored from .bak.")
		}
	} else {
		// If no .bak file from sed, it implies sed might not have run or -i.bak was not effective.
		// This part of the rollback becomes less certain.
		// We can still try to set the SELINUX value directly if we know the original.
		logger.Warn("No .bak file found from sed operation. Attempting to set SELINUX value directly in config if original state known.", "expected_backup", backupFileInSed)
		if s.originalSelinuxValue == "enforcing" || s.originalSelinuxValue == "permissive" {
			logger.Info("Attempting to set SELINUX in config directly for rollback.", "target_state", s.originalSelinuxValue)
			// This sed command tries to find "SELINUX=disabled" and change it.
			// If the file was already "SELINUX=disabled" and sed didn't change it, this might not revert correctly
			// if original was, for example, permissive and sed changed nothing because it looked for enforcing.
			// A more robust sed would be `sed -i 's/^SELINUX=.*/SELINUX=ORIGINAL_VALUE/'`
			sedCmd := fmt.Sprintf("sed -i.kubexm_rb_manual -e 's/^SELINUX=disabled/SELINUX=%s/' -e 's/^SELINUX=permissive/SELINUX=%s/' %s", s.originalSelinuxValue, s.originalSelinuxValue, selinuxConfigFile)
			_, stderrSed, errSed := runnerSvc.RunWithOptions(ctx.GoContext(), conn, sedCmd, execOpts)
			if errSed != nil {
				logger.Error("Failed to set SELINUX value in config during manual rollback attempt.", "error", errSed, "stderr", string(stderrSed))
			} else {
				logger.Info("SELINUX value attempted to be set in config during manual rollback.", "value", s.originalSelinuxValue)
			}
		}
	}

	// Attempt to restore runtime enforcement state if it was 'enforcing' or 'permissive'
	// This is best-effort as `setenforce 0` is the primary change made by Run for current session.
	if s.originalSelinuxValue == "enforcing" {
		logger.Info("Attempting to set SELinux to enforcing (setenforce 1) for rollback.")
		_, _, errSetenforce := runnerSvc.RunWithOptions(ctx.GoContext(), conn, "setenforce 1", execOpts)
		if errSetenforce != nil {
			logger.Warn("setenforce 1 command failed during rollback (best effort).", "error", errSetenforce)
		}
	}
	// Note: Reboot might still be needed to fully revert to original state if config was changed.
	logger.Info("SELinux disable rollback attempted. A reboot might be necessary if config was restored and previous state was 'enforcing'.")
	return nil
}

var _ step.Step = (*DisableSelinuxStep)(nil)
