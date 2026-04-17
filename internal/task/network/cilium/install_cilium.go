package cilium

import (
	"fmt"
	"github.com/mensylisir/kubexm/internal/common"
	"github.com/mensylisir/kubexm/internal/remotefw"
	"github.com/mensylisir/kubexm/internal/plan"
	"github.com/mensylisir/kubexm/internal/runtime"
	"github.com/mensylisir/kubexm/internal/spec"
	"github.com/mensylisir/kubexm/internal/step/network/cilium"
	"github.com/mensylisir/kubexm/internal/task"
)

type DeployCiliumTask struct {
	task.Base
}

func NewDeployCiliumTask() task.Task {
	return &DeployCiliumTask{
		Base: task.Base{
			Meta: spec.TaskMeta{
				Name:        "DeployCilium",
				Description: "Deploy Cilium CNI network addon with eBPF",
			},
		},
	}
}

func (t *DeployCiliumTask) Name() string {
	return t.Meta.Name
}

func (t *DeployCiliumTask) Description() string {
	return t.Meta.Description
}

func (t *DeployCiliumTask) IsRequired(ctx runtime.TaskContext) (bool, error) {
	if ctx.GetClusterConfig().Spec.Network == nil {
		return false, nil
	}
	netSpec := ctx.GetClusterConfig().Spec.Network
	if netSpec == nil {
		return false, nil
	}
	return netSpec.Plugin == string(common.CNITypeCilium), nil
}

func (t *DeployCiliumTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy cilium")
	}
	executionHost := masterHosts[0]

	generateStep, err := cilium.NewGenerateCiliumValuesStepBuilder(runtimeCtx, "GenerateCiliumManifest").Build()
	if err != nil {
		return nil, err
	}
	distributeStep, err := cilium.NewDistributeCiliumArtifactsStepBuilder(runtimeCtx, "DistributeCiliumAssets").Build()
	if err != nil {
		return nil, err
	}
	installStep, err := cilium.NewInstallCiliumHelmChartStepBuilder(runtimeCtx, "InstallCilium").Build()
	if err != nil {
		return nil, err
	}

	nodeGenerate := &plan.ExecutionNode{Name: "GenerateCiliumManifest", Step: generateStep, Hosts: []remotefw.Host{controlNode}}
	nodeDistribute := &plan.ExecutionNode{Name: "DistributeCiliumAssets", Step: distributeStep, Hosts: []remotefw.Host{executionHost}}
	nodeInstall := &plan.ExecutionNode{Name: "InstallCilium", Step: installStep, Hosts: []remotefw.Host{executionHost}}

	fragment.AddNode(nodeGenerate)
	fragment.AddNode(nodeDistribute)
	fragment.AddNode(nodeInstall)

	fragment.AddDependency("GenerateCiliumManifest", "DistributeCiliumAssets")
	fragment.AddDependency("DistributeCiliumAssets", "InstallCilium")

	// Downloads are handled centrally in Preflight PrepareAssets/ExtractBundle.

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
