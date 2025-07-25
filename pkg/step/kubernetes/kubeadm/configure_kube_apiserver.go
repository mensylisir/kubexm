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

type ConfigureKubeApiServerStep struct {
	step.Base
	TargetPath           string
	TemplatePath         string
	ControlPlaneEndpoint string
	APIServerCertSANs    string
	EtcdEndpoints        string
	EtcdCaFile           string
	EtcdCertFile         string
	EtcdKeyFile          string
	KubernetesVersion    string
}

type ConfigureKubeApiServerStepBuilder struct {
	step.Builder[ConfigureKubeApiServerStepBuilder, *ConfigureKubeApiServerStep]
}

func NewConfigureKubeApiServerStepBuilder(ctx runtime.Context, instanceName string) *ConfigureKubeApiServerStepBuilder {
	cfg := ctx.GetClusterConfig().Spec
	s := &ConfigureKubeApiServerStep{
		TargetPath:           filepath.Join(common.KubeConfigDir, "kube-apiserver.yaml"),
		TemplatePath:         "kubernetes/kube-apiserver.yaml.tmpl",
		ControlPlaneEndpoint: "apiserver.cluster.local",
		APIServerCertSANs:    "apiserver.cluster.local",
		EtcdEndpoints:        "https://127.0.0.1:2379",
		EtcdCaFile:           filepath.Join(common.DefaultEtcdPKIDir, "ca.pem"),
		EtcdCertFile:         filepath.Join(common.DefaultEtcdPKIDir, "etcd.pem"),
		EtcdKeyFile:          filepath.Join(common.DefaultEtcdPKIDir, "etcd-key.pem"),
		KubernetesVersion:    cfg.Kubernetes.Version,
	}

	if cfg.ControlPlane != nil && cfg.ControlPlane.Endpoint != "" {
		s.ControlPlaneEndpoint = cfg.ControlPlane.Endpoint
	}

	if cfg.ApiServer != nil && len(cfg.ApiServer.CertSANs) > 0 {
		s.APIServerCertSANs = cfg.ApiServer.CertSANs[0]
	}

	if cfg.Etcd != nil && len(cfg.Etcd.Endpoints) > 0 {
		s.EtcdEndpoints = cfg.Etcd.Endpoints[0]
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure kube-apiserver", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(ConfigureKubeApiServerStepBuilder).Init(s)
	return b
}

func (s *ConfigureKubeApiServerStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureKubeApiServerStep) renderContent(ctx runtime.ExecutionContext) (string, error) {
	tmplStr, err := templates.Get(s.TemplatePath)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("kube-apiserver.yaml").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse kube-apiserver config template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s); err != nil {
		return "", fmt.Errorf("failed to render kube-apiserver config template: %w", err)
	}
	return buf.String(), nil
}

func (s *ConfigureKubeApiServerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
			logger.Infof("KubeApiServer config file '%s' already exists and content matches. Step is done.", s.TargetPath)
			return true, nil
		}
		logger.Infof("KubeApiServer config file '%s' exists but content differs. Step needs to run.", s.TargetPath)
		return false, nil
	}

	logger.Infof("KubeApiServer config file '%s' does not exist. Configuration is required.", s.TargetPath)
	return false, nil
}

func (s *ConfigureKubeApiServerStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Writing kube-apiserver config file to %s", s.TargetPath)
	err = runner.WriteFile(ctx.GoContext(), conn, []byte(content), s.TargetPath, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write kube-apiserver config file: %w", err)
	}

	return nil
}

func (s *ConfigureKubeApiServerStep) Rollback(ctx runtime.ExecutionContext) error {
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
