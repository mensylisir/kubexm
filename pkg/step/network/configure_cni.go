package network

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type ConfigureCniStep struct {
	step.Base
	TargetPath   string
	TemplatePath string
	PodCidr      string
}

type ConfigureCniStepBuilder struct {
	step.Builder[ConfigureCniStepBuilder, *ConfigureCniStep]
}

func NewConfigureCniStepBuilder(ctx runtime.Context, instanceName string) *ConfigureCniStepBuilder {
	cfg := ctx.GetClusterConfig().Spec
	s := &ConfigureCniStep{
		TargetPath:   filepath.Join(common.DefaultCNIConfDirTarget, "10-kubexm-cni.conf"),
		TemplatePath: "cni/10-kubexm-cni.conf.tmpl",
		PodCidr:      "10.244.0.0/16",
	}

	if cfg.Network != nil && cfg.Network.KubePodsCIDR != "" {
		s.PodCidr = cfg.Network.KubePodsCIDR
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure CNI", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.Timeout = 1 * time.Minute

	b := new(ConfigureCniStepBuilder).Init(s)
	return b
}

func (s *ConfigureCniStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureCniStep) renderContent(ctx runtime.ExecutionContext) (string, error) {
	tmplStr, err := templates.Get(s.TemplatePath)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("10-kubexm-cni.conf").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse cni config template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s); err != nil {
		return "", fmt.Errorf("failed to render cni config template: %w", err)
	}
	return buf.String(), nil
}

func (s *ConfigureCniStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
		return false, fmt.Errorf("failed to check for config file '%s': %w", s.TargetPath, err)
	}
	if exists {
		remoteContent, err := runner.ReadFile(ctx.GoContext(), conn, s.TargetPath)
		if err != nil {
			logger.Warnf("Config file '%s' exists but failed to read, will overwrite. Error: %v", s.TargetPath, err)
			return false, nil
		}
		if string(remoteContent) == expectedContent {
			logger.Infof("CNI config file '%s' already exists and content matches. Step is done.", s.TargetPath)
			return true, nil
		}
		logger.Infof("CNI config file '%s' exists but content differs. Step needs to run.", s.TargetPath)
		return false, nil
	}

	logger.Infof("CNI config file '%s' does not exist. Configuration is required.", s.TargetPath)
	return false, nil
}

func (s *ConfigureCniStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	//runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	content, err := s.renderContent(ctx)
	if err != nil {
		return err
	}

	logger.Infof("Writing CNI config file to %s", s.TargetPath)
	err = helpers.WriteContentToRemote(ctx, conn, content, s.TargetPath, "0644", s.Sudo)
	if err != nil {
		return fmt.Errorf("failed to write CNI config file: %w", err)
	}

	return nil
}

func (s *ConfigureCniStep) Rollback(ctx runtime.ExecutionContext) error {
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

	return nil
}
