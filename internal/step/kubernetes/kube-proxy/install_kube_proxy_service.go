package kube_proxy

import (
	"bytes"
	"fmt"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"github.com/mensylisir/kubexm/internal/templates"
	"github.com/mensylisir/kubexm/internal/types"
)

// InstallKubeProxyServiceStep 负责在目标节点上生成 kube-proxy 的 systemd 服务文件。
type InstallKubeProxyServiceStep struct {
	step.Base
	// 模板渲染所需的数据
	ConfigYAMLPath string
	LogLevel       int
	// 文件路径
	RemoteServiceFile string
}

type InstallKubeProxyServiceStepBuilder struct {
	step.Builder[InstallKubeProxyServiceStepBuilder, *InstallKubeProxyServiceStep]
}

func NewInstallKubeProxyServiceStepBuilder(ctx runtime.ExecutionContext, instanceName string) *InstallKubeProxyServiceStepBuilder {
	s := &InstallKubeProxyServiceStep{
		ConfigYAMLPath:    common.KubeproxyConfigYAMLPathTarget,
		LogLevel:          2,
		RemoteServiceFile: common.DefaultKubeProxyrServiceFile,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install kube-proxy systemd service", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	b := new(InstallKubeProxyServiceStepBuilder).Init(s)
	return b
}

func (s *InstallKubeProxyServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallKubeProxyServiceStep) render() (string, error) {
	tmplContent, err := templates.Get("kubernetes/kube-proxy.service.tmpl")
	if err != nil {
		return "", err
	}
	tmpl, err := template.New("kube-proxy.service").Parse(tmplContent)
	if err != nil {
		return "", err
	}
	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, s); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func (s *InstallKubeProxyServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		logger.Info("kube-proxy.service file does not exist. Installation is required.")
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
		logger.Info("Remote kube-proxy.service file is up to date. Step is done.")
		return true, nil
	}

	logger.Warn("Remote kube-proxy.service file content mismatch. Re-installation is required.")
	return false, nil
}

func (s *InstallKubeProxyServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	content, err := s.render()
	if err != nil {
		err = fmt.Errorf("failed to render service content: %w", err)
		result.MarkFailed(err, "failed to render content")
		return result, err
	}

	logger.Infof("Writing kube-proxy.service file to %s", s.RemoteServiceFile)
	if err := helpers.WriteContentToRemote(ctx, conn, content, s.RemoteServiceFile, "0644", s.Sudo); err != nil {
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

	logger.Info("kube-proxy service has been installed successfully.")
	result.MarkCompleted("service installed successfully")
	return result, nil
}

func (s *InstallKubeProxyServiceStep) Rollback(ctx runtime.ExecutionContext) error {
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
	facts, _ := runner.GatherFacts(ctx.GoContext(), conn)
	if facts != nil {
		_ = runner.DaemonReload(ctx.GoContext(), conn, facts)
	}

	return nil
}

var _ step.Step = (*InstallKubeProxyServiceStep)(nil)
