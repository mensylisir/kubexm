package network

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runner"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

// ConfigureCNIStepSpec defines the configuration for setting up CNI
type ConfigureCNIStepSpec struct {
	// Common step metadata
	StepMeta spec.StepMeta `json:"stepMeta,omitempty" yaml:"stepMeta,omitempty"`

	// CNIBinDir specifies the CNI binary directory
	CNIBinDir string `json:"cniBinDir,omitempty" yaml:"cniBinDir,omitempty"`

	// CNIConfigDir specifies the CNI configuration directory
	CNIConfigDir string `json:"cniConfigDir,omitempty" yaml:"cniConfigDir,omitempty"`

	// CNIVersion specifies the CNI plugins version to install
	CNIVersion string `json:"cniVersion,omitempty" yaml:"cniVersion,omitempty"`

	// Arch specifies the architecture (amd64, arm64, etc.)
	Arch string `json:"arch,omitempty" yaml:"arch,omitempty"`

	// DownloadURL specifies the base URL for downloading CNI plugins
	DownloadURL string `json:"downloadURL,omitempty" yaml:"downloadURL,omitempty"`

	// InstallPlugins indicates whether to install CNI plugins
	InstallPlugins bool `json:"installPlugins,omitempty" yaml:"installPlugins,omitempty"`

	// PluginsList specifies which plugins to install (if empty, installs all)
	PluginsList []string `json:"pluginsList,omitempty" yaml:"pluginsList,omitempty"`

	// CreateLoopbackConfig indicates whether to create loopback configuration
	CreateLoopbackConfig bool `json:"createLoopbackConfig,omitempty" yaml:"createLoopbackConfig,omitempty"`

	// NetworkName specifies the network name for configurations
	NetworkName string `json:"networkName,omitempty" yaml:"networkName,omitempty"`

	// CustomConfigs allows adding custom CNI configurations
	CustomConfigs map[string]string `json:"customConfigs,omitempty" yaml:"customConfigs,omitempty"`
}

// ConfigureCNIStep implements the Step interface for configuring CNI
type ConfigureCNIStep struct {
	spec ConfigureCNIStepSpec
}

// NewConfigureCNIStep creates a new ConfigureCNIStep
func NewConfigureCNIStep(spec ConfigureCNIStepSpec) *ConfigureCNIStep {
	// Set default values
	if spec.CNIBinDir == "" {
		spec.CNIBinDir = "/opt/cni/bin"
	}
	if spec.CNIConfigDir == "" {
		spec.CNIConfigDir = "/etc/cni/net.d"
	}
	if spec.CNIVersion == "" {
		spec.CNIVersion = "v1.3.0"
	}
	if spec.Arch == "" {
		spec.Arch = "amd64"
	}
	if spec.DownloadURL == "" {
		spec.DownloadURL = "https://github.com/containernetworking/plugins/releases/download/%s/cni-plugins-linux-%s-%s.tgz"
	}
	if spec.NetworkName == "" {
		spec.NetworkName = "cni0"
	}
	// Install plugins by default
	spec.InstallPlugins = true

	return &ConfigureCNIStep{spec: spec}
}

// Meta returns the step metadata
func (s *ConfigureCNIStep) Meta() *spec.StepMeta {
	return &s.spec.StepMeta
}

// Precheck determines if CNI is already configured correctly
func (s *ConfigureCNIStep) Precheck(ctx step.StepContext, host connector.Host) (bool, error) {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()

	// Check if CNI directories exist
	binDirExists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.spec.CNIBinDir)
	if err != nil {
		return false, fmt.Errorf("failed to check CNI bin directory: %w", err)
	}

	configDirExists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.spec.CNIConfigDir)
	if err != nil {
		return false, fmt.Errorf("failed to check CNI config directory: %w", err)
	}

	if !binDirExists || !configDirExists {
		ctx.GetLogger().Infof("CNI directories not found")
		return false, nil
	}

	// Check if CNI plugins are installed (if required)
	if s.spec.InstallPlugins {
		// Check for essential plugins
		essentialPlugins := []string{"bridge", "host-local", "loopback"}
		for _, plugin := range essentialPlugins {
			pluginPath := filepath.Join(s.spec.CNIBinDir, plugin)
			exists, err := runnerSvc.Exists(ctx.GoContext(), conn, pluginPath)
			if err != nil || !exists {
				ctx.GetLogger().Infof("CNI plugin %s not found", plugin)
				return false, nil
			}
		}
	}

	ctx.GetLogger().Infof("CNI appears to be configured correctly")
	return true, nil
}

// Run executes the CNI configuration
func (s *ConfigureCNIStep) Run(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Configuring CNI on host %s", host.GetName())

	// Create CNI directories
	err = s.createCNIDirectories(ctx, runner, conn)
	if err != nil {
		return fmt.Errorf("failed to create CNI directories: %w", err)
	}

	// Install CNI plugins if requested
	if s.spec.InstallPlugins {
		err = s.installCNIPlugins(ctx, runner, conn)
		if err != nil {
			return fmt.Errorf("failed to install CNI plugins: %w", err)
		}
	}

	// Create loopback configuration if requested
	if s.spec.CreateLoopbackConfig {
		err = s.createLoopbackConfig(ctx, runner, conn)
		if err != nil {
			return fmt.Errorf("failed to create loopback config: %w", err)
		}
	}

	// Create custom configurations
	err = s.createCustomConfigs(ctx, runner, conn)
	if err != nil {
		return fmt.Errorf("failed to create custom configs: %w", err)
	}

	logger.Infof("CNI configuration completed successfully")
	return nil
}

