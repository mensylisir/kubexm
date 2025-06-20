package etcd

import (
	"fmt"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// UninstallEtcdStepSpec defines parameters for uninstalling etcd.
type UninstallEtcdStepSpec struct {
	spec.StepMeta `json:",inline"`

	ServiceName         string   `json:"serviceName,omitempty"`
	SystemdUnitPath     string   `json:"systemdUnitPath,omitempty"`
	BinariesToRemove    []string `json:"binariesToRemove,omitempty"`
	ConfigDirToRemove   string   `json:"configDirToRemove,omitempty"` // e.g. /etc/etcd
	DataDirToRemove     string   `json:"dataDirToRemove,omitempty"`   // e.g. /var/lib/etcd
	ClearData           bool     `json:"clearData,omitempty"`       // If true and DataDirToRemove is set, remove data.
	Sudo                bool     `json:"sudo,omitempty"`
}

// NewUninstallEtcdStepSpec creates a new UninstallEtcdStepSpec.
func NewUninstallEtcdStepSpec(name, description string) *UninstallEtcdStepSpec {
	finalName := name
	if finalName == "" {
		finalName = "Uninstall Etcd"
	}
	finalDescription := description
	// Description will be refined in populateDefaults.

	return &UninstallEtcdStepSpec{
		StepMeta: spec.StepMeta{
			Name:        finalName,
			Description: finalDescription,
		},
		// Defaults will be set in populateDefaults
	}
}

// Name returns the step's name.
func (s *UninstallEtcdStepSpec) Name() string { return s.StepMeta.Name }

// Description returns the step's description.
func (s *UninstallEtcdStepSpec) Description() string { return s.StepMeta.Description }

// GetName returns the step's name for spec interface.
func (s *UninstallEtcdStepSpec) GetName() string { return s.StepMeta.Name }

// GetDescription returns the step's description for spec interface.
func (s *UninstallEtcdStepSpec) GetDescription() string { return s.StepMeta.Description }

// GetSpec returns the spec itself.
func (s *UninstallEtcdStepSpec) GetSpec() interface{} { return s }

// Meta returns the step's metadata.
func (s *UninstallEtcdStepSpec) Meta() *spec.StepMeta { return &s.StepMeta }

func (s *UninstallEtcdStepSpec) populateDefaults(logger runtime.Logger) {
	if s.ServiceName == "" {
		s.ServiceName = "etcd"
		logger.Debug("ServiceName defaulted.", "name", s.ServiceName)
	}
	if s.SystemdUnitPath == "" {
		s.SystemdUnitPath = "/etc/systemd/system/etcd.service"
		logger.Debug("SystemdUnitPath defaulted.", "path", s.SystemdUnitPath)
	}
	if len(s.BinariesToRemove) == 0 {
		s.BinariesToRemove = []string{
			"/usr/local/bin/etcd",
			"/usr/local/bin/etcdctl",
			"/usr/local/bin/etcdutl",
		}
		logger.Debug("BinariesToRemove defaulted.", "binaries", s.BinariesToRemove)
	}
	if s.ConfigDirToRemove == "" {
		s.ConfigDirToRemove = "/etc/etcd"
		logger.Debug("ConfigDirToRemove defaulted.", "path", s.ConfigDirToRemove)
	}
	if s.DataDirToRemove == "" {
		s.DataDirToRemove = "/var/lib/etcd"
		logger.Debug("DataDirToRemove defaulted.", "path", s.DataDirToRemove)
	}
	// ClearData defaults to false (zero value for bool).
	if !s.Sudo { // Default to true if not explicitly set false
		s.Sudo = true
		logger.Debug("Sudo defaulted to true for uninstall operations.")
	}

	if s.StepMeta.Description == "" {
		desc := fmt.Sprintf("Uninstalls etcd: stops/disables service '%s', removes binaries, systemd unit, and config directory '%s'.",
			s.ServiceName, s.ConfigDirToRemove)
		if s.ClearData && s.DataDirToRemove != "" {
			desc += fmt.Sprintf(" Also removes data directory '%s'.", s.DataDirToRemove)
		}
		s.StepMeta.Description = desc
	}
}

// isUnitPresentAndActive checks if a systemd unit is active or enabled.
func isUnitPresentAndActive(ctx runtime.StepContext, conn connector.Connector, unitName string, sudo bool) bool {
	logger := ctx.GetLogger().With("unit", unitName)
	execOpts := &connector.ExecOptions{Sudo: sudo}

	// Check if enabled
	isEnabledCmd := fmt.Sprintf("systemctl is-enabled %s", unitName)
	_, _, errEnabled := conn.Exec(ctx.GoContext(), isEnabledCmd, execOpts)
	if errEnabled == nil { // Exit code 0 means enabled
		logger.Debug("Unit is enabled.")
		return true
	}

	// Check if active
	isActiveCmd := fmt.Sprintf("systemctl is-active %s", unitName)
	_, _, errActive := conn.Exec(ctx.GoContext(), isActiveCmd, execOpts)
	if errActive == nil { // Exit code 0 means active
		logger.Debug("Unit is active.")
		return true
	}

	logger.Debug("Unit is not active or enabled (or does not exist).")
	return false
}

// Precheck determines if etcd seems already uninstalled.
func (s *UninstallEtcdStepSpec) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Precheck")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	// 1. Check service status
	if isUnitPresentAndActive(ctx, conn, s.ServiceName, s.Sudo) {
		logger.Info("Etcd service appears to be active or enabled. Uninstall needed.")
		return false, nil
	}

	// 2. Check for files and directories
	pathsToPotentiallyExist := []string{s.SystemdUnitPath, s.ConfigDirToRemove}
	pathsToPotentiallyExist = append(pathsToPotentiallyExist, s.BinariesToRemove...)
	if s.ClearData && s.DataDirToRemove != "" { // Only consider data dir if ClearData is true
		pathsToPotentiallyExist = append(pathsToPotentiallyExist, s.DataDirToRemove)
	}

	anyFileExists := false
	for _, p := range pathsToPotentiallyExist {
		if p == "" { continue }
		exists, errExist := conn.Exists(ctx.GoContext(), p)
		if errExist != nil {
			logger.Warn("Failed to check existence of path during precheck, assuming it might exist.", "path", p, "error", errExist)
			return false, nil // Cannot be sure, let Run proceed
		}
		if exists {
			logger.Info("Found remaining etcd artifact, uninstall needed.", "path", p)
			anyFileExists = true
			break
		}
	}

	if !anyFileExists {
		logger.Info("Etcd service inactive/disabled and no common artifacts found. Assuming already uninstalled.")
		return true, nil
	}

	logger.Info("Etcd components still present. Uninstall needed.")
	return false, nil
}

