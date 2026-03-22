package kubexm

import (
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubeproxystep "github.com/mensylisir/kubexm/internal/step/kubernetes/kube-proxy"
	kubeletstep "github.com/mensylisir/kubexm/internal/step/kubernetes/kubelet"
	"github.com/mensylisir/kubexm/internal/task"
)

type GenerateNodeComponentConfigsTask struct {
	task.Base
}

func NewGenerateNodeComponentConfigsTask() task.Task {
	return &GenerateNodeComponentConfigsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateNodeComponentConfigs",
				Description: "Generate config.yaml files for kubelet and kube-proxy on all nodes",
			},
		},
	}
}

func (t *GenerateNodeComponentConfigsTask) Name() string        { return t.Meta.Name }
func (t *GenerateNodeComponentConfigsTask) Description() string { return t.Meta.Description }
func (t *GenerateNodeComponentConfigsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *GenerateNodeComponentConfigsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	createKubeletConfig, err := kubeletstep.NewCreateKubeletConfigYAMLStepBuilder(runtimeCtx, "CreateKubeletConfigYAML").Build()
	if err != nil {
		return nil, err
	}
	createKubeProxyConfig, err := kubeproxystep.NewCreateKubeProxyConfigYAMLStepBuilder(runtimeCtx, "CreateKubeProxyConfigYAML").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "CreateKubeletConfigYAML", Step: createKubeletConfig, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CreateKubeProxyConfigYAML", Step: createKubeProxyConfig, Hosts: allHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
