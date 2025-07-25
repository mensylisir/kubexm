package kube_scheduler

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
	"github.com/mensylisir/kubexm/pkg/templates"
)

type InstallKubeSchedulerServiceStep struct {
	step.Base
	KubeconfigPath    string
	BindAddress       string
	FeatureGates      string
	ExtraArgs         map[string]string
	LogLevel          int
	RemoteServiceFile string
}

type InstallKubeSchedulerServiceStepBuilder struct {
	step.Builder[InstallKubeSchedulerServiceStepBuilder, *InstallKubeSchedulerServiceStep]
}

func NewInstallKubeSchedulerServiceStepBuilder(ctx runtime.Context, instanceName string) *InstallKubeSchedulerServiceStepBuilder {
	clusterCfg := ctx.GetClusterConfig()
	k8sSpec := clusterCfg.Spec.Kubernetes

	s := &InstallKubeSchedulerServiceStep{
		KubeconfigPath:    filepath.Join(common.KubernetesConfigDir, common.SchedulerKubeconfigFileName),
		BindAddress:       "127.0.0.1",
		LogLevel:          2,
		RemoteServiceFile: common.DefaultKubeSchedulerServiceFile,
	}

	if k8sSpec.Scheduler != nil {
		schedulerCfg := k8sSpec.Scheduler

		if len(schedulerCfg.FeatureGates) > 0 {
			var fg []string
			for k, v := range schedulerCfg.FeatureGates {
				fg = append(fg, fmt.Sprintf("%s=%t", k, v))
			}
			s.FeatureGates = strings.Join(fg, ",")
		}

		if len(schedulerCfg.ExtraArgs) > 0 {
			s.ExtraArgs = schedulerCfg.ExtraArgs
		}
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install kube-scheduler systemd service", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallKubeSchedulerServiceStepBuilder).Init(s)
	return b
}

func (s *InstallKubeSchedulerServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallKubeSchedulerServiceStep) render() (string, error) {
	tmplContent, err := templates.Get("kubernetes/kube-scheduler.service.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to get kube-scheduler service template: %w", err)
	}
	tmpl, err := template.New("kube-scheduler.service").Parse(tmplContent)
	if err != nil {
		return "", fmt.Errorf("failed to parse kube-scheduler service template: %w", err)
	}
	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, s); err != nil {
		return "", fmt.Errorf("failed to render kube-scheduler service template: %w", err)
	}
	return buffer.String(), nil
}

func (s *InstallKubeSchedulerServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}
	exists, err := runner.Exists(ctx.GoContext(), conn, s.RemoteServiceFile)
	if err != nil {
		return false, err
	}
	if !exists {
		logger.Info("kube-scheduler.service file does not exist. Installation is required.")
		return false, nil
	}
	remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, s.RemoteServiceFile)
	if err != nil {
		return false, err
	}
	expectedContent, err := s.render()
	if err != nil {
		return false, err
	}
	if string(remoteContent) == expectedContent {
		logger.Info("Remote kube-scheduler.service file is up to date. Step is done.")
		return true, nil
	}
	logger.Warn("Remote kube-scheduler.service file content mismatch. Re-installation is required.")
	return false, nil
}

func (s *InstallKubeSchedulerServiceStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}
	content, err := s.render()
	if err != nil {
		return err
	}
	logger.Info("Writing kube-scheduler.service file to remote host...")
	if err := runner.WriteFile(ctx.GoContext(), conn, []byte(content), s.RemoteServiceFile, "0644", s.Sudo); err != nil {
		return fmt.Errorf("failed to write service file to %s: %w", s.RemoteServiceFile, err)
	}
	logger.Info("Reloading systemd daemon to apply changes...")
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Warnf("Failed to gather facts, falling back to raw command for daemon-reload: %v", err)
		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, "systemctl daemon-reload", s.Sudo); err != nil {
			return fmt.Errorf("failed to run daemon-reload on host %s: %w", ctx.GetHost().GetName(), err)
		}
	} else {
		if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
			return fmt.Errorf("failed to run daemon-reload on host %s: %w", ctx.GetHost().GetName(), err)
		}
	}
	logger.Info("kube-scheduler service has been installed successfully.")
	return nil
}

func (s *InstallKubeSchedulerServiceStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Errorf("Failed to get connector for rollback: %v", err)
		return nil
	}
	logger.Warnf("Rolling back by removing %s", s.RemoteServiceFile)
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteServiceFile, s.Sudo, false); err != nil {
		logger.Errorf("Failed to remove service file during rollback: %v", err)
	}
	return nil
}

var _ step.Step = (*InstallKubeSchedulerServiceStep)(nil)
