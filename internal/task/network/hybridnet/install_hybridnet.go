package hybridnet

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/connector"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	hybridnetstep "github.com/mensylisir/kubexm/internal/step/network/hybridnet"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployHybridnetTask struct {
	task.Base
}

func NewDeployHybridnetTask() task.Task {
	return &DeployHybridnetTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployHybridnet",
				Description: "Deploy Hybridnet CNI network addon using Helm",
			},
		},
	}
}

func (t *DeployHybridnetTask) Name() string {
	return t.Meta.Name
}

func (t *DeployHybridnetTask) Description() string {
	return t.Meta.Description
}

func (t *DeployHybridnetTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	return ctx.GetClusterConfig().Spec.Network.Plugin == string(common.CNITypeHybridnet), nil
}

func (t *DeployHybridnetTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())

	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy hybridnet")
	}
	executionHost := masterHosts[0]
	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	generateValues, err := hybridnetstep.NewGenerateHybridnetValuesStepBuilder(runtimeCtx, "GenerateHybridnetValues").Build()
	if err != nil {
		return nil, err
	}
	distributeArtifacts, err := hybridnetstep.NewDistributeHybridnetArtifactsStepBuilder(runtimeCtx, "DistributeHybridnetArtifacts").Build()
	if err != nil {
		return nil, err
	}
	installChart, err := hybridnetstep.NewInstallHybridnetHelmChartStepBuilder(runtimeCtx, "InstallHybridnetChart").Build()
	if err != nil {
		return nil, err
	}

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateHybridnetValues", Step: generateValues, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeHybridnetArtifacts", Step: distributeArtifacts, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallHybridnetChart", Step: installChart, Hosts: []connector.Host{executionHost}})

	fragment.AddDependency("GenerateHybridnetValues", "DistributeHybridnetArtifacts")
	fragment.AddDependency("DistributeHybridnetArtifacts", "InstallHybridnetChart")

	_ = controlNode
	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
