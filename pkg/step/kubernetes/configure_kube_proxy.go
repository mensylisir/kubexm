package kubernetes

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

type ConfigureKubeProxyStep struct {
	step.Base
	TargetPath   string
	TemplatePath string
	ClusterCidr  string
}

type ConfigureKubeProxyStepBuilder struct {
	step.Builder[ConfigureKubeProxyStepBuilder, *ConfigureKubeProxyStep]
}

func NewConfigureKubeProxyStepBuilder(ctx runtime.Context, instanceName string) *ConfigureKubeProxyStepBuilder {
	cfg := ctx.GetClusterConfig().Spec
	s := &ConfigureKubeProxyStep{
		TargetPath:   filepath.Join(common.KubeConfigDir, "kube-proxy.yaml"),
		TemplatePath: "kubernetes/kube-proxy.yaml.tmpl",
		ClusterCidr:  "10.244.0.0/16",
	}

	if cfg.Network != nil && cfg.Network.PodCidr != "" {
		s.ClusterCidr = cfg.Network.PodCidr
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure kube-proxy", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(ConfigureKubeProxyStepBuilder).Init(s)
	return b
}

func (s *ConfigureKubeProxyStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureKubeProxyStep) renderContent(ctx runtime.ExecutionContext) (string, error) {
	tmplStr, err := templates.Get(s.TemplatePath)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("kube-proxy.yaml").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse kube-proxy config template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s); err != nil {
		return "", fmt.Errorf("failed to render kube-proxy config template: %w", err)
	}
	return buf.String(), nil
}

func (s *ConfigureKubeProxyStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	expectedContent, err := s.renderContent(ctx)
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
			logger.Infof("KubeProxy config file '%s' already exists and content matches. Step is done.", s.TargetPath)
			return true, nil
		}
		logger.Infof("KubeProxy config file '%s' exists but content differs. Step needs to run.", s.TargetPath)
		return false, nil
	}

	logger.Infof("KubeProxy config file '%s' does not exist. Configuration is required.", s.TargetPath)
	return false, nil
}

func (s *ConfigureKubeProxyStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	content, err := s.renderContent(ctx)
	if err != nil {
		return err
	}

	logger.Infof("Writing kube-proxy config file to %s", s.TargetPath)
	err = runner.WriteFile(ctx.GoContext(), conn, []byte(content), s.TargetPath, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write kube-proxy config file: %w", err)
	}

	return nil
}

func (s *ConfigureKubeProxyStep) Rollback(ctx runtime.ExecutionContext) error {
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

	return nil
}
