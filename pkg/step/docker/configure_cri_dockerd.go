package docker

import (
	"bytes"
	"fmt"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

const (
	CriDockerdServiceTemplatePath = "docker/cri-dockerd.service.tmpl"
)

type ConfigureCriDockerdStep struct {
	step.Base
	TargetPath                  string
	CriDockerdExecStartPath    string
	DockerEndpoint              string
	PodSandboxImage             string
}

type ConfigureCriDockerdStepBuilder struct {
	step.Builder[ConfigureCriDockerdStepBuilder, *ConfigureCriDockerdStep]
}

func NewConfigureCriDockerdStepBuilder(ctx runtime.Context, instanceName string) *ConfigureCriDockerdStepBuilder {
	s := &ConfigureCriDockerdStep{
		TargetPath:                  common.CriDockerdDefaultSystemdFile,
		CriDockerdExecStartPath:    filepath.Join(common.DefaultBinDir, "cri-dockerd"),
		DockerEndpoint:              common.DockerDefaultEndpoint,
		PodSandboxImage:             "",
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install cri-dockerd systemd service from template", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(ConfigureCriDockerdStepBuilder).Init(s)
	return b
}

func (b *ConfigureCriDockerdStepBuilder) WithTargetPath(path string) *ConfigureCriDockerdStepBuilder {
	b.Step.TargetPath = path
	return b
}

func (b *ConfigureCriDockerdStepBuilder) WithCriDockerdExecStartPath(path string) *ConfigureCriDockerdStepBuilder {
	b.Step.CriDockerdExecStartPath = path
	return b
}

func (b *ConfigureCriDockerdStepBuilder) WithDockerEndpoint(endpoint string) *ConfigureCriDockerdStepBuilder {
	b.Step.DockerEndpoint = endpoint
	return b
}

func (b *ConfigureCriDockerdStepBuilder) WithPodSandboxImage(image string) *ConfigureCriDockerdStepBuilder {
	b.Step.PodSandboxImage = image
	return b
}

func (s *ConfigureCriDockerdStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureCriDockerdStep) renderContent() (string, error) {
	tmplStr, err := templates.Get(CriDockerdServiceTemplatePath)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("cri-dockerd.service").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse cri-dockerd service template: %w", err)
	}

	data := struct {
		CriDockerdExecStartPath string
		DockerEndpoint          string
		PodSandboxImage         string
	}{
		CriDockerdExecStartPath: s.CriDockerdExecStartPath,
		DockerEndpoint:          s.DockerEndpoint,
		PodSandboxImage:         s.PodSandboxImage,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render cri-dockerd service template: %w", err)
	}

	return buf.String(), nil
}

func (s *ConfigureCriDockerdStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	expectedContent, err := s.renderContent()
	if err != nil {
		return false, fmt.Errorf("failed to render expected content for precheck: %w", err)
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.TargetPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for service file '%s': %w", s.TargetPath, err)
	}
	if exists {
		remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, s.TargetPath)
		if err != nil {
			logger.Warnf("Service file '%s' exists but failed to read, will overwrite. Error: %v", s.TargetPath, err)
			return false, nil
		}
		if string(remoteContent) == expectedContent {
			logger.Infof("CriDockerd service file '%s' already exists and content matches. Step is done.", s.TargetPath)
			return true, nil
		}
		logger.Infof("CriDockerd service file '%s' exists but content differs. Step needs to run.", s.TargetPath)
		return false, nil
	}

	logger.Infof("CriDockerd service file '%s' does not exist. Installation is required.", s.TargetPath)
	return false, nil
}

func (s *ConfigureCriDockerdStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	content, err := s.renderContent()
	if err != nil {
		return err
	}

	logger.Infof("Writing systemd service file to %s", s.TargetPath)
	err = runner.WriteFile(ctx.GoContext(), conn, []byte(content), s.TargetPath, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write cri-dockerd service file: %w", err)
	}

	logger.Info("Reloading systemd daemon")
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return fmt.Errorf("failed to gather facts for daemon-reload: %w", err)
	}
	if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	return nil
}

func (s *ConfigureCriDockerdStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}

	logger.Warnf("Rolling back by removing: %s", s.TargetPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.TargetPath, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove '%s' during rollback: %v", s.TargetPath, err)
	}

	logger.Info("Reloading systemd daemon after rollback")
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for daemon-reload during rollback: %v", err)
		return nil
	}
	if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
		logger.Errorf("Failed to reload systemd daemon during rollback: %v", err)
	}

	return nil
}
