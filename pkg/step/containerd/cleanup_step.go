package containerd

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// CleanupContainerdStepSpec defines the configuration for cleaning up containerd
type CleanupContainerdStepSpec struct {
	// Common step metadata
	StepMeta spec.StepMeta `json:"stepMeta,omitempty" yaml:"stepMeta,omitempty"`

	// RemoveBinaries indicates whether to remove containerd binaries
	RemoveBinaries bool `json:"removeBinaries,omitempty" yaml:"removeBinaries,omitempty"`

	// RemoveConfig indicates whether to remove configuration files
	RemoveConfig bool `json:"removeConfig,omitempty" yaml:"removeConfig,omitempty"`

	// RemoveData indicates whether to remove containerd data directory
	RemoveData bool `json:"removeData,omitempty" yaml:"removeData,omitempty"`

	// RemoveService indicates whether to remove systemd service file
	RemoveService bool `json:"removeService,omitempty" yaml:"removeService,omitempty"`

	// StopService indicates whether to stop the service before cleanup
	StopService bool `json:"stopService,omitempty" yaml:"stopService,omitempty"`

	// CleanupContainers indicates whether to remove all containers
	CleanupContainers bool `json:"cleanupContainers,omitempty" yaml:"cleanupContainers,omitempty"`

	// CleanupImages indicates whether to remove all images
	CleanupImages bool `json:"cleanupImages,omitempty" yaml:"cleanupImages,omitempty"`

	// BinaryPath is where containerd binaries are located
	BinaryPath string `json:"binaryPath,omitempty" yaml:"binaryPath,omitempty"`

	// ConfigPath is where containerd configuration is located
	ConfigPath string `json:"configPath,omitempty" yaml:"configPath,omitempty"`

	// DataRoot is the containerd data directory
	DataRoot string `json:"dataRoot,omitempty" yaml:"dataRoot,omitempty"`

	// ServicePath is where the systemd service file is located
	ServicePath string `json:"servicePath,omitempty" yaml:"servicePath,omitempty"`

	// Socket is the containerd socket path
	Socket string `json:"socket,omitempty" yaml:"socket,omitempty"`

	// Force indicates whether to force cleanup even if errors occur
	Force bool `json:"force,omitempty" yaml:"force,omitempty"`
}

// CleanupContainerdStep implements the Step interface for cleaning up containerd
type CleanupContainerdStep struct {
	spec CleanupContainerdStepSpec
}

// NewCleanupContainerdStep creates a new CleanupContainerdStep
func NewCleanupContainerdStep(spec CleanupContainerdStepSpec) *CleanupContainerdStep {
	// Set default values
	if spec.BinaryPath == "" {
		spec.BinaryPath = "/usr/local/bin"
	}
	if spec.ConfigPath == "" {
		spec.ConfigPath = "/etc/containerd/config.toml"
	}
	if spec.DataRoot == "" {
		spec.DataRoot = "/var/lib/containerd"
	}
	if spec.ServicePath == "" {
		spec.ServicePath = "/etc/systemd/system/containerd.service"
	}
	if spec.Socket == "" {
		spec.Socket = "/run/containerd/containerd.sock"
	}

	return &CleanupContainerdStep{spec: spec}
}

// Meta returns the step metadata
func (s *CleanupContainerdStep) Meta() *spec.StepMeta {
	return &s.spec.StepMeta
}

// Precheck determines if cleanup is needed
func (s *CleanupContainerdStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()

	// Check if any containerd components exist that need cleanup
	needsCleanup := false

	// Check binaries
	if s.spec.RemoveBinaries {
		binaries := []string{"containerd", "ctr", "containerd-shim", "containerd-shim-runc-v1", "containerd-shim-runc-v2"}
		for _, binary := range binaries {
			binaryPath := filepath.Join(s.spec.BinaryPath, binary)
			exists, err := runner.Exists(ctx.GoContext(), conn, binaryPath)
			if err != nil {
				ctx.GetLogger().Debugf("error checking binary %s: %v", binaryPath, err)
				continue
			}
			if exists {
				needsCleanup = true
				break
			}
		}
	}

	// Check config
	if s.spec.RemoveConfig && !needsCleanup {
		exists, err := runner.Exists(ctx.GoContext(), conn, s.spec.ConfigPath)
		if err == nil && exists {
			needsCleanup = true
		}
	}

	// Check service
	if s.spec.RemoveService && !needsCleanup {
		exists, err := runner.Exists(ctx.GoContext(), conn, s.spec.ServicePath)
		if err == nil && exists {
			needsCleanup = true
		}
	}

	// Check data directory
	if s.spec.RemoveData && !needsCleanup {
		exists, err := runner.Exists(ctx.GoContext(), conn, s.spec.DataRoot)
		if err == nil && exists {
			needsCleanup = true
		}
	}

	if !needsCleanup {
		ctx.GetLogger().Infof("containerd cleanup not needed - no components found")
		return true, nil
	}

	return false, nil
}

