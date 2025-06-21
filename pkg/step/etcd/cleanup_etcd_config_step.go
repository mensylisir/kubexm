package etcd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// CleanupEtcdConfigStep removes etcd configuration files, directories, and the systemd service file.
type CleanupEtcdConfigStep struct {
	meta             spec.StepMeta
	ConfigDir        string // e.g., /etc/etcd
	ServiceFilePath  string // e.g., /etc/systemd/system/etcd.service
	Sudo             bool
}

// NewCleanupEtcdConfigStep creates a new CleanupEtcdConfigStep.
func NewCleanupEtcdConfigStep(instanceName, configDir, serviceFilePath string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "CleanupEtcdConfiguration"
	}
	cd := configDir
	if cd == "" {
		cd = "/etc/etcd" // Default config directory
	}
	sfp := serviceFilePath
	if sfp == "" {
		sfp = EtcdServiceFileRemotePath // Default from generate_etcd_service_step.go
	}

	return &CleanupEtcdConfigStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Removes etcd config directory (%s) and service file (%s).", cd, sfp),
		},
		ConfigDir:       cd,
		ServiceFilePath: sfp,
		Sudo:            true, // Removing system files/dirs usually requires sudo
	}
}

func (s *CleanupEtcdConfigStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *CleanupEtcdConfigStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	configDirExists, errConfig := runnerSvc.Exists(ctx.GoContext(), conn, s.ConfigDir)
	if errConfig != nil {
		logger.Warn("Failed to check existence of etcd config directory, assuming it might exist.", "path", s.ConfigDir, "error", errConfig)
		return false, nil // Let Run attempt removal
	}

	serviceFileExists, errService := runnerSvc.Exists(ctx.GoContext(), conn, s.ServiceFilePath)
	if errService != nil {
		logger.Warn("Failed to check existence of etcd service file, assuming it might exist.", "path", s.ServiceFilePath, "error", errService)
		return false, nil // Let Run attempt removal
	}

	if !configDirExists && !serviceFileExists {
		logger.Info("Etcd config directory and service file already removed.")
		return true, nil
	}
	if !configDirExists {
		logger.Info("Etcd config directory already removed, but service file might exist.", "service_file", s.ServiceFilePath)
	}
	if !serviceFileExists {
		logger.Info("Etcd service file already removed, but config directory might exist.", "config_dir", s.ConfigDir)
	}

	return false, nil // At least one item exists and needs removal
}

func (s *CleanupEtcdConfigStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	var lastErr error

	// Remove config directory
	logger.Info("Removing etcd config directory.", "path", s.ConfigDir)
	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.ConfigDir, s.Sudo); err != nil {
		logger.Error("Failed to remove etcd config directory (best effort).", "path", s.ConfigDir, "error", err)
		lastErr = fmt.Errorf("failed to remove %s: %w (previous error: %v)", s.ConfigDir, err, lastErr)
	}

	// Remove service file
	logger.Info("Removing etcd service file.", "path", s.ServiceFilePath)
	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.ServiceFilePath, s.Sudo); err != nil {
		logger.Error("Failed to remove etcd service file (best effort).", "path", s.ServiceFilePath, "error", err)
		lastErr = fmt.Errorf("failed to remove %s: %w (previous error: %v)", s.ServiceFilePath, err, lastErr)
	}

	// After removing service file, a daemon-reload might be needed if the service was active.
	// This step doesn't do it automatically to keep it focused.
	// A task can schedule a ManageEtcdServiceStep(ActionDaemonReload) if needed.

	if lastErr != nil {
		return fmt.Errorf("one or more errors occurred during etcd config cleanup: %w", lastErr)
	}
	logger.Info("Etcd configuration and service file cleanup successful.")
	return nil
}

func (s *CleanupEtcdConfigStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for CleanupEtcdConfigStep is not applicable (would mean regenerating config and service files).")
	return nil
}

var _ step.Step = (*CleanupEtcdConfigStep)(nil)
