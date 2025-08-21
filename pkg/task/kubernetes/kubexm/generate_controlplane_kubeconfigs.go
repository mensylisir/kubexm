package kubexm

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	controllermanagerstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-controller-manager"
	schedulerstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-scheduler"
	"github.com/mensylisir/kubexm/pkg/task"
)

type GenerateControlPlaneKubeconfigsTask struct {
	task.Base
}

func NewGenerateControlPlaneKubeconfigsTask() task.Task {
	return &GenerateControlPlaneKubeconfigsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateControlPlaneConfigs",
				Description: "Generate kubeconfig files for controller-manager and scheduler on each master node",
			},
		},
	}
}

func (t *GenerateControlPlaneKubeconfigsTask) Name() string        { return t.Meta.Name }
func (t *GenerateControlPlaneKubeconfigsTask) Description() string { return t.Meta.Description }
func (t *GenerateControlPlaneKubeconfigsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *GenerateControlPlaneKubeconfigsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}

	createCMKubeconfig := controllermanagerstep.NewCreateControllerManagerKubeconfigStepBuilder(*runtimeCtx, "CreateControllerManagerKubeconfig").Build()
	createSchedulerKubeconfig := schedulerstep.NewCreateSchedulerKubeconfigStepBuilder(*runtimeCtx, "CreateSchedulerKubeconfig").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "CreateControllerManagerKubeconfig", Step: createCMKubeconfig, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CreateSchedulerKubeconfig", Step: createSchedulerKubeconfig, Hosts: masterHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
