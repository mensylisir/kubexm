package cilium

import (
	"fmt"
	"github.com/mensylisir/kubexm/pkg/common"
	"github.com/mensylisir/kubexm/pkg/connector"
	"github.com/mensylisir/kubexm/pkg/plan"
	"github.com/mensylisir/kubexm/pkg/runtime"
	"github.com/mensylisir/kubexm/pkg/spec"
	"github.com/mensylisir/kubexm/pkg/step/network/cilium"
	"github.com/mensylisir/kubexm/pkg/task"
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
	return ctx.GetClusterConfig().Spec.Network.Plugin == string(common.CNITypeCilium), nil
}

func (t *DeployCiliumTask) Plan(ctx runtime.TaskContext) (*plan.ExecutionFragment, error) {
	fragment := plan.NewExecutionFragment(t.Name())
	runtimeCtx := ctx.(*runtime.Context).ForTask(t.Name())

	controlNode, err := ctx.GetControlNode()
	if err != nil {
		return nil, err
	}
	masterHosts := ctx.GetHostsByRole(common.RoleMaster)
	if len(masterHosts) == 0 {
		return nil, fmt.Errorf("no master hosts found to deploy cilium")
	}
	executionHost := masterHosts[0]

	downloadStep := cilium.NewDownloadCiliumChartStepBuilder(*runtimeCtx, "DownloadCiliumAssets").Build()
	generateStep := cilium.NewGenerateCiliumValuesStepBuilder(*runtimeCtx, "GenerateCiliumManifest").Build()
	distributeStep := cilium.NewDistributeCiliumArtifactsStepBuilder(*runtimeCtx, "DistributeCiliumAssets").Build()
	installStep := cilium.NewInstallCiliumHelmChartStepBuilder(*runtimeCtx, "InstallCilium").Build()

	nodeGenerate := &plan.ExecutionNode{Name: "GenerateCiliumManifest", Step: generateStep, Hosts: []connector.Host{controlNode}}
	nodeDistribute := &plan.ExecutionNode{Name: "DistributeCiliumAssets", Step: distributeStep, Hosts: []connector.Host{executionHost}}
	nodeInstall := &plan.ExecutionNode{Name: "InstallCilium", Step: installStep, Hosts: []connector.Host{executionHost}}

	fragment.AddNode(nodeGenerate)
	fragment.AddNode(nodeDistribute)
	fragment.AddNode(nodeInstall)

	fragment.AddDependency("GenerateCiliumManifest", "DistributeCiliumAssets")
	fragment.AddDependency("DistributeCiliumAssets", "InstallCilium")

	if !ctx.IsOfflineMode() {
		nodeDownload := &plan.ExecutionNode{
			Name:  "DownloadCiliumAssets",
			Step:  downloadStep,
			Hosts: []connector.Host{controlNode},
		}
		fragment.AddNode(nodeDownload)
		fragment.AddDependency("DownloadCiliumAssets", "GenerateCiliumManifest")
	}

	fragment.CalculateEntryAndExitNodes()
	return fragment, nil
}
