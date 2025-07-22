package docker

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
)

const DefaultDockerDaemonJSONPath = "/etc/docker/daemon.json"

type DockerDaemonConfig struct {
	ExecOpts           []string          `json:"exec-opts,omitempty"`
	LogDriver          string            `json:"log-driver,omitempty"`
	LogOpts            map[string]string `json:"log-opts,omitempty"`
	StorageDriver      string            `json:"storage-driver,omitempty"`
	RegistryMirrors    []string          `json:"registry-mirrors,omitempty"`
	InsecureRegistries []string          `json:"insecure-registries,omitempty"`
	CgroupDriver       string            `json:"cgroupdriver,omitempty"`
}

type ConfigureDockerStep struct {
	step.Base
	Config         DockerDaemonConfig
	ConfigFilePath string
}

type ConfigureDockerStepBuilder struct {
	step.Builder[ConfigureDockerStepBuilder, *ConfigureDockerStep]
}

func NewConfigureDockerStepBuilder(ctx runtime.Context, instanceName string) *ConfigureDockerStepBuilder {
	s := &ConfigureDockerStep{
		ConfigFilePath: DefaultDockerDaemonJSONPath,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Docker's daemon.json", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(ConfigureDockerStepBuilder).Init(s)
	return b
}

func (b *ConfigureDockerStepBuilder) WithConfig(config DockerDaemonConfig) *ConfigureDockerStepBuilder {
	b.Step.Config = config
	return b
}

func (b *ConfigureDockerStepBuilder) WithConfigFilePath(path string) *ConfigureDockerStepBuilder {
	b.Step.ConfigFilePath = path
	return b
}

func (s *ConfigureDockerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureDockerStep) renderExpectedConfig() (string, error) {
	jsonData, err := json.MarshalIndent(s.Config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal DockerDaemonConfig to JSON: %w", err)
	}
	return string(jsonData), nil
}

func (s *ConfigureDockerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.ConfigFilePath)
	if err != nil {
		logger.Warn("Failed to check for existing daemon.json, will attempt generation.", "error", err)
		return false, nil
	}
	if !exists {
		logger.Info("daemon.json does not exist.", "path", s.ConfigFilePath)
		return false, nil
	}

	logger.Info("daemon.json exists. Verifying content...")
	currentContentBytes, err := runner.ReadFile(ctx.GoContext(), conn, s.ConfigFilePath)
	if err != nil {
		logger.Warn("Failed to read existing daemon.json for comparison, will regenerate.", "error", err)
		return false, nil
	}

	expectedContent, err := s.renderExpectedConfig()
	if err != nil {
		logger.Error("Failed to render expected daemon.json for comparison.", err)
		return false, fmt.Errorf("failed to render expected config: %w", err)
	}

	if strings.TrimSpace(string(currentContentBytes)) == strings.TrimSpace(expectedContent) {
		logger.Info("Existing daemon.json content matches expected content.")
		return true, nil
	}

	logger.Info("Existing daemon.json content does not match. Will regenerate.")
	return false, nil
}

func (s *ConfigureDockerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	configContent, err := s.renderExpectedConfig()
	if err != nil {
		return err
	}

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	remoteDir := filepath.Dir(s.ConfigFilePath)
	logger.Info("Ensuring remote directory for daemon.json exists.", "path", remoteDir)
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create remote directory %s for daemon.json: %w", remoteDir, err)
	}

	logger.Info("Writing Docker daemon.json file.", "path", s.ConfigFilePath)
	err = runner.WriteFile(ctx.GoContext(), conn, []byte(configContent), s.ConfigFilePath, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write Docker daemon.json to %s: %w", s.ConfigFilePath, err)
	}

	logger.Info("Docker daemon.json file generated successfully. Restart Docker for changes to take effect.")
	return nil
}

func (s *ConfigureDockerStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	logger.Info("Attempting to remove Docker daemon.json file for rollback.", "path", s.ConfigFilePath)

	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error(err, "Failed to get connector for rollback.")
		return nil
	}

	if err := runner.Remove(ctx.GoContext(), conn, s.ConfigFilePath, s.Sudo, false); err != nil {
		logger.Warn("Failed to remove Docker daemon.json during rollback (best effort).", "error", err)
	} else {
		logger.Info("Successfully removed Docker daemon.json (if it existed).")
	}
	return nil
}
