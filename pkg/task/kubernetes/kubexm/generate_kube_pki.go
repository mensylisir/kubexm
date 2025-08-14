package kubexm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	kubecertsstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/certs"
	kubeproxycertsstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-proxy"
	kubeletcertsstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kubelet"
	"github.com/mensylisir/kubexm/pkg/task"
)

type GenerateKubePKITask struct {
	task.Base
}

func NewGenerateKubePKITask() task.Task {
	return &GenerateKubePKITask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateKubePKI",
				Description: "Generate Kubernetes CAs and all component certificates",
			},
		},
	}
}

func (t *GenerateKubePKITask) Name() string                                     { return t.Meta.Name }
func (t *GenerateKubePKITask) Description() string                              { return t.Meta.Description }
func (t *GenerateKubePKITask) IsRequired(ctx runtime.TaskContext) (bool, error) { return true, nil }

func (t *GenerateKubePKITask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	executionHost := []connector.Host{controlNode}

	generateKubeCA := kubecertsstep.NewGenerateKubeCAStepBuilder(*runtimeCtx, "GenerateKubeCAs").Build()
	generateKubeCerts := kubecertsstep.NewGenerateKubeCertsStepBuilder(*runtimeCtx, "GenerateKubeComponentCerts").Build()
	generateKubeletCerts := kubeletcertsstep.NewGenerateKubeletCertsForAllNodesStepBuilder(*runtimeCtx, "GenerateKubeletCerts").Build()
	generateKubeProxyCerts := kubeproxycertsstep.NewGenerateKubeProxyCertsStepBuilder(*runtimeCtx, "GenerateKubeProxyCerts").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKubeCAs", Step: generateKubeCA, Hosts: executionHost})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKubeComponentCerts", Step: generateKubeCerts, Hosts: executionHost})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKubeletCerts", Step: generateKubeletCerts, Hosts: executionHost})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKubeProxyCerts", Step: generateKubeProxyCerts, Hosts: executionHost})

	fragment.AddDependency("GenerateKubeCAs", "GenerateKubeComponentCerts")
	fragment.AddDependency("GenerateKubeCAs", "GenerateKubeletCerts")
	fragment.AddDependency("GenerateKubeCAs", "GenerateKubeProxyCerts")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
