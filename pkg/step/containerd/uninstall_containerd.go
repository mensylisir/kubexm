package containerd

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// UninstallContainerdStepSpec defines parameters for uninstalling containerd.
type UninstallContainerdStepSpec struct {
	spec.StepMeta `json:",inline"`

	ServiceName            string   `json:"serviceName,omitempty"`
	SystemdUnitFilePath    string   `json:"systemdUnitFilePath,omitempty"`
	BinariesToRemove       []string `json:"binariesToRemove,omitempty"`
	ConfigFilesToRemove    []string `json:"configFilesToRemove,omitempty"`
	CniConfigFilesToRemove []string `json:"cniConfigFilesToRemove,omitempty"`
	Sudo                   bool     `json:"sudo,omitempty"`
}

// NewUninstallContainerdStepSpec creates a new UninstallContainerdStepSpec.
func NewUninstallContainerdStepSpec(name, description string) *UninstallContainerdStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Uninstall Containerd"
	}
	finalDescription := description
	// Description will be refined in populateDefaults.

	return &UninstallContainerdStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		// Defaults will be set in populateDefaults
	}
}

// Name returns the step's name.
func (s *UninstallContainerdStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *UninstallContainerdStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *UninstallContainerdStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *UninstallContainerdStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *UninstallContainerdStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *UninstallContainerdStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *UninstallContainerdStepSpec) populateDefaults(logger runtime.Logger) {
	if s.ServiceName == "" {
		s.ServiceName = "containerd" // Common service name, might be containerd.service on some systems
		logger.Debug("ServiceName defaulted.", "name", s.ServiceName)
	}
	if s.SystemdUnitFilePath == "" {
		// Common paths for systemd unit files
		// This could be more sophisticated by checking multiple common locations
		s.SystemdUnitFilePath = "/etc/systemd/system/containerd.service"
		logger.Debug("SystemdUnitFilePath defaulted.", "path", s.SystemdUnitFilePath)
	}
	if len(s.BinariesToRemove) == 0 {
		s.BinariesToRemove = []string{
			"/usr/local/bin/containerd",
			"/usr/local/bin/containerd-shim",
			"/usr/local/bin/containerd-shim-runc-v1",
			"/usr/local/bin/containerd-shim-runc-v2",
			"/usr/local/bin/ctr",
			"/usr/local/sbin/runc", // Often here, or /usr/local/bin/runc
			"/usr/bin/containerd", // Some package managers might place it here
			"/usr/bin/ctr",
			"/usr/sbin/runc",
		}
		logger.Debug("BinariesToRemove defaulted.", "binaries", s.BinariesToRemove)
	}
	if len(s.ConfigFilesToRemove) == 0 {
		s.ConfigFilesToRemove = []string{
			"/etc/containerd/config.toml",
			"/etc/containerd", // The whole directory
		}
		logger.Debug("ConfigFilesToRemove defaulted.", "files", s.ConfigFilesToRemove)
	}
	if len(s.CniConfigFilesToRemove) == 0 {
		s.CniConfigFilesToRemove = []string{
			"/etc/cni/net.d/10-containerd-net.conflist", // Example, actual name can vary
			// Consider adding more common CNI config paths if this step aims to be thorough
		}
		logger.Debug("CniConfigFilesToRemove defaulted.", "files", s.CniConfigFilesToRemove)
	}

	// Default Sudo to true as most uninstall operations require it.
	// If the struct field is its zero value (false) and it should be true by default.
	// This ensures if a spec is created with Sudo=false explicitly, it's respected.
	// This check is a bit redundant if factory always sets it, but good for safety.
	if !s.Sudo { // If it's false (zero value or explicitly set)
	    // Check if any paths are privileged to truly decide if sudo is needed.
	    // For uninstall, it's almost always needed.
	    s.Sudo = true
	    logger.Debug("Sudo defaulted to true for uninstall operations.")
	}


	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Uninstalls containerd: stops/disables service '%s', removes specified binaries, configs, and CNI configs.", s.ServiceName)
	}
}

