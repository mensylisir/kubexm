package crio

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
)

const crictlConfigTemplate = `runtime-endpoint: "{{ .Endpoint }}"
image-endpoint: "{{ .Endpoint }}"
timeout: 10
debug: false
`

type crictlTemplateData struct {
	Endpoint string
}

type ConfigureCrictlStep struct {
	step.Base
	Data       crictlTemplateData
	TargetPath string
}

type ConfigureCrictlStepBuilder struct {
	step.Builder[ConfigureCrictlStepBuilder, *ConfigureCrictlStep]
}

func NewConfigureCrictlStepBuilder(ctx runtime.Context, instanceName string) *ConfigureCrictlStepBuilder {
	cfg := ctx.GetClusterConfig().Spec

	data := crictlTemplateData{
		Endpoint: common.CRIODefaultEndpoint,
	}

	if cfg.Kubernetes.ContainerRuntime != nil && cfg.Kubernetes.ContainerRuntime.Crio != nil {
		userCfg := cfg.Kubernetes.ContainerRuntime.Crio
		if userCfg.Endpoint != "" {
			data.Endpoint = userCfg.Endpoint
		}
	}

	s := &ConfigureCrictlStep{
		Data:       data,
		TargetPath: common.CrictlDefaultConfigFile,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure crictl CLI", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(ConfigureCrictlStepBuilder).Init(s)
	return b
}

func (s *ConfigureCrictlStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureCrictlStep) renderContent() (string, error) {
	tmpl, err := template.New("crictl.yaml").Parse(crictlConfigTemplate)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s.Data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (s *ConfigureCrictlStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		return false, fmt.Errorf("failed to check for config file '%s': %w", s.TargetPath, err)
	}

	if exists {
		remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, s.TargetPath)
		if err != nil {
			logger.Warn(err, "Config file exists but failed to read, will overwrite.", "path", s.TargetPath)
			return false, nil
		}
		if string(remoteContent) == expectedContent {
			logger.Info("crictl config file already exists and content matches. Step is done.", "path", s.TargetPath)
			return true, nil
		}
		logger.Info("crictl config file exists but content differs. Step needs to run.", "path", s.TargetPath)
		return false, nil
	}

	logger.Info("crictl config file does not exist. Configuration is required.", "path", s.TargetPath)
	return false, nil
}

func (s *ConfigureCrictlStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	targetDir := filepath.Dir(s.TargetPath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, targetDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create crictl config directory: %w", err)
	}

	content, err := s.renderContent()
	if err != nil {
		return err
	}

	logger.Info("Writing crictl config file.", "path", s.TargetPath)
	return helpers.WriteContentToRemote(ctx, conn, content, s.TargetPath, "0644", s.Sudo)
}

func (s *ConfigureCrictlStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error(err, "Failed to get connector for rollback.")
		return nil
	}
	logger.Warn("Rolling back by removing.", "path", s.TargetPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.TargetPath, s.Sudo, false); err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			logger.Error(err, "Failed to remove path during rollback.", "path", s.TargetPath)
		}
	}
	return nil
}

var _ step.Step = (*ConfigureCrictlStep)(nil)
