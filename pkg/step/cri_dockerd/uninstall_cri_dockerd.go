package cri_dockerd

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// UninstallCriDockerdStepSpec defines parameters for uninstalling cri-dockerd.
type UninstallCriDockerdStepSpec struct {
	spec.StepMeta `json:",inline"`

	ServiceName            string `json:"serviceName,omitempty"`
	SocketName             string `json:"socketName,omitempty"`
	SystemdServiceUnitPath string `json:"systemdServiceUnitPath,omitempty"`
	SystemdSocketUnitPath  string `json:"systemdSocketUnitPath,omitempty"`
	BinaryPath             string `json:"binaryPath,omitempty"`
	Sudo                   bool   `json:"sudo,omitempty"`
}

// NewUninstallCriDockerdStepSpec creates a new UninstallCriDockerdStepSpec.
func NewUninstallCriDockerdStepSpec(name, description string) *UninstallCriDockerdStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Uninstall cri-dockerd"
	}
	finalDescription := description
	// Description will be refined in populateDefaults.

	return &UninstallCriDockerdStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		// Defaults will be set in populateDefaults
	}
}

// Name returns the step's name.
func (s *UninstallCriDockerdStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *UninstallCriDockerdStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *UninstallCriDockerdStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *UninstallCriDockerdStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *UninstallCriDockerdStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *UninstallCriDockerdStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *UninstallCriDockerdStepSpec) populateDefaults(logger runtime.Logger) {
	if s.ServiceName == "" {
		s.ServiceName = "cri-docker.service"
		logger.Debug("ServiceName defaulted.", "name", s.ServiceName)
	}
	if s.SocketName == "" {
		s.SocketName = "cri-docker.socket"
		logger.Debug("SocketName defaulted.", "name", s.SocketName)
	}
	if s.SystemdServiceUnitPath == "" {
		s.SystemdServiceUnitPath = DefaultCriDockerdServiceUnitPath // From setup_cri_dockerd_service.go
		logger.Debug("SystemdServiceUnitPath defaulted.", "path", s.SystemdServiceUnitPath)
	}
	if s.SystemdSocketUnitPath == "" {
		s.SystemdSocketUnitPath = DefaultCriDockerdSocketUnitPath // From setup_cri_dockerd_service.go
		logger.Debug("SystemdSocketUnitPath defaulted.", "path", s.SystemdSocketUnitPath)
	}
	if s.BinaryPath == "" {
		s.BinaryPath = "/usr/local/bin/cri-dockerd" // Common install path
		logger.Debug("BinaryPath defaulted.", "path", s.BinaryPath)
	}
	if !s.Sudo { // Default to true
		s.Sudo = true
		logger.Debug("Sudo defaulted to true for uninstall operations.")
	}

	if s.StepMeta.Description == "" {
		s.StepMeta.Description = fmt.Sprintf("Uninstalls cri-dockerd: stops/disables service '%s' and socket '%s', removes binary and systemd units.",
			s.ServiceName, s.SocketName)
	}
}

// isServiceOrSocketActive checks if a systemd unit is active.
func isUnitActive(ctx runtime.StepContext, conn connector.Connector, unitName string, sudo bool) bool {
	logger := ctx.GetLogger().With("unit", unitName)
	isActiveCmd := fmt.Sprintf("systemctl is-active %s", unitName)
	_, _, errActive := conn.Exec(ctx.GoContext(), isActiveCmd, &connector.ExecOptions{Sudo: sudo})
	isActive := errActive == nil // Exit code 0 means active
	logger.Debug("Unit active status.", "isActive", isActive)
	return isActive
}

// isServiceOrSocketEnabled checks if a systemd unit is enabled.
func isUnitEnabled(ctx runtime.StepContext, conn connector.Connector, unitName string, sudo bool) bool {
	logger := ctx.GetLogger().With("unit", unitName)
	isEnabledCmd := fmt.Sprintf("systemctl is-enabled %s", unitName)
	_, _, errEnabled := conn.Exec(ctx.GoContext(), isEnabledCmd, &connector.ExecOptions{Sudo: sudo})
	isEnabled := errEnabled == nil // Exit code 0 means enabled
	logger.Debug("Unit enabled status.", "isEnabled", isEnabled)
	return isEnabled
}


