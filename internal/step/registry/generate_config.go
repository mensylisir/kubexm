package registry

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step"
		"github.com/mensylisir/kubexm/internal/step/helpers/bom/binary"
	"github.com/mensylisir/kubexm/internal/types"
)

type registryConfigForRender struct {
	StorageRootDirectory string
}

type GenerateRegistryConfigStep struct {
	step.Base
	RenderedConfigPath string
	RenderConfig       registryConfigForRender
}

type GenerateRegistryConfigStepBuilder struct {
	step.Builder[GenerateRegistryConfigStepBuilder, *GenerateRegistryConfigStep]
}

func NewGenerateRegistryConfigStepBuilder(ctx runtime.ExecutionContext, instanceName string) *GenerateRegistryConfigStepBuilder {
	provider := binary.NewBinaryProvider(ctx)
	const representativeArch = "amd64"
	binaryInfo, err := provider.GetBinary(binary.ComponentRegistry, representativeArch)
	if err != nil || binaryInfo == nil {
		return nil
	}

	cfg := ctx.GetClusterConfig().Spec
	localCfg := cfg.Registry.LocalDeployment

	if localCfg == nil || localCfg.Type != "registry" {
		return nil
	}

	storageRoot := "/var/lib/registry"
	if localCfg.DataRoot != "" {
		storageRoot = localCfg.DataRoot
	}

	renderCfg := registryConfigForRender{
		StorageRootDirectory: storageRoot,
	}

	s := &GenerateRegistryConfigStep{
		RenderedConfigPath: filepath.Join(ctx.GetGlobalWorkDir(), "registry", "config.yml"),
		RenderConfig:       renderCfg,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate registry config.yml", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 1 * time.Minute

	b := new(GenerateRegistryConfigStepBuilder).Init(s)
	return b
}

func (s *GenerateRegistryConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateRegistryConfigStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	expectedContent, err := s.renderContent()
	if err != nil {
		return false, fmt.Errorf("failed to render expected content for precheck: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.RenderedConfigPath), 0755); err != nil {
		return false, fmt.Errorf("failed to create directory for rendered config: %w", err)
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
		logger.Info("Rendered registry config.yml already exists and content matches. Step is done.")
		return true, nil
	}

	logger.Info("Rendered registry config.yml exists but content differs. Regeneration is required.")
	return false, nil
}

func (s *GenerateRegistryConfigStep) Run(ctx runtime.ExecutionContext) (*types.StepResult, error) {
	result := types.NewStepResult(s.Base.Meta.Name, ctx.GetStepExecutionID(), ctx.GetHost())
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	content, err := s.renderContent()
	if err != nil {
		result.MarkFailed(err, "failed to render registry config")
		return result, err
	}

	logger.Infof("Writing rendered registry config.yml to %s", s.RenderedConfigPath)
	if err := os.WriteFile(s.RenderedConfigPath, []byte(content), 0644); err != nil {
		result.MarkFailed(err, "failed to write rendered registry config.yml")
		return result, err
	}

	logger.Info("Successfully generated registry config.yml.")
	result.MarkCompleted("registry config generated successfully")
	return result, nil
}

func (s *GenerateRegistryConfigStep) renderContent() (string, error) {
	tmplStr := `version: 0.1
log:
  fields:
    service: registry
storage:
  cache:
    blobdescriptor: inmemory
  filesystem:
    rootdirectory: {{ .StorageRootDirectory }}
http:
  addr: :5000
  headers:
    X-Content-Type-Options: [nosniff]
health:
  storagedriver:
    enabled: true
    interval: 10s
    threshold: 3
`
	tmpl, err := template.New("registryConfig").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("failed to parse registry config template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, s.RenderConfig); err != nil {
		return "", fmt.Errorf("failed to render registry config template: %w", err)
	}
	return buf.String(), nil
}

func (s *GenerateRegistryConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	logger.Warnf("Rolling back by removing generated config file: %s", s.RenderedConfigPath)
	if err := os.Remove(s.RenderedConfigPath); err != nil && !os.IsNotExist(err) {
		logger.Errorf("Failed to remove file '%s' during rollback: %v", s.RenderedConfigPath, err)
	}

	return nil
}

var _ step.Step = (*GenerateRegistryConfigStep)(nil)
