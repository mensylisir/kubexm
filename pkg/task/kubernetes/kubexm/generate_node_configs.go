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

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	allHosts := ctx.GetHostsByRole("")
	if len(allHosts) == 0 {
		return fragment, nil
	}

	createKubeletConfig := kubeletstep.NewCreateKubeletConfigYAMLStepBuilder(*runtimeCtx, "CreateKubeletConfigYAML").Build()
	createKubeProxyConfig := kubeproxystep.NewCreateKubeProxyConfigYAMLStepBuilder(*runtimeCtx, "CreateKubeProxyConfigYAML").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "CreateKubeletConfigYAML", Step: createKubeletConfig, Hosts: allHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CreateKubeProxyConfigYAML", Step: createKubeProxyConfig, Hosts: allHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
