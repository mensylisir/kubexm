package scheduler

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
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

func (s *GenerateSchedulerKubeconfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())

	// This is a simplified placeholder.
	// A real implementation would call a helper function to generate a kubeconfig.

	dummyKubeconfig := `
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: "..." # Base64 encoded CA cert
    server: "https://127.0.0.1:6443"
  name: "kubernetes"
contexts:
- context:
    cluster: "kubernetes"
    user: "system:kube-scheduler"
  name: "system:kube-scheduler@kubernetes"
current-context: "system:kube-scheduler@kubernetes"
users:
- name: "system:kube-scheduler"
  user:
    client-certificate-data: "..." # Base64 encoded client cert
    client-key-data: "..." # Base64 encoded client key
`

	ctx.Set("scheduler.kubeconfig", []byte(dummyKubeconfig))
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
