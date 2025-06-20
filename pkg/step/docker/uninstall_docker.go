package docker

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// UninstallDockerStepSpec defines parameters for uninstalling Docker.
type UninstallDockerStepSpec struct {
	spec.StepMeta `json:",inline"`

	ServiceName         string   `json:"serviceName,omitempty"`
	SystemdUnitPath     string   `json:"systemdUnitFilePath,omitempty"`
	BinariesToRemove    []string `json:"binariesToRemove,omitempty"`
	DaemonConfigPath    string   `json:"daemonConfigPath,omitempty"`
	DockerRootDir       string   `json:"dockerRootDir,omitempty"` // For removing /etc/docker
	Sudo                bool     `json:"sudo,omitempty"`
}

// NewUninstallDockerStepSpec creates a new UninstallDockerStepSpec.
func NewUninstallDockerStepSpec(name, description string) *UninstallDockerStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Uninstall Docker"
	}
	finalDescription := description
	// Description will be refined in populateDefaults.

	return &UninstallDockerStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		// Defaults will be set in populateDefaults
	}
}

// Name returns the step's name.
func (s *UninstallDockerStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *UninstallDockerStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *UninstallDockerStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *UninstallDockerStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *UninstallDockerStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *UninstallDockerStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *UninstallDockerStepSpec) populateDefaults(logger runtime.Logger) {
	if s.ServiceName == "" {
		s.ServiceName = "docker"
		logger.Debug("ServiceName defaulted.", "name", s.ServiceName)
	}
	if s.SystemdUnitPath == "" {
		s.SystemdUnitPath = "/etc/systemd/system/docker.service"
		// Also check /usr/lib/systemd/system/docker.service if the above doesn't exist in Precheck/Run
		logger.Debug("SystemdUnitFilePath defaulted.", "path", s.SystemdUnitPath)
	}
	if len(s.BinariesToRemove) == 0 {
		s.BinariesToRemove = []string{
			"/usr/bin/docker",
			"/usr/bin/dockerd",
			"/usr/bin/docker-proxy",
			"/usr/bin/docker-init",
			"/usr/bin/containerd", // Often installed with Docker, but might be separate
			"/usr/bin/containerd-shim-runc-v2",
			"/usr/bin/ctr",
			"/usr/bin/runc",
			// Some systems might use /usr/local/bin
			"/usr/local/bin/docker",
			"/usr/local/bin/dockerd",
		}
		logger.Debug("BinariesToRemove defaulted.", "binaries", s.BinariesToRemove)
	}
	if s.DaemonConfigPath == "" {
		s.DaemonConfigPath = "/etc/docker/daemon.json"
		logger.Debug("DaemonConfigPath defaulted.", "path", s.DaemonConfigPath)
	}
	if s.DockerRootDir == "" {
		s.DockerRootDir = "/etc/docker" // For removing the /etc/docker directory itself
		logger.Debug("DockerRootDir defaulted.", "path", s.DockerRootDir)
	}

	if !s.Sudo { // If not explicitly set by user (zero value is false)
		s.Sudo = true
		logger.Debug("Sudo defaulted to true for uninstall operations.")
	}

	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Uninstalls Docker: stops/disables service '%s', removes specified binaries and configuration files.", s.ServiceName)
	}
}

// Precheck determines if Docker seems already uninstalled.
func (s *UninstallDockerStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// 1. Check service status
	isActiveCmd := fmt.Sprintf("systemctl is-active %s", s.ServiceName)
	_, _, errActive := conn.Exec(ctx.GoContext(), isActiveCmd, execOpts)
	// errActive != nil implies service is not active (or doesn't exist)

	isEnabledCmd := fmt.Sprintf("systemctl is-enabled %s", s.ServiceName)
	_, _, errEnabled := conn.Exec(ctx.GoContext(), isEnabledCmd, execOpts)
	// errEnabled != nil implies service is not enabled (or doesn't exist)

	serviceSeemsInactive := errActive != nil && errEnabled != nil
	if serviceSeemsInactive {
		logger.Debug("Docker service seems inactive and not enabled (or does not exist).")
	} else {
		logger.Info("Docker service may still be active or enabled. Uninstall needed.")
		return false, nil // Service still active/enabled, so not "done" (uninstalled)
	}

	// 2. Check for files (only if service seems gone)
	pathsToPotentiallyExist := []string{s.SystemdUnitPath, s.DaemonConfigPath, s.DockerRootDir}
	pathsToPotentiallyExist = append(pathsToPotentiallyExist, s.BinariesToRemove...)

	anyFileExists := false
	for _, p := range pathsToPotentiallyExist {
		if p == "" { continue } // Skip empty paths that might result from empty defaults not being overridden
		exists, errExist := conn.Exists(ctx.GoContext(), p)
		if errExist != nil {
			logger.Warn("Failed to check existence of path during precheck, assuming it might exist.", "path", p, "error", errExist)
			return false, nil // Cannot be sure, let Run proceed
		}
		if exists {
			logger.Info("Found remaining Docker artifact, uninstall needed.", "path", p)
			anyFileExists = true
			break
		}
	}

	if !anyFileExists && serviceSeemsInactive {
		logger.Info("Docker service inactive/disabled and no common artifacts found. Assuming already uninstalled.")
		return true, nil
	}

	logger.Info("Docker components still present. Uninstall needed.")
	return false, nil
}

