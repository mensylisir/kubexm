package registry

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"text/template"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/binary"
)

// registryConfigForRender 用于渲染 config.yml 模板
type registryConfigForRender struct {
	StorageRootDirectory string
}

// GenerateRegistryConfigStep 是一个无状态的编排步骤。
type GenerateRegistryConfigStep struct {
	step.Base
	RenderedConfigPath string
	RenderConfig       registryConfigForRender
}

type GenerateRegistryConfigStepBuilder struct {
	step.Builder[GenerateRegistryConfigStepBuilder, *GenerateRegistryConfigStep]
}

// NewGenerateRegistryConfigStepBuilder 在创建前会检查 Registry 是否被启用。
func NewGenerateRegistryConfigStepBuilder(ctx runtime.Context, instanceName string) *GenerateRegistryConfigStepBuilder {
	provider := binary.NewBinaryProvider(&ctx)
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

	// 确定存储根目录
	storageRoot := "/var/lib/registry" // 官方默认
	if localCfg.DataRoot != "" {
		storageRoot = localCfg.DataRoot
	}

	renderCfg := registryConfigForRender{
		StorageRootDirectory: storageRoot,
	}

	s := &GenerateRegistryConfigStep{
		// 将配置文件生成在 artifacts 目录，以便后续分发
		RenderedConfigPath: filepath.Join(ctx.GetClusterArtifactsDir(), "registry", "config.yml"),
		RenderConfig:       renderCfg,
	}

	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate registry config.yml", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.Timeout = 1 * time.Minute

	b := new(GenerateRegistryConfigStepBuilder).Init(s)
	return b
}

func (s *GenerateRegistryConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

// Precheck 检查渲染后的 config.yml 是否已存在且内容一致。
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

// Run 执行模板渲染并写入文件。
func (s *GenerateRegistryConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	content, err := s.renderContent()
	if err != nil {
		return err
	}

	logger.Infof("Writing rendered registry config.yml to %s", s.RenderedConfigPath)
	if err := os.WriteFile(s.RenderedConfigPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write rendered registry config.yml: %w", err)
	}

	logger.Info("Successfully generated registry config.yml.")
	return nil
}

// renderContent 是一个辅助函数，负责读取模板并执行渲染。
func (s *GenerateRegistryConfigStep) renderContent() (string, error) {
	// 这个模板非常简单，我们可以直接内联，或者从 templates 包读取
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
