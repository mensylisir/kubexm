package kubelet

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/internal/step/helpers"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
	"github.com/mensylisir/kubexm/internal/templates"
	"github.com/mensylisir/kubexm/internal/types"
)

const (
	KubeletServiceTemplatePath = "kubernetes/kubelet.service.tmpl"
)

var _ step.Step = (*InstallKubeletServiceStep)(nil)

type InstallKubeletServiceStep struct {
	step.Base
	TargetPath           string
	KubeletExecStartPath string
}

type InstallKubeletServiceStepBuilder struct {
	step.Builder[InstallKubeletServiceStepBuilder, *InstallKubeletServiceStep]
}

func NewInstallKubeletServiceStepBuilder(ctx runtime.ExecutionContext, instanceName string) *InstallKubeletServiceStepBuilder {
	s := &InstallKubeletServiceStep{
		TargetPath:           common.DefaultKubeletServiceFile,
		KubeletExecStartPath: filepath.Join(common.DefaultBinDir, "kubelet"),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Install kubelet systemd service from template", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(InstallKubeletServiceStepBuilder).Init(s)
	return b
}

func (b *InstallKubeletServiceStepBuilder) WithTargetPath(path string) *InstallKubeletServiceStepBuilder {
	b.Step.TargetPath = path
	return b
}

func (b *InstallKubeletServiceStepBuilder) WithKubeletExecStartPath(path string) *InstallKubeletServiceStepBuilder {
	b.Step.KubeletExecStartPath = path
	return b
}

func (s *InstallKubeletServiceStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *InstallKubeletServiceStep) renderContent(ctx runtime.ExecutionContext) (string, error) {
	tmplStr, err := templates.Get(KubeletServiceTemplatePath)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("kubelet.service").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse kubelet service template: %w", err)
	}

	data := struct {
		KubeletExecStartPath string
	}{
		KubeletExecStartPath: s.KubeletExecStartPath,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render kubelet service template: %w", err)
	}

	return buf.String(), nil
}

func (s *InstallKubeletServiceStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
			logger.Infof("Kubelet service file '%s' already exists and content matches. Step is done.", s.TargetPath)
			return true, nil
		}
		logger.Infof("Kubelet service file '%s' exists but content differs. Step needs to run.", s.TargetPath)
		return false, nil
	}

	logger.Infof("Kubelet service file '%s' does not exist. Installation is required.", s.TargetPath)
	return false, nil
}

func (s *InstallKubeletServiceStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		result.MarkFailed(err, "failed to get current host connector")
		return result, err
	}

	content, err := s.renderContent(ctx)
	if err != nil {
		result.MarkFailed(err, "failed to render service content")
		return result, err
	}

	logger.Infof("Writing systemd service file to %s", s.TargetPath)
	err = helpers.WriteContentToRemote(ctx, conn, content, s.TargetPath, "0644", s.Sudo)
	if err != nil {
		err = fmt.Errorf("failed to write kubelet service file: %w", err)
		result.MarkFailed(err, "failed to write service file")
		return result, err
	}

	logger.Info("Reloading systemd daemon")
	facts, err := ctx.GetHostFacts(ctx.GetHost())
	if err != nil {
		err = fmt.Errorf("failed to gather facts for daemon-reload: %w", err)
		result.MarkFailed(err, "failed to gather facts")
		return result, err
	}
	if err := runner.DaemonReload(ctx.GoContext(), conn, facts); err != nil {
		err = fmt.Errorf("failed to reload systemd daemon: %w", err)
		result.MarkFailed(err, "failed to reload daemon")
		return result, err
	}

	result.MarkCompleted("service installed successfully")
	return result, nil
}

func (s *InstallKubeletServiceStep) Rollback(ctx runtime.ExecutionContext) error {
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