// Run executes the containerd cleanup
func (s *CleanupContainerdStep) Run(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Cleaning up containerd on host %s", host.GetName())

	// Step 1: Stop the service if requested
	if s.spec.StopService {
		err = s.stopContainerdService(ctx, runner, conn)
		if err != nil && !s.spec.Force {
			return fmt.Errorf("failed to stop containerd service: %w", err)
		}
		if err != nil {
			logger.Warnf("failed to stop containerd service (continuing due to force flag): %v", err)
		}
	}

	// Step 2: Cleanup containers if requested
	if s.spec.CleanupContainers {
		err = s.cleanupContainers(ctx, runner, conn)
		if err != nil && !s.spec.Force {
			return fmt.Errorf("failed to cleanup containers: %w", err)
		}
		if err != nil {
			logger.Warnf("failed to cleanup containers (continuing due to force flag): %v", err)
		}
	}

	// Step 3: Cleanup images if requested
	if s.spec.CleanupImages {
		err = s.cleanupImages(ctx, runner, conn)
		if err != nil && !s.spec.Force {
			return fmt.Errorf("failed to cleanup images: %w", err)
		}
		if err != nil {
			logger.Warnf("failed to cleanup images (continuing due to force flag): %v", err)
		}
	}

	// Step 4: Remove data directory if requested
	if s.spec.RemoveData {
		err = s.removeDataDirectory(ctx, runner, conn)
		if err != nil && !s.spec.Force {
			return fmt.Errorf("failed to remove data directory: %w", err)
		}
		if err != nil {
			logger.Warnf("failed to remove data directory (continuing due to force flag): %v", err)
		}
	}

	// Step 5: Remove service file if requested
	if s.spec.RemoveService {
		err = s.removeServiceFile(ctx, runner, conn)
		if err != nil && !s.spec.Force {
			return fmt.Errorf("failed to remove service file: %w", err)
		}
		if err != nil {
			logger.Warnf("failed to remove service file (continuing due to force flag): %v", err)
		}
	}

	// Step 6: Remove configuration if requested
	if s.spec.RemoveConfig {
		err = s.removeConfigFiles(ctx, runner, conn)
		if err != nil && !s.spec.Force {
			return fmt.Errorf("failed to remove config files: %w", err)
		}
		if err != nil {
			logger.Warnf("failed to remove config files (continuing due to force flag): %v", err)
		}
	}

	// Step 7: Remove binaries if requested
	if s.spec.RemoveBinaries {
		err = s.removeBinaries(ctx, runner, conn)
		if err != nil && !s.spec.Force {
			return fmt.Errorf("failed to remove binaries: %w", err)
		}
		if err != nil {
			logger.Warnf("failed to remove binaries (continuing due to force flag): %v", err)
		}
	}

	// Step 8: Clean up socket if it exists
	err = s.cleanupSocket(ctx, runner, conn)
	if err != nil && !s.spec.Force {
		return fmt.Errorf("failed to cleanup socket: %w", err)
	}
	if err != nil {
		logger.Warnf("failed to cleanup socket (continuing due to force flag): %v", err)
	}

	logger.Infof("containerd cleanup completed successfully")
	return nil
}

// Rollback is not applicable for cleanup operations
func (s *CleanupContainerdStep) Rollback(ctx step.StepContext, host connector.Host) error {
	// Cleanup operations are generally not reversible
	ctx.GetLogger().Infof("Rollback not applicable for containerd cleanup operation")
	return nil
}

// stopContainerdService stops the containerd service
func (s *CleanupContainerdStep) stopContainerdService(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	// Try to stop the service gracefully
	logger.Infof("Stopping containerd service")
	_, err := runner.Run(ctx.GoContext(), conn, "systemctl stop containerd", false)
	if err != nil {
		logger.Debugf("systemctl stop failed: %v", err)
	}

	// Disable the service
	_, err = runner.Run(ctx.GoContext(), conn, "systemctl disable containerd", false)
	if err != nil {
		logger.Debugf("systemctl disable failed: %v", err)
	}

	// Kill any remaining containerd processes
	killCmd := "pkill -f containerd || true"
	_, err = runner.Run(ctx.GoContext(), conn, killCmd, false)
	if err != nil {
		logger.Debugf("pkill failed: %v", err)
	}

	return nil
}

// cleanupContainers removes all containers
func (s *CleanupContainerdStep) cleanupContainers(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	ctrPath := filepath.Join(s.spec.BinaryPath, "ctr")
	
	// Check if ctr binary exists
	exists, err := runner.Exists(ctx.GoContext(), conn, ctrPath)
	if err != nil || !exists {
		logger.Debugf("ctr binary not found, skipping container cleanup")
		return nil
	}

	logger.Infof("Cleaning up containerd containers")
	
	// List and remove all containers
	cleanupCmd := fmt.Sprintf(`
		%s containers list -q | xargs -r %s containers rm || true
		%s containers list -q -n k8s.io | xargs -r %s containers rm -n k8s.io || true
	`, ctrPath, ctrPath, ctrPath, ctrPath)
	
	_, err = runner.Run(ctx.GoContext(), conn, cleanupCmd, false)
	if err != nil {
		return fmt.Errorf("failed to cleanup containers: %w", err)
	}

	return nil
}

