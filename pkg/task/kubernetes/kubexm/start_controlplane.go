package kubexm

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	apiserverstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-apiserver"
	controllerstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-controller-manager"
	schedulerstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-scheduler"
	"github.com/mensylisir/kubexm/pkg/task"
)

type StartControlPlaneTask struct {
	task.Base
}

func NewStartControlPlaneTask() task.Task {
	return &StartControlPlaneTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "StartControlPlane",
				Description: "Enable, start, and check health for all control plane components on master nodes",
			},
		},
	}
}

func (t *StartControlPlaneTask) Name() string {
	return t.Meta.Name
}

func (t *StartControlPlaneTask) Description() string {
	return t.Meta.Description
}

func (t *StartControlPlaneTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *StartControlPlaneTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}

	enableApiServer := apiserverstep.NewEnableKubeAPIServerStepBuilder(*runtimeCtx, "EnableKubeAPIServer").Build()
	restartApiServer := apiserverstep.NewRestartKubeApiServerStepBuilder(*runtimeCtx, "RestartKubeAPIServer").Build()
	checkApiServerHealth := apiserverstep.NewCheckAPIServerHealthStepBuilder(*runtimeCtx, "CheckAPIServerHealth").Build()

	enableControllerManager := controllerstep.NewEnableKubeControllerManagerStepBuilder(*runtimeCtx, "EnableKubeControllerManager").Build()
	startControllerManager := controllerstep.NewStartKubeControllerManagerStepBuilder(*runtimeCtx, "StartKubeControllerManager").Build()
	checkControllerManagerHealth := controllerstep.NewCheckControllerManagerHealthStepBuilder(*runtimeCtx, "CheckControllerManagerHealth").Build()

	enableScheduler := schedulerstep.NewEnableKubeSchedulerStepBuilder(*runtimeCtx, "EnableKubeScheduler").Build()
	startScheduler := schedulerstep.NewStartKubeSchedulerStepBuilder(*runtimeCtx, "StartKubeScheduler").Build()
	checkSchedulerHealth := schedulerstep.NewCheckSchedulerHealthStepBuilder(*runtimeCtx, "CheckSchedulerHealth").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "EnableKubeAPIServer", Step: enableApiServer, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "RestartKubeAPIServer", Step: restartApiServer, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CheckAPIServerHealth", Step: checkApiServerHealth, Hosts: masterHosts})

	fragment.AddNode(&plan.ExecutionNode{Name: "EnableKubeControllerManager", Step: enableControllerManager, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "StartKubeControllerManager", Step: startControllerManager, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CheckControllerManagerHealth", Step: checkControllerManagerHealth, Hosts: masterHosts})

	fragment.AddNode(&plan.ExecutionNode{Name: "EnableKubeScheduler", Step: enableScheduler, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "StartKubeScheduler", Step: startScheduler, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "CheckSchedulerHealth", Step: checkSchedulerHealth, Hosts: masterHosts})

	fragment.AddDependency("EnableKubeAPIServer", "RestartKubeAPIServer")
	fragment.AddDependency("RestartKubeAPIServer", "CheckAPIServerHealth")

	fragment.AddDependency("EnableKubeControllerManager", "StartKubeControllerManager")
	fragment.AddDependency("StartKubeControllerManager", "CheckControllerManagerHealth")

	fragment.AddDependency("EnableKubeScheduler", "StartKubeScheduler")
	fragment.AddDependency("StartKubeScheduler", "CheckSchedulerHealth")

	fragment.AddDependency("CheckAPIServerHealth", "EnableKubeControllerManager")
	fragment.AddDependency("CheckAPIServerHealth", "EnableKubeScheduler")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
