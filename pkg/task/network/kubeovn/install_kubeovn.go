package kubeovn

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	kubeovnstep "github.com/mensylisir/kubexm/pkg/step/network/kubeovn"
	"github.com/mensylisir/kubexm/pkg/task"
)

type DeployKubeovnTask struct {
	task.Base
}

func NewDeployKubeovnTask() task.Task {
	return &DeployKubeovnTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployKubeovn",
				Description: "Deploy Kube-OVN CNI network addon using Helm",
			},
		},
	}
}

func (t *DeployKubeovnTask) Name() string {
	return t.Meta.Name
}

func (t *DeployKubeovnTask) Description() string {
	return t.Meta.Description
}

func (t *DeployKubeovnTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return ctx.GetClusterConfig().Spec.Network.Plugin == string(common.CNITypeKubeOvn), nil
}

func (t *DeployKubeovnTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy kube-ovn")
	}
	executionHost := masterHosts[0]

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	generateValues := kubeovnstep.NewGenerateKubeovnValuesStepBuilder(*runtimeCtx, "GenerateKubeovnValues").Build()
	distributeArtifacts := kubeovnstep.NewDistributeKubeovnArtifactsStepBuilder(*runtimeCtx, "DistributeKubeovnArtifacts").Build()
	installChart := kubeovnstep.NewInstallKubeOvnHelmChartStepBuilder(*runtimeCtx, "InstallKubeovnChart").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKubeovnValues", Step: generateValues, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeKubeovnArtifacts", Step: distributeArtifacts, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeovnChart", Step: installChart, Hosts: []connector.Host{executionHost}})

	fragment.AddDependency("GenerateKubeovnValues", "DistributeKubeovnArtifacts")
	fragment.AddDependency("DistributeKubeovnArtifacts", "InstallKubeovnChart")

	if !ctx.IsOfflineMode() {
		ctx.GetLogger().Info("Online mode detected. Downloading Kube-OVN chart.")
		downloadChart := kubeovnstep.NewDownloadKubeovnChartStepBuilder(*runtimeCtx, "DownloadKubeovnChart").Build()
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadKubeovnChart", Step: downloadChart, Hosts: []connector.Host{controlNode}})
		fragment.AddDependency("DownloadKubeovnChart", "GenerateKubeovnValues")
	} else {
		ctx.GetLogger().Info("Offline mode detected. Skipping download for Kube-OVN artifacts.")
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
