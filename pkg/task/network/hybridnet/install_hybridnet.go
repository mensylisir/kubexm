package hybridnet

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	hybridnetstep "github.com/mensylisir/kubexm/pkg/step/network/hybridnet"
	"github.com/mensylisir/kubexm/pkg/task"
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

	runtimeCtx, ok := ctx.(*runtime.Context)
	if !ok {
		return nil, fmt.Errorf("internal error: TaskContext is not of type *runtime.Context")
	}

	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy hybridnet")
	}
	executionHost := masterHosts[0]
	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}

	generateValues := hybridnetstep.NewGenerateHybridnetValuesStepBuilder(*runtimeCtx, "GenerateHybridnetValues").Build()
	distributeArtifacts := hybridnetstep.NewDistributeHybridnetArtifactsStepBuilder(*runtimeCtx, "DistributeHybridnetArtifacts").Build()
	installChart := hybridnetstep.NewInstallHybridnetHelmChartStepBuilder(*runtimeCtx, "InstallHybridnetChart").Build()

	fragment.AddNode(&plan.ExecutionNode{Name: "GenerateHybridnetValues", Step: generateValues, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "DistributeHybridnetArtifacts", Step: distributeArtifacts, Hosts: []connector.Host{executionHost}})
	fragment.AddNode(&plan.ExecutionNode{Name: "InstallHybridnetChart", Step: installChart, Hosts: []connector.Host{executionHost}})

	fragment.AddDependency("GenerateHybridnetValues", "DistributeHybridnetArtifacts")
	fragment.AddDependency("DistributeHybridnetArtifacts", "InstallHybridnetChart")

	if !ctx.IsOfflineMode() {
		ctx.GetLogger().Info("Online mode detected. Downloading Hybridnet chart.")
		downloadChart := hybridnetstep.NewDownloadHybridnetChartStepBuilder(*runtimeCtx, "DownloadHybridnetChart").Build()
		fragment.AddNode(&plan.ExecutionNode{Name: "DownloadHybridnetChart", Step: downloadChart, Hosts: []connector.Host{controlNode}})
		fragment.AddDependency("DownloadHybridnetChart", "GenerateHybridnetValues")
	} else {
		ctx.GetLogger().Info("Offline mode detected. Distributing Hybridnet artifacts.")
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
