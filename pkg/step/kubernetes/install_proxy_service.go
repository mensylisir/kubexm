package kubernetes

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

const (
	KubeProxyServiceTemplatePath = "kubernetes/kube-proxy.service.tmpl"
)

type InstallKubeProxyServiceStep struct {
	step.Base
	TargetPath              string
	KubeProxyExecStartPath string
}

type InstallKubeProxyServiceStepBuilder struct {
	step.Builder[InstallKubeProxyServiceStepBuilder, *InstallKubeProxyServiceStep]
}

func NewInstallKubeProxyServiceStepBuilder(ctx runtime.Context, instanceName string) *InstallKubeProxyServiceStepBuilder {
	s := &InstallKubeProxyServiceStep{
		TargetPath:              common.KubeProxyDefaultSystemdFile,
		KubeProxyExecStartPath: filepath.Join(common.DefaultBinDir, "kube-proxy"),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install kube-proxy systemd service from template", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(InstallKubeProxyServiceStepBuilder).Init(s)
	return b
}

func (b *InstallKubeProxyServiceStepBuilder) WithTargetPath(path string) *InstallKubeProxyServiceStepBuilder {
	b.Step.TargetPath = path
	return b
}

func (b *InstallKubeProxyServiceStepBuilder) WithKubeProxyExecStartPath(path string) *InstallKubeProxyServiceStepBuilder {
	b.Step.KubeProxyExecStartPath = path
	return b
}

func (s *InstallKubeProxyServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallKubeProxyServiceStep) renderContent(ctx runtime.ExecutionContext) (string, error) {
	tmplStr, err := templates.Get(KubeProxyServiceTemplatePath)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("kube-proxy.service").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse kube-proxy service template: %w", err)
	}

	data := struct {
		KubeProxyExecStartPath string
	}{
		KubeProxyExecStartPath: s.KubeProxyExecStartPath,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render kube-proxy service template: %w", err)
	}

	return buf.String(), nil
}

func (s *InstallKubeProxyServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		return false, fmt.Errorf("failed to check for service file '%s': %w", s.TargetPath, err)
	}
	if exists {
		remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, s.TargetPath)
		if err != nil {
			logger.Warnf("Service file '%s' exists but failed to read, will overwrite. Error: %v", s.TargetPath, err)
			return false, nil
		}
		if string(remoteContent) == expectedContent {
			logger.Infof("KubeProxy service file '%s' already exists and content matches. Step is done.", s.TargetPath)
			return true, nil
		}
		logger.Infof("KubeProxy service file '%s' exists but content differs. Step needs to run.", s.TargetPath)
		return false, nil
	}

	logger.Infof("KubeProxy service file '%s' does not exist. Installation is required.", s.TargetPath)
	return false, nil
}

func (s *InstallKubeProxyServiceStep) Run(ctx runtime.ExecutionContext) error {
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

	logger.Infof("Writing systemd service file to %s", s.TargetPath)
	err = runner.WriteFile(ctx.GoContext(), conn, []byte(content), s.TargetPath, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write kube-proxy service file: %w", err)
	}

	logger.Info("Reloading systemd daemon")
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		return fmt.Errorf("failed to gather facts for daemon-reload: %w", err)
	}
	if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	return nil
}

func (s *InstallKubeProxyServiceStep) Rollback(ctx runtime.ExecutionContext) error {
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

	logger.Info("Reloading systemd daemon after rollback")
	facts, err := runner.GatherFacts(ctx.GoContext(), conn)
	if err != nil {
		logger.Errorf("Failed to gather facts for daemon-reload during rollback: %v", err)
		return nil
	}
	if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
		logger.Errorf("Failed to reload systemd daemon during rollback: %v", err)
	}

	return nil
}