// Precheck determines if cri-dockerd seems already uninstalled.
func (s *UninstallCriDockerdStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// 1. Check service & socket status
	serviceActive := isUnitActive(ctx, conn, s.ServiceName, s.Sudo)
	socketActive := isUnitActive(ctx, conn, s.SocketName, s.Sudo)
	serviceEnabled := isUnitEnabled(ctx, conn, s.ServiceName, s.Sudo)
	socketEnabled := isUnitEnabled(ctx, conn, s.SocketName, s.Sudo)

	if serviceActive || socketActive || serviceEnabled || socketEnabled {
		logger.Info("cri-dockerd service or socket appears to be active or enabled. Uninstall needed.")
		return false, nil
	}
	logger.Debug("cri-dockerd service and socket are not active or enabled.")

	// 2. Check for files
	pathsToPotentiallyExist := []string{s.SystemdServiceUnitPath, s.SystemdSocketUnitPath, s.BinaryPath}
	anyFileExists := false
	for _, p := range pathsToPotentiallyExist {
		if p == "" { continue }
		exists, errExist := conn.Exists(ctx.GoContext(), p)
		if errExist != nil {
			logger.Warn("Failed to check existence of path during precheck, assuming it might exist.", "path", p, "error", errExist)
			return false, nil // Cannot be sure, let Run proceed
		}
		if exists {
			logger.Info("Found remaining cri-dockerd artifact, uninstall needed.", "path", p)
			anyFileExists = true
			break
		}
	}

	if !anyFileExists {
		logger.Info("cri-dockerd service/socket inactive/disabled and no common artifacts found. Assuming already uninstalled.")
		return true, nil
	}

	logger.Info("cri-dockerd components still present. Uninstall needed.")
	return false, nil
}

// Run performs the uninstallation of cri-dockerd.
func (s *UninstallCriDockerdStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// Stop and Disable Service & Socket
	unitsToManage := []struct{name string; unitType string}{
		{s.ServiceName, "service"},
		{s.SocketName, "socket"},
	}
	for _, unit := range unitsToManage {
		if unit.name == "" { continue }
		logger.Info(fmt.Sprintf("Stopping %s.", unit.unitType), "name", unit.name)
		stopCmd := fmt.Sprintf("systemctl stop %s", unit.name)
		if _, stderr, errStop := conn.Exec(ctx.GoContext(), stopCmd, execOpts); errStop != nil {
			logger.Warn(fmt.Sprintf("Failed to stop %s (might not be running or exist).", unit.unitType), "name", unit.name, "stderr", string(stderr), "error", errStop)
		}

		logger.Info(fmt.Sprintf("Disabling %s.", unit.unitType), "name", unit.name)
		disableCmd := fmt.Sprintf("systemctl disable %s", unit.name)
		if _, stderr, errDisable := conn.Exec(ctx.GoContext(), disableCmd, execOpts); errDisable != nil {
			logger.Warn(fmt.Sprintf("Failed to disable %s (might not exist or already disabled).", unit.unitType), "name", unit.name, "stderr", string(stderr), "error", errDisable)
		}
	}

	// Remove Systemd Unit Files
	filesRemoved := false
	for _, path := range []string{s.SystemdServiceUnitPath, s.SystemdSocketUnitPath} {
		if path == "" { continue }
		logger.Info("Removing systemd unit file.", "path", path)
		rmUnitCmd := fmt.Sprintf("rm -f %s", path)
		if _, stderr, errRmUnit := conn.Exec(ctx.GoContext(), rmUnitCmd, execOpts); errRmUnit != nil {
			logger.Warn("Failed to remove systemd unit file.", "path", path, "stderr", string(stderr), "error", errRmUnit)
		} else {
			filesRemoved = true
		}
	}

	if filesRemoved {
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

	// Remove Binary
	if s.BinaryPath != "" {
		logger.Info("Removing binary.", "path", s.BinaryPath)
		rmCmd := fmt.Sprintf("rm -f %s", s.BinaryPath)
		if _, stderr, errRm := conn.Exec(ctx.GoContext(), rmCmd, execOpts); errRm != nil {
			logger.Warn("Failed to remove binary (might not exist).", "path", s.BinaryPath, "stderr", string(stderr), "error", errRm)
		}
	}

	logger.Info("cri-dockerd uninstallation process completed (best-effort).")
	return nil
}

// Rollback for uninstall is not supported.
func (s *UninstallCriDockerdStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Warn("Rollback for cri-dockerd uninstall is not supported. Please run installation steps if cri-dockerd is needed again.")
	return nil
}

var _ step.Step = (*UninstallCriDockerdStepSpec)(nil)
