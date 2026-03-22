package kubexm

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	apiserverstep "github.com/mensylisir/kubexm/internal/step/kubernetes/kube-apiserver"
	controllerstep "github.com/mensylisir/kubexm/internal/step/kubernetes/kube-controller-manager"
	schedulerstep "github.com/mensylisir/kubexm/internal/step/kubernetes/kube-scheduler"
	"github.com/mensylisir/kubexm/internal/task"
)

type GenerateControlPlaneConfigsTask struct {
	task.Base
}

func NewGenerateControlPlaneConfigsTask() task.Task {
	return &GenerateControlPlaneConfigsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "GenerateControlPlaneConfigs",
				Description: "Generate configuration files for apiserver, controller-manager, and scheduler on each master node",
			},
		},
	}
}

func (t *GenerateControlPlaneConfigsTask) Name() string {
	return t.Meta.Name
}

func (t *GenerateControlPlaneConfigsTask) Description() string {
	return t.Meta.Description
}

func (t *GenerateControlPlaneConfigsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *GenerateControlPlaneConfigsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}

	configureApiServer, err := apiserverstep.NewConfigureKubeAPIServerStepBuilder(runtimeCtx, "ConfigureKubeAPIServer").Build()
	if err != nil {
		return nil, err
	}
	configureControllerManager, err := controllerstep.NewConfigureKubeControllerManagerStepBuilder(runtimeCtx, "ConfigureKubeControllerManager").Build()
	if err != nil {
		return nil, err
	}
	configureScheduler, err := schedulerstep.NewConfigureKubeSchedulerStepBuilder(runtimeCtx, "ConfigureKubeScheduler").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureKubeAPIServer", Step: configureApiServer, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureKubeControllerManager", Step: configureControllerManager, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureKubeScheduler", Step: configureScheduler, Hosts: masterHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
