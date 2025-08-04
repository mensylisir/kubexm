package kube_scheduler

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type InstallKubeSchedulerServiceStep struct {
	step.Base
	LogLevel          int
	ExtraArgs         map[string]string
	ExtraArgsStr      string
	RemoteConfigFile  string
	RemoteServiceFile string
}

type InstallKubeSchedulerServiceStepBuilder struct {
	step.Builder[InstallKubeSchedulerServiceStepBuilder, *InstallKubeSchedulerServiceStep]
}

func NewInstallKubeSchedulerServiceStepBuilder(ctx runtime.Context, instanceName string) *InstallKubeSchedulerServiceStepBuilder {
	k8sSpec := ctx.GetClusterConfig().Spec.Kubernetes
	schedulerCfg := k8sSpec.Scheduler

	s := &InstallKubeSchedulerServiceStep{
		LogLevel:          2,
		RemoteConfigFile:  filepath.Join(common.KubernetesConfigDir, "kube-scheduler.yaml"),
		RemoteServiceFile: common.DefaultKubeSchedulerServiceFile,
	}

	if schedulerCfg != nil && len(schedulerCfg.ExtraArgs) > 0 {
		s.ExtraArgs = schedulerCfg.ExtraArgs
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install kube-scheduler systemd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallKubeSchedulerServiceStepBuilder).Init(s)
	return b
}

func (s *InstallKubeSchedulerServiceStep) Meta() *spec.StepMeta { return &s.Base.Meta }

func (s *InstallKubeSchedulerServiceStep) formatExtraArgs() {
	var args []string
	for k, v := range s.ExtraArgs {
		args = append(args, fmt.Sprintf("--%s=%s", k, v))
	}
	s.ExtraArgsStr = strings.Join(args, " ")
}

func (s *InstallKubeSchedulerServiceStep) renderService() (string, error) {
	s.formatExtraArgs()
	tmplContent, err := templates.Get("kubernetes/kube-scheduler.service.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to get kube-scheduler service template: %w", err)
	}
	return templates.Render(tmplContent, s)
}

func (s *InstallKubeSchedulerServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	expectedSvc, err := s.renderService()
	if err != nil {
		return false, fmt.Errorf("failed to render service file for precheck: %w", err)
	}

	remoteSvc, err := runner.ReadFile(ctx.GoContext(), conn, s.RemoteServiceFile)
	if err != nil {
		logger.Infof("Remote service file %s not found, installation is required.", s.RemoteServiceFile)
		return false, nil
	}

	if string(remoteSvc) != expectedSvc {
		logger.Warn("Remote kube-scheduler.service file content mismatch. Re-installation is required.")
		return false, nil
	}

	logger.Info("kube-scheduler.service file is up to date. Step is done.")
	return true, nil
}

func (s *InstallKubeSchedulerServiceStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	svcContent, err := s.renderService()
	if err != nil {
		return fmt.Errorf("failed to render kube-scheduler service file: %w", err)
	}

	if err := helpers.WriteContentToRemote(ctx, conn, svcContent, s.RemoteServiceFile, "0644", true); err != nil {
		return fmt.Errorf("failed to write service file to %s: %w", s.RemoteServiceFile, err)
	}

	logger.Info("Reloading systemd daemon to apply changes...")
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Warnf("Failed to gather facts, falling back to raw command for daemon-reload: %v", err)
		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, "systemctl daemon-reload", true); err != nil {
			return fmt.Errorf("failed to run daemon-reload on host %s: %w", ctx.GetHost().GetName(), err)
		}
	} else {
		if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
			return fmt.Errorf("failed to run daemon-reload on host %s: %w", ctx.GetHost().GetName(), err)
		}
	}

	logger.Info("kube-scheduler systemd service has been configured successfully.")
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
	if err := runner.Remove(ctx.GoContext(), conn, s.RemoteServiceFile, true, false); err != nil {
		logger.Errorf("Failed to remove service file during rollback: %v", err)
	}

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Warnf("Failed to gather facts during rollback, falling back to raw command for daemon-reload: %v", err)
		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, "systemctl daemon-reload", true); err != nil {
			logger.Errorf("Failed to run daemon-reload during rollback: %v", err)
		}
	} else {
		if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
			logger.Errorf("Failed to run daemon-reload during rollback: %v", err)
		}
	}

	return nil
}

var _ step.Step = (*InstallKubeSchedulerServiceStep)(nil)
