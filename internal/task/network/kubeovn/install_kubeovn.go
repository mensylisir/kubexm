package kubeovn

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	kubeovnstep "github.com/mensylisir/kubexm/internal/step/network/kubeovn"
	"github.com/mensylisir/kubexm/internal/task"
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
	netSpec := ctx.GetClusterConfig().Spec.Network
	if netSpec == nil {
		return false, nil
	}
	return netSpec.Plugin == string(common.CNITypeKubeOvn), nil
}

func (t *DeployKubeovnTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy kube-ovn")
	}
	executionHost := masterHosts[0]

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	generateValues, err := kubeovnstep.NewGenerateKubeovnValuesStepBuilder(runtimeCtx, "GenerateKubeovnValues").Build()
	if err != nil {
		return nil, err
	}
	distributeArtifacts, err := kubeovnstep.NewDistributeKubeovnArtifactsStepBuilder(runtimeCtx, "DistributeKubeovnArtifacts").Build()
	if err != nil {
		return nil, err
	}
	installChart, err := kubeovnstep.NewInstallKubeOvnHelmChartStepBuilder(runtimeCtx, "InstallKubeovnChart").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateKubeovnValues", Step: generateValues, Hosts: []remotefw.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeKubeovnArtifacts", Step: distributeArtifacts, Hosts: []remotefw.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallKubeovnChart", Step: installChart, Hosts: []remotefw.Host{executionHost}})

	fragment.AddDependency("GenerateKubeovnValues", "DistributeKubeovnArtifacts")
	fragment.AddDependency("DistributeKubeovnArtifacts", "InstallKubeovnChart")

	_ = controlNode
	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
