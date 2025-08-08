package ingressnginx

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

type GenerateIngressNginxValuesStep struct {
	step.Base
	ImageRegistry          string
	ImageTag               string
	WebhookImageTag        string
	DefaultBackendImageTag string
}

type GenerateIngressNginxValuesStepBuilder struct {
	step.Builder[GenerateIngressNginxValuesStepBuilder, *GenerateIngressNginxValuesStep]
}

func NewGenerateIngressNginxValuesStepBuilder(ctx runtime.Context, instanceName string) *GenerateIngressNginxValuesStepBuilder {
	imageProvider := images.NewImageProvider(&ctx)
	controllerImage := imageProvider.GetImage("ingress-nginx-controller")
	webhookImage := imageProvider.GetImage("ingress-nginx-webhook")
	defaultBackendImage := imageProvider.GetImage("ingress-nginx-defaultbackend")

	if controllerImage == nil || webhookImage == nil || defaultBackendImage == nil {
		return nil
	}

	s := &GenerateIngressNginxValuesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Ingress-Nginx Helm values file from configuration", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	s.ImageRegistry = controllerImage.RegistryAddr()
	if reg := ctx.GetClusterConfig().Spec.Registry.MirroringAndRewriting.PrivateRegistry; reg != "" {
		s.ImageRegistry = reg
	}

	s.ImageTag = controllerImage.Tag()
	s.WebhookImageTag = webhookImage.Tag()
	s.DefaultBackendImageTag = defaultBackendImage.Tag()

	b := new(GenerateIngressNginxValuesStepBuilder).Init(s)
	return b
}

func (s *GenerateIngressNginxValuesStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}

func (s *GenerateIngressNginxValuesStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("ingress-nginx")
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for ingress-nginx in BOM")
	}

	chartDir := filepath.Dir(chart.LocalPath(ctx.GetGlobalWorkDir()))

	valuesFileName := fmt.Sprintf("%s-values.yaml", chart.RepoName())

	return filepath.Join(chartDir, chart.Version, valuesFileName), nil
}

func (s *GenerateIngressNginxValuesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Precheck")

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		logger.Infof("Skipping step, could not determine values path: %v", err)
		return true, nil
	}

	if _, err := os.Stat(localPath); err == nil {
		logger.Infof("Ingress-Nginx values file %s already exists. Step is complete.", localPath)
		return true, nil
	}

	return false, nil
}

func (s *GenerateIngressNginxValuesStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Run")

	valuesTemplateContent, err := templates.Get("gateway/ingress-nginx/values.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get embedded ingress-nginx values.yaml.tmpl: %w", err)
	}

	tmpl, err := template.New("ingressNginxValues").Parse(valuesTemplateContent)
	if err != nil {
		return fmt.Errorf("failed to parse ingress-nginx values.yaml.tmpl: %w", err)
	}
	var valuesBuffer bytes.Buffer
	if err := tmpl.Execute(&valuesBuffer, s); err != nil {
		return fmt.Errorf("failed to render ingress-nginx values.yaml.tmpl: %w", err)
	}

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		return err
	}

	logger.Infof("Generating Ingress-Nginx Helm values file to: %s", localPath)

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create local directory for values file: %w", err)
	}

	if err := os.WriteFile(localPath, valuesBuffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write generated values file to %s: %w", localPath, err)
	}

	logger.Info("Successfully generated Ingress-Nginx Helm values file.")
	return nil
}

func (s *GenerateIngressNginxValuesStep) Rollback(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Base.Meta.Name, "phase", "Rollback")

	localPath, err := s.getLocalValuesPath(ctx)
	if err != nil {
		logger.Infof("Skipping rollback as no values path could be determined: %v", err)
		return nil
	}

	if _, statErr := os.Stat(localPath); statErr == nil {
		logger.Warnf("Rolling back by deleting generated values file: %s", localPath)
		if err := os.Remove(localPath); err != nil {
			logger.Errorf("Failed to remove file during rollback: %v", err)
		}
	} else {
		logger.Infof("Rollback unnecessary, file to be deleted does not exist: %s", localPath)
	}

	return nil
}

var _ step.Step = (*GenerateIngressNginxValuesStep)(nil)
