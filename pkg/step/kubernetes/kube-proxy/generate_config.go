package kube_proxy

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type CreateKubeProxyConfigYAMLStep struct {
	step.Base
	KubeconfigPath       string
	ClusterCIDR          string
	Mode                 string
	FeatureGates         map[string]bool
	RemoteConfigYAMLFile string
}

type CreateKubeProxyConfigYAMLStepBuilder struct {
	step.Builder[CreateKubeProxyConfigYAMLStepBuilder, *CreateKubeProxyConfigYAMLStep]
}

func NewCreateKubeProxyConfigYAMLStepBuilder(ctx runtime.Context, instanceName string) *CreateKubeProxyConfigYAMLStepBuilder {
	clusterCfg := ctx.GetClusterConfig()
	k8sSpec := clusterCfg.Spec.Kubernetes

	s := &CreateKubeProxyConfigYAMLStep{
		KubeconfigPath:       filepath.Join(common.KubernetesConfigDir, common.KubeProxyKubeconfigFileName),
		ClusterCIDR:          common.DefaultKubePodsCIDR,
		Mode:                 common.KubeProxyModeIPVS,
		RemoteConfigYAMLFile: common.KubeproxyConfigYAMLPathTarget,
	}

	if clusterCfg.Spec.Network.KubePodsCIDR != "" {
		s.ClusterCIDR = clusterCfg.Spec.Network.KubePodsCIDR
	}
	if k8sSpec.KubeProxy.Mode != "" {
		s.Mode = k8sSpec.KubeProxy.Mode
	}
	if k8sSpec.KubeProxy.FeatureGates != nil {
		s.FeatureGates = k8sSpec.KubeProxy.FeatureGates
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Create kube-proxy configuration file (config.yaml) on node", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(CreateKubeProxyConfigYAMLStepBuilder).Init(s)
	return b
}

func (s *CreateKubeProxyConfigYAMLStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *CreateKubeProxyConfigYAMLStep) render() (string, error) {
	tmplContent, err := templates.Get("kubernetes/kube-proxy-config.yaml.tmpl")
	if err != nil {
		return "", err
	}
	tmpl, err := template.New("kube-proxy-config").Parse(tmplContent)
	if err != nil {
		return "", err
	}
	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, s); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func (s *CreateKubeProxyConfigYAMLStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteConfigYAMLFile)
	if err != nil {
		return false, err
	}
	if !exists {
		logger.Info("Kube-proxy config.yaml does not exist. Configuration is required.")
		return false, nil
	}

	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, s.RemoteConfigYAMLFile)
	if err != nil {
		return false, err
	}

	expectedContent, err := s.render()
	if err != nil {
		return false, err
	}

	if string(remoteContent) == expectedContent {
		logger.Info("Kube-proxy config.yaml is up to date. Step is done.")
		return true, nil
	}

	logger.Warn("Kube-proxy config.yaml content mismatch. Re-configuration is required.")
	return false, nil
}

func (s *CreateKubeProxyConfigYAMLStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	remoteDir := filepath.Dir(s.RemoteConfigYAMLFile)
	if err := runner.Mkdirp(ctx.GoContext(), conn, remoteDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create directory for kube-proxy config: %w", err)
	}

	content, err := s.render()
	if err != nil {
		return err
	}

	logger.Infof("Writing kube-proxy config.yaml to %s", s.RemoteConfigYAMLFile)
	return helpers.WriteContentToRemote(ctx, conn, content, s.RemoteConfigYAMLFile, "0644", s.Sudo)
}

func (s *CreateKubeProxyConfigYAMLStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}
	logger.Warnf("Rolling back by removing %s", s.RemoteConfigYAMLFile)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteConfigYAMLFile, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove kube-proxy config file during rollback: %v", err)
	}
	return nil
}

var _ step.Step = (*CreateKubeProxyConfigYAMLStep)(nil)