// Run performs the uninstallation of Docker.
func (s *UninstallDockerStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// Stop and Disable Service
	if s.ServiceName != "" {
		logger.Info("Stopping Docker service.", "service", s.ServiceName)
		stopCmd := fmt.Sprintf("systemctl stop %s", s.ServiceName)
		if _, stderr, errStop := conn.Exec(ctx.GoContext(), stopCmd, execOpts); errStop != nil {
			logger.Warn("Failed to stop service (might not be running or exist).", "service", s.ServiceName, "stderr", string(stderr), "error", errStop)
		}

		logger.Info("Disabling Docker service.", "service", s.ServiceName)
		disableCmd := fmt.Sprintf("systemctl disable %s", s.ServiceName)
		if _, stderr, errDisable := conn.Exec(ctx.GoContext(), disableCmd, execOpts); errDisable != nil {
			logger.Warn("Failed to disable service (might not exist or already disabled).", "service", s.ServiceName, "stderr", string(stderr), "error", errDisable)
		}
	}

	// Remove Systemd Unit File
	if s.SystemdUnitPath != "" {
		logger.Info("Removing systemd unit file.", "path", s.SystemdUnitPath)
		rmUnitCmd := fmt.Sprintf("rm -f %s", s.SystemdUnitPath)
		if _, stderr, errRmUnit := conn.Exec(ctx.GoContext(), rmUnitCmd, execOpts); errRmUnit != nil {
			logger.Warn("Failed to remove systemd unit file.", "path", s.SystemdUnitPath, "stderr", string(stderr), "error", errRmUnit)
		} else { // Only reload if unit file was likely removed
			logger.Info("Reloading systemd daemon.")
			daemonReloadCmd := "systemctl daemon-reload"
			if _, stderr, errReload := conn.Exec(ctx.GoContext(), daemonReloadCmd, execOpts); errReload != nil {
				logger.Warn("Failed to reload systemd daemon.", "stderr", string(stderr), "error", errReload)
			}
			logger.Info("Resetting failed systemd units.")
			resetFailedCmd := "systemctl reset-failed"
			if _, stderr, errReset := conn.Exec(ctx.GoContext(), resetFailedCmd, execOpts); errReset != nil {
				logger.Warn("Failed to reset failed systemd units.", "stderr", string(stderr), "error", errReset)
			}
		}
	}

	// Remove Binaries
	for _, path := range s.BinariesToRemove {
		if path == "" { continue }
		logger.Info("Removing binary.", "path", path)
		rmCmd := fmt.Sprintf("rm -f %s", path)
		if _, stderr, errRm := conn.Exec(ctx.GoContext(), rmCmd, execOpts); errRm != nil {
			logger.Warn("Failed to remove binary (might not exist).", "path", path, "stderr", string(stderr), "error", errRm)
		}
	}

	// Remove Config Files and Root Dir
	if s.DaemonConfigPath != "" {
		logger.Info("Removing Docker daemon config file.", "path", s.DaemonConfigPath)
		rmCmd := fmt.Sprintf("rm -f %s", s.DaemonConfigPath)
		if _, stderr, errRm := conn.Exec(ctx.GoContext(), rmCmd, execOpts); errRm != nil {
			logger.Warn("Failed to remove daemon config file (might not exist).", "path", s.DaemonConfigPath, "stderr", string(stderr), "error", errRm)
		}
	}
	if s.DockerRootDir != "" && s.DockerRootDir != "/" { // Safety check
		logger.Info("Removing Docker root config directory.", "path", s.DockerRootDir)
		rmRfCmd := fmt.Sprintf("rm -rf %s", s.DockerRootDir)
		if _, stderr, errRmRf := conn.Exec(ctx.GoContext(), rmRfCmd, execOpts); errRmRf != nil {
			logger.Warn("Failed to remove Docker root config directory.", "path", s.DockerRootDir, "stderr", string(stderr), "error", errRmRf)
		}
	}

	logger.Info("Docker uninstall process completed (best-effort).")
	return nil
}

// Rollback for uninstall is not supported.
func (s *UninstallDockerStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Warn("Rollback for Docker uninstall is not supported. Please run installation steps if Docker is needed again.")
	return nil
}

var _ step.Step = (*UninstallDockerStepSpec)(nil)
