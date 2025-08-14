package kubexm

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	apiserverstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-apiserver"
	controllerstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-controller-manager"
	schedulerstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-scheduler"
	"github.com/mensylisir/kubexm/pkg/task"
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

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}

	configureApiServer := apiserverstep.NewConfigureKubeAPIServerStepBuilder(*runtimeCtx, "ConfigureKubeAPIServer").Build()
	installApiServerService := apiserverstep.NewInstallKubeAPIServerServiceStepBuilder(*runtimeCtx, "InstallKubeAPIServerService").Build()

	configureControllerManager := controllerstep.NewConfigureKubeControllerManagerStepBuilder(*runtimeCtx, "ConfigureKubeControllerManager").Build()
	installControllerManagerService := controllerstep.NewInstallKubeControllerManagerServiceStepBuilder(*runtimeCtx, "InstallKubeControllerManagerService").Build()

	configureScheduler := schedulerstep.NewConfigureKubeSchedulerStepBuilder(*runtimeCtx, "ConfigureKubeScheduler").Build()
	installSchedulerService := schedulerstep.NewInstallKubeSchedulerServiceStepBuilder(*runtimeCtx, "InstallKubeSchedulerService").Build()

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
