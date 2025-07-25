package kubeadm

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

type ConfigureKubeControllerManagerStep struct {
	step.Base
	TargetPath            string
	TemplatePath          string
	ClusterCidr           string
	ServiceClusterIpRange string
	KubernetesVersion     string
}

type ConfigureKubeControllerManagerStepBuilder struct {
	step.Builder[ConfigureKubeControllerManagerStepBuilder, *ConfigureKubeControllerManagerStep]
}

func NewConfigureKubeControllerManagerStepBuilder(ctx runtime.Context, instanceName string) *ConfigureKubeControllerManagerStepBuilder {
	cfg := ctx.GetClusterConfig().Spec
	s := &ConfigureKubeControllerManagerStep{
		TargetPath:            filepath.Join(common.KubeConfigDir, "kube-controller-manager.yaml"),
		TemplatePath:          "kubernetes/kube-controller-manager.yaml.tmpl",
		ClusterCidr:           "10.244.0.0/16",
		ServiceClusterIpRange: "10.96.0.0/12",
		KubernetesVersion:     cfg.Kubernetes.Version,
	}

	if cfg.Network != nil && cfg.Network.PodCidr != "" {
		s.ClusterCidr = cfg.Network.PodCidr
	}

	if cfg.Network != nil && cfg.Network.ServiceCidr != "" {
		s.ServiceClusterIpRange = cfg.Network.ServiceCidr
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure kube-controller-manager", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(ConfigureKubeControllerManagerStepBuilder).Init(s)
	return b
}

func (s *ConfigureKubeControllerManagerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureKubeControllerManagerStep) renderContent(ctx runtime.ExecutionContext) (string, error) {
	tmplStr, err := templates.Get(s.TemplatePath)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("kube-controller-manager.yaml").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse kube-controller-manager config template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s); err != nil {
		return "", fmt.Errorf("failed to render kube-controller-manager config template: %w", err)
	}
	return buf.String(), nil
}

func (s *ConfigureKubeControllerManagerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
			logger.Infof("KubeControllerManager config file '%s' already exists and content matches. Step is done.", s.TargetPath)
			return true, nil
		}
		logger.Infof("KubeControllerManager config file '%s' exists but content differs. Step needs to run.", s.TargetPath)
		return false, nil
	}

	logger.Infof("KubeControllerManager config file '%s' does not exist. Configuration is required.", s.TargetPath)
	return false, nil
}

func (s *ConfigureKubeControllerManagerStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Writing kube-controller-manager config file to %s", s.TargetPath)
	err = runner.WriteFile(ctx.GoContext(), conn, []byte(content), s.TargetPath, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write kube-controller-manager config file: %w", err)
	}

	return nil
}

func (s *ConfigureKubeControllerManagerStep) Rollback(ctx runtime.ExecutionContext) error {
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
