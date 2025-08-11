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
			logger.Warnf("Config file '%s' exists but failed to read, will overwrite. Error: %v", s.TargetPath, err)
			return false, nil
		}
		if string(remoteContent) == expectedContent {
			logger.Infof("crictl config file '%s' already exists and content matches. Step is done.", s.TargetPath)
			return true, nil
		}
		logger.Infof("crictl config file '%s' exists but content differs. Step needs to run.", s.TargetPath)
		return false, nil
	}

	logger.Infof("crictl config file '%s' does not exist. Configuration is required.", s.TargetPath)
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

	logger.Infof("Writing crictl config file to %s", s.TargetPath)
	return helpers.WriteContentToRemote(ctx, conn, content, s.TargetPath, "0644", s.Sudo)
}

func (s *ConfigureCrictlStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return nil
	}
	logger.Warnf("Rolling back by removing: %s", s.TargetPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.TargetPath, s.Sudo, false); err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			logger.Errorf("Failed to remove '%s': %v", s.TargetPath, err)
		}
	}
	return nil
}

var _ step.Step = (*ConfigureCrictlStep)(nil)
