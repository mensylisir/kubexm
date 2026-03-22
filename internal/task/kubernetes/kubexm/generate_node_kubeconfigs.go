package kubexm

import (
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubeproxystep "github.com/mensylisir/kubexm/internal/step/kubernetes/kube-proxy"
	kubeletstep "github.com/mensylisir/kubexm/internal/step/kubernetes/kubelet"
	"github.com/mensylisir/kubexm/internal/task"
)

type GenerateNodeKubeconfigsTask struct {
	task.Base
}

func NewGenerateNodeKubeconfigsTask() task.Task {
	return &GenerateNodeKubeconfigsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateNodeKubeconfigs",
				Description: "Generate kubeconfig files for kubelet and kube-proxy on all nodes",
			},
		},
	}
}

func (t *GenerateNodeKubeconfigsTask) Name() string {
	return t.Meta.Name
}

func (t *GenerateNodeKubeconfigsTask) Description() string {
	return t.Meta.Description
}

func (t *GenerateNodeKubeconfigsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *GenerateNodeKubeconfigsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	generateKubeletKubeconfig, err := kubeletstep.NewCreateKubeletKubeconfigStepBuilder(runtimeCtx, "GenerateKubeletKubeconfig").Build()
	if err != nil {
		return nil, err
	}
	generateKubeProxyKubeconfig, err := kubeproxystep.NewCreateKubeProxyKubeconfigStepBuilder(runtimeCtx, "GenerateKubeProxyKubeconfig").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKubeletKubeconfig", Step: generateKubeletKubeconfig, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKubeProxyKubeconfig", Step: generateKubeProxyKubeconfig, Hosts: allHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
