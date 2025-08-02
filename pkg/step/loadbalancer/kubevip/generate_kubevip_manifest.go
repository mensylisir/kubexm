package kubevip

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateKubeVipManifestStep struct {
	step.Base
}

func NewGenerateKubeVipManifestStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *GenerateKubeVipManifestStep] {
	s := &GenerateKubeVipManifestStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate kube-vip static pod manifest"
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

type KubeVipTemplateData struct {
	Image         string
	VIP           string
	Interface     string
}

func (s *GenerateKubeVipManifestStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())

	// In a real scenario, this data would come from the cluster spec
	data := KubeVipTemplateData{
		Image:     "ghcr.io/kube-vip/kube-vip:v0.5.7",
		VIP:       "192.168.1.200",
		Interface: "eth0",
	}

	templateContent, err := templates.Get("loadbalancer/kubevip/kube-vip.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get kube-vip manifest template: %w", err)
	}

	renderedManifest, err := templates.Render(templateContent, data)
	if err != nil {
		return fmt.Errorf("failed to render kube-vip manifest: %w", err)
	}

	ctx.Set("kube-vip.yaml", renderedManifest)
	logger.Info("kube-vip.yaml generated successfully.")

	return nil
}

func (s *GenerateKubeVipManifestStep) Rollback(ctx runtime.ExecutionContext) error {
	ctx.Delete("kube-vip.yaml")
	return nil
}

func (s *GenerateKubeVipManifestStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