// Rollback removes the CNI configuration
func (s *ConfigureCNIStep) Rollback(ctx step.StepContext, host connector.Host) error {
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector for host %s: %w", host.GetName(), err)
	}

	runner := ctx.GetRunner()
	logger := ctx.GetLogger()

	logger.Infof("Rolling back CNI configuration on host %s", host.GetName())

	// Remove CNI configuration directory
	configDirExists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.spec.CNIConfigDir)
	if err == nil && configDirExists {
		rmCmd := fmt.Sprintf("rm -rf %s", s.spec.CNIConfigDir)
		_, err = runnerSvc.Run(ctx.GoContext(), conn, rmCmd)
		if err != nil {
			logger.Warnf("failed to remove CNI config directory: %v", err)
		}
	}

	// Remove CNI binary directory if we installed plugins
	if s.spec.InstallPlugins {
		binDirExists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.spec.CNIBinDir)
		if err == nil && binDirExists {
			rmCmd := fmt.Sprintf("rm -rf %s", s.spec.CNIBinDir)
			_, err = runnerSvc.Run(ctx.GoContext(), conn, rmCmd)
			if err != nil {
				logger.Warnf("failed to remove CNI bin directory: %v", err)
			}
		}
	}

	logger.Infof("CNI configuration rollback completed")
	return nil
}

// createCNIDirectories creates the required CNI directories
func (s *ConfigureCNIStep) createCNIDirectories(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()

	// Create CNI bin directory
	logger.Infof("Creating CNI bin directory: %s", s.spec.CNIBinDir)
	err := runner.Mkdirp(ctx.GoContext(), conn, s.spec.CNIBinDir, "0755", true)
	if err != nil {
		return fmt.Errorf("failed to create CNI bin directory: %w", err)
	}

	// Create CNI config directory
	logger.Infof("Creating CNI config directory: %s", s.spec.CNIConfigDir)
	err = runner.Mkdirp(ctx.GoContext(), conn, s.spec.CNIConfigDir, "0755", true)
	if err != nil {
		return fmt.Errorf("failed to create CNI config directory: %w", err)
	}

	return nil
}

// installCNIPlugins downloads and installs CNI plugins
func (s *ConfigureCNIStep) installCNIPlugins(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()

	downloadURL := fmt.Sprintf(s.spec.DownloadURL, s.spec.CNIVersion, s.spec.Arch, s.spec.CNIVersion)
	downloadPath := "/tmp/cni-plugins.tgz"

	logger.Infof("Downloading CNI plugins from %s", downloadURL)

	// Download CNI plugins
	err := runner.DownloadFile(ctx.GoContext(), conn, downloadURL, downloadPath, 0644)
	if err != nil {
		return fmt.Errorf("failed to download CNI plugins: %w", err)
	}

	// Extract CNI plugins
	logger.Infof("Extracting CNI plugins to %s", s.spec.CNIBinDir)
	extractCmd := fmt.Sprintf("tar -xzf %s -C %s", downloadPath, s.spec.CNIBinDir)
	output, err := runnerSvc.Run(ctx.GoContext(), conn, extractCmd)
	if err != nil {
		return fmt.Errorf("failed to extract CNI plugins: %w\nOutput: %s", err, output)
	}

	// Set executable permissions
	chmodCmd := fmt.Sprintf("chmod +x %s/*", s.spec.CNIBinDir)
	_, err = runnerSvc.Run(ctx.GoContext(), conn, chmodCmd)
	if err != nil {
		return fmt.Errorf("failed to set executable permissions: %w", err)
	}

	// Clean up downloaded file
	rmCmd := fmt.Sprintf("rm -f %s", downloadPath)
	_, err = runnerSvc.Run(ctx.GoContext(), conn, rmCmd)
	if err != nil {
		logger.Warnf("failed to remove downloaded file: %v", err)
	}

	logger.Infof("CNI plugins installed successfully")
	return nil
}

// createLoopbackConfig creates the loopback CNI configuration
func (s *ConfigureCNIStep) createLoopbackConfig(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	logger := ctx.GetLogger()

	loopbackConfig := `{
    "cniVersion": "0.3.1",
    "name": "lo",
    "type": "loopback"
}`

	configPath := filepath.Join(s.spec.CNIConfigDir, "99-loopback.conf")
	logger.Infof("Creating loopback configuration: %s", configPath)

	err := runner.WriteFile(ctx.GoContext(), conn, loopbackConfig, configPath, "0644", true)
	if err != nil {
		return fmt.Errorf("failed to write loopback config: %w", err)
	}

	return nil
}

// createCustomConfigs creates custom CNI configurations
func (s *ConfigureCNIStep) createCustomConfigs(ctx step.StepContext, runner runner.Runner, conn connector.Connector) error {
	if len(s.spec.CustomConfigs) == 0 {
		return nil
	}

	logger := ctx.GetLogger()

	for name, config := range s.spec.CustomConfigs {
		configPath := filepath.Join(s.spec.CNIConfigDir, name)
		logger.Infof("Creating custom CNI configuration: %s", configPath)

		err := runner.WriteFile(ctx.GoContext(), conn, config, configPath, "0644", true)
		if err != nil {
			return fmt.Errorf("failed to write custom config %s: %w", name, err)
		}
	}

	return nil
}