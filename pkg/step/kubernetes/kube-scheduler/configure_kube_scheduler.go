package kube_scheduler

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/templates"
	"path/filepath"
	"time"
)

type ConfigureKubeSchedulerStep struct {
	step.Base
	KubeconfigPath   string
	BindAddress      string
	FeatureGates     map[string]bool
	RemoteConfigFile string
}

type ConfigureKubeSchedulerStepBuilder struct {
	step.Builder[ConfigureKubeSchedulerStepBuilder, *ConfigureKubeSchedulerStep]
}

func NewConfigureKubeSchedulerStepBuilder(ctx runtime.Context, instanceName string) *ConfigureKubeSchedulerStepBuilder {
	k8sSpec := ctx.GetClusterConfig().Spec.Kubernetes
	schedulerCfg := k8sSpec.Scheduler

	s := &ConfigureKubeSchedulerStep{
		KubeconfigPath:   filepath.Join(common.KubernetesConfigDir, common.SchedulerKubeconfigFileName),
		BindAddress:      "127.0.0.1",
		RemoteConfigFile: filepath.Join(common.KubernetesConfigDir, "kube-scheduler.yaml"),
	}

	if schedulerCfg != nil {
		if len(schedulerCfg.FeatureGates) > 0 {
			s.FeatureGates = schedulerCfg.FeatureGates
		}
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure kube-scheduler config file", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(ConfigureKubeSchedulerStepBuilder).Init(s)
	return b
}

func (s *ConfigureKubeSchedulerStep) Meta() *spec.StepMeta { return &s.Base.Meta }

func (s *ConfigureKubeSchedulerStep) renderConfig() (string, error) {
	tmplContent, err := templates.Get("kubernetes/kube-scheduler.yaml.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to get kube-scheduler config template: %w", err)
	}
	return templates.Render(tmplContent, s)
}

func (s *ConfigureKubeSchedulerStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		logger.Warn("Remote kube-scheduler config file content mismatch. Re-configuration is required.")
		return false, nil
	}

	logger.Info("kube-scheduler config file is up to date. Step is done.")
	return true, nil
}

func (s *ConfigureKubeSchedulerStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	configContent, err := s.renderConfig()
	if err != nil {
		return fmt.Errorf("failed to render kube-scheduler config: %w", err)
	}

	if err := helpers.WriteContentToRemote(ctx, conn, configContent, s.RemoteConfigFile, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to write kube-scheduler config file: %w", err)
	}

	logger.Info("kube-scheduler config file has been created successfully.")
	return nil
}

func (s *ConfigureKubeSchedulerStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*ConfigureKubeSchedulerStep)(nil)
