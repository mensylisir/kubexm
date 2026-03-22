package kubexm

import (
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubecertsstep "github.com/mensylisir/kubexm/internal/step/kubernetes/certs"
	kubeproxycertsstep "github.com/mensylisir/kubexm/internal/step/kubernetes/kube-proxy"
	kubeletcertsstep "github.com/mensylisir/kubexm/internal/step/kubernetes/kubelet"
	"github.com/mensylisir/kubexm/internal/task"
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

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	executionHost := []connector.Host{controlNode}

	generateKubeCA, err := kubecertsstep.NewGenerateKubeCAStepBuilder(runtimeCtx, "GenerateKubeCAs").Build()
	if err != nil {
		return nil, err
	}
	generateKubeCerts, err := kubecertsstep.NewGenerateKubeCertsStepBuilder(runtimeCtx, "GenerateKubeComponentCerts").Build()
	if err != nil {
		return nil, err
	}
	generateKubeletCerts, err := kubeletcertsstep.NewGenerateKubeletCertsForAllNodesStepBuilder(runtimeCtx, "GenerateKubeletCerts").Build()
	if err != nil {
		return nil, err
	}
	generateKubeProxyCerts, err := kubeproxycertsstep.NewGenerateKubeProxyCertsStepBuilder(runtimeCtx, "GenerateKubeProxyCerts").Build()
	if err != nil {
		return nil, err
	}

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
