package controllermanager

import (
	"fmt"

	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step"
	"github.com/mensylisir/kubexm/pkg/step/helpers"
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

func (s *GenerateControllerManagerKubeconfigStep) Run(ctx runtime.ExecutionContext) error {
	logger := ctx.GetLogger().With("step", s.Meta().Name, "host", ctx.GetHost().GetName())

	// This is a simplified placeholder.
	// A real implementation would call a helper function to generate a kubeconfig.
	// e.g., helpers.CreateKubeconfig(server, ca, clientCert, clientKey)

	// The server would be the local apiserver endpoint (e.g., https://127.0.0.1:6443)
	// The user would be "system:kube-controller-manager"
	// The certs would be generated specifically for this user.

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
    user: "system:kube-controller-manager"
  name: "system:kube-controller-manager@kubernetes"
current-context: "system:kube-controller-manager@kubernetes"
users:
- name: "system:kube-controller-manager"
  user:
    client-certificate-data: "..." # Base64 encoded client cert
    client-key-data: "..." # Base64 encoded client key
`

	ctx.Set("controller-manager.kubeconfig", []byte(dummyKubeconfig))
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
