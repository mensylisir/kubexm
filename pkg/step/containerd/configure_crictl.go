package containerd

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

const (
	crictlConfigTemplatePath = "containerd/crictl.yaml.tmpl"
	remoteCriCtlConfigPath   = common.CrictlDefaultConfigFile
)

type ConfigureCriCtlStep struct {
	step.Base
	TargetPath      string
	RuntimeEndpoint string
	ImageEndpoint   string
	Timeout         int
	Debug           bool
}

type ConfigureCriCtlStepBuilder struct {
	step.Builder[ConfigureCriCtlStepBuilder, *ConfigureCriCtlStep]
}

func NewConfigureCriCtlStepBuilder(ctx runtime.Context, instanceName string) *ConfigureCriCtlStepBuilder {
	containerdEndpoint := common.ContainerdDefaultEndpoint

	s := &ConfigureCriCtlStep{
		TargetPath:      remoteCriCtlConfigPath,
		RuntimeEndpoint: containerdEndpoint,
		ImageEndpoint:   containerdEndpoint,
		Timeout:         10,
		Debug:           false,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure crictl", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(ConfigureCriCtlStepBuilder).Init(s)
	return b
}

func (s *ConfigureCriCtlStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureCriCtlStep) renderContent() (string, error) {
	tmplStr, err := templates.Get(crictlConfigTemplatePath)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New("crictl.yaml").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse crictl config template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s); err != nil {
		return "", fmt.Errorf("failed to render crictl config template: %w", err)
	}
	return buf.String(), nil
}

func (s *ConfigureCriCtlStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	expectedContent, err := s.renderContent()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.TargetPath)
	if err != nil {
		return false, err
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

func (s *ConfigureCriCtlStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	content, err := s.renderContent()
	if err != nil {
		return err
	}

	logger.Info("Writing crictl config file.", "path", s.TargetPath)
	return helpers.WriteContentToRemote(ctx, conn, content, s.TargetPath, "0644", s.Sudo)
}

func (s *ConfigureCriCtlStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error(err, "Failed to get connector for rollback.")
		return nil
	}

	logger.Warn("Rolling back by removing.", "path", s.TargetPath)
	runner.Remove(ctx.GoContext(), conn, s.TargetPath, s.Sudo, false)
	return nil
}

var _ step.Step = (*ConfigureCriCtlStep)(nil)
