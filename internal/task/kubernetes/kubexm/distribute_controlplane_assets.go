package kubexm

import (
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	certsstep "github.com/mensylisir/kubexm/internal/step/kubernetes/certs"
	apiserverstep "github.com/mensylisir/kubexm/internal/step/kubernetes/apiserver"
	controllerstep "github.com/mensylisir/kubexm/internal/step/kubernetes/controller-manager"
	schedulerstep "github.com/mensylisir/kubexm/internal/step/kubernetes/scheduler"
	"github.com/mensylisir/kubexm/internal/task"
)

type DistributeControlPlaneAssetsTask struct {
	task.Base
}

func NewDistributeControlPlaneAssetsTask() task.Task {
	return &DistributeControlPlaneAssetsTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DistributeControlPlaneAssets",
				Description: "Distribute binaries, certificates, and kubeconfigs for control plane components to all master nodes",
			},
		},
	}
}

func (t *DistributeControlPlaneAssetsTask) Name() string {
	return t.Meta.Name
}

func (t *DistributeControlPlaneAssetsTask) Description() string {
	return t.Meta.Description
}

func (t *DistributeControlPlaneAssetsTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return true, nil
}

func (t *DistributeControlPlaneAssetsTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}

	distributeKubeCerts, err := certsstep.NewDistributeKubeCertsStepBuilder(runtimeCtx, "DistributeAllKubeCertsToMasters").Build()
	if err != nil {
		return nil, err
	}
	distributeKubeconfigs, err := certsstep.NewDistributeKubeconfigsStepBuilder(runtimeCtx, "DistributeControlPlaneKubeconfigs").Build()
	if err != nil {
		return nil, err
	}
	installApiServer, err := apiserverstep.NewInstallKubeApiServerStepBuilder(runtimeCtx, "InstallKubeApiServerBinary").Build()
	if err != nil {
		return nil, err
	}
	installControllerManager, err := controllerstep.NewInstallKubeControllerManagerStepBuilder(runtimeCtx, "InstallKubeControllerManagerBinary").Build()
	if err != nil {
		return nil, err
	}
	installScheduler, err := schedulerstep.NewInstallKubeSchedulerStepBuilder(runtimeCtx, "InstallKubeSchedulerBinary").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeAllKubeCertsToMasters", Step: distributeKubeCerts, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeControlPlaneKubeconfigs", Step: distributeKubeconfigs, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeApiServerBinary", Step: installApiServer, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeControllerManagerBinary", Step: installControllerManager, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeSchedulerBinary", Step: installScheduler, Hosts: masterHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
