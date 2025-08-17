package crio

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
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"github.com/mensylisir/kubexm/pkg/templates"
)

const (
	crioConfigTemplatePath = "crio/10-crio.conf.tmpl"
)

type templateData struct {
	PauseImage          string
	SignaturePolicyPath string
	CgroupDriver        string
	RuntimePath         string
	MonitorPath         string
}

type ConfigureCrioStep struct {
	step.Base
	Data       templateData
	TargetPath string
}

type ConfigureCrioStepBuilder struct {
	step.Builder[ConfigureCrioStepBuilder, *ConfigureCrioStep]
}

func NewConfigureCrioStepBuilder(ctx runtime.Context, instanceName string) *ConfigureCrioStepBuilder {
	cfg := ctx.GetClusterConfig().Spec

	data := templateData{
		SignaturePolicyPath: common.SignaturePolicyPath,
		CgroupDriver:        common.CgroupDriverSystemd,
		RuntimePath:         common.CRIORuntimePath,
		MonitorPath:         common.CRIOMonitorPath,
	}

	imageProvider := images.NewImageProvider(&ctx)
	pauseImage := imageProvider.GetImage("pause")
	if pauseImage == nil {
		ctx.GetLogger().Error("Critical: Failed to get pause image from BOM, cannot configure CRI-O.")
		return nil
	}
	data.PauseImage = pauseImage.FullName()

	if cfg.Kubernetes.ContainerRuntime != nil && cfg.Kubernetes.ContainerRuntime.Crio != nil {
		userCfg := cfg.Kubernetes.ContainerRuntime.Crio
		if userCfg.Pause != "" {
			data.PauseImage = userCfg.Pause
		}
		if userCfg.CgroupDriver != nil && *userCfg.CgroupDriver != "" {
			data.CgroupDriver = *userCfg.CgroupDriver
		}
		if userCfg.Runtimes != nil {
			if runc, ok := userCfg.Runtimes["runc"]; ok && runc.RuntimePath != "" {
				data.RuntimePath = filepath.Dir(runc.RuntimePath)
			}
		}
		if userCfg.Conmon != nil && *userCfg.Conmon != "" {
			data.MonitorPath = filepath.Dir(*userCfg.Conmon)
		}
	}

	s := &ConfigureCrioStep{
		Data:       data,
		TargetPath: filepath.Join(common.CRIODefaultConfDir, "crio.conf.d", "00-kubexm-crio.conf"),
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure CRI-O main settings", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(ConfigureCrioStepBuilder).Init(s)
	return b
}

func (s *ConfigureCrioStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureCrioStep) renderContent() (string, error) {
	tmplStr, err := templates.Get(crioConfigTemplatePath)
	if err != nil {
		return "", err
	}
	tmpl, err := template.New("crio.conf").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse crio config template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s.Data); err != nil {
		return "", fmt.Errorf("failed to render crio config template: %w", err)
	}
	return buf.String(), nil
}

func (s *ConfigureCrioStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return false, err
	}

	expectedContent, err := s.renderContent()
	if err != nil {
		return false, fmt.Errorf("failed to render expected content for precheck: %w", err)
	}

	exists, err := runner.Exists(ctx.GoContext(), conn, s.TargetPath)
	if err != nil {
		return false, fmt.Errorf("failed to check for config file '%s': %w", s.TargetPath, err)
	}

	if exists {
		remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, s.TargetPath)
		if err != nil {
			logger.Warn(err, "Config file exists but failed to read, will overwrite.", "path", s.TargetPath)
			return false, nil
		}
		if string(remoteContent) == expectedContent {
			logger.Info("CRI-O config file already exists and content matches. Step is done.", "path", s.TargetPath)
			return true, nil
		}
		logger.Info("CRI-O config file exists but content differs. Step needs to run.", "path", s.TargetPath)
		return false, nil
	}

	logger.Info("CRI-O config file does not exist. Configuration is required.", "path", s.TargetPath)
	return false, nil
}

func (s *ConfigureCrioStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	targetDir := filepath.Dir(s.TargetPath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, targetDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create CRI-O config directory '%s': %w", targetDir, err)
	}

	content, err := s.renderContent()
	if err != nil {
		return err
	}

	logger.Info("Writing CRI-O config file.", "path", s.TargetPath)
	return helpers.WriteContentToRemote(ctx, conn, content, s.TargetPath, "0644", s.Sudo)
}

func (s *ConfigureCrioStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		logger.Error(err, "Failed to get connector for rollback.")
		return nil
	}

	logger.Warn("Rolling back by removing.", "path", s.TargetPath)
	if err := runner.Remove(ctx.GoContext(), conn, s.TargetPath, s.Sudo, false); err != nil {
		if !strings.Contains(err.Error(), "no such file or directory") {
			logger.Error(err, "Failed to remove path during rollback.", "path", s.TargetPath)
		}
	}
	return nil
}

var _ step.Step = (*ConfigureCrioStep)(nil)
