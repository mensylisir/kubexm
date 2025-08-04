package kube_controller_manager

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

type InstallKubeControllerManagerServiceStep struct {
	step.Base
	LogLevel          int
	ExtraArgs         map[string]string
	ExtraArgsStr      string
	RemoteConfigFile  string
	RemoteServiceFile string
}

type InstallKubeControllerManagerServiceStepBuilder struct {
	step.Builder[InstallKubeControllerManagerServiceStepBuilder, *InstallKubeControllerManagerServiceStep]
}

func NewInstallKubeControllerManagerServiceStepBuilder(ctx runtime.Context, instanceName string) *InstallKubeControllerManagerServiceStepBuilder {
	k8sSpec := ctx.GetClusterConfig().Spec.Kubernetes
	cmCfg := k8sSpec.ControllerManager

	s := &InstallKubeControllerManagerServiceStep{
		LogLevel:          2,
		RemoteConfigFile:  filepath.Join(common.KubernetesConfigDir, "kube-controller-manager.yaml"),
		RemoteServiceFile: common.DefaultKubeControllerManagerServiceFile,
	}

	if cmCfg != nil && len(cmCfg.ExtraArgs) > 0 {
		s.ExtraArgs = cmCfg.ExtraArgs
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install kube-controller-manager systemd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallKubeControllerManagerServiceStepBuilder).Init(s)
	return b
}

func (s *InstallKubeControllerManagerServiceStep) Meta() *spec.StepMeta { return &s.Base.Meta }

func (s *InstallKubeControllerManagerServiceStep) formatExtraArgs() {
	var args []string
	for k, v := range s.ExtraArgs {
		args = append(args, fmt.Sprintf("--%s=%s", k, v))
	}
	s.ExtraArgsStr = strings.Join(args, " ")
}

func (s *InstallKubeControllerManagerServiceStep) renderService() (string, error) {
	s.formatExtraArgs()
	tmplContent, err := templates.Get("kubernetes/kube-controller-manager.service.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to get kube-controller-manager service template: %w", err)
	}
	return templates.Render(tmplContent, s)
}

func (s *InstallKubeControllerManagerServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		logger.Warn("Remote kube-controller-manager.service file content mismatch. Re-installation is required.")
		return false, nil
	}

	logger.Info("kube-controller-manager.service file is up to date. Step is done.")
	return true, nil
}

func (s *InstallKubeControllerManagerServiceStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	svcContent, err := s.renderService()
	if err != nil {
		return fmt.Errorf("failed to render kube-controller-manager service file: %w", err)
	}

	if err := helpers.WriteContentToRemote(ctx, conn, svcContent, s.RemoteServiceFile, "0644", s.Sudo); err != nil {
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

	logger.Info("kube-controller-manager systemd service has been configured successfully.")
	return nil
}

func (s *InstallKubeControllerManagerServiceStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*InstallKubeControllerManagerServiceStep)(nil)
