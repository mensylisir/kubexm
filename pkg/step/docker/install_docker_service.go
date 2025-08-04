package docker

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

const (
	dockerServiceFilePath     = common.DockerDefaultSystemdFile
	dockerServiceTemplatePath = "docker/docker.service.tmpl"
)

type ServiceConfig struct {
	ExecStart string
}

type SetupDockerServiceStep struct {
	step.Base
	Config          ServiceConfig
	ServiceFilePath string
}

type SetupDockerServiceStepBuilder struct {
	step.Builder[SetupDockerServiceStepBuilder, *SetupDockerServiceStep]
}

func NewSetupDockerServiceStepBuilder(ctx runtime.Context, instanceName string) *SetupDockerServiceStepBuilder {
	execStartPath := filepath.Join(common.DefaultBinDir, "dockerd")

	s := &SetupDockerServiceStep{
		ServiceFilePath: dockerServiceFilePath,
		Config: ServiceConfig{
			ExecStart: execStartPath,
		},
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Setup Docker systemd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(SetupDockerServiceStepBuilder).Init(s)
	return b
}

func (s *SetupDockerServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *SetupDockerServiceStep) renderServiceFile() (string, error) {
	templateContent, err := templates.Get(dockerServiceTemplatePath)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("docker.service").Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse docker.service template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s.Config); err != nil {
		return "", fmt.Errorf("failed to execute docker.service template: %w", err)
	}
	return buf.String(), nil
}

func (s *SetupDockerServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.ServiceFilePath)
	if err != nil || !exists {
		return false, nil
	}

	currentContent, err := runner.ReadFile(ctx.GoContext(), conn, s.ServiceFilePath)
	if err != nil {
		logger.Warn("Failed to read existing docker.service file, will regenerate.", "error", err)
		return false, nil
	}

	expectedContent, err := s.renderServiceFile()
	if err != nil {
		return false, fmt.Errorf("failed to render expected service file for precheck: %w", err)
	}

	if strings.TrimSpace(string(currentContent)) == strings.TrimSpace(expectedContent) {
		logger.Info("Existing docker.service content matches expected content.")
		return true, nil
	}

	logger.Info("Existing docker.service content does not match. Regeneration is required.")
	return false, nil
}

func (s *SetupDockerServiceStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	//runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	serviceContent, err := s.renderServiceFile()
	if err != nil {
		return err
	}

	logger.Info("Writing Docker systemd service file.", "path", s.ServiceFilePath)
	err = helpers.WriteContentToRemote(ctx, conn, serviceContent, s.ServiceFilePath, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write docker.service to %s: %w", s.ServiceFilePath, err)
	}

	logger.Info("Successfully wrote Docker service file. Running 'systemctl daemon-reload' is required next.")
	return nil
}

func (s *SetupDockerServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing %s", s.ServiceFilePath)
	if err := runner.Remove(ctx.GoContext(), conn, s.ServiceFilePath, s.Sudo, true); err != nil {
		logger.Error(err, "Failed to remove docker.service during rollback.")
	}
	logger.Info("Running 'systemctl daemon-reload' after rollback.")
	if _, _, err := runner.OriginRun(ctx.GoContext(), conn, "systemctl daemon-reload", s.Sudo); err != nil {
		logger.Warn("Failed to run 'systemctl daemon-reload' during rollback.", "error", err)
	}
	return nil
}

var _ step.Step = (*SetupDockerServiceStep)(nil)
