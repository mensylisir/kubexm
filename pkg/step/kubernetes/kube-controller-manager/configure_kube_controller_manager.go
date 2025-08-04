package kube_controller_manager

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type ConfigureKubeControllerManagerStep struct {
	step.Base
	KubeconfigPath         string
	BindAddress            string
	PKIDir                 string
	PodSubnet              string
	ClusterName            string
	ServiceClusterIPRange  string
	NodeCidrMaskSize       int
	NodeCidrMaskSizeIPv6   *int
	PodEvictionTimeout     string
	NodeMonitorGracePeriod string
	FeatureGates           map[string]bool
	RemoteConfigFile       string
}

type ConfigureKubeControllerManagerStepBuilder struct {
	step.Builder[ConfigureKubeControllerManagerStepBuilder, *ConfigureKubeControllerManagerStep]
}

func NewConfigureKubeControllerManagerStepBuilder(ctx runtime.Context, instanceName string) *ConfigureKubeControllerManagerStepBuilder {
	clusterCfg := ctx.GetClusterConfig()
	k8sSpec := clusterCfg.Spec.Kubernetes
	cmCfg := k8sSpec.ControllerManager

	s := &ConfigureKubeControllerManagerStep{
		KubeconfigPath:         filepath.Join(common.KubernetesConfigDir, common.ControllerManagerKubeconfigFileName),
		BindAddress:            "127.0.0.1",
		PKIDir:                 common.KubernetesPKIDir,
		ClusterName:            k8sSpec.ClusterName,
		PodSubnet:              clusterCfg.Spec.Network.KubePodsCIDR,
		ServiceClusterIPRange:  clusterCfg.Spec.Network.KubeServiceCIDR,
		RemoteConfigFile:       filepath.Join(common.KubernetesConfigDir, "kube-controller-manager.yaml"),
		NodeCidrMaskSize:       24,
		PodEvictionTimeout:     "5m0s",
		NodeMonitorGracePeriod: "40s",
	}

	if cmCfg != nil {
		if cmCfg.NodeCidrMaskSize != nil {
			s.NodeCidrMaskSize = *cmCfg.NodeCidrMaskSize
		}
		if cmCfg.NodeCidrMaskSizeIPv6 != nil {
			s.NodeCidrMaskSizeIPv6 = cmCfg.NodeCidrMaskSizeIPv6
		}
		if cmCfg.PodEvictionTimeout != "" {
			s.PodEvictionTimeout = cmCfg.PodEvictionTimeout
		}
		if cmCfg.NodeMonitorGracePeriod != "" {
			s.NodeMonitorGracePeriod = cmCfg.NodeMonitorGracePeriod
		}
		if len(cmCfg.FeatureGates) > 0 {
			s.FeatureGates = cmCfg.FeatureGates
		}
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure kube-controller-manager config file", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(ConfigureKubeControllerManagerStepBuilder).Init(s)
	return b
}

func (s *ConfigureKubeControllerManagerStep) Meta() *spec.StepMeta { return &s.Base.Meta }

func (s *ConfigureKubeControllerManagerStep) renderConfig() (string, error) {
	tmplContent, err := templates.Get("kubernetes/kube-controller-manager.yaml.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to get kube-controller-manager config template: %w", err)
	}
	return templates.Render(tmplContent, s)
}

func (s *ConfigureKubeControllerManagerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	expectedContent, err := s.renderConfig()
	if err != nil {
		return false, fmt.Errorf("failed to render expected config for precheck: %w", err)
	}

	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, s.RemoteConfigFile)
	if err != nil {
		logger.Infof("Remote config file %s not found, configuration is required.", s.RemoteConfigFile)
		return false, nil
	}
	if string(remoteContent) != expectedContent {
		logger.Warn("Remote kube-controller-manager config file content mismatch. Re-configuration is required.")
		return false, nil
	}

	logger.Info("kube-controller-manager config file is up to date. Step is done.")
	return true, nil
}

func (s *ConfigureKubeControllerManagerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	configContent, err := s.renderConfig()
	if err != nil {
		return fmt.Errorf("failed to render kube-controller-manager config: %w", err)
	}

	if err := helpers.WriteContentToRemote(ctx, conn, configContent, s.RemoteConfigFile, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to write kube-controller-manager config file: %w", err)
	}

	logger.Info("kube-controller-manager config file has been created successfully.")
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

	logger.Warnf("Rolling back by removing %s", s.RemoteConfigFile)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteConfigFile, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove config file during rollback: %v", err)
	}
	return nil
}

var _ step.Step = (*ConfigureKubeControllerManagerStep)(nil)
