package containerd

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// CleanupContainerdConfigStep removes containerd configuration files and the systemd service file.
type CleanupContainerdConfigStep struct {
	meta            spec.StepMeta
	ConfigFilePath  string // e.g., /etc/containerd/config.toml
	CertsDir        string // e.g., /etc/containerd/certs.d or /etc/docker/certs.d if shared
	ServiceFilePath string // e.g., /etc/systemd/system/containerd.service
	Sudo            bool
}

// NewCleanupContainerdConfigStep creates a new CleanupContainerdConfigStep.
func NewCleanupContainerdConfigStep(instanceName, configFilePath, certsDir, serviceFilePath string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "CleanupContainerdConfiguration"
	}
	cfgPath := configFilePath
	if cfgPath == "" {
		cfgPath = DefaultContainerdConfigPath // From configure_containerd_step.go
	}
	cDir := certsDir // Optional, might not always exist or be managed this way

	sfp := serviceFilePath
	if sfp == "" {
		sfp = ContainerdServiceFileRemotePath // From generate_containerd_service_step.go
	}

	return &CleanupContainerdConfigStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Removes containerd config file (%s), certs dir (%s), and service file (%s).", cfgPath, cDir, sfp),
		},
		ConfigFilePath:  cfgPath,
		CertsDir:        cDir,
		ServiceFilePath: sfp,
		Sudo:            true,
	}
}

func (s *CleanupContainerdConfigStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *CleanupContainerdConfigStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector: %w", err)
	}

	pathsToCheck := []string{s.ConfigFilePath, s.ServiceFilePath}
	if s.CertsDir != "" {
		pathsToCheck = append(pathsToCheck, s.CertsDir)
	}

	allMissing := true
	for _, p := range pathsToCheck {
		if p == "" {
			continue
		} // Skip if optional path like CertsDir is not provided
		exists, err := runnerSvc.Exists(ctx.GoContext(), conn, p)
		if err != nil {
			logger.Warn("Failed to check existence, assuming it might exist.", "path", p, "error", err)
			return false, nil
		}
		if exists {
			logger.Info("Configuration item still exists.", "path", p)
			allMissing = false
		}
	}

	if allMissing {
		logger.Info("All specified containerd configuration items already removed.")
		return true, nil
	}
	return false, nil
}

func (s *CleanupContainerdConfigStep) Run(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector: %w", err)
	}

	var lastErr error

	itemsToRemove := []string{s.ConfigFilePath, s.ServiceFilePath}
	if s.CertsDir != "" {
		itemsToRemove = append(itemsToRemove, s.CertsDir)
	}

	for _, itemPath := range itemsToRemove {
		if itemPath == "" {
			continue
		}
		logger.Info("Removing containerd configuration item.", "path", itemPath)
		if err := runnerSvc.Remove(ctx.GoContext(), conn, itemPath, s.Sudo); err != nil {
			logger.Error("Failed to remove item (best effort).", "path", itemPath, "error", err)
			lastErr = fmt.Errorf("failed to remove %s: %w (previous error: %v)", itemPath, err, lastErr)
		}
	}

	// Also remove CNI config if it exists, typically /etc/cni/net.d
	cniConfigDir := "/etc/cni/net.d"
	logger.Info("Removing CNI configuration directory.", "path", cniConfigDir)
	if err := runnerSvc.Remove(ctx.GoContext(), conn, cniConfigDir, s.Sudo); err != nil {
		logger.Error("Failed to remove CNI configuration directory (best effort).", "path", cniConfigDir, "error", err)
		lastErr = fmt.Errorf("failed to remove %s: %w (previous error: %v)", cniConfigDir, err, lastErr)
	}

	if lastErr != nil {
		return fmt.Errorf("one or more errors occurred during containerd config cleanup: %w", lastErr)
	}
	logger.Info("Containerd configuration cleanup successful.")
	return nil
}

func (s *CleanupContainerdConfigStep) Rollback(ctx step.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Rollback for CleanupContainerdConfigStep is not applicable.")
	return nil
}

var _ step.Step = (*CleanupContainerdConfigStep)(nil)
