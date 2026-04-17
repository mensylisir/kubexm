package kubexm

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	apiserverstep "github.com/mensylisir/kubexm/internal/step/kubernetes/apiserver"
	controllerstep "github.com/mensylisir/kubexm/internal/step/kubernetes/controller-manager"
	schedulerstep "github.com/mensylisir/kubexm/internal/step/kubernetes/scheduler"
	"github.com/mensylisir/kubexm/internal/task"
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

	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}

	enableApiServer, err := apiserverstep.NewEnableKubeAPIServerStepBuilder(runtimeCtx, "EnableKubeAPIServer").Build()
	if err != nil {
		return nil, err
	}
	restartApiServer, err := apiserverstep.NewRestartKubeApiServerStepBuilder(runtimeCtx, "RestartKubeAPIServer").Build()
	if err != nil {
		return nil, err
	}
	checkApiServerHealth, err := apiserverstep.NewCheckAPIServerHealthStepBuilder(runtimeCtx, "CheckAPIServerHealth").Build()
	if err != nil {
		return nil, err
	}

	enableControllerManager, err := controllerstep.NewEnableKubeControllerManagerStepBuilder(runtimeCtx, "EnableKubeControllerManager").Build()
	if err != nil {
		return nil, err
	}
	startControllerManager, err := controllerstep.NewStartKubeControllerManagerStepBuilder(runtimeCtx, "StartKubeControllerManager").Build()
	if err != nil {
		return nil, err
	}
	checkControllerManagerHealth, err := controllerstep.NewCheckControllerManagerHealthStepBuilder(runtimeCtx, "CheckControllerManagerHealth").Build()
	if err != nil {
		return nil, err
	}

	enableScheduler, err := schedulerstep.NewEnableKubeSchedulerStepBuilder(runtimeCtx, "EnableKubeScheduler").Build()
	if err != nil {
		return nil, err
	}
	startScheduler, err := schedulerstep.NewStartKubeSchedulerStepBuilder(runtimeCtx, "StartKubeScheduler").Build()
	if err != nil {
		return nil, err
	}
	checkSchedulerHealth, err := schedulerstep.NewCheckSchedulerHealthStepBuilder(runtimeCtx, "CheckSchedulerHealth").Build()
	if err != nil {
		return nil, err
	}

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
