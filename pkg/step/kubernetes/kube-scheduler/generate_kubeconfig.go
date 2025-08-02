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

type GenerateSchedulerKubeconfigStep struct {
	step.Base
}

func NewGenerateSchedulerKubeconfigStepBuilder(ctx runtime.Context, instanceName string) *step.Builder[step.EmptyStepBuilder, *GenerateSchedulerKubeconfigStep] {
	s := &GenerateSchedulerKubeconfigStep{}
	s.Base.Meta.Name = instanceName
	s.Base.Meta.Description = "Generate kube-scheduler kubeconfig"
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

func (s *GenerateSchedulerKubeconfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())

	pkiPath := common.DefaultKubernetesPKIDir

	data := KubeconfigTemplateData{
		ServerURL:      fmt.Sprintf("https://%s:%d", "127.0.0.1", common.DefaultAPIServerPort),
		CaPath:         filepath.Join(pkiPath, "ca.pem"),
		UserName:       "system:kube-scheduler",
		ClientCertPath: filepath.Join(pkiPath, "kube-scheduler.pem"),
		ClientKeyPath:  filepath.Join(pkiPath, "kube-scheduler-key.pem"),
	}

	templateContent, err := templates.Get("kubernetes/kubeconfig.yaml.tmpl")
	if err != nil {
		return fmt.Errorf("failed to get generic kubeconfig template: %w", err)
	}

	renderedConfig, err := templates.Render(templateContent, data)
	if err != nil {
		return fmt.Errorf("failed to render scheduler kubeconfig: %w", err)
	}

	ctx.Set("scheduler.kubeconfig", renderedConfig)
	logger.Info("scheduler.kubeconfig generated successfully.")

	return nil
}

func (s *GenerateSchedulerKubeconfigStep) Rollback(ctx runtime.ExecutionContext) error {
	ctx.Delete("scheduler.kubeconfig")
	return nil
}

func (s *GenerateSchedulerKubeconfigStep) Meta() *spec.StepMeta {
	return &s.Base.Meta
}
