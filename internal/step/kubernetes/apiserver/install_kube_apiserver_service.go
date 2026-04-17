package kube_apiserver

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/templates"
	"github.com/mensylisir/kubexm/internal/types"
)

type InstallKubeAPIServerServiceStep struct {
	step.Base
	LogLevel          int
	ExtraArgs         map[string]string
	ExtraArgsStr      string
	RemoteConfigFile  string
	RemoteServiceFile string
}

type InstallKubeAPIServerServiceStepBuilder struct {
	step.Builder[InstallKubeAPIServerServiceStepBuilder, *InstallKubeAPIServerServiceStep]
}

func NewInstallKubeAPIServerServiceStepBuilder(ctx runtime.ExecutionContext, instanceName string) *InstallKubeAPIServerServiceStepBuilder {
	apiCfg := struct {
		ExtraArgs    map[string]string
		AuditConfig  interface{}
		FeatureGates map[string]bool
	}{
		ExtraArgs: nil,
	}
	if cfg := ctx.GetClusterConfig(); cfg != nil && cfg.Spec.Kubernetes != nil && cfg.Spec.Kubernetes.APIServer != nil {
		apiCfg.ExtraArgs = cfg.Spec.Kubernetes.APIServer.ExtraArgs
		apiCfg.AuditConfig = cfg.Spec.Kubernetes.APIServer.AuditConfig
		apiCfg.FeatureGates = cfg.Spec.Kubernetes.APIServer.FeatureGates
	}

	s := &InstallKubeAPIServerServiceStep{
		LogLevel:          2,
		RemoteConfigFile:  filepath.Join(common.KubernetesConfigDir, "kube-apiserver.yaml"),
		RemoteServiceFile: common.DefaultKubeApiserverServiceFile,
	}

	if apiCfg.ExtraArgs != nil {
		s.ExtraArgs = apiCfg.ExtraArgs
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install kube-apiserver systemd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 5 * time.Minute

	b := new(InstallKubeAPIServerServiceStepBuilder).Init(s)
	return b
}

func (s *InstallKubeAPIServerServiceStep) Meta() *spec.StepMeta { return &s.Base.Meta }

func (s *InstallKubeAPIServerServiceStep) formatExtraArgs() {
	var args []string
	for k, v := range s.ExtraArgs {
		args = append(args, fmt.Sprintf("--%s=%s", k, v))
	}
	s.ExtraArgsStr = strings.Join(args, " ")
}

func (s *InstallKubeAPIServerServiceStep) renderService() (string, error) {
	s.formatExtraArgs()
	tmplContent, err := templates.Get("kubernetes/kube-apiserver.service.tmpl")
	if err != nil {
		return "", fmt.Errorf("failed to get kube-apiserver service template: %w", err)
	}
	return templates.Render(tmplContent, s)
}

func (s *InstallKubeAPIServerServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		logger.Warn("Remote kube-apiserver.service file content mismatch. Re-installation is required.")
		return false, nil
	}

	logger.Info("kube-apiserver.service file is up to date. Step is done.")
	return true, nil
}

func (s *InstallKubeAPIServerServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	svcContent, err := s.renderService()
	if err != nil {
		err = fmt.Errorf("failed to render kube-apiserver service file: %w", err)
		result.MarkFailed(err, "failed to render service file")
		return result, err
	}
	if err := helpers.WriteContentToRemote(ctx, conn, svcContent, s.RemoteServiceFile, "0644", s.Sudo); err != nil {
		err = fmt.Errorf("failed to write service file to %s: %w", s.RemoteServiceFile, err)
		result.MarkFailed(err, "failed to write service file")
		return result, err
	}

	logger.Info("Reloading systemd daemon to apply changes...")
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Warnf("Failed to gather facts, falling back to raw command for daemon-reload: %v", err)
		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, "systemctl daemon-reload", s.Sudo); err != nil {
			err = fmt.Errorf("failed to run daemon-reload on host %s: %w", ctx.GetHost().GetName(), err)
			result.MarkFailed(err, "failed to run daemon-reload")
			return result, err
		}
	} else {
		if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
			err = fmt.Errorf("failed to run daemon-reload on host %s: %w", ctx.GetHost().GetName(), err)
			result.MarkFailed(err, "failed to run daemon-reload")
			return result, err
		}
	}

	logger.Info("kube-apiserver systemd service has been configured successfully.")
	result.MarkCompleted("service configured successfully")
	return result, nil
}

func (s *InstallKubeAPIServerServiceStep) Rollback(ctx runtime.ExecutionContext) error {
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

	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Warnf("Failed to gather facts during rollback, falling back to raw command for daemon-reload: %v", err)
		if _, _, err := runner.OriginRun(ctx.GoContext(), conn, "systemctl daemon-reload", s.Sudo); err != nil {
			logger.Errorf("Failed to run daemon-reload during rollback: %v", err)
		}
	} else {
		if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
			logger.Errorf("Failed to run daemon-reload during rollback: %v", err)
		}
	}

	return nil
}

var _ step.Step = (*InstallKubeAPIServerServiceStep)(nil)
