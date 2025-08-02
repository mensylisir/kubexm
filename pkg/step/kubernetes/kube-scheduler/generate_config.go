package scheduler

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateSchedulerConfigStep struct {
	step.Base
}

func NewGenerateSchedulerConfigStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *GenerateSchedulerConfigStep] {
	s := &GenerateSchedulerConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate kube-scheduler config file"
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

type SchedulerTemplateData struct {
	KubeconfigPath string
}

func (s *GenerateSchedulerConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())

	// Placeholder data
	data := SchedulerTemplateData{
		KubeconfigPath: filepath.Join(common.KubernetesConfigDir, "scheduler.kubeconfig"),
	}

	templateContent, err := templates.Get("kubernetes/kube-scheduler/config.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get scheduler config template: %w", err)
	}

	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return fmt.Errorf("failed to render scheduler config template: %w", err)
	}

	ctx.Set("kube-scheduler.config.yaml", renderedConfig)
	logger.Info("kube-scheduler.config.yaml generated successfully.")

	return nil
}

func (s *GenerateSchedulerConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	ctx.Delete("kube-scheduler.config.yaml")
	return nil
}

func (s *GenerateSchedulerConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
