package kubexm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	kubeproxystep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-proxy"
	kubeletstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kubelet"
	"github.com/mensylisir/kubexm/pkg/task"
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

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	generateKubeletKubeconfig := kubeletstep.NewCreateKubeletKubeconfigStepBuilder(*runtimeCtx, "GenerateKubeletKubeconfig").Build()
	generateKubeProxyKubeconfig := kubeproxystep.NewCreateKubeProxyKubeconfigStepBuilder(*runtimeCtx, "GenerateKubeProxyKubeconfig").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKubeletKubeconfig", Step: generateKubeletKubeconfig, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKubeProxyKubeconfig", Step: generateKubeProxyKubeconfig, Hosts: allHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
