package kubexm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	kubeproxycertsstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-proxy"
	kubeletcertsstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kubelet"
	"github.com/mensylisir/kubexm/pkg/task"
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

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, fmt.Errorf("failed to get control node to generate node PKI: %w", err)
	}
	executionHost := []connector.Host{controlNode}

	generateKubeletCerts := kubeletcertsstep.NewGenerateKubeletCertsForAllNodesStepBuilder(*runtimeCtx, "GenerateAllKubeletClientCerts").Build()
	generateKubeProxyCerts := kubeproxycertsstep.NewGenerateKubeProxyCertsStepBuilder(*runtimeCtx, "GenerateKubeProxyClientCert").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateAllKubeletClientCerts", Step: generateKubeletCerts, Hosts: executionHost})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKubeProxyClientCert", Step: generateKubeProxyCerts, Hosts: executionHost})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