// Run performs the uninstallation of etcd.
func (s *UninstallEtcdStepSpec) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Run")
	s.populateDefaults(logger)

	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}
	execOpts := &connector.ExecOptions{Sudo: s.Sudo}

	// Stop and Disable Service
	if s.ServiceName != "" {
		logger.Info("Stopping etcd service.", "service", s.ServiceName)
		stopCmd := fmt.Sprintf("systemctl stop %s", s.ServiceName)
		if _, stderr, errStop := conn.Exec(ctx.GoContext(), stopCmd, execOpts); errStop != nil {
			logger.Warn("Failed to stop service (might not be running or exist).", "service", s.ServiceName, "stderr", string(stderr), "error", errStop)
		}

		logger.Info("Disabling etcd service.", "service", s.ServiceName)
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
		} else {
			logger.Info("Reloading systemd daemon after removing unit file.")
			daemonReloadCmd := "systemctl daemon-reload"
			if _, stderrR, errR := conn.Exec(ctx.GoContext(), daemonReloadCmd, execOpts); errR != nil {
				logger.Warn("Failed to reload systemd daemon.", "stderr", string(stderrR), "error", errR)
			}
			logger.Info("Resetting failed systemd units.")
			resetFailedCmd := "systemctl reset-failed"
			if _, stderrRf, errRf := conn.Exec(ctx.GoContext(), resetFailedCmd, execOpts); errRf != nil {
				logger.Warn("Failed to reset failed systemd units.", "stderr", string(stderrRf), "error", errRf)
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

	// Remove Config Directory
	if s.ConfigDirToRemove != "" {
		logger.Info("Removing etcd config directory.", "path", s.ConfigDirToRemove)
		rmRfCmd := fmt.Sprintf("rm -rf %s", s.ConfigDirToRemove)
		if _, stderr, errRmRf := conn.Exec(ctx.GoContext(), rmRfCmd, execOpts); errRmRf != nil {
			logger.Warn("Failed to remove etcd config directory.", "path", s.ConfigDirToRemove, "stderr", string(stderr), "error", errRmRf)
		}
	}

	// Optionally Clear Data Dir
	if s.ClearData && s.DataDirToRemove != "" {
		// Safety check for DataDirToRemove path
		if s.DataDirToRemove == "/" || s.DataDirToRemove == "/var" || s.DataDirToRemove == "/var/lib" {
			logger.Error("Skipping removal of data directory due to unsafe path.", "path", s.DataDirToRemove)
		} else {
			logger.Info("Removing etcd data directory.", "path", s.DataDirToRemove)
			rmRfCmd := fmt.Sprintf("rm -rf %s", s.DataDirToRemove)
			if _, stderr, errRmRf := conn.Exec(ctx.GoContext(), rmRfCmd, execOpts); errRmRf != nil {
				logger.Warn("Failed to remove etcd data directory.", "path", s.DataDirToRemove, "stderr", string(stderr), "error", errRmRf)
			}
		}
	}

	logger.Info("Etcd uninstall process completed (best-effort).")
	return nil
}

// Rollback for uninstall is not supported.
func (s *UninstallEtcdStepSpec) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.GetName(), "host", host.GetName(), "phase", "Rollback")
	logger.Warn("Rollback for etcd uninstall is not supported. Please run installation steps if etcd is needed again.")
	return nil
}

var _ step.Step = (*UninstallEtcdStepSpec)(nil)