// cleanupImages removes all images
func (s *CleanupContainerdStep) cleanupImages(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	ctrPath := filepath.Join(s.spec.BinaryPath, "ctr")
	
	// Check if ctr binary exists
	exists, err := runner.Exists(ctx.GoContext(), conn, ctrPath)
	if err != nil || !exists {
		logger.Debugf("ctr binary not found, skipping image cleanup")
		return nil
	}

	logger.Infof("Cleaning up containerd images")
	
	// List and remove all images
	cleanupCmd := fmt.Sprintf(`
		%s images list -q | xargs -r %s images rm || true
		%s images list -q -n k8s.io | xargs -r %s images rm -n k8s.io || true
	`, ctrPath, ctrPath, ctrPath, ctrPath)
	
	_, err = runner.Run(ctx.GoContext(), conn, cleanupCmd, false)
	if err != nil {
		return fmt.Errorf("failed to cleanup images: %w", err)
	}

	return nil
}

// removeDataDirectory removes the containerd data directory
func (s *CleanupContainerdStep) removeDataDirectory(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	exists, err := runner.Exists(ctx.GoContext(), conn, s.spec.DataRoot)
	if err != nil || !exists {
		logger.Debugf("Data directory %s does not exist", s.spec.DataRoot)
		return nil
	}

	logger.Infof("Removing containerd data directory: %s", s.spec.DataRoot)
	
	rmCmd := fmt.Sprintf("rm -rf %s", s.spec.DataRoot)
	_, err = runner.Run(ctx.GoContext(), conn, rmCmd, false)
	if err != nil {
		return fmt.Errorf("failed to remove data directory: %w", err)
	}

	return nil
}

// removeServiceFile removes the systemd service file
func (s *CleanupContainerdStep) removeServiceFile(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	exists, err := runner.Exists(ctx.GoContext(), conn, s.spec.ServicePath)
	if err != nil || !exists {
		logger.Debugf("Service file %s does not exist", s.spec.ServicePath)
		return nil
	}

	logger.Infof("Removing containerd service file: %s", s.spec.ServicePath)
	
	rmCmd := fmt.Sprintf("rm -f %s", s.spec.ServicePath)
	_, err = runner.Run(ctx.GoContext(), conn, rmCmd, false)
	if err != nil {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	// Reload systemd daemon
	_, err = runner.Run(ctx.GoContext(), conn, "systemctl daemon-reload", false)
	if err != nil {
		logger.Warnf("failed to reload systemd daemon: %v", err)
	}

	return nil
}

// removeConfigFiles removes containerd configuration files
func (s *CleanupContainerdStep) removeConfigFiles(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	configDir := filepath.Dir(s.spec.ConfigPath)
	
	exists, err := runner.Exists(ctx.GoContext(), conn, configDir)
	if err != nil || !exists {
		logger.Debugf("Config directory %s does not exist", configDir)
		return nil
	}

	logger.Infof("Removing containerd configuration directory: %s", configDir)
	
	rmCmd := fmt.Sprintf("rm -rf %s", configDir)
	_, err = runner.Run(ctx.GoContext(), conn, rmCmd, false)
	if err != nil {
		return fmt.Errorf("failed to remove config directory: %w", err)
	}

	return nil
}

// removeBinaries removes containerd binaries
func (s *CleanupContainerdStep) removeBinaries(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	binaries := []string{"containerd", "ctr", "containerd-shim", "containerd-shim-runc-v1", "containerd-shim-runc-v2"}
	
	for _, binary := range binaries {
		binaryPath := filepath.Join(s.spec.BinaryPath, binary)
		
		exists, err := runner.Exists(ctx.GoContext(), conn, binaryPath)
		if err != nil || !exists {
			continue
		}

		logger.Infof("Removing containerd binary: %s", binaryPath)
		
		rmCmd := fmt.Sprintf("rm -f %s", binaryPath)
		_, err = runner.Run(ctx.GoContext(), conn, rmCmd, false)
		if err != nil {
			return fmt.Errorf("failed to remove binary %s: %w", binaryPath, err)
		}
	}

	return nil
}

// cleanupSocket removes the containerd socket
func (s *CleanupContainerdStep) cleanupSocket(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()
	
	socketDir := filepath.Dir(s.spec.Socket)
	
	exists, err := runner.Exists(ctx.GoContext(), conn, socketDir)
	if err != nil || !exists {
		logger.Debugf("Socket directory %s does not exist", socketDir)
		return nil
	}

	logger.Infof("Cleaning up containerd socket directory: %s", socketDir)
	
	rmCmd := fmt.Sprintf("rm -rf %s", socketDir)
	_, err = runner.Run(ctx.GoContext(), conn, rmCmd, false)
	if err != nil {
		return fmt.Errorf("failed to cleanup socket directory: %w", err)
	}

	return nil
}