package docker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const DefaultDockerDaemonJSONPath = "/etc/docker/daemon.json"

// DockerDaemonConfig represents the structure of daemon.json.
// Using a struct helps in managing and marshalling the JSON.
type DockerDaemonConfig struct {
	ExecOpts        []string          `json:"exec-opts,omitempty"`
	LogDriver       string            `json:"log-driver,omitempty"`
	LogOpts         map[string]string `json:"log-opts,omitempty"`
	StorageDriver   string            `json:"storage-driver,omitempty"`
	RegistryMirrors []string          `json:"registry-mirrors,omitempty"`
	InsecureRegistries []string       `json:"insecure-registries,omitempty"`
	// Add other fields as needed, e.g., CgroupParent, Bridge, Bip, etc.
	CgroupDriver    string            `json:"cgroupdriver,omitempty"` // For older Docker versions, new ones use "exec-opts": ["native.cgroupdriver=systemd"]
}

// GenerateDockerDaemonJSONStep creates or updates the /etc/docker/daemon.json file.
type GenerateDockerDaemonJSONStep struct {
	meta           spec.StepMeta
	Config         DockerDaemonConfig
	ConfigFilePath string
	Sudo           bool
}

// NewGenerateDockerDaemonJSONStep creates a new GenerateDockerDaemonJSONStep.
func NewGenerateDockerDaemonJSONStep(instanceName string, config DockerDaemonConfig, configFilePath string, sudo bool) step.Step {
	name := instanceName
	if name == "" {
		name = "GenerateDockerDaemonJSON"
	}
	cfgPath := configFilePath
	if cfgPath == "" {
		cfgPath = DefaultDockerDaemonJSONPath
	}

	// Apply some defaults if not provided by the task
	if config.LogDriver == "" {
		config.LogDriver = "json-file"
	}
	if config.LogOpts == nil {
		config.LogOpts = map[string]string{"max-size": "100m"}
	}
	if config.StorageDriver == "" {
		// Docker usually auto-detects a good storage driver.
		// "overlay2" is common on modern kernels.
		// Explicitly setting it can be useful for consistency.
		// config.StorageDriver = "overlay2"
	}

	// Ensure cgroupdriver is set correctly via exec-opts for newer Docker versions
	// or directly for older ones. For Kubernetes, "systemd" is required.
	cgroupDriverIsSetInExecOpts := false
	for _, opt := range config.ExecOpts {
		if strings.Contains(opt, "native.cgroupdriver=systemd") {
			cgroupDriverIsSetInExecOpts = true
			break
		}
	}
	if !cgroupDriverIsSetInExecOpts && config.CgroupDriver == "" {
		// Prefer exec-opts for modern Docker, but ensure systemd is specified.
		config.ExecOpts = append(config.ExecOpts, "native.cgroupdriver=systemd")
	} else if config.CgroupDriver != "" && config.CgroupDriver != "systemd" {
		// Warn or error if a non-systemd cgroupdriver is explicitly set directly,
		// as K8s usually needs systemd.
		// For now, assume the input `config` is what the user wants.
	}


	return &GenerateDockerDaemonJSONStep{
		meta: spec.StepMeta{
			Name:        name,
			Description: fmt.Sprintf("Generates or updates Docker's daemon.json at %s.", cfgPath),
		},
		Config:         config,
		ConfigFilePath: cfgPath,
		Sudo:           true, // Writing to /etc/docker usually requires sudo
	}
}

func (s *GenerateDockerDaemonJSONStep) Meta() *spec.StepMeta {
	return &s.meta
}

func (s *GenerateDockerDaemonJSONStep) renderExpectedConfig() (string, error) {
	jsonData, err := json.MarshalIndent(s.Config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal DockerDaemonConfig to JSON: %w", err)
	}
	return string(jsonData), nil
}

func (s *GenerateDockerDaemonJSONStep) Precheck(ctx runtime.StepContext, host connector.Host) (bool, error) {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Precheck")
	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return false, fmt.Errorf("failed to get connector: %w", err)
	}

	exists, err := runnerSvc.Exists(ctx.GoContext(), conn, s.ConfigFilePath)
	if err != nil {
		logger.Warn("Failed to check for existing daemon.json, will attempt generation.", "error", err)
		return false, nil
	}
	if !exists {
		logger.Info("daemon.json does not exist.", "path", s.ConfigFilePath)
		return false, nil
	}

	logger.Info("daemon.json exists. Verifying content...")
	currentContentBytes, err := runnerSvc.ReadFile(ctx.GoContext(), conn, s.ConfigFilePath)
	if err != nil {
		logger.Warn("Failed to read existing daemon.json for comparison, will regenerate.", "error", err)
		return false, nil
	}

	expectedContent, err := s.renderExpectedConfig()
	if err != nil {
		logger.Error("Failed to render expected daemon.json for comparison.", "error", err)
		return false, fmt.Errorf("failed to render expected config: %w", err)
	}

	// Basic string comparison after normalizing (e.g. unmarshalling and re-marshalling current content, or careful string ops)
	// For simplicity, direct string comparison after trim. More robust would be to unmarshal current and compare structs.
	currentContent := strings.TrimSpace(string(currentContentBytes))
	if currentContent == strings.TrimSpace(expectedContent) {
		logger.Info("Existing daemon.json content matches expected content.")
		return true, nil
	}

	logger.Info("Existing daemon.json content does not match. Will regenerate.")
	// For debugging:
	// logger.Debug("Current content", "value", currentContent)
	// logger.Debug("Expected content", "value", expectedContent)
	return false, nil
}

func (s *GenerateDockerDaemonJSONStep) Run(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Run")

	configContent, err := s.renderExpectedConfig()
	if err != nil {
		return err
	}

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		return fmt.Errorf("failed to get connector: %w", err)
	}

	remoteDir := filepath.Dir(s.ConfigFilePath)
	logger.Info("Ensuring remote directory for daemon.json exists.", "path", remoteDir)
	if err := runnerSvc.Mkdirp(ctx.GoContext(), conn, remoteDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote directory %s for daemon.json: %w", remoteDir, err)
	}

	logger.Info("Writing Docker daemon.json file.", "path", s.ConfigFilePath)
	err = runnerSvc.WriteFile(ctx.GoContext(), conn, []byte(configContent), s.ConfigFilePath, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write Docker daemon.json to %s: %w", s.ConfigFilePath, err)
	}

	logger.Info("Docker daemon.json file generated successfully. Restart Docker for changes to take effect.")
	return nil
}

func (s *GenerateDockerDaemonJSONStep) Rollback(ctx runtime.StepContext, host connector.Host) error {
	logger := ctx.GetLogger().With("step", s.meta.Name, "host", host.GetName(), "phase", "Rollback")
	logger.Info("Attempting to remove Docker daemon.json file for rollback.", "path", s.ConfigFilePath)

	runnerSvc := ctx.GetRunner()
	conn, err := ctx.GetConnectorForHost(host)
	if err != nil {
		logger.Error("Failed to get connector for rollback.", "error", err)
		return nil
	}

	if err := runnerSvc.Remove(ctx.GoContext(), conn, s.ConfigFilePath, s.Sudo); err != nil {
		logger.Warn("Failed to remove Docker daemon.json during rollback (best effort).", "error", err)
	} else {
		logger.Info("Successfully removed Docker daemon.json (if it existed).")
	}
	return nil
}

var _ step.Step = (*GenerateDockerDaemonJSONStep)(nil)
