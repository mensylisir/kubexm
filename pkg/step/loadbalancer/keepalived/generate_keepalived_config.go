package keepalived

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateKeepalivedConfigStep struct {
	step.Base
}

func NewGenerateKeepalivedConfigStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *GenerateKeepalivedConfigStep] {
	s := &GenerateKeepalivedConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate keepalived config file"
	s.Base.Sudo = false
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

type KeepalivedTemplateData struct {
	VRRPInterface      string
	VRRPVirtualRouterID int
	VRRPPriority       int
	VRRPVirtualIP      string
	VRRPAuthPass       string
}

func (s *GenerateKeepalivedConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())

	// This is a placeholder for the actual logic to get the config from the cluster spec.
	// A real implementation would populate KeepalivedTemplateData from cluster.Spec.HA.LoadBalancer...
	data := KeepalivedTemplateData{
		VRRPInterface:      "eth0",
		VRRPVirtualRouterID: 51,
		VRRPPriority:       100, // This should be different for MASTER and BACKUP nodes
		VRRPVirtualIP:      "192.168.1.200",
		VRRPAuthPass:       "kubexm_vrrp",
	}

	templateContent, err := templates.Get("loadbalancer/keepalived/keepalived.conf.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get keepalived config template: %w", err)
	}

	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return fmt.Errorf("failed to render keepalived config template: %w", err)
	}

	ctx.Set("keepalived.conf", renderedConfig)
	logger.Info("keepalived config generated successfully.")

	return nil
}

func (s *GenerateKeepalivedConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	ctx.Delete("keepalived.conf")
	return nil
}

func (s *GenerateKeepalivedConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
