// pkg/step/crio/configure_registries.go

package crio

import (
	"bytes"
	"fmt"
	"github.com/mensylisir/kubexm/pkg/apis/kubexms/v1alpha1"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type registriesTemplateData struct {
	UnqualifiedSearchRegistries []string
	InsecureRegistries          []string
	Registries                  []v1alpha1.RegistryMirror
}

type ConfigureRegistriesStep struct {
	step.Base
	Data       registriesTemplateData
	TargetPath string
}

type ConfigureRegistriesStepBuilder struct {
	step.Builder[ConfigureRegistriesStepBuilder, *ConfigureRegistriesStep]
}

func NewConfigureRegistriesStepBuilder(ctx runtime.Context, instanceName string) *ConfigureRegistriesStepBuilder {
	cfg := ctx.GetClusterConfig().Spec

	if cfg.Kubernetes.ContainerRuntime == nil || cfg.Kubernetes.ContainerRuntime.Crio == nil {
		return nil
	}
	userCrioCfg := cfg.Kubernetes.ContainerRuntime.Crio

	data := registriesTemplateData{}

	searchRegistries := common.DefaultUnqualifiedSearchRegistries
	if cfg.Registry != nil && cfg.Registry.MirroringAndRewriting != nil && cfg.Registry.MirroringAndRewriting.PrivateRegistry != "" {
		privateRegistryHost := cfg.Registry.MirroringAndRewriting.PrivateRegistry
		searchRegistries = prependIfNotExist(searchRegistries, privateRegistryHost)
	}
	if userCrioCfg.Registry != nil && len(userCrioCfg.Registry.UnqualifiedSearchRegistries) > 0 {
		searchRegistries = append(searchRegistries, userCrioCfg.Registry.UnqualifiedSearchRegistries...)
	}
	data.UnqualifiedSearchRegistries = searchRegistries

	insecureRegistries := []string{}
	if cfg.Registry != nil && len(cfg.Registry.Auths) > 0 {
		for server, auth := range cfg.Registry.Auths {
			isPlainHTTP := auth.PlainHTTP != nil && *auth.PlainHTTP
			isSkipTLSVerify := auth.SkipTLSVerify != nil && *auth.SkipTLSVerify
			if isPlainHTTP || isSkipTLSVerify {
				insecureRegistries = append(insecureRegistries, server)
			}
		}
	}

	mirrors := make([]v1alpha1.RegistryMirror, 0)

	if userCrioCfg.Registry != nil && len(userCrioCfg.Registry.Registries) > 0 {
		for _, registry := range userCrioCfg.Registry.Registries {
			if registry.Prefix == "" {
				if registry.Insecure != nil && *registry.Insecure {
					insecureRegistries = append(insecureRegistries, registry.Location)
				}
			} else {
				mirrors = append(mirrors, registry)
			}
		}
	}

	data.InsecureRegistries = helpers.RemoveDuplicates(insecureRegistries)
	data.Registries = mirrors

	s := &ConfigureRegistriesStep{
		Data:       data,
		TargetPath: common.RegistriesDefaultConfigFile,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Configure CRI-O registries", s.Base.Meta.Name)
	s.Base.Sudo = true
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(ConfigureRegistriesStepBuilder).Init(s)
	return b
}

func prependIfNotExist(slice []string, elem string) []string {
	for _, s := range slice {
		if s == elem {
			return slice
		}
	}
	return append([]string{elem}, slice...)
}

func (s *ConfigureRegistriesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *ConfigureRegistriesStep) renderContent() (string, error) {
	tmplStr, err := templates.Get("crio/registries.conf.tmpl")
	if err != nil {
		return "", err
	}
	tmpl, err := template.New("registries.conf").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse registries config template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s.Data); err != nil {
		return "", fmt.Errorf("failed to render registries config template: %w", err)
	}
	return buf.String(), nil
}

func (s *ConfigureRegistriesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
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
			logger.Info("Registries config file already exists and content matches. Step is done.", "path", s.TargetPath)
			return true, nil
		}

		logger.Info("Registries config file exists but content differs. Step needs to run.", "path", s.TargetPath)
		return false, nil
	}

	logger.Info("Registries config file does not exist. Configuration is required.", "path", s.TargetPath)
	return false, nil
}

func (s *ConfigureRegistriesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")
	runner := ctx.GetRunner()
	conn, err := ctx.GetCurrentHostConnector()
	if err != nil {
		return err
	}

	targetDir := filepath.Dir(s.TargetPath)
	if err := runner.Mkdirp(ctx.GoContext(), conn, targetDir, "0755", s.Sudo); err != nil {
		return fmt.Errorf("failed to create registries config directory '%s': %w", targetDir, err)
	}

	content, err := s.renderContent()
	if err != nil {
		return err
	}

	logger.Info("Writing registries config file.", "path", s.TargetPath)
	return helpers.WriteContentToRemote(ctx, conn, content, s.TargetPath, "0644", s.Sudo)
}

func (s *ConfigureRegistriesStep) Rollback(ctx runtime.ExecutionContext) error {
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

var _ step.Step = (*ConfigureRegistriesStep)(nil)
