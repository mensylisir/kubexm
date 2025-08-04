package harbor

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type harborConfigForRender struct {
	Hostname            string `json:"hostname"`
	HarborAdminPassword string `json:"harbor_admin_password"`
	DataVolume          string `json:"data_volume"`
}

type GenerateHarborConfigStep struct {
	step.Base
	LocalExtractedPath string
	RenderedConfigPath string
	RenderConfig       harborConfigForRender
}

type GenerateHarborConfigStepBuilder struct {
	step.Builder[GenerateHarborConfigStepBuilder, *GenerateHarborConfigStep]
}

func NewGenerateHarborConfigStepBuilder(ctx runtime.Context, instanceName string) *GenerateHarborConfigStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentHarbor, representativeArch)
	if err != nil || binaryInfo == nil {
		return nil
	}

	cfg := ctx.GetClusterConfig().Spec
	localCfg := cfg.Registry.LocalDeployment

	if localCfg == nil || localCfg.Type != "harbor" ||
		cfg.Registry.MirroringAndRewriting == nil || cfg.Registry.MirroringAndRewriting.PrivateRegistry == "" {
		return nil
	}

	domain := cfg.Registry.MirroringAndRewriting.PrivateRegistry
	if u, err := url.Parse("scheme://" + domain); err == nil {
		domain = u.Host
	}

	adminPassword := "Harbor12345"
	if cfg.Registry.Auths != nil {
		auth, found := cfg.Registry.Auths[domain]
		if !found {
			auth, found = cfg.Registry.Auths[cfg.Registry.MirroringAndRewriting.PrivateRegistry]
		}
		if found && auth.Password != "" {
			adminPassword = auth.Password
		}
	}

	installRoot := "/opt"
	if localCfg.DataRoot != "" {
		installRoot = localCfg.DataRoot
	}
	dataVolume := filepath.Join(installRoot, "harbor", "data")

	renderCfg := harborConfigForRender{
		Hostname:            domain,
		HarborAdminPassword: adminPassword,
		DataVolume:          dataVolume,
	}

	binaryInfoNoArch, _ := provider.GetBinary(binary.ComponentHarbor, "")
	extractedDirName := strings.TrimSuffix(binaryInfoNoArch.FileName(), ".tgz")
	localExtractedPath := filepath.Join(ctx.GetExtractDir(), extractedDirName, "harbor")

	s := &GenerateHarborConfigStep{
		LocalExtractedPath: localExtractedPath,
		RenderedConfigPath: filepath.Join(localExtractedPath, "harbor.yml"),
		RenderConfig:       renderCfg,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate harbor.yml using official template", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.Timeout = 1 * time.Minute

	b := new(GenerateHarborConfigStepBuilder).Init(s)
	return b
}

func (s *GenerateHarborConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateHarborConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	templatePath := filepath.Join(s.LocalExtractedPath, "harbor.yml.tmpl")
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read official harbor.yml.tmpl: %w", err)
	}

	finalContent, err := templates.Render(string(templateContent), s.RenderConfig)
	if err != nil {
		return fmt.Errorf("failed to render official harbor template: %w", err)
	}

	logger.Infof("Writing rendered harbor.yml to %s", s.RenderedConfigPath)
	if err := os.WriteFile(s.RenderedConfigPath, []byte(finalContent), 0644); err != nil {
		return fmt.Errorf("failed to write final harbor.yml: %w", err)
	}

	logger.Info("Successfully generated harbor.yml configuration file.")
	return nil
}

func (s *GenerateHarborConfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	templatePath := filepath.Join(s.LocalExtractedPath, "harbor.yml.tmpl")
	templateContent, err := os.ReadFile(templatePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("official harbor.yml.tmpl not found at %s, ensure extract step ran successfully", templatePath)
		}
		return false, err
	}
	expectedContent, err := templates.Render(string(templateContent), s.RenderConfig)
	if err != nil {
		return false, fmt.Errorf("failed to render expected content for precheck: %w", err)
	}

	if _, err := os.Stat(s.RenderedConfigPath); os.IsNotExist(err) {
		logger.Infof("Rendered config file '%s' does not exist. Generation is required.", s.RenderedConfigPath)
		return false, nil
	}

	currentContent, err := os.ReadFile(s.RenderedConfigPath)
	if err != nil {
		logger.Warnf("Failed to read existing config file '%s', will regenerate. Error: %v", s.RenderedConfigPath, err)
		return false, nil
	}

	if string(currentContent) == expectedContent {
		logger.Info("Rendered harbor.yml already exists and content matches. Step is done.")
		return true, nil
	}

	logger.Info("Rendered harbor.yml exists but content differs. Regeneration is required.")
	return false, nil
}

func (s *GenerateHarborConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")
	finalConfigPath := filepath.Join(s.LocalExtractedPath, "harbor.yml")
	logger.Warnf("Rolling back by removing generated config file: %s", finalConfigPath)
	if err := os.Remove(finalConfigPath); err != nil && !os.IsNotExist(err) {
		logger.Errorf("Failed to remove file '%s' during rollback: %v", finalConfigPath, err)
	}
	return nil
}

var _ step.Step = (*GenerateHarborConfigStep)(nil)
