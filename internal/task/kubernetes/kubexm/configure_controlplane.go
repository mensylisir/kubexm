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

type ConfigureControlPlaneTask struct {
	task.Base
}

func NewConfigureControlPlaneTask() task.Task {
	return &ConfigureControlPlaneTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "ConfigureControlPlane",
				Description: "Generate configuration and systemd service files for control plane components on all master nodes",
			},
		},
	}
}

func (t *ConfigureControlPlaneTask) Name() string {
	return t.Meta.Name
}

func (t *ConfigureControlPlaneTask) Description() string {
	return t.Meta.Description
}

func (t *ConfigureControlPlaneTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *ConfigureControlPlaneTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}

	configureApiServer, err := apiserverstep.NewConfigureKubeAPIServerStepBuilder(runtimeCtx, "ConfigureKubeAPIServer").Build()
	if err != nil {
		return nil, err
	}
	installApiServerService, err := apiserverstep.NewInstallKubeAPIServerServiceStepBuilder(runtimeCtx, "InstallKubeAPIServerService").Build()
	if err != nil {
		return nil, err
	}

	configureControllerManager, err := controllerstep.NewConfigureKubeControllerManagerStepBuilder(runtimeCtx, "ConfigureKubeControllerManager").Build()
	if err != nil {
		return nil, err
	}
	installControllerManagerService, err := controllerstep.NewInstallKubeControllerManagerServiceStepBuilder(runtimeCtx, "InstallKubeControllerManagerService").Build()
	if err != nil {
		return nil, err
	}

	configureScheduler, err := schedulerstep.NewConfigureKubeSchedulerStepBuilder(runtimeCtx, "ConfigureKubeScheduler").Build()
	if err != nil {
		return nil, err
	}
	installSchedulerService, err := schedulerstep.NewInstallKubeSchedulerServiceStepBuilder(runtimeCtx, "InstallKubeSchedulerService").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureKubeAPIServer", Step: configureApiServer, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeAPIServerService", Step: installApiServerService, Hosts: masterHosts})

	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureKubeControllerManager", Step: configureControllerManager, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeControllerManagerService", Step: installControllerManagerService, Hosts: masterHosts})

	fragment.AddNode(&plan.ExecutionNode{Name: "ConfigureKubeScheduler", Step: configureScheduler, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeSchedulerService", Step: installSchedulerService, Hosts: masterHosts})

	fragment.AddDependency("ConfigureKubeAPIServer", "InstallKubeAPIServerService")
	fragment.AddDependency("ConfigureKubeControllerManager", "InstallKubeControllerManagerService")
	fragment.AddDependency("ConfigureKubeScheduler", "InstallKubeSchedulerService")

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
