package haproxy

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateHAProxyStaticPodManifestStep struct {
	step.Base
}

func NewGenerateHAProxyStaticPodManifestStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *GenerateHAProxyStaticPodManifestStep] {
	s := &GenerateHAProxyStaticPodManifestStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate haproxy static pod manifest"
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

type HAProxyStaticPodTemplateData struct {
	Image string
}

func (s *GenerateHAProxyStaticPodManifestStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())

	// In a real scenario, the image would come from the ImageProvider/BOM
	data := HAProxyStaticPodTemplateData{
		Image: "haproxy:2.5",
	}

	templateContent, err := templates.Get("loadbalancer/haproxy/haproxy.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get haproxy static pod manifest template: %w", err)
	}

	renderedManifest, err := templates.Render(templateContent, data)
	if err != nil {
		return fmt.Errorf("failed to render haproxy static pod manifest: %w", err)
	}

	ctx.Set("haproxy.yaml", renderedManifest)
	logger.Info("haproxy.yaml generated successfully.")

	return nil
}

func (s *GenerateHAProxyStaticPodManifestStep) Rollback(ctx runtime.ExecutionContext) error {
	ctx.Delete("haproxy.yaml")
	return nil
}

func (s *GenerateHAProxyStaticPodManifestStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
