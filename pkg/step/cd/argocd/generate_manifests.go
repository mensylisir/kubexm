package argocd

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
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/helm"
	"github.com/mensylisir/kubexm/pkg/step/helpers/bom/images"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateArgoCDValuesStep struct {
	step.Base
	ImageRegistry string
	ImageTag      string
	DexImageTag   string
	RedisImageTag string
}

type GenerateArgoCDValuesStepBuilder struct {
	step.Builder[GenerateArgoCDValuesStepBuilder, *GenerateArgoCDValuesStep]
}

func NewGenerateArgoCDValuesStepBuilder(ctx runtime.Context, instanceName string) *GenerateArgoCDValuesStepBuilder {
	imageProvider := images.NewImageProvider(&ctx)
	argocdImage := imageProvider.GetImage("argocd-server")
	dexImage := imageProvider.GetImage("argocd-dex")
	redisImage := imageProvider.GetImage("argocd-redis")

	if argocdImage == nil || dexImage == nil || redisImage == nil {
		return nil
	}

	s := &GenerateArgoCDValuesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Argo CD Helm values file from configuration", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	s.ImageRegistry = argocdImage.RegistryAddr()
	if reg := ctx.GetClusterConfig().Spec.Registry.MirroringAndRewriting.PrivateRegistry; reg != "" {
		s.ImageRegistry = reg
	}

	s.ImageTag = argocdImage.Tag()
	s.DexImageTag = dexImage.Tag()
	s.RedisImageTag = redisImage.Tag()

	b := new(GenerateArgoCDValuesStepBuilder).Init(s)
	return b
}

func (s *GenerateArgoCDValuesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateArgoCDValuesStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("argocd")
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for argocd in BOM")
	}

	chartDir := filepath.Dir(chart.LocalPath(ctx.GetGlobalWorkDir()))

	valuesFileName := fmt.Sprintf("%s-values.yaml", chart.RepoName())

	return filepath.Join(chartDir, chart.Version, valuesFileName), nil
}

func (s *GenerateArgoCDValuesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Precheck")
	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		logger.Info("Skipping step, could not determine values path.", "error", err)
		return true, nil
	}

	if _, err := os.Stat(localPath); err == nil {
		logger.Info("Argo CD values file already exists. Step is complete.", "path", localPath)
		return true, nil
	}

	return false, nil
}

func (s *GenerateArgoCDValuesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Run")

	valuesTemplateContent, err := templates.Get("cd/argocd/values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded argocd values.yaml.tmpl: %w", err)
	}

	tmpl, err := template.New("argoCDValues").Parse(valuesTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse argocd values.yaml.tmpl: %w", err)
	}
	var valuesBuffer bytes.Buffer
	if err := tmpl.Execute(&valuesBuffer, s); err != nil {
		return fmt.Errorf("failed to render argocd values.yaml.tmpl: %w", err)
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return err
	}

	logger.Info("Generating Argo CD Helm values file.", "path", localPath)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create local directory for values file: %w", err)
	}

	if err := os.WriteFile(localPath, valuesBuffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write generated values file to %s: %w", localPath, err)
	}

	logger.Info("Successfully generated Argo CD Helm values file.")
	return nil
}

func (s *GenerateArgoCDValuesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "host", ctx.GetHost().GetName(), "phase", "Rollback")

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		logger.Info("Skipping rollback as no values path could be determined.", "error", err)
		return nil
	}

	if _, statErr := os.Stat(localPath); statErr == nil {
		logger.Warn("Rolling back by deleting generated values file.", "path", localPath)
		if err := os.Remove(localPath); err != nil {
			logger.Error(err, "Failed to remove file during rollback.", "path", localPath)
		}
	} else {
		logger.Info("Rollback unnecessary, file to be deleted does not exist.", "path", localPath)
	}

	return nil
}

var _ step.Step = (*GenerateArgoCDValuesStep)(nil)
