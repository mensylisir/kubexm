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
	KubeApiServerServiceTemplatePath = "kubernetes/kube-apiserver.service.tmpl"
)

type InstallKubeApiServerServiceStep struct {
	step.Base
	TargetPath           string
	KubeApiServerExecStartPath string
}

type InstallKubeApiServerServiceStepBuilder struct {
	step.Builder[InstallKubeApiServerServiceStepBuilder, *InstallKubeApiServerServiceStep]
}

func NewInstallKubeApiServerServiceStepBuilder(ctx runtime.Context, instanceName string) *InstallKubeApiServerServiceStepBuilder {
	s := &InstallKubeApiServerServiceStep{
		TargetPath:           common.KubeApiServerDefaultSystemdFile,
		KubeApiServerExecStartPath: filepath.Join(common.DefaultBinDir, "kube-apiserver"),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install kube-apiserver systemd service from template", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(InstallKubeApiServerServiceStepBuilder).Init(s)
	return b
}

func (b *InstallKubeApiServerServiceStepBuilder) WithTargetPath(path string) *InstallKubeApiServerServiceStepBuilder {
	b.Step.TargetPath = path
	return b
}

func (b *InstallKubeApiServerServiceStepBuilder) WithKubeApiServerExecStartPath(path string) *InstallKubeApiServerServiceStepBuilder {
	b.Step.KubeApiServerExecStartPath = path
	return b
}

func (s *InstallKubeApiServerServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallKubeApiServerServiceStep) renderContent(ctx runtime.ExecutionContext) (string, error) {
	tmplStr, err := templates.Get(KubeApiServerServiceTemplatePath)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("kube-apiserver.service").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse kube-apiserver service template: %w", err)
	}

	data := struct {
		KubeApiServerExecStartPath string
	}{
		KubeApiServerExecStartPath: s.KubeApiServerExecStartPath,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render kube-apiserver service template: %w", err)
	}

	return buf.String(), nil
}

func (s *InstallKubeApiServerServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
			logger.Infof("KubeApiServer service file '%s' already exists and content matches. Step is done.", s.TargetPath)
			return true, nil
		}
		logger.Infof("KubeApiServer service file '%s' exists but content differs. Step needs to run.", s.TargetPath)
		return false, nil
	}

	logger.Infof("KubeApiServer service file '%s' does not exist. Installation is required.", s.TargetPath)
	return false, nil
}

func (s *InstallKubeApiServerServiceStep) Run(ctx runtime.ExecutionContext) error {
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
		return fmt.Errorf("failed to write kube-apiserver service file: %w", err)
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

func (s *InstallKubeApiServerServiceStep) Rollback(ctx runtime.ExecutionContext) error {
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
