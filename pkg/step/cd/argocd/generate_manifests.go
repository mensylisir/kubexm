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

// GenerateArgoCDValuesStep is responsible for generating the Helm values file for Argo CD.
type GenerateArgoCDValuesStep struct {
	step.Base
	ImageRegistry string
	ImageTag      string
	DexImageTag   string
	RedisImageTag string
}

// GenerateArgoCDValuesStepBuilder is used to build instances.
type GenerateArgoCDValuesStepBuilder struct {
	step.Builder[GenerateArgoCDValuesStepBuilder, *GenerateArgoCDValuesStep]
}

// NewGenerateArgoCDValuesStepBuilder is the constructor.
func NewGenerateArgoCDValuesStepBuilder(ctx runtime.Context, instanceName string) *GenerateArgoCDValuesStepBuilder {
	imageProvider := images.NewImageProvider(&ctx)

	argocdImage := imageProvider.GetImage("argocd-server") // Main image
	dexImage := imageProvider.GetImage("argocd-dex")
	redisImage := imageProvider.GetImage("argocd-redis")

	if argocdImage == nil || dexImage == nil || redisImage == nil {
		// TODO: Add a check for whether argocd is enabled
		fmt.Fprintf(os.Stderr, "Error: Argo CD is enabled but one or more required images are not found in BOM for K8s version %s\n", ctx.GetClusterConfig().Spec.Kubernetes.Version)
		return nil
	}

	s := &GenerateArgoCDValuesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Argo CD Helm values file from configuration", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	s.ImageRegistry = argocdImage.Registry()
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

func (s *GenerateArgoCDValuesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// TODO: Add a check to see if argocd is enabled
	return false, nil
}

func (s *GenerateArgoCDValuesStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("argocd")
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for argocd in BOM")
	}
	chartTgzPath := chart.LocalPath(ctx.GetClusterArtifactsDir())
	chartDir := filepath.Dir(chartTgzPath)
	return filepath.Join(chartDir, "argocd-values.yaml"), nil
}

func (s *GenerateArgoCDValuesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

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

	logger.Infof("Generating Argo CD Helm values file to: %s", localPath)

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
	if localPath, err := s.getLocalValuesPath(ctx); err == nil {
		os.Remove(localPath)
	}
	return nil
}

var _ step.Step = (*GenerateArgoCDValuesStep)(nil)
