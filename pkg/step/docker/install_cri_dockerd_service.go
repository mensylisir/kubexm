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
	criDockerdServiceTemplatePath = "docker/cri-dockerd.service.tmpl"
	criDockerdServiceFilePath     = common.CniDockerdSystemdFile
)

type CRIDockerdServiceConfig struct {
	ExecStartPath   string
	CRISocket       string
	CNIBinDir       string
	CNIConfDir      string
	PodSandboxImage string
}

type SetupCriDockerdServiceStep struct {
	step.Base
	Config          CRIDockerdServiceConfig
	ServiceFilePath string
}

type SetupCriDockerdServiceStepBuilder struct {
	step.Builder[SetupCriDockerdServiceStepBuilder, *SetupCriDockerdServiceStep]
}

func NewSetupCriDockerdServiceStepBuilder(ctx runtime.Context, instanceName string) *SetupCriDockerdServiceStepBuilder {

	s := &SetupCriDockerdServiceStep{
		ServiceFilePath: criDockerdServiceFilePath,
		Config: CRIDockerdServiceConfig{
			ExecStartPath: filepath.Join(common.DefaultBinDir, "cri-dockerd"),
			CRISocket:     common.CriDockerdSocketPath,
			CNIBinDir:     common.DefaultCNIBin,
			CNIConfDir:    common.DefaultCNIConfDirTarget,
			//PodSandboxImage: podSandboxImage,
		},
	}

	if s.Config.PodSandboxImage == "" {
		pauseImage := helpers.GetImage(ctx, "pause")
		s.Config.PodSandboxImage = pauseImage.ImageName()
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Setup cri-dockerd systemd service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	cfg := ctx.GetClusterConfig().Spec
	containerdCfg := cfg.Kubernetes.ContainerRuntime.Docker
	if containerdCfg.Pause != "" {
		s.Config.PodSandboxImage = containerdCfg.Pause
	}
	b := new(SetupCriDockerdServiceStepBuilder).Init(s)
	return b
}

func (s *SetupCriDockerdServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *SetupCriDockerdServiceStep) renderServiceFile() (string, error) {
	templateContent, err := templates.Get(criDockerdServiceTemplatePath)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New("cri-dockerd.service").Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse service template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s.Config); err != nil {
		return "", fmt.Errorf("failed to execute service template: %w", err)
	}
	return buf.String(), nil
}

func (s *SetupCriDockerdServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.ServiceFilePath)
	if err != nil || !exists {
		return false, err
	}

	currentContent, err := runner.ReadFile(ctx.GoContext(), conn, s.ServiceFilePath)
	if err != nil {
		logger.Warn("Failed to read existing cri-dockerd.service file, will regenerate.", "error", err)
		return false, nil
	}

	expectedContent, err := s.renderServiceFile()
	if err != nil {
		return false, fmt.Errorf("failed to render expected service file for precheck: %w", err)
	}

	if strings.TrimSpace(string(currentContent)) == strings.TrimSpace(expectedContent) {
		logger.Info("Existing cri-dockerd.service content matches expected content.")
		return true, nil
	}

	logger.Info("Existing cri-dockerd.service content does not match. Regeneration is required.")
	return false, nil
}

func (s *SetupCriDockerdServiceStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	serviceContent, err := s.renderServiceFile()
	if err != nil {
		return err
	}

	logger.Infof("Writing systemd service file to %s", s.ServiceFilePath)
	if err := runner.WriteFile(ctx.GoContext(), conn, []byte(serviceContent), s.ServiceFilePath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to write cri-dockerd.service: %w", err)
	}

	logger.Info("Successfully wrote cri-dockerd.service file. 'systemctl daemon-reload' is required to apply changes.")
	return nil
}

func (s *SetupCriDockerdServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing %s", s.ServiceFilePath)
	if err := runner.Remove(ctx.GoContext(), conn, s.ServiceFilePath, s.Sudo, true); err != nil {
		logger.Error(err, "Failed to remove cri-dockerd.service during rollback.")
	}
	return nil
}

var _ step.Step = (*SetupCriDockerdServiceStep)(nil)
