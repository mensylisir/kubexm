package haproxy

import (
	"fmt"
	"time"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateHAProxyConfigStep struct {
	step.Base
}

func NewGenerateHAProxyConfigStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *GenerateHAProxyConfigStep] {
	s := &GenerateHAProxyConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate haproxy config file"
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

type HAProxyTemplateData struct {
	MasterNodes []spec.Host
	BindPort    int
}

func (s *GenerateHAProxyConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())

	data := HAProxyTemplateData{
		MasterNodes: ctx.GetHostsByRole("master"),
		BindPort:    common.DefaultLBPort,
	}

	templateContent, err := templates.Get("loadbalancer/haproxy/haproxy.cfg.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get haproxy config template: %w", err)
	}

	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return fmt.Errorf("failed to render haproxy config template: %w", err)
	}

	ctx.Set("haproxy.cfg", renderedConfig)
	logger.Info("haproxy.cfg generated successfully.")

	return nil
}

func (s *GenerateHAProxyConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	ctx.Delete("haproxy.cfg")
	return nil
}

func (s *GenerateHAProxyConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
