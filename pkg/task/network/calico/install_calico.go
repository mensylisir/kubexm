package calico

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/network/calico"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DeployCalicoTask struct {
	task.Base
}

func NewDeployCalicoTask() task.Task {
	return &DeployCalicoTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployCalico",
				Description: "Deploy Calico CNI network addon",
			},
		},
	}
}

func (t *DeployCalicoTask) Name() string {
	return t.Meta.Name
}

func (t *DeployCalicoTask) Description() string {
	return t.Meta.Description
}

func (t *DeployCalicoTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return ctx.GetClusterConfig().Spec.Network.Plugin == string(common.CNITypeCalico), nil
}

func (t *DeployCalicoTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy calico")
	}
	executionHost := masterHosts[0]

	generateConfig := calico.NewGenerateCalicoValuesStepBuilder(*runtimeCtx, "GenerateCalicoValues").Build()
	distributeCalico := calico.NewDistributeCalicoArtifactsStepBuilder(*runtimeCtx, "DistributeCalico").Build()
	installCalico := calico.NewInstallCalicoHelmChartStepBuilder(*runtimeCtx, "InstallCalico").Build()
	installCalicoctl := calico.NewInstallCalicoctlStepBuilder(*runtimeCtx, "InstallCalicoctl").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateCalicoValues", Step: generateConfig, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeCalico", Step: distributeCalico, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCalico", Step: installCalico, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallCalicoctl", Step: installCalicoctl, Hosts: masterHosts})

	fragment.AddDependency("GenerateCalicoValues", "DistributeCalico")
	fragment.AddDependency("DistributeCalico", "InstallCalico")

	isOffline := ctx.IsOfflineMode()
	if !isOffline {
		ctx.GetLogger().Info("Online mode detected. Adding download step for network plugins.")
		downloadCalico := calico.NewDownloadCalicoChartStepBuilder(*runtimeCtx, "DownloadCalicoManifests").Build()
		downloadCalicoctl := calico.NewDownloadCalicoctlStepBuilder(*runtimeCtx, "DownloadCalicoctl").Build()

		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadCalicoManifests", Step: downloadCalico, Hosts: []connector.Host{controlNode}})
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadCalicoctl", Step: downloadCalicoctl, Hosts: []connector.Host{controlNode}})

		fragment.AddDependency("DownloadCalicoctl", "InstallCalicoctl")
		fragment.AddDependency("DownloadCalicoManifests", "GenerateCalicoValues")
	} else {
		ctx.GetLogger().Info("Offline mode detected. Skipping download step for CNI plugins.")
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
