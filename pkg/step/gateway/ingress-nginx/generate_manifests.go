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

// GenerateIngressNginxValuesStep is responsible for generating the Helm values file for Ingress-Nginx.
type GenerateIngressNginxValuesStep struct {
	step.Base
	ImageRegistry          string
	ImageTag               string
	WebhookImageTag        string
	DefaultBackendImageTag string
}

// GenerateIngressNginxValuesStepBuilder is used to build instances.
type GenerateIngressNginxValuesStepBuilder struct {
	step.Builder[GenerateIngressNginxValuesStepBuilder, *GenerateIngressNginxValuesStep]
}

// NewGenerateIngressNginxValuesStepBuilder is the constructor.
func NewGenerateIngressNginxValuesStepBuilder(ctx runtime.Context, instanceName string) *GenerateIngressNginxValuesStepBuilder {
	imageProvider := images.NewImageProvider(&ctx)

	// Assuming image names in BOM are 'ingress-nginx-controller', 'ingress-nginx-webhook', 'ingress-nginx-defaultbackend'
	controllerImage := imageProvider.GetImage("ingress-nginx-controller")
	webhookImage := imageProvider.GetImage("ingress-nginx-webhook")
	defaultBackendImage := imageProvider.GetImage("ingress-nginx-defaultbackend")

	if controllerImage == nil || webhookImage == nil || defaultBackendImage == nil {
		// TODO: Add a check for whether ingress-nginx is enabled
		fmt.Fprintf(os.Stderr, "Error: Ingress-Nginx is enabled but one or more required images are not found in BOM for K8s version %s\n", ctx.GetClusterConfig().Spec.Kubernetes.Version)
		return nil
	}

	s := &GenerateIngressNginxValuesStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = fmt.Sprintf("[%s]>>Generate Ingress-Nginx Helm values file from configuration", s.Base.Meta.Name)
	s.Base.Sudo = false
	s.Base.IgnoreError = false
	s.Base.Timeout = 2 * time.Minute

	// By default, the registry part of the image name from BOM is used.
	// This can be overridden by a global private registry setting.
	s.ImageRegistry = controllerImage.Registry()
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

func (s *GenerateIngressNginxValuesStep) Precheck(ctx runtime.ExecutionContext) (isDone bool, err error) {
	// TODO: Add a check to see if ingress-nginx is enabled
	return false, nil
}

func (s *GenerateIngressNginxValuesStep) getLocalValuesPath(ctx runtime.ExecutionContext) (string, error) {
	helmProvider := helm.NewHelmProvider(ctx)
	chart := helmProvider.GetChart("ingress-nginx")
	if chart == nil {
		return "", fmt.Errorf("cannot find chart info for ingress-nginx in BOM")
	}
	chartTgzPath := chart.LocalPath(ctx.GetClusterArtifactsDir())
	chartDir := filepath.Dir(chartTgzPath)
	return filepath.Join(chartDir, "ingress-nginx-values.yaml"), nil
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
	if localPath, err := s.getLocalValuesPath(ctx); err == nil {
		os.Remove(localPath)
	}
	return nil
}

var _ step.Step = (*GenerateIngressNginxValuesStep)(nil)
