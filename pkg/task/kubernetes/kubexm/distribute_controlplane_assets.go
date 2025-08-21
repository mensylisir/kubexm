package kubexm

import (
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	certsstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/certs"
	apiserverstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-apiserver"
	controllerstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-controller-manager"
	schedulerstep "github.com/mensylisir/kubexm/pkg/step/kubernetes/kube-scheduler"
	"github.com/mensylisir/kubexm/pkg/task"
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

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return fragment, nil
	}

	distributeKubeCerts := certsstep.NewDistributeKubeCertsStepBuilder(*runtimeCtx, "DistributeAllKubeCertsToMasters").Build()
	distributeKubeconfigs := certsstep.NewDistributeKubeconfigsStepBuilder(*runtimeCtx, "DistributeControlPlaneKubeconfigs").Build()
	installApiServer := apiserverstep.NewInstallKubeApiServerStepBuilder(*runtimeCtx, "InstallKubeApiServerBinary").Build()
	installControllerManager := controllerstep.NewInstallKubeControllerManagerStepBuilder(*runtimeCtx, "InstallKubeControllerManagerBinary").Build()
	installScheduler := schedulerstep.NewInstallKubeSchedulerStepBuilder(*runtimeCtx, "InstallKubeSchedulerBinary").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeAllKubeCertsToMasters", Step: distributeKubeCerts, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeControlPlaneKubeconfigs", Step: distributeKubeconfigs, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeApiServerBinary", Step: installApiServer, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeControllerManagerBinary", Step: installControllerManager, Hosts: masterHosts})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeSchedulerBinary", Step: installScheduler, Hosts: masterHosts})

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
