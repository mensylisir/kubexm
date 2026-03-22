package kubexm

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubeproxycertsstep "github.com/mensylisir/kubexm/internal/step/kubernetes/kube-proxy"
	kubeletcertsstep "github.com/mensylisir/kubexm/internal/step/kubernetes/kubelet"
	"github.com/mensylisir/kubexm/internal/task"
)

type GenerateNodePKITask struct {
	task.Base
}

func NewGenerateNodePKITask() task.Task {
	return &GenerateNodePKITask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateNodePKI",
				Description: "Generate client certificates for node-level components (kubelet, kube-proxy)",
			},
		},
	}
}

func (t *GenerateNodePKITask) Name() string {
	return t.Meta.Name
}

func (t *GenerateNodePKITask) Description() string {
	return t.Meta.Description
}

func (t *GenerateNodePKITask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *GenerateNodePKITask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node to generate node PKI: %w", err)
	}
	executionHost := []connector.Host{controlNode}

	generateKubeletCerts, err := kubeletcertsstep.NewGenerateKubeletCertsForAllNodesStepBuilder(runtimeCtx, "GenerateAllKubeletClientCerts").Build()
	if err != nil {
		return nil, err
	}
	generateKubeProxyCerts, err := kubeproxycertsstep.NewGenerateKubeProxyCertsStepBuilder(runtimeCtx, "GenerateKubeProxyClientCert").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateAllKubeletClientCerts", Step: generateKubeletCerts, Hosts: executionHost})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKubeProxyClientCert", Step: generateKubeProxyCerts, Hosts: executionHost})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