// Precheck determines if containerd seems already uninstalled.
func (s *UninstallContainerdStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// 1. Check service status (is-active, is-enabled)
	isActiveCmd := fmt.Sprintf("systemctl is-active %s", s.ServiceName)
	_, _, errActive := conn.Exec(ctx.GoContext(), isActiveCmd, execOpts)
	// errActive != nil implies service is not active (or doesn't exist)

	isEnabledCmd := fmt.Sprintf("systemctl is-enabled %s", s.ServiceName)
	_, _, errEnabled := conn.Exec(ctx.GoContext(), isEnabledCmd, execOpts)
	// errEnabled != nil implies service is not enabled (or doesn't exist)

	if errActive != nil && errEnabled != nil { // Both commands failed, likely service doesn't exist or is fully wiped
		logger.Debug("Service seems inactive and not enabled (or does not exist).")
		// Now check for files
		pathsToCheck := append([]string{s.SystemdUnitFilePath}, s.BinariesToRemove...)
		pathsToCheck = append(pathsToCheck, s.ConfigFilesToRemove...)
		pathsToCheck = append(pathsToCheck, s.CniConfigFilesToRemove...)

		anyFileExists := false
		for _, p := range pathsToCheck {
			if p == "" { continue }
			exists, errExist := conn.Exists(ctx.GoContext(), p)
			if errExist != nil {
				logger.Warn("Failed to check existence of path, assuming it might exist.", "path", p, "error", errExist)
				return false, nil // Cannot be sure, let Run proceed
			}
			if exists {
				logger.Info("Found remaining containerd artifact, uninstall needed.", "path", p)
				anyFileExists = true
				break
			}
		}
		if !anyFileExists {
			logger.Info("Containerd service inactive/disabled and no common artifacts found. Assuming already uninstalled.")
			return true, nil
		}
		// Some files exist, so not fully uninstalled
		return false, nil
	}

	logger.Info("Containerd service might still be active, enabled, or files present. Uninstall needed.")
	return false, nil // Service might be running or files might exist
}

// Run performs the uninstallation of containerd.
func (s *UninstallContainerdStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// Stop and Disable Service
	logger.Info("Stopping containerd service.", "service", s.ServiceName)
	stopCmd := fmt.Sprintf("systemctl stop %s", s.ServiceName)
	_, stderrStop, errStop := conn.Exec(ctx.GoContext(), stopCmd, execOpts)
	if errStop != nil {
		logger.Warn("Failed to stop service (might not be running or exist).", "service", s.ServiceName, "stderr", string(stderrStop), "error", errStop)
	}

	logger.Info("Disabling containerd service.", "service", s.ServiceName)
	disableCmd := fmt.Sprintf("systemctl disable %s", s.ServiceName)
	_, stderrDisable, errDisable := conn.Exec(ctx.GoContext(), disableCmd, execOpts)
	if errDisable != nil {
		logger.Warn("Failed to disable service (might not exist or already disabled).", "service", s.ServiceName, "stderr", string(stderrDisable), "error", errDisable)
	}

	// Remove Systemd Unit File
	if s.SystemdUnitFilePath != "" {
		logger.Info("Removing systemd unit file.", "path", s.SystemdUnitFilePath)
		rmUnitCmd := fmt.Sprintf("rm -f %s", s.SystemdUnitFilePath)
		_, stderrRmUnit, errRmUnit := conn.Exec(ctx.GoContext(), rmUnitCmd, execOpts)
		if errRmUnit != nil {
			logger.Warn("Failed to remove systemd unit file.", "path", s.SystemdUnitFilePath, "stderr", string(stderrRmUnit), "error", errRmUnit)
		}

		logger.Info("Reloading systemd daemon.")
		daemonReloadCmd := "systemctl daemon-reload"
		if _, stderrReload, errReload := conn.Exec(ctx.GoContext(), daemonReloadCmd, execOpts); errReload != nil {
			logger.Warn("Failed to reload systemd daemon.", "stderr", string(stderrReload), "error", errReload)
		}
		logger.Info("Resetting failed systemd units (optional).")
		resetFailedCmd := "systemctl reset-failed"
		if _, stderrReset, errReset := conn.Exec(ctx.GoContext(), resetFailedCmd, execOpts); errReset != nil {
			logger.Warn("Failed to reset failed systemd units.", "stderr", string(stderrReset), "error", errReset)
		}
	}

	// Remove Binaries, Config Files, CNI Configs
	pathsToRemove := [][]string{s.BinariesToRemove, s.ConfigFilesToRemove, s.CniConfigFilesToRemove}
	isDir := []bool{false, true, false} // ConfigFilesToRemove might contain directories

	for i, group := range pathsToRemove {
		for _, path := range group {
			if path == "" { continue }
			removeCmd := fmt.Sprintf("rm -f %s", path)
			if isDir[i] { // For directories (like /etc/containerd)
				removeCmd = fmt.Sprintf("rm -rf %s", path)
			}
			logger.Info("Removing path.", "path", path, "command", removeCmd)
			_, stderrRmPath, errRmPath := conn.Exec(ctx.GoContext(), removeCmd, execOpts)
			if errRmPath != nil {
				logger.Warn("Failed to remove path (might not exist).", "path", path, "stderr", string(stderrRmPath), "error", errRmPath)
			}
		}
	}

	logger.Info("Containerd uninstallation process completed (best-effort).")
	return nil
}

// Rollback for uninstall is typically not supported.
func (s *UninstallContainerdStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Warn("Rollback for containerd uninstall is not supported. Reinstallation would require running the installation steps.")
	return nil
}

var _ step.Step = (*UninstallContainerdStepSpec)(nil)
