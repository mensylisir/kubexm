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

type GenerateControllerManagerKubeconfigStep struct {
	step.Base
}

func NewGenerateControllerManagerKubeconfigStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *GenerateControllerManagerKubeconfigStep] {
	s := &GenerateControllerManagerKubeconfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate kube-controller-manager kubeconfig"
	b := new(step.EmptyStepBuilder).Init(s)
	return b
}

type KubeconfigTemplateData struct {
	ServerURL      string
	CaPath         string
	UserName       string
	ClientCertPath string
	ClientKeyPath  string
}

func (s *GenerateControllerManagerKubeconfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())

	pkiPath := common.DefaultKubernetesPKIDir

	data := KubeconfigTemplateData{
		ServerURL:      fmt.Sprintf("https://%s:%d", "127.0.0.1", common.DefaultAPIServerPort),
		CaPath:         filepath.Join(pkiPath, "ca.pem"),
		UserName:       "system:kube-controller-manager",
		ClientCertPath: filepath.Join(pkiPath, "kube-controller-manager.pem"),
		ClientKeyPath:  filepath.Join(pkiPath, "kube-controller-manager-key.pem"),
	}

	templateContent, err := templates.Get("kubernetes/kubeconfig.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get generic kubeconfig template: %w", err)
	}

	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return fmt.Errorf("failed to render controller-manager kubeconfig: %w", err)
	}

	ctx.Set("controller-manager.kubeconfig", renderedConfig)
	logger.Info("controller-manager.kubeconfig generated successfully.")

	return nil
}

func (s *GenerateControllerManagerKubeconfigStep) Rollback(ctx runtime.ExecutionContext) error {
	ctx.Delete("controller-manager.kubeconfig")
	return nil
}

func (s *GenerateControllerManagerKubeconfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
