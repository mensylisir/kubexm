package docker

import (
	"bytes"
	"fmt"
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
	criDockerdSocketTemplatePath = "docker/cri-dockerd.socket.tmpl"
	criDockerdSocketFilePath     = common.CriDockerdDefaultSystemdSocketFile
)

type CRIDockerdSocketConfig struct {
	CRISocketPath string
}

type SetupCriDockerdSocketStep struct {
	step.Base
	Config         CRIDockerdSocketConfig
	SocketFilePath string
}

type SetupCriDockerdSocketStepBuilder struct {
	step.Builder[SetupCriDockerdSocketStepBuilder, *SetupCriDockerdSocketStep]
}

func NewSetupCriDockerdSocketStepBuilder(ctx runtime.Context, instanceName string) *SetupCriDockerdSocketStepBuilder {
	s := &SetupCriDockerdSocketStep{
		SocketFilePath: criDockerdSocketFilePath,
		Config: CRIDockerdSocketConfig{
			CRISocketPath: common.CriDockerdSocketPath,
		},
	}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Setup cri-dockerd systemd socket file", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(SetupCriDockerdSocketStepBuilder).Init(s)
	return b
}

func (s *SetupCriDockerdSocketStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *SetupCriDockerdSocketStep) renderSocketFile() (string, error) {
	templateContent, err := templates.Get(criDockerdSocketTemplatePath)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New("cri-dockerd.socket").Parse(templateContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse socket template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s.Config); err != nil {
		return "", fmt.Errorf("failed to execute socket template: %w", err)
	}
	return buf.String(), nil
}

func (s *SetupCriDockerdSocketStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.SocketFilePath)
	if err != nil || !exists {
		return false, err
	}

	currentContent, err := runner.ReadFile(ctx.GoContext(), conn, s.SocketFilePath)
	if err != nil {
		logger.Warn("Failed to read existing cri-dockerd.socket file, will regenerate.", "error", err)
		return false, nil
	}

	expectedContent, err := s.renderSocketFile()
	if err != nil {
		return false, fmt.Errorf("failed to render expected socket file for precheck: %w", err)
	}

	if strings.TrimSpace(string(currentContent)) == strings.TrimSpace(expectedContent) {
		logger.Info("Existing cri-dockerd.socket content matches expected content.")
		return true, nil
	}

	logger.Info("Existing cri-dockerd.socket content does not match. Regeneration is required.")
	return false, nil
}

func (s *SetupCriDockerdSocketStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	socketContent, err := s.renderSocketFile()
	if err != nil {
		return err
	}

	logger.Infof("Writing systemd socket file to %s", s.SocketFilePath)
	if err := runner.WriteFile(ctx.GoContext(), conn, []byte(socketContent), s.SocketFilePath, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to write cri-dockerd.socket: %w", err)
	}

	logger.Info("Successfully wrote cri-dockerd.socket file. 'systemctl daemon-reload' is required to apply changes.")
	return nil
}

func (s *SetupCriDockerdSocketStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}

	logger.Warnf("Rolling back by removing %s", s.SocketFilePath)
	if err := runner.Remove(ctx.GoContext(), conn, s.SocketFilePath, s.Sudo, true); err != nil {
		logger.Error(err, "Failed to remove cri-dockerd.socket during rollback.")
	}
	return nil
}

var _ step.Step = (*SetupCriDockerdSocketStep)(nil)
