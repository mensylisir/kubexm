package controllermanager

import (
	"fmt"
	"path/filepath"

	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/templates"
)

type GenerateControllerManagerConfigStep struct {
	step.Base
}

func NewGenerateControllerManagerConfigStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *GenerateControllerManagerConfigStep] {
	s := &GenerateControllerManagerConfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate kube-controller-manager config file"
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

type ControllerManagerTemplateData struct {
	KubeconfigPath         string
	ClusterCIDR            string
	ClusterName            string
	ClusterSigningCertFile string
	ClusterSigningKeyFile  string
}

func (s *GenerateControllerManagerConfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())

	pkiPath := common.DefaultKubernetesPKIDir

	// Placeholder data
	data := ControllerManagerTemplateData{
		KubeconfigPath:         filepath.Join(common.KubernetesConfigDir, "controller-manager.kubeconfig"),
		ClusterCIDR:            "10.244.0.0/16",
		ClusterName:            "kubernetes",
		ClusterSigningCertFile: filepath.Join(pkiPath, "ca.pem"),
		ClusterSigningKeyFile:  filepath.Join(pkiPath, "ca-key.pem"),
	}

	templateContent, err := templates.Get("kubernetes/kube-controller-manager/config.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get controller-manager config template: %w", err)
	}

	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return fmt.Errorf("failed to render controller-manager config template: %w", err)
	}

	ctx.Set("kube-controller-manager.config.yaml", renderedConfig)
	logger.Info("kube-controller-manager.config.yaml generated successfully.")

	return nil
}

func (s *GenerateControllerManagerConfigStep) Rollback(ctx runtime.ExecutionContext) error {
	ctx.Delete("kube-controller-manager.config.yaml")
	return nil
}

func (s *GenerateControllerManagerConfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
